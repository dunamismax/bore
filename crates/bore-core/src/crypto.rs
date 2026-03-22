//! Cryptographic layer for bore.
//!
//! Implements the Noise Protocol XX handshake with PSK binding to the
//! rendezvous code, and a `SecureChannel` for encrypted post-handshake
//! communication.
//!
//! # Protocol
//!
//! Pattern: `Noise_XXpsk0_25519_ChaChaPoly_SHA256`
//!
//! The rendezvous code (e.g., `7-apple-beach-crown`) is derived via
//! HKDF-SHA256 into a 32-byte PSK that is mixed into the handshake at
//! position 0. This means:
//!
//! - An attacker who doesn't know the code cannot complete the handshake.
//! - Both peers authenticate mutually via static key exchange.
//! - The PSK adds code-derived entropy to session key derivation.
//!
//! After the 3-message XX handshake completes, `snow`'s `TransportState`
//! handles ChaCha20-Poly1305 AEAD encryption with counter-based nonces.
//!
//! # Framing
//!
//! Snow's transport messages are limited to 65535 bytes. This module adds
//! a simple length-prefix framing layer so callers can send arbitrarily
//! large payloads (automatically chunked into snow-sized segments).
//!
//! # Key material safety
//!
//! All key-holding types implement `Zeroize` or use `snow`'s internal
//! zeroization. The `SecureChannel` clears any buffers it owns on drop.

use hkdf::Hkdf;
use sha2::Sha256;
use snow::{Builder, HandshakeState, TransportState};
use tokio::io::{AsyncRead, AsyncReadExt, AsyncWrite, AsyncWriteExt};
use tracing::{debug, instrument};
use zeroize::{Zeroize, ZeroizeOnDrop};

use crate::error::CryptoError;

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

/// The Noise protocol pattern. XXpsk0 = XX with a pre-shared key mixed at
/// position 0 (before the first handshake message).
const NOISE_PATTERN: &str = "Noise_XXpsk0_25519_ChaChaPoly_SHA256";

/// HKDF salt for deriving the PSK from the rendezvous code.
const PSK_SALT: &[u8] = b"bore-pake-v0";

/// HKDF info for deriving the PSK from the rendezvous code.
const PSK_INFO: &[u8] = b"bore handshake psk";

/// PSK length required by snow (32 bytes).
const PSK_LEN: usize = 32;

/// Maximum snow transport message payload. Snow uses 65535-byte messages
/// including the 16-byte AEAD tag, so max plaintext per segment is 65535 - 16.
const MAX_SNOW_PLAINTEXT: usize = 65535 - 16;

/// Maximum encrypted message size from snow (plaintext + 16-byte tag).
const MAX_SNOW_MESSAGE: usize = 65535;

// ---------------------------------------------------------------------------
// PSK derivation
// ---------------------------------------------------------------------------

/// Derives a 32-byte PSK from a rendezvous code string using HKDF-SHA256.
///
/// The code is low-entropy (~34 bits default). HKDF is a formatting step
/// that produces a fixed-length key suitable for the Noise PSK slot.
/// It does not add entropy.
#[derive(Zeroize, ZeroizeOnDrop)]
struct DerivedPsk([u8; PSK_LEN]);

fn derive_psk(code: &str) -> DerivedPsk {
    let hk = Hkdf::<Sha256>::new(Some(PSK_SALT), code.as_bytes());
    let mut psk = [0u8; PSK_LEN];
    hk.expand(PSK_INFO, &mut psk)
        .expect("HKDF-SHA256 expand to 32 bytes should never fail");
    DerivedPsk(psk)
}

// ---------------------------------------------------------------------------
// Handshake role
// ---------------------------------------------------------------------------

/// Which side of the handshake this peer plays.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum HandshakeRole {
    /// Sends the first handshake message (the sender in bore's model).
    Initiator,
    /// Receives the first handshake message (the receiver in bore's model).
    Responder,
}

// ---------------------------------------------------------------------------
// Handshake
// ---------------------------------------------------------------------------

