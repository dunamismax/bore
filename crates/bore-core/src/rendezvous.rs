//! Rendezvous coordination for bore.
//!
//! Maps between human-friendly rendezvous codes and relay connection
//! parameters (relay URL + room ID + PAKE code). Orchestrates the full
//! sender and receiver flows:
//!
//! **Sender flow:**
//! 1. Connect to relay → receive room ID
//! 2. Generate PAKE code (wordlist words)
//! 3. Compose rendezvous code: `relay_url#room_id#channel-word-word-word`
//! 4. Display rendezvous code to user
//! 5. Wait for receiver → perform Noise handshake → transfer
//!
//! **Receiver flow:**
//! 1. Parse rendezvous code → extract relay URL, room ID, PAKE code
//! 2. Connect to relay with room ID
//! 3. Perform Noise handshake → receive transfer
//!
//! # Rendezvous code format
//!
//! The full rendezvous code encodes three pieces of information:
//!
//! ```text
//! <room_id>-<channel>-<word>-<word>-<word>
//! ```
//!
//! The relay URL is not encoded in the code — it defaults to the public
//! relay and can be overridden via CLI flag. This keeps codes short and
//! human-friendly. The room ID prefix is a short alphanumeric string
//! (base64url-encoded, from the relay).
//!
//! For display purposes, the code is shown as a single string that the
//! receiver can copy-paste. For cases where the relay URL is non-default,
//! the sender displays both the relay URL and the code separately.

use rand::Rng;
use tracing::{debug, info};
use url::Url;

use crate::code::{self, RendezvousCode};
use crate::crypto::{self, HandshakeRole};
use crate::engine::{self, ReceiveResult, SendResult};
use crate::error::CodeError;
use crate::transport::{self, Transport, WebSocketTransport};

// ---------------------------------------------------------------------------
// Default relay URL
// ---------------------------------------------------------------------------

/// Default relay server URL.
///
/// This is the public bore relay. Users can override it with `--relay`.
pub const DEFAULT_RELAY_URL: &str = "http://localhost:8080";

// ---------------------------------------------------------------------------
// Rendezvous code with relay context
// ---------------------------------------------------------------------------

/// A full rendezvous code that encodes relay connection parameters.
///
/// Format: `<room_id>-<channel>-<word>-<word>-<word>`
///
/// The room_id is the relay-assigned room identifier. The channel and words
/// form the PAKE code used for the Noise handshake.
#[derive(Debug, Clone)]
pub struct FullRendezvousCode {
    /// The relay-assigned room ID.
    pub room_id: String,
    /// The PAKE code (channel + words) for the Noise handshake.
    pub pake_code: RendezvousCode,
    /// The relay URL (not encoded in the code string).
    pub relay_url: Url,
}

impl FullRendezvousCode {
    /// Format the code as a string for display to the user.
    ///
    /// Format: `<room_id>-<channel>-<word>-<word>-<word>`
    pub fn code_string(&self) -> String {
        format!("{}-{}", self.room_id, self.pake_code)
    }

    /// Parse a full rendezvous code from a string.
    ///
    /// The relay URL is provided separately (from CLI flag or default).
    /// The code string format is: `<room_id>-<channel>-<word>-<word>-<word>`
    ///
    /// The room_id is everything before the first numeric-dash-word sequence.
    /// Since room IDs are base64url (alphanumeric + `-` + `_`) and PAKE codes
    /// start with a numeric channel, we split at the first component that
    /// parses as a channel number (1-999).
    pub fn parse(code_str: &str, relay_url: Url) -> Result<Self, CodeError> {
        let parts: Vec<&str> = code_str.split('-').collect();

        // We need at least: room_id_part(s) + channel + MIN_WORDS words
        // Minimum parts: 1 (room_id) + 1 (channel) + 2 (min words) = 4
        if parts.len() < 4 {
            return Err(CodeError::Malformed(format!(
                "rendezvous code too short: expected at least 4 dash-separated parts, got {}",
                parts.len()
            )));
        }

        // Find the split point: the first part that looks like a channel number.
        // Room IDs from the relay are base64url which can contain digits, but
        // they're 22 chars long. Channel numbers are 1-999. We scan from right
        // to left, looking for the rightmost valid split.
        let mut split_idx = None;
        for i in 0..parts.len() {
            if let Ok(n) = parts[i].parse::<u16>() {
                if (1..=999).contains(&n) {
                    // Check if the remaining parts could form a valid PAKE code
                    let remaining_words = parts.len() - i - 1;
                    if (2..=5).contains(&remaining_words) {
                        split_idx = Some(i);
                        break;
                    }
                }
            }
        }

        let split_idx = split_idx.ok_or_else(|| {
            CodeError::Malformed("could not find channel number in rendezvous code".to_string())
        })?;

        // Everything before split_idx is the room ID
        let room_id = parts[..split_idx].join("-");
        if room_id.is_empty() {
            return Err(CodeError::Malformed(
                "rendezvous code has empty room ID".to_string(),
            ));
        }

        // Everything from split_idx onward is the PAKE code
        let pake_str = parts[split_idx..].join("-");
        let pake_code = RendezvousCode::parse(&pake_str)?;

        Ok(Self {
            room_id,
            pake_code,
            relay_url,
        })
    }
}

