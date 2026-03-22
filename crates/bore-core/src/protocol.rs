//! Protocol types for bore.
//!
//! These types define the wire protocol: versioning, message types, frame
//! structure, and concrete message payloads. The transport layer will use
//! the codec module for frame encoding/decoding.
//!
//! Protocol design principles:
//! - Versioned from the start (no retroactive compatibility hacks)
//! - Length-prefixed binary frames with type tags
//! - Every message type is explicit — no overloaded semantics
//! - All payloads are serde-serializable for wire transport

use serde::{Deserialize, Serialize};

use crate::session::{Capability, SessionId, TransferRole};

// ---------------------------------------------------------------------------
// Protocol version
// ---------------------------------------------------------------------------

/// Current protocol version.
///
/// Incremented when the wire format changes in incompatible ways.
/// Peers negotiate the version during handshake.
pub const PROTOCOL_VERSION: u32 = 0;

/// Protocol version with comparison support.
#[derive(Debug, Clone, Copy, PartialEq, Eq, PartialOrd, Ord, Hash, Serialize, Deserialize)]
pub struct ProtocolVersion(pub u32);

impl ProtocolVersion {
    /// The current version of the bore protocol.
    pub const CURRENT: Self = Self(PROTOCOL_VERSION);

    /// Returns whether this version is compatible with `other`.
    ///
    /// For now, versions must match exactly. Future versions may support
    /// a compatibility range.
    pub const fn is_compatible_with(self, other: Self) -> bool {
        self.0 == other.0
    }
}

impl std::fmt::Display for ProtocolVersion {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        write!(f, "v{}", self.0)
    }
}

// ---------------------------------------------------------------------------
// Message types (type tags)
// ---------------------------------------------------------------------------

/// Protocol message type tags.
///
/// These define the vocabulary of the bore protocol. Each message type has
/// well-defined semantics and valid contexts (which session states it can
/// appear in).
///
/// Wire format: each message is a length-prefixed frame with a 1-byte type tag
/// followed by the message payload.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[repr(u8)]
pub enum MessageType {
    /// Initial handshake message. Contains protocol version and capabilities.
    Hello = 0x01,
    /// Sender offers a transfer manifest to the receiver.
    Offer = 0x02,
    /// Receiver accepts the offered transfer.
    Accept = 0x03,
    /// Receiver rejects the offered transfer (with reason).
    Reject = 0x04,
    /// A chunk of file data.
    Data = 0x10,
    /// Acknowledgment of received data (per-chunk or cumulative).
    Ack = 0x11,
    /// Transfer is complete (sent by sender after all data is acknowledged).
    Done = 0x20,
    /// Error message (can be sent by either side at any time).
    Error = 0xE0,
    /// Graceful session close.
    Close = 0xF0,
}

impl MessageType {
    /// Parse a message type from a raw byte.
    pub const fn from_byte(byte: u8) -> Option<Self> {
        match byte {
            0x01 => Some(Self::Hello),
            0x02 => Some(Self::Offer),
            0x03 => Some(Self::Accept),
            0x04 => Some(Self::Reject),
            0x10 => Some(Self::Data),
            0x11 => Some(Self::Ack),
            0x20 => Some(Self::Done),
            0xE0 => Some(Self::Error),
            0xF0 => Some(Self::Close),
            _ => None,
        }
    }

    /// The raw byte representation of this message type.
    pub const fn as_byte(self) -> u8 {
        self as u8
    }

    /// Human-readable name for this message type.
    pub const fn name(self) -> &'static str {
        match self {
            Self::Hello => "Hello",
            Self::Offer => "Offer",
            Self::Accept => "Accept",
            Self::Reject => "Reject",
            Self::Data => "Data",
            Self::Ack => "Ack",
            Self::Done => "Done",
            Self::Error => "Error",
            Self::Close => "Close",
        }
    }
}

impl std::fmt::Display for MessageType {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.write_str(self.name())
    }
}