/// Performs the Noise XXpsk0 handshake over the provided async stream.
///
/// On success, returns a `SecureChannel` that encrypts/decrypts all
/// subsequent communication. On failure, returns a `CryptoError`.
///
/// The `code` parameter is the rendezvous code string (e.g.,
/// `"7-apple-beach-crown"`). Both peers must use the same code.
#[instrument(skip(reader, writer, code), fields(role = ?role))]
pub async fn handshake<R, W>(
    role: HandshakeRole,
    code: &str,
    reader: &mut R,
    writer: &mut W,
) -> std::result::Result<SecureChannel, CryptoError>
where
    R: AsyncRead + Unpin,
    W: AsyncWrite + Unpin,
{
    let psk = derive_psk(code);

    let keypair = Builder::new(NOISE_PATTERN.parse().map_err(|e| {
        CryptoError::HandshakeFailed(format!("failed to parse noise pattern: {e}"))
    })?)
    .generate_keypair()
    .map_err(|e| CryptoError::HandshakeFailed(format!("failed to generate keypair: {e}")))?;

    let mut hs = build_handshake_state(role, &psk, &keypair)?;

    // XX handshake is 3 messages:
    //   → e                     (initiator sends)
    //   ← e, ee, s, es          (responder sends)
    //   → s, se                 (initiator sends)
    match role {
        HandshakeRole::Initiator => {
            // Message 1: initiator → responder
            send_handshake_message(&mut hs, &[], writer).await?;
            // Message 2: responder → initiator
            recv_handshake_message(&mut hs, reader).await?;
            // Message 3: initiator → responder
            send_handshake_message(&mut hs, &[], writer).await?;
        }
        HandshakeRole::Responder => {
            // Message 1: initiator → responder
            recv_handshake_message(&mut hs, reader).await?;
            // Message 2: responder → initiator
            send_handshake_message(&mut hs, &[], writer).await?;
            // Message 3: initiator → responder
            recv_handshake_message(&mut hs, reader).await?;
        }
    }

    if !hs.is_handshake_finished() {
        return Err(CryptoError::HandshakeFailed(
            "handshake did not complete after 3 messages".to_string(),
        ));
    }

    let transport = hs.into_transport_mode().map_err(|e| {
        CryptoError::HandshakeFailed(format!("failed to enter transport mode: {e}"))
    })?;

    debug!("handshake complete, entering transport mode");

    Ok(SecureChannel { transport })
}

fn build_handshake_state(
    role: HandshakeRole,
    psk: &DerivedPsk,
    keypair: &snow::Keypair,
) -> std::result::Result<HandshakeState, CryptoError> {
    let builder = Builder::new(NOISE_PATTERN.parse().map_err(|e| {
        CryptoError::HandshakeFailed(format!("failed to parse noise pattern: {e}"))
    })?)
    .local_private_key(&keypair.private)
    .map_err(|e| CryptoError::HandshakeFailed(format!("failed to set private key: {e}")))?
    .psk(0, &psk.0)
    .map_err(|e| CryptoError::HandshakeFailed(format!("failed to set PSK: {e}")))?;

    match role {
        HandshakeRole::Initiator => builder
            .build_initiator()
            .map_err(|e| CryptoError::HandshakeFailed(format!("failed to build initiator: {e}"))),
        HandshakeRole::Responder => builder
            .build_responder()
            .map_err(|e| CryptoError::HandshakeFailed(format!("failed to build responder: {e}"))),
    }
}

/// Sends a single handshake message with length prefix.
async fn send_handshake_message<W: AsyncWrite + Unpin>(
    hs: &mut HandshakeState,
    payload: &[u8],
    writer: &mut W,
) -> std::result::Result<(), CryptoError> {
    let mut buf = vec![0u8; MAX_SNOW_MESSAGE];
    let len = hs
        .write_message(payload, &mut buf)
        .map_err(|e| CryptoError::HandshakeFailed(format!("write_message failed: {e}")))?;

    // Length-prefix: 4-byte big-endian
    writer
        .write_all(&(len as u32).to_be_bytes())
        .await
        .map_err(|e| CryptoError::HandshakeFailed(format!("write length failed: {e}")))?;
    writer
        .write_all(&buf[..len])
        .await
        .map_err(|e| CryptoError::HandshakeFailed(format!("write payload failed: {e}")))?;
    writer
        .flush()
        .await
        .map_err(|e| CryptoError::HandshakeFailed(format!("flush failed: {e}")))?;

    Ok(())
}