impl std::fmt::Display for FullRendezvousCode {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        write!(f, "{}", self.code_string())
    }
}

// ---------------------------------------------------------------------------
// Sender flow
// ---------------------------------------------------------------------------

/// Result of a sender session.
#[derive(Debug)]
pub struct SenderResult {
    /// The full rendezvous code that was displayed to the user.
    pub code: FullRendezvousCode,
    /// The transfer result.
    pub transfer: SendResult,
}

/// Execute the sender flow: connect to relay, generate code, wait for
/// receiver, handshake, and send data.
///
/// Returns the rendezvous code (for display) and the transfer result.
///
/// # Arguments
///
/// * `relay_url` — URL of the relay server.
/// * `filename` — Name of the file to send.
/// * `data` — File contents.
/// * `word_count` — Number of words in the PAKE code (2-5).
pub async fn send(
    relay_url: &Url,
    filename: &str,
    data: &[u8],
    word_count: u8,
) -> Result<SenderResult, crate::error::BoreError> {
    // Step 1: Connect to relay as sender.
    info!("connecting to relay: {}", relay_url);
    let (room_id, ws_transport) = transport::connect_as_sender(relay_url).await?;
    info!("room created: {}", room_id);

    // Step 2: Generate PAKE code.
    let pake_code = generate_pake_code(word_count)?;
    let full_code = FullRendezvousCode {
        room_id,
        pake_code,
        relay_url: relay_url.clone(),
    };

    info!("rendezvous code: {}", full_code.code_string());

    // Step 3: Perform handshake and transfer.
    let pake_str = full_code.pake_code.to_string();
    let transfer = send_over_transport(ws_transport, &pake_str, filename, data).await?;

    Ok(SenderResult {
        code: full_code,
        transfer,
    })
}

/// Execute the sender flow with a callback that receives the code before
/// waiting for the receiver. This allows the CLI to display the code while
/// the sender waits.
pub async fn send_with_code_callback<F>(
    relay_url: &Url,
    filename: &str,
    data: &[u8],
    word_count: u8,
    on_code: F,
) -> Result<SenderResult, crate::error::BoreError>
where
    F: FnOnce(&FullRendezvousCode),
{
    // Step 1: Connect to relay as sender.
    info!("connecting to relay: {}", relay_url);
    let (room_id, ws_transport) = transport::connect_as_sender(relay_url).await?;
    info!("room created: {}", room_id);

    // Step 2: Generate PAKE code.
    let pake_code = generate_pake_code(word_count)?;
    let full_code = FullRendezvousCode {
        room_id,
        pake_code,
        relay_url: relay_url.clone(),
    };

    // Step 3: Notify caller with the code.
    on_code(&full_code);

    // Step 4: Perform handshake and transfer.
    let pake_str = full_code.pake_code.to_string();
    let transfer = send_over_transport(ws_transport, &pake_str, filename, data).await?;

    Ok(SenderResult {
        code: full_code,
        transfer,
    })
}

/// Internal: perform handshake + send over an already-connected transport.
async fn send_over_transport(
    ws_transport: WebSocketTransport,
    pake_code: &str,
    filename: &str,
    data: &[u8],
) -> Result<SendResult, crate::error::BoreError> {
    let (mut reader, mut writer) = ws_transport.split();

    debug!("starting Noise handshake as initiator");
    let mut channel = crypto::handshake(
        HandshakeRole::Initiator,
        pake_code,
        &mut reader,
        &mut writer,
    )
    .await?;

    debug!("handshake complete, starting transfer");
    let result = engine::send_data(&mut channel, &mut writer, filename, data).await?;

    info!(
        "transfer complete: {} ({} bytes, {} chunks)",
        result.filename, result.size, result.chunks_sent
    );

    Ok(result)
}

// ---------------------------------------------------------------------------
// Receiver flow
// ---------------------------------------------------------------------------

/// Result of a receiver session.
#[derive(Debug)]
pub struct ReceiverResult {
    /// The transfer result including the received data.
    pub transfer: ReceiveResult,
}

/// Execute the receiver flow: parse code, connect to relay, handshake,
/// and receive data.
///
/// # Arguments
///
/// * `code_str` — The rendezvous code from the sender.
/// * `relay_url` — URL of the relay server (overrides default if provided).
pub async fn receive(
    code_str: &str,
    relay_url: &Url,
) -> Result<ReceiverResult, crate::error::BoreError> {
    // Step 1: Parse the rendezvous code.
    let full_code = FullRendezvousCode::parse(code_str, relay_url.clone())?;
    info!(
        "parsed code: room={}, pake={}",
        full_code.room_id, full_code.pake_code
    );

    // Step 2: Connect to relay as receiver.
    let ws_transport =
        transport::connect_as_receiver(&full_code.relay_url, &full_code.room_id).await?;

    // Step 3: Perform handshake and receive.
    let pake_str = full_code.pake_code.to_string();
    let (mut reader, mut writer) = ws_transport.split();

    debug!("starting Noise handshake as responder");
    let mut channel = crypto::handshake(
        HandshakeRole::Responder,
        &pake_str,
        &mut reader,
        &mut writer,
    )
    .await?;

    debug!("handshake complete, receiving transfer");
    let result = engine::receive_data(&mut channel, &mut reader).await?;

    info!(
        "transfer complete: {} ({} bytes, {} chunks)",
        result.filename, result.size, result.chunks_received
    );

    Ok(ReceiverResult { transfer: result })
}

