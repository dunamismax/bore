//! Error types for bore-core.
//!
//! Every error variant maps to a real failure mode in the transfer lifecycle.
//! These are typed errors suitable for a library crate — not `anyhow::Error`.
//!
//! Note: these types are defined now to establish the error taxonomy. Most variants
//! will be exercised as the transfer engine, crypto layer, and transport are implemented.

use std::fmt;

/// Top-level error type for bore-core operations.
#[derive(Debug)]
pub enum BoreError {
    /// Session-related errors (creation, state transitions, expiry).
    Session(SessionError),
    /// Transfer-related errors (manifest, chunks, integrity).
    Transfer(TransferError),
    /// Protocol-related errors (framing, versioning, messages).
    Protocol(ProtocolError),
    /// Crypto-related errors (handshake, encryption, key material).
    Crypto(CryptoError),
    /// Transport-related errors (connection, relay, timeout).
    Transport(TransportError),
}

impl fmt::Display for BoreError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            Self::Session(e) => write!(f, "session error: {e}"),
            Self::Transfer(e) => write!(f, "transfer error: {e}"),
            Self::Protocol(e) => write!(f, "protocol error: {e}"),
            Self::Crypto(e) => write!(f, "crypto error: {e}"),
            Self::Transport(e) => write!(f, "transport error: {e}"),
        }
    }
}

impl std::error::Error for BoreError {
    fn source(&self) -> Option<&(dyn std::error::Error + 'static)> {
        match self {
            Self::Session(e) => Some(e),
            Self::Transfer(e) => Some(e),
            Self::Protocol(e) => Some(e),
            Self::Crypto(e) => Some(e),
            Self::Transport(e) => Some(e),
        }
    }
}

// ---------------------------------------------------------------------------
// Session errors
// ---------------------------------------------------------------------------

#[derive(Debug)]
pub enum SessionError {
    /// Attempted an invalid state transition.
    InvalidTransition {
        from: &'static str,
        to: &'static str,
    },
    /// Session has expired.
    Expired,
    /// Session was cancelled by a peer.
    Cancelled,
    /// Session ID is malformed or unknown.
    InvalidSessionId,
}

impl fmt::Display for SessionError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            Self::InvalidTransition { from, to } => {
                write!(f, "invalid transition from {from} to {to}")
            }
            Self::Expired => write!(f, "session expired"),
            Self::Cancelled => write!(f, "session cancelled"),
            Self::InvalidSessionId => write!(f, "invalid session ID"),
        }
    }
}

impl std::error::Error for SessionError {}

impl From<SessionError> for BoreError {
    fn from(e: SessionError) -> Self {
        Self::Session(e)
    }
}

// ---------------------------------------------------------------------------
// Transfer errors
// ---------------------------------------------------------------------------

#[derive(Debug)]
pub enum TransferError {
    /// File manifest is invalid or inconsistent.
    InvalidManifest(String),
    /// A chunk failed integrity verification.
    ChunkIntegrityFailure { chunk_index: u64 },
    /// Transfer was rejected by the receiver.
    Rejected(String),
    /// Not enough disk space for the transfer.
    InsufficientSpace { required: u64, available: u64 },
    /// A file could not be read or written.
    FileIo(String),
}

impl fmt::Display for TransferError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            Self::InvalidManifest(msg) => write!(f, "invalid manifest: {msg}"),
            Self::ChunkIntegrityFailure { chunk_index } => {
                write!(f, "chunk {chunk_index} failed integrity check")
            }
            Self::Rejected(reason) => write!(f, "transfer rejected: {reason}"),
            Self::InsufficientSpace {
                required,
                available,
            } => write!(
                f,
                "insufficient space: need {required} bytes, have {available}"
            ),
            Self::FileIo(msg) => write!(f, "file IO error: {msg}"),
        }
    }
}

impl std::error::Error for TransferError {}

impl From<TransferError> for BoreError {
    fn from(e: TransferError) -> Self {
        Self::Transfer(e)
    }
}

// ---------------------------------------------------------------------------
// Protocol errors
// ---------------------------------------------------------------------------

#[derive(Debug)]
pub enum ProtocolError {
    /// Received a message with an unknown or unsupported type tag.
    UnknownMessageType(u8),
    /// Protocol version mismatch.
    VersionMismatch { ours: u32, theirs: u32 },
    /// Message frame is malformed (wrong length, bad encoding).
    MalformedFrame(String),
    /// Unexpected message in the current protocol state.
    UnexpectedMessage { expected: &'static str, got: String },
}

impl fmt::Display for ProtocolError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            Self::UnknownMessageType(tag) => write!(f, "unknown message type: 0x{tag:02x}"),
            Self::VersionMismatch { ours, theirs } => {
                write!(f, "version mismatch: ours={ours}, theirs={theirs}")
            }
            Self::MalformedFrame(msg) => write!(f, "malformed frame: {msg}"),
            Self::UnexpectedMessage { expected, got } => {
                write!(f, "expected {expected}, got {got}")
            }
        }
    }
}

impl std::error::Error for ProtocolError {}

impl From<ProtocolError> for BoreError {
    fn from(e: ProtocolError) -> Self {
        Self::Protocol(e)
    }
}

// ---------------------------------------------------------------------------
// Crypto errors
// ---------------------------------------------------------------------------

#[derive(Debug)]
pub enum CryptoError {
    /// Handshake failed (wrong code, malformed message, timeout).
    HandshakeFailed(String),
    /// Peer authentication failed.
    AuthenticationFailed,
    /// Decryption failed (bad key, corrupted ciphertext, replay).
    DecryptionFailed,
    /// Key material is missing or invalid.
    InvalidKeyMaterial,
}

impl fmt::Display for CryptoError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            Self::HandshakeFailed(msg) => write!(f, "handshake failed: {msg}"),
            Self::AuthenticationFailed => write!(f, "peer authentication failed"),
            Self::DecryptionFailed => write!(f, "decryption failed"),
            Self::InvalidKeyMaterial => write!(f, "invalid key material"),
        }
    }
}

impl std::error::Error for CryptoError {}

impl From<CryptoError> for BoreError {
    fn from(e: CryptoError) -> Self {
        Self::Crypto(e)
    }
}

// ---------------------------------------------------------------------------
// Transport errors
// ---------------------------------------------------------------------------

#[derive(Debug)]
pub enum TransportError {
    /// Connection to peer failed.
    ConnectionFailed(String),
    /// Connection timed out.
    Timeout,
    /// Connection was reset by peer.
    ConnectionReset,
    /// Relay is unreachable or returned an error.
    RelayError(String),
    /// DNS resolution failed.
    DnsFailure(String),
}

impl fmt::Display for TransportError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            Self::ConnectionFailed(msg) => write!(f, "connection failed: {msg}"),
            Self::Timeout => write!(f, "connection timed out"),
            Self::ConnectionReset => write!(f, "connection reset by peer"),
            Self::RelayError(msg) => write!(f, "relay error: {msg}"),
            Self::DnsFailure(msg) => write!(f, "DNS resolution failed: {msg}"),
        }
    }
}

impl std::error::Error for TransportError {}

impl From<TransportError> for BoreError {
    fn from(e: TransportError) -> Self {
        Self::Transport(e)
    }
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
        let _: BoreError = CryptoError::DecryptionFailed.into();
        let _: BoreError = TransportError::Timeout.into();
    }
}