/// Receives a single handshake message with length prefix.
async fn recv_handshake_message<R: AsyncRead + Unpin>(
    hs: &mut HandshakeState,
    reader: &mut R,
) -> std::result::Result<Vec<u8>, CryptoError> {
    let mut len_buf = [0u8; 4];
    reader
        .read_exact(&mut len_buf)
        .await
        .map_err(|e| CryptoError::HandshakeFailed(format!("read length failed: {e}")))?;
    let len = u32::from_be_bytes(len_buf) as usize;

    if len > MAX_SNOW_MESSAGE {
        return Err(CryptoError::HandshakeFailed(format!(
            "handshake message too large: {len} bytes"
        )));
    }

    let mut msg = vec![0u8; len];
    reader
        .read_exact(&mut msg)
        .await
        .map_err(|e| CryptoError::HandshakeFailed(format!("read payload failed: {e}")))?;

    let mut payload = vec![0u8; len];
    let payload_len = hs
        .read_message(&msg, &mut payload)
        .map_err(|e| CryptoError::HandshakeFailed(format!("read_message failed: {e}")))?;

    payload.truncate(payload_len);
    Ok(payload)
}

// ---------------------------------------------------------------------------
// SecureChannel
// ---------------------------------------------------------------------------

/// An encrypted bidirectional channel established after a successful Noise
/// handshake.
///
/// All data sent through this channel is encrypted with ChaCha20-Poly1305
/// using counter-based nonces managed by `snow`'s `TransportState`.
///
/// Snow internally tracks send and receive nonce counters. Each call to
/// `write_message` increments the sending nonce; each call to `read_message`
/// expects the next sequential nonce. Replay and reorder are rejected by
/// snow's AEAD verification.
pub struct SecureChannel {
    transport: TransportState,
}

impl SecureChannel {
    /// Encrypts and sends a message over the provided writer.
    ///
    /// Payloads larger than snow's 65519-byte limit are automatically split
    /// into multiple segments, each independently encrypted. The receiver
    /// reassembles them transparently.
    ///
    /// Wire format per segment:
    /// ```text
    /// [4 bytes: segment length (big-endian u32)]
    /// [encrypted segment (up to 65535 bytes)]
    /// ```
    ///
    /// Multi-segment messages use an additional header:
    /// ```text
    /// [4 bytes: total segment count (big-endian u32)] — sent once before segments
    /// ```
    pub async fn send<W: AsyncWrite + Unpin>(
        &mut self,
        data: &[u8],
        writer: &mut W,
    ) -> std::result::Result<(), CryptoError> {
        let chunks: Vec<&[u8]> = if data.is_empty() {
            vec![&[]]
        } else {
            data.chunks(MAX_SNOW_PLAINTEXT).collect()
        };
        let segment_count = chunks.len() as u32;

        // Send segment count header
        writer
            .write_all(&segment_count.to_be_bytes())
            .await
            .map_err(|e| CryptoError::HandshakeFailed(format!("write segment count: {e}")))?;

        let mut buf = vec![0u8; MAX_SNOW_MESSAGE];
        for chunk in &chunks {
            let len = self
                .transport
                .write_message(chunk, &mut buf)
                .map_err(|e| CryptoError::DecryptionFailed(format!("encrypt failed: {e}")))?;

            writer
                .write_all(&(len as u32).to_be_bytes())
                .await
                .map_err(|e| CryptoError::DecryptionFailed(format!("write segment length: {e}")))?;
            writer
                .write_all(&buf[..len])
                .await
                .map_err(|e| CryptoError::DecryptionFailed(format!("write segment data: {e}")))?;
        }
        writer
            .flush()
            .await
            .map_err(|e| CryptoError::DecryptionFailed(format!("flush: {e}")))?;

        Ok(())
    }