// ---------------------------------------------------------------------------
// PAKE code generation
// ---------------------------------------------------------------------------

/// Generate a random PAKE code (channel + words) for use in the Noise handshake.
fn generate_pake_code(word_count: u8) -> Result<RendezvousCode, CodeError> {
    let mut rng = rand::rng();
    let needed = 2 + word_count as usize;
    let random: Vec<u8> = (0..needed).map(|_| rng.random::<u8>()).collect();
    code::code_from_random_bytes(&random, word_count)
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn full_code_format() {
        let pake = RendezvousCode::parse("42-apple-beach-crown").unwrap();
        let full = FullRendezvousCode {
            room_id: "abc123XYZ".to_string(),
            pake_code: pake,
            relay_url: Url::parse("http://localhost:8080").unwrap(),
        };
        assert_eq!(full.code_string(), "abc123XYZ-42-apple-beach-crown");
        assert_eq!(full.to_string(), "abc123XYZ-42-apple-beach-crown");
    }

    #[test]
    fn full_code_parse_simple() {
        let relay = Url::parse("http://localhost:8080").unwrap();
        let code = FullRendezvousCode::parse("abc123-42-apple-beach-crown", relay.clone()).unwrap();
        assert_eq!(code.room_id, "abc123");
        assert_eq!(code.pake_code.channel(), 42);
        assert_eq!(code.pake_code.words(), &["apple", "beach", "crown"]);
        assert_eq!(code.relay_url, relay);
    }

    #[test]
    fn full_code_parse_base64url_room_id() {
        let relay = Url::parse("http://localhost:8080").unwrap();
        // Room IDs from the relay are base64url, which can contain hyphens
        let code = FullRendezvousCode::parse("aB3_xY7z-kLm9pQrS-tUvW-100-delta-storm-noble", relay)
            .unwrap();
        assert_eq!(code.room_id, "aB3_xY7z-kLm9pQrS-tUvW");
        assert_eq!(code.pake_code.channel(), 100);
        assert_eq!(code.pake_code.words(), &["delta", "storm", "noble"]);
    }

    #[test]
    fn full_code_round_trip() {
        let relay = Url::parse("http://localhost:8080").unwrap();
        let pake = RendezvousCode::parse("7-apple-beach-crown").unwrap();
        let original = FullRendezvousCode {
            room_id: "testRoom123".to_string(),
            pake_code: pake,
            relay_url: relay.clone(),
        };

        let code_str = original.code_string();
        let parsed = FullRendezvousCode::parse(&code_str, relay).unwrap();

        assert_eq!(parsed.room_id, original.room_id);
        assert_eq!(parsed.pake_code, original.pake_code);
    }

    #[test]
    fn full_code_parse_two_words() {
        let relay = Url::parse("http://localhost:8080").unwrap();
        let code = FullRendezvousCode::parse("room1-42-apple-beach", relay).unwrap();
        assert_eq!(code.room_id, "room1");
        assert_eq!(code.pake_code.channel(), 42);
        assert_eq!(code.pake_code.word_count(), 2);
    }

    #[test]
    fn full_code_parse_five_words() {
        let relay = Url::parse("http://localhost:8080").unwrap();
        let code =
            FullRendezvousCode::parse("room1-42-apple-beach-crown-delta-ember", relay).unwrap();
        assert_eq!(code.room_id, "room1");
        assert_eq!(code.pake_code.channel(), 42);
        assert_eq!(code.pake_code.word_count(), 5);
    }

    #[test]
    fn full_code_parse_rejects_too_short() {
        let relay = Url::parse("http://localhost:8080").unwrap();
        let result = FullRendezvousCode::parse("abc-42", relay);
        assert!(result.is_err());
    }

    #[test]
    fn full_code_parse_rejects_no_channel() {
        let relay = Url::parse("http://localhost:8080").unwrap();
        let result = FullRendezvousCode::parse("abc-xyz-apple-beach-crown", relay);
        assert!(result.is_err());
    }

    #[test]
    fn generate_pake_code_valid() {
        let code = generate_pake_code(3).unwrap();
        assert_eq!(code.word_count(), 3);
        assert!(code.channel() >= 1);
        assert!(code.channel() <= 999);
    }

    #[test]
    fn generate_pake_code_different_word_counts() {
        for wc in 2..=5 {
            let code = generate_pake_code(wc).unwrap();
            assert_eq!(code.word_count(), wc as usize);
        }
    }

    #[test]
    fn generate_pake_code_rejects_invalid_count() {
        assert!(generate_pake_code(1).is_err());
        assert!(generate_pake_code(6).is_err());
    }
}
