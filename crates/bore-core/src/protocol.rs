//! Protocol types for bore.
//!
//! These types define the wire protocol: versioning, message types, and frame
//! structure. The actual serialization and transport will be implemented in
//! later phases.
//!
//! Protocol design principles:
//! - Versioned from the start (no retroactive compatibility hacks)
//! - Length-prefixed binary frames with type tags
//! - Every message type is explicit — no overloaded semantics

// ---------------------------------------------------------------------------
// Protocol version
// ---------------------------------------------------------------------------

/// Current protocol version.
///
/// Incremented when the wire format changes in incompatible ways.
/// Peers negotiate the version during handshake.
pub const PROTOCOL_VERSION: u32 = 0;

/// Protocol version with comparison support.
#[derive(Debug, Clone, Copy, PartialEq, Eq, PartialOrd, Ord, Hash)]
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
// Message types
// ---------------------------------------------------------------------------

/// Protocol message type tags.
///
/// These define the vocabulary of the bore protocol. Each message type has
/// well-defined semantics and valid contexts (which session states it can
/// appear in).
///
/// Wire format: each message is a length-prefixed frame with a 1-byte type tag
/// followed by the message payload.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
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
}