    /// Receives and decrypts a message from the provided reader.
    ///
    /// Handles multi-segment messages transparently, reassembling the full
    /// plaintext.
    pub async fn recv<R: AsyncRead + Unpin>(
        &mut self,
        reader: &mut R,
    ) -> std::result::Result<Vec<u8>, CryptoError> {
        // Read segment count
        let mut count_buf = [0u8; 4];
        reader
            .read_exact(&mut count_buf)
            .await
            .map_err(|e| CryptoError::DecryptionFailed(format!("read segment count: {e}")))?;
        let segment_count = u32::from_be_bytes(count_buf) as usize;

        if segment_count == 0 {
            return Err(CryptoError::DecryptionFailed(
                "segment count is zero".to_string(),
            ));
        }

        // Sanity limit: no single message should have more than 64K segments
        // (~4 GB payload). This prevents memory exhaustion from a malicious peer.
        if segment_count > 65536 {
            return Err(CryptoError::DecryptionFailed(format!(
                "segment count too large: {segment_count}"
            )));
        }

        let mut result = Vec::new();
        let mut buf = vec![0u8; MAX_SNOW_MESSAGE];

        for _ in 0..segment_count {
            let mut len_buf = [0u8; 4];
            reader
                .read_exact(&mut len_buf)
                .await
                .map_err(|e| CryptoError::DecryptionFailed(format!("read segment length: {e}")))?;
            let len = u32::from_be_bytes(len_buf) as usize;

            if len > MAX_SNOW_MESSAGE {
                return Err(CryptoError::DecryptionFailed(format!(
                    "segment too large: {len}"
                )));
            }

            let mut encrypted = vec![0u8; len];
            reader
                .read_exact(&mut encrypted)
                .await
                .map_err(|e| CryptoError::DecryptionFailed(format!("read segment data: {e}")))?;

            let plaintext_len = self
                .transport
                .read_message(&encrypted, &mut buf)
                .map_err(|e| CryptoError::DecryptionFailed(format!("decrypt failed: {e}")))?;

            result.extend_from_slice(&buf[..plaintext_len]);
        }

        Ok(result)
    }

    /// Returns the sending nonce counter (number of messages sent).
    pub fn sending_nonce(&self) -> u64 {
        self.transport.sending_nonce()
    }

    /// Returns the receiving nonce counter (number of messages received).
    pub fn receiving_nonce(&self) -> u64 {
        self.transport.receiving_nonce()
    }

    /// Returns whether this channel was the handshake initiator.
    pub fn is_initiator(&self) -> bool {
        self.transport.is_initiator()
    }

    /// Triggers a rekey of the outgoing cipher state.
    ///
    /// Call this on both sides (coordinated via a protocol message) for
    /// long-running transfers that approach nonce limits.
    pub fn rekey_outgoing(&mut self) {
        self.transport.rekey_outgoing();
    }

    /// Triggers a rekey of the incoming cipher state.
    pub fn rekey_incoming(&mut self) {
        self.transport.rekey_incoming();
    }
}

impl Drop for SecureChannel {
    fn drop(&mut self) {
        // TransportState's internal cipher keys are zeroized by snow on drop.
        // We don't hold any additional key material.
        debug!("SecureChannel dropped, transport state cleaned up");
    }
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

#[cfg(test)]
mod tests {
    use super::*;
    use tokio::io::duplex;

    /// Helper: perform handshake between two in-process peers over duplex streams.
    async fn handshake_pair(
        code: &str,
    ) -> std::result::Result<(SecureChannel, SecureChannel), CryptoError> {
        // Create two duplex streams (each gives a read+write half)
        let (client_stream, server_stream) = duplex(65536);
        let (client_read, client_write) = tokio::io::split(client_stream);
        let (server_read, server_write) = tokio::io::split(server_stream);

        let code_init = code.to_string();
        let code_resp = code.to_string();

        let (init_result, resp_result) = tokio::join!(
            async move {
                let mut r = client_read;
                let mut w = client_write;
                handshake(HandshakeRole::Initiator, &code_init, &mut r, &mut w).await
            },
            async move {
                let mut r = server_read;
                let mut w = server_write;
                handshake(HandshakeRole::Responder, &code_resp, &mut r, &mut w).await
            },
        );

        Ok((init_result?, resp_result?))
    }

    #[tokio::test]
    async fn handshake_succeeds_with_same_code() {
        let (init_ch, resp_ch) = handshake_pair("7-apple-beach-crown").await.unwrap();
        assert!(init_ch.is_initiator());
        assert!(!resp_ch.is_initiator());
        assert_eq!(init_ch.sending_nonce(), 0);
        assert_eq!(resp_ch.sending_nonce(), 0);
    }

