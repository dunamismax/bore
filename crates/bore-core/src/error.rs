//! Error types for bore-core.
//!
//! Every error variant maps to a real failure mode in the transfer lifecycle.
//! These are typed errors suitable for a library crate — not `anyhow::Error`.
//!
//! Note: these types are defined now to establish the error taxonomy. Most variants
//! will be exercised as the transfer engine, crypto layer, and transport are implemented.

use thiserror::Error;

/// Top-level error type for bore-core operations.
#[derive(Debug, Error)]
pub enum BoreError {
    /// Session-related errors (creation, state transitions, expiry).
    #[error("session error: {0}")]
    Session(#[from] SessionError),
    /// Transfer-related errors (manifest, chunks, integrity).
    #[error("transfer error: {0}")]
    Transfer(#[from] TransferError),
    /// Protocol-related errors (framing, versioning, messages).
    #[error("protocol error: {0}")]
    Protocol(#[from] ProtocolError),
    /// Crypto-related errors (handshake, encryption, key material).
    #[error("crypto error: {0}")]
    Crypto(#[from] CryptoError),
    /// Transport-related errors (connection, relay, timeout).
    #[error("transport error: {0}")]
    Transport(#[from] TransportError),
    /// Code-related errors (generation, parsing, validation).
    #[error("code error: {0}")]
    Code(#[from] CodeError),
}

// ---------------------------------------------------------------------------
// Session errors
// ---------------------------------------------------------------------------

#[derive(Debug, Error)]
pub enum SessionError {
    /// Attempted an invalid state transition.
    #[error("invalid transition from {from} to {to}")]
    InvalidTransition {
        from: &'static str,
        to: &'static str,
    },
    /// Session has expired.
    #[error("session expired")]
    Expired,
    /// Session was cancelled by a peer.
    #[error("session cancelled")]
    Cancelled,
    /// Session ID is malformed or unknown.
    #[error("invalid session ID")]
    InvalidSessionId,
}

// ---------------------------------------------------------------------------
// Transfer errors
// ---------------------------------------------------------------------------

#[derive(Debug, Error)]
pub enum TransferError {
    /// File manifest is invalid or inconsistent.
    #[error("invalid manifest: {0}")]
    InvalidManifest(String),
    /// A chunk failed integrity verification.
    #[error("chunk {chunk_index} failed integrity check")]
    ChunkIntegrityFailure { chunk_index: u64 },
    /// Transfer was rejected by the receiver.
    #[error("transfer rejected: {0}")]
    Rejected(String),
    /// Not enough disk space for the transfer.
    #[error("insufficient space: need {required} bytes, have {available}")]
    InsufficientSpace { required: u64, available: u64 },
    /// A file could not be read or written.
    #[error("file IO error: {0}")]
    FileIo(String),
}

// ---------------------------------------------------------------------------
// Protocol errors
// ---------------------------------------------------------------------------

#[derive(Debug, Error)]
pub enum ProtocolError {
    /// Received a message with an unknown or unsupported type tag.
    #[error("unknown message type: 0x{0:02x}")]
    UnknownMessageType(u8),
    /// Protocol version mismatch.
    #[error("version mismatch: ours={ours}, theirs={theirs}")]
    VersionMismatch { ours: u32, theirs: u32 },
    /// Message frame is malformed (wrong length, bad encoding).
    #[error("malformed frame: {0}")]
    MalformedFrame(String),
    /// Unexpected message in the current protocol state.
    #[error("expected {expected}, got {got}")]
    UnexpectedMessage { expected: &'static str, got: String },
    /// Serialization or deserialization error.
    #[error("serialization error: {0}")]
    Serialization(String),
}

// ---------------------------------------------------------------------------
// Crypto errors
// ---------------------------------------------------------------------------

#[derive(Debug, Error)]
pub enum CryptoError {
    /// Handshake failed (wrong code, malformed message, timeout).
    #[error("handshake failed: {0}")]
    HandshakeFailed(String),
    /// Peer authentication failed.
    #[error("peer authentication failed")]
    AuthenticationFailed,
    /// Decryption or encryption failed (bad key, corrupted ciphertext, replay).
    #[error("crypto operation failed: {0}")]
    DecryptionFailed(String),
    /// Key material is missing or invalid.
    #[error("invalid key material")]
    InvalidKeyMaterial,
    /// Nonce limit reached — rekey required before sending more messages.
    #[error("nonce limit reached, rekey required")]
    NonceLimitReached,
}

// ---------------------------------------------------------------------------
// Transport errors
// ---------------------------------------------------------------------------

#[derive(Debug, Error)]
pub enum TransportError {
    /// Connection to peer failed.
    #[error("connection failed: {0}")]
    ConnectionFailed(String),
    /// Connection timed out.
    #[error("connection timed out")]
    Timeout,
    /// Connection was reset by peer.
    #[error("connection reset by peer")]
    ConnectionReset,
    /// Relay is unreachable or returned an error.
    #[error("relay error: {0}")]
    RelayError(String),
    /// DNS resolution failed.
    #[error("DNS resolution failed: {0}")]
    DnsFailure(String),
}

// ---------------------------------------------------------------------------
// Code errors
// ---------------------------------------------------------------------------

#[derive(Debug, Error)]
pub enum CodeError {
    /// The code string is malformed (wrong format, missing parts).
    #[error("malformed code: {0}")]
    Malformed(String),
    /// A word in the code is not in the wordlist.
    #[error("unknown word in code: {0}")]
    UnknownWord(String),
    /// The channel number is out of range.
    #[error("channel number out of range: {0}")]
    InvalidChannel(u16),
}

// ---------------------------------------------------------------------------
// Convenience result type
// ---------------------------------------------------------------------------

/// Convenience alias for results using [`BoreError`].
pub type Result<T> = std::result::Result<T, BoreError>;

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn error_display_is_useful() {
        let err = BoreError::Session(SessionError::InvalidTransition {
            from: "Created",
            to: "Complete",
        });
        let msg = err.to_string();
        assert!(msg.contains("Created"));
        assert!(msg.contains("Complete"));
    }

    #[test]
    fn error_source_chain_works() {
        let err = BoreError::Crypto(CryptoError::AuthenticationFailed);
        let source = std::error::Error::source(&err).unwrap();
        assert!(source.to_string().contains("authentication"));
    }

    #[test]
    fn from_conversions_work() {
        let _: BoreError = SessionError::Expired.into();
        let _: BoreError = TransferError::Rejected("no thanks".into()).into();
        let _: BoreError = ProtocolError::UnknownMessageType(0xff).into();
        let _: BoreError = CryptoError::DecryptionFailed("test".into()).into();
        let _: BoreError = TransportError::Timeout.into();
        let _: BoreError = CodeError::Malformed("bad".into()).into();
    }

    #[test]
    fn thiserror_display_matches_expected_format() {
        assert_eq!(SessionError::Expired.to_string(), "session expired");
        assert_eq!(
            TransferError::ChunkIntegrityFailure { chunk_index: 42 }.to_string(),
            "chunk 42 failed integrity check"
        );
        assert_eq!(
            ProtocolError::UnknownMessageType(0xab).to_string(),
            "unknown message type: 0xab"
        );
        assert_eq!(
            CryptoError::DecryptionFailed("bad ciphertext".into()).to_string(),
            "crypto operation failed: bad ciphertext"
        );
        assert_eq!(TransportError::Timeout.to_string(), "connection timed out");
        assert_eq!(
            CodeError::InvalidChannel(9999).to_string(),
            "channel number out of range: 9999"
        );
    }
}