// ---------------------------------------------------------------------------
// Concrete message payloads
// ---------------------------------------------------------------------------

/// Hello message — sent by both peers at the start of a session.
///
/// Contains protocol version, role, capabilities, and optional session identity.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct HelloMessage {
    /// The protocol version this peer supports.
    pub version: ProtocolVersion,
    /// The role this peer plays.
    pub role: TransferRole,
    /// Capabilities this peer advertises.
    pub capabilities: Vec<Capability>,
    /// Session ID (set by the sender, echoed by the receiver).
    pub session_id: Option<SessionId>,
}

/// Offer message — sender describes what it wants to transfer.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct OfferMessage {
    /// Human-readable name for the transfer.
    pub name: String,
    /// Individual file entries in the transfer.
    pub files: Vec<OfferFileEntry>,
    /// Total size in bytes across all files.
    pub total_bytes: u64,
}

/// A single file entry in an offer manifest.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct OfferFileEntry {
    /// Path relative to the transfer root.
    pub path: String,
    /// Size in bytes (0 for directories).
    pub size: u64,
    /// Whether this is a directory.
    pub is_directory: bool,
    /// Whether this file should be executable (Unix).
    pub executable: bool,
}

/// Accept message — receiver agrees to the transfer.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct AcceptMessage {
    /// Session ID being accepted.
    pub session_id: SessionId,
}

/// Reject message — receiver declines the transfer.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct RejectMessage {
    /// Session ID being rejected.
    pub session_id: SessionId,
    /// Human-readable reason for rejection.
    pub reason: String,
}

/// Data message — a chunk of file data.
///
/// The `payload` field is raw bytes, encoded as base64 in JSON representation.
/// In the binary wire format, it is sent as raw bytes after the header.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct DataMessage {
    /// Index of the file in the offer manifest (0-based).
    pub file_index: u32,
    /// Index of this chunk within the file (0-based).
    pub chunk_index: u64,
    /// Byte offset within the file.
    pub offset: u64,
    /// The chunk payload.
    #[serde(with = "base64_bytes")]
    pub payload: Vec<u8>,
}

/// Ack message — receiver acknowledges receipt of a chunk.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct AckMessage {
    /// Index of the file being acknowledged.
    pub file_index: u32,
    /// The chunk index being acknowledged.
    pub chunk_index: u64,
}

/// Done message — sender signals all data has been sent and acknowledged.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct DoneMessage {
    /// Total bytes transferred.
    pub total_bytes: u64,
    /// Total files transferred.
    pub total_files: u32,
}

/// Error message — either side reports an error.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct ErrorMessage {
    /// Machine-readable error code.
    pub code: u32,
    /// Human-readable error description.
    pub message: String,
    /// Whether the error is fatal (session should be terminated).
    pub fatal: bool,
}

/// Close message — graceful session termination.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct CloseMessage {
    /// Optional reason for closing.
    pub reason: Option<String>,
}

// ---------------------------------------------------------------------------
// Envelope: tagged union of all message types
// ---------------------------------------------------------------------------

/// A protocol message with its type tag — the top-level envelope for wire transport.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(tag = "type", rename_all = "lowercase")]
pub enum ProtocolMessage {
    Hello(HelloMessage),
    Offer(OfferMessage),
    Accept(AcceptMessage),
    Reject(RejectMessage),
    Data(DataMessage),
    Ack(AckMessage),
    Done(DoneMessage),
    Error(ErrorMessage),
    Close(CloseMessage),
}

impl ProtocolMessage {
    /// Returns the message type tag for this message.
    pub fn message_type(&self) -> MessageType {
        match self {
            Self::Hello(_) => MessageType::Hello,
            Self::Offer(_) => MessageType::Offer,
            Self::Accept(_) => MessageType::Accept,
            Self::Reject(_) => MessageType::Reject,
            Self::Data(_) => MessageType::Data,
            Self::Ack(_) => MessageType::Ack,
            Self::Done(_) => MessageType::Done,
            Self::Error(_) => MessageType::Error,
            Self::Close(_) => MessageType::Close,
        }
    }
}