    #[tokio::test]
    async fn handshake_fails_with_different_codes() {
        let (client_stream, server_stream) = duplex(65536);
        let (client_read, client_write) = tokio::io::split(client_stream);
        let (server_read, server_write) = tokio::io::split(server_stream);

        let (init_result, resp_result) = tokio::join!(
            async move {
                let mut r = client_read;
                let mut w = client_write;
                handshake(
                    HandshakeRole::Initiator,
                    "7-apple-beach-crown",
                    &mut r,
                    &mut w,
                )
                .await
            },
            async move {
                let mut r = server_read;
                let mut w = server_write;
                handshake(
                    HandshakeRole::Responder,
                    "3-delta-eagle-frost",
                    &mut r,
                    &mut w,
                )
                .await
            },
        );

        // At least one side should fail (the PSK mismatch causes decryption
        // failure during the handshake)
        let either_failed = init_result.is_err() || resp_result.is_err();
        assert!(either_failed, "handshake should fail when codes differ");
    }

    #[tokio::test]
    async fn send_recv_small_message() {
        let (mut init_ch, mut resp_ch) = handshake_pair("test-code").await.unwrap();

        let (client_stream, server_stream) = duplex(65536);
        let (_client_read, client_write) = tokio::io::split(client_stream);
        let (server_read, _server_write) = tokio::io::split(server_stream);

        let payload = b"hello, encrypted world!";

        let (send_result, recv_result) = tokio::join!(
            async move {
                let mut w = client_write;
                init_ch.send(payload, &mut w).await
            },
            async move {
                let mut r = server_read;
                resp_ch.recv(&mut r).await
            },
        );

        send_result.unwrap();
        let received = recv_result.unwrap();
        assert_eq!(received, payload);
    }

    #[tokio::test]
    async fn send_recv_empty_message() {
        let (mut init_ch, mut resp_ch) = handshake_pair("test-code").await.unwrap();

        let (client_stream, server_stream) = duplex(65536);
        let (_client_read, client_write) = tokio::io::split(client_stream);
        let (server_read, _server_write) = tokio::io::split(server_stream);

        let (send_result, recv_result) = tokio::join!(
            async move {
                let mut w = client_write;
                init_ch.send(&[], &mut w).await
            },
            async move {
                let mut r = server_read;
                resp_ch.recv(&mut r).await
            },
        );

        send_result.unwrap();
        let received = recv_result.unwrap();
        assert!(received.is_empty());
    }

    #[tokio::test]
    async fn send_recv_multiple_messages() {
        let (mut init_ch, mut resp_ch) = handshake_pair("multi-msg").await.unwrap();

        let (client_stream, server_stream) = duplex(65536);
        let (_client_read, mut client_write) = tokio::io::split(client_stream);
        let (mut server_read, _server_write) = tokio::io::split(server_stream);

        let messages: Vec<Vec<u8>> = (0..5)
            .map(|i| format!("message number {i}").into_bytes())
            .collect();

        let send_messages = messages.clone();
        let send_handle = tokio::spawn(async move {
            for msg in &send_messages {
                init_ch.send(msg, &mut client_write).await.unwrap();
            }
        });

        let recv_handle = tokio::spawn(async move {
            let mut received = Vec::new();
            for _ in 0..5 {
                let msg = resp_ch.recv(&mut server_read).await.unwrap();
                received.push(msg);
            }
            received
        });

        send_handle.await.unwrap();
        let received = recv_handle.await.unwrap();

        for (i, (sent, got)) in messages.iter().zip(received.iter()).enumerate() {
            assert_eq!(sent, got, "message {i} mismatch");
        }
    }

    #[tokio::test]
    async fn send_recv_large_payload() {
        // 1 MB payload — tests multi-segment framing
        let (mut init_ch, mut resp_ch) = handshake_pair("large-test").await.unwrap();

        let (client_stream, server_stream) = duplex(1024 * 1024);
        let (_client_read, client_write) = tokio::io::split(client_stream);
        let (server_read, _server_write) = tokio::io::split(server_stream);

        let payload: Vec<u8> = (0..1_000_000).map(|i| (i % 256) as u8).collect();
        let expected = payload.clone();

        let (send_result, recv_result) = tokio::join!(
            async move {
                let mut w = client_write;
                init_ch.send(&payload, &mut w).await
            },
            async move {
                let mut r = server_read;
                resp_ch.recv(&mut r).await
            },
        );

        send_result.unwrap();
        let received = recv_result.unwrap();
        assert_eq!(received.len(), expected.len());
        assert_eq!(received, expected);
    }

    #[tokio::test]
    async fn bidirectional_communication() {
        let (mut init_ch, mut resp_ch) = handshake_pair("bidir-test").await.unwrap();

        // One duplex stream — each end gets a read and write half.
        // duplex(a, b): writes to a are readable from b and vice versa.
        let (stream_init, stream_resp) = duplex(65536);
        let (init_read, init_write) = tokio::io::split(stream_init);
        let (resp_read, resp_write) = tokio::io::split(stream_resp);

        // Initiator sends "ping", responder receives it and sends "pong" back.
        let (init_result, _resp_result) = tokio::join!(
            async move {
                let mut w = init_write;
                let mut r = init_read;
                init_ch.send(b"ping", &mut w).await.unwrap();
                init_ch.recv(&mut r).await.unwrap()
            },
            async move {
                let mut r = resp_read;
                let mut w = resp_write;
                let msg = resp_ch.recv(&mut r).await.unwrap();
                assert_eq!(msg, b"ping");
                resp_ch.send(b"pong", &mut w).await.unwrap();
            },
        );

        assert_eq!(init_result, b"pong");
    }

    #[tokio::test]
    async fn nonce_counters_increment() {
        let (mut init_ch, mut resp_ch) = handshake_pair("nonce-test").await.unwrap();

        let (client_stream, server_stream) = duplex(65536);
        let (_client_read, mut client_write) = tokio::io::split(client_stream);
        let (mut server_read, _server_write) = tokio::io::split(server_stream);

        assert_eq!(init_ch.sending_nonce(), 0);

        let send_handle = tokio::spawn(async move {
            for i in 0u64..3 {
                init_ch.send(b"test", &mut client_write).await.unwrap();
                // Each send uses one nonce per segment (small payload = 1 segment)
                assert_eq!(init_ch.sending_nonce(), i + 1);
            }
        });

        let recv_handle = tokio::spawn(async move {
            for i in 0u64..3 {
                resp_ch.recv(&mut server_read).await.unwrap();
                assert_eq!(resp_ch.receiving_nonce(), i + 1);
            }
        });

        send_handle.await.unwrap();
        recv_handle.await.unwrap();
    }

    #[tokio::test]
    async fn psk_derivation_is_deterministic() {
        let psk1 = derive_psk("7-apple-beach-crown");
        let psk2 = derive_psk("7-apple-beach-crown");
        assert_eq!(psk1.0, psk2.0);
    }

    #[tokio::test]
    async fn different_codes_produce_different_psks() {
        let psk1 = derive_psk("7-apple-beach-crown");
        let psk2 = derive_psk("3-delta-eagle-frost");
        assert_ne!(psk1.0, psk2.0);
    }

    #[tokio::test]
    async fn secure_channel_drop_does_not_panic() {
        let (init_ch, resp_ch) = handshake_pair("drop-test").await.unwrap();
        drop(init_ch);
        drop(resp_ch);
        // If we get here without panic, the test passes.
    }

    #[tokio::test]
    async fn rekey_works_without_panic() {
        let (mut init_ch, mut resp_ch) = handshake_pair("rekey-test").await.unwrap();

        let (client_stream, server_stream) = duplex(65536);
        let (_client_read, client_write) = tokio::io::split(client_stream);
        let (server_read, _server_write) = tokio::io::split(server_stream);

        // Rekey both sides (in a real protocol this would be coordinated)
        init_ch.rekey_outgoing();
        resp_ch.rekey_incoming();

        // Communication should still work after rekey
        let (send_result, recv_result) = tokio::join!(
            async move {
                let mut w = client_write;
                init_ch.send(b"after rekey", &mut w).await
            },
            async move {
                let mut r = server_read;
                resp_ch.recv(&mut r).await
            },
        );

        send_result.unwrap();
        let received = recv_result.unwrap();
        assert_eq!(received, b"after rekey");
    }
}