// ---------------------------------------------------------------------------
// Frame constants
// ---------------------------------------------------------------------------

/// Maximum frame payload size (16 MiB).
///
/// This is the maximum size of a single protocol frame's payload, not
/// including the frame header. Chosen to be large enough for data chunks
/// but small enough to avoid memory issues.
pub const MAX_FRAME_PAYLOAD: u32 = 16 * 1024 * 1024;

/// Frame header size: 4 bytes length + 1 byte type tag.
pub const FRAME_HEADER_SIZE: usize = 5;

// ---------------------------------------------------------------------------
// Base64 serde helper for binary payloads in JSON
// ---------------------------------------------------------------------------

mod base64_bytes {
    use serde::{Deserialize, Deserializer, Serialize, Serializer};

    pub fn serialize<S: Serializer>(bytes: &[u8], serializer: S) -> Result<S::Ok, S::Error> {
        // Use standard base64 encoding via a simple implementation.
        // For JSON representation, encode bytes as an array of u8 values.
        // This avoids adding a base64 dependency just for serialization.
        bytes.to_vec().serialize(serializer)
    }

    pub fn deserialize<'de, D: Deserializer<'de>>(deserializer: D) -> Result<Vec<u8>, D::Error> {
        Vec::<u8>::deserialize(deserializer)
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn protocol_version_compatibility() {
        let v0 = ProtocolVersion(0);
        let v1 = ProtocolVersion(1);
        assert!(v0.is_compatible_with(v0));
        assert!(!v0.is_compatible_with(v1));
    }

    #[test]
    fn message_type_round_trip() {
        let types = [
            MessageType::Hello,
            MessageType::Offer,
            MessageType::Accept,
            MessageType::Reject,
            MessageType::Data,
            MessageType::Ack,
            MessageType::Done,
            MessageType::Error,
            MessageType::Close,
        ];

        for msg_type in types {
            let byte = msg_type.as_byte();
            let parsed = MessageType::from_byte(byte).unwrap();
            assert_eq!(parsed, msg_type);
        }
    }

    #[test]
    fn unknown_message_type_returns_none() {
        assert!(MessageType::from_byte(0x00).is_none());
        assert!(MessageType::from_byte(0xFF).is_none());
        assert!(MessageType::from_byte(0x05).is_none());
    }

    #[test]
    fn current_protocol_version() {
        assert_eq!(ProtocolVersion::CURRENT, ProtocolVersion(0));
    }

    // -----------------------------------------------------------------------
    // Serde round-trip tests for all message types
    // -----------------------------------------------------------------------

    #[test]
    fn hello_message_serde_round_trip() {
        let msg = ProtocolMessage::Hello(HelloMessage {
            version: ProtocolVersion::CURRENT,
            role: crate::session::TransferRole::Sender,
            capabilities: vec![
                crate::session::Capability::Resume,
                crate::session::Capability::Compression,
            ],
            session_id: Some(SessionId::from_hex("abcdef0123456789abcdef0123456789").unwrap()),
        });
        let json = serde_json::to_string(&msg).unwrap();
        let back: ProtocolMessage = serde_json::from_str(&json).unwrap();
        assert_eq!(msg, back);
    }

    #[test]
    fn offer_message_serde_round_trip() {
        let msg = ProtocolMessage::Offer(OfferMessage {
            name: "photos".to_string(),
            files: vec![
                OfferFileEntry {
                    path: "a.jpg".to_string(),
                    size: 1024,
                    is_directory: false,
                    executable: false,
                },
                OfferFileEntry {
                    path: "subdir".to_string(),
                    size: 0,
                    is_directory: true,
                    executable: false,
                },
            ],
            total_bytes: 1024,
        });
        let json = serde_json::to_string(&msg).unwrap();
        let back: ProtocolMessage = serde_json::from_str(&json).unwrap();
        assert_eq!(msg, back);
    }

    #[test]
    fn accept_message_serde_round_trip() {
        let msg = ProtocolMessage::Accept(AcceptMessage {
            session_id: SessionId::from_hex("abcdef0123456789abcdef0123456789").unwrap(),
        });
        let json = serde_json::to_string(&msg).unwrap();
        let back: ProtocolMessage = serde_json::from_str(&json).unwrap();
        assert_eq!(msg, back);
    }

    #[test]
    fn reject_message_serde_round_trip() {
        let msg = ProtocolMessage::Reject(RejectMessage {
            session_id: SessionId::from_hex("abcdef0123456789abcdef0123456789").unwrap(),
            reason: "not enough disk space".to_string(),
        });
        let json = serde_json::to_string(&msg).unwrap();
        let back: ProtocolMessage = serde_json::from_str(&json).unwrap();
        assert_eq!(msg, back);
    }

    #[test]
    fn data_message_serde_round_trip() {
        let msg = ProtocolMessage::Data(DataMessage {
            file_index: 0,
            chunk_index: 42,
            offset: 10752,
            payload: vec![0xDE, 0xAD, 0xBE, 0xEF, 0x01, 0x02, 0x03],
        });
        let json = serde_json::to_string(&msg).unwrap();
        let back: ProtocolMessage = serde_json::from_str(&json).unwrap();
        assert_eq!(msg, back);
    }

    #[test]
    fn ack_message_serde_round_trip() {
        let msg = ProtocolMessage::Ack(AckMessage {
            file_index: 0,
            chunk_index: 42,
        });
        let json = serde_json::to_string(&msg).unwrap();
        let back: ProtocolMessage = serde_json::from_str(&json).unwrap();
        assert_eq!(msg, back);
    }

    #[test]
    fn done_message_serde_round_trip() {
        let msg = ProtocolMessage::Done(DoneMessage {
            total_bytes: 1_048_576,
            total_files: 3,
        });
        let json = serde_json::to_string(&msg).unwrap();
        let back: ProtocolMessage = serde_json::from_str(&json).unwrap();
        assert_eq!(msg, back);
    }

    #[test]
    fn error_message_serde_round_trip() {
        let msg = ProtocolMessage::Error(ErrorMessage {
            code: 1001,
            message: "disk full".to_string(),
            fatal: true,
        });
        let json = serde_json::to_string(&msg).unwrap();
        let back: ProtocolMessage = serde_json::from_str(&json).unwrap();
        assert_eq!(msg, back);
    }

    #[test]
    fn close_message_serde_round_trip() {
        let msg = ProtocolMessage::Close(CloseMessage {
            reason: Some("transfer complete".to_string()),
        });
        let json = serde_json::to_string(&msg).unwrap();
        let back: ProtocolMessage = serde_json::from_str(&json).unwrap();
        assert_eq!(msg, back);
    }

    #[test]
    fn close_message_no_reason_serde_round_trip() {
        let msg = ProtocolMessage::Close(CloseMessage { reason: None });
        let json = serde_json::to_string(&msg).unwrap();
        let back: ProtocolMessage = serde_json::from_str(&json).unwrap();
        assert_eq!(msg, back);
    }

    #[test]
    fn message_type_accessor_matches() {
        let hello = ProtocolMessage::Hello(HelloMessage {
            version: ProtocolVersion::CURRENT,
            role: crate::session::TransferRole::Sender,
            capabilities: vec![],
            session_id: None,
        });
        assert_eq!(hello.message_type(), MessageType::Hello);

        let done = ProtocolMessage::Done(DoneMessage {
            total_bytes: 0,
            total_files: 0,
        });
        assert_eq!(done.message_type(), MessageType::Done);
    }

    #[test]
    fn protocol_version_serde_round_trip() {
        let v = ProtocolVersion::CURRENT;
        let json = serde_json::to_string(&v).unwrap();
        let back: ProtocolVersion = serde_json::from_str(&json).unwrap();
        assert_eq!(v, back);
    }
}
