//! Frame encoding and decoding for the bore wire protocol.
//!
//! Frames are the transport-level unit for bore protocol messages. Each frame
//! consists of a header (4-byte big-endian length + 1-byte type tag) followed
//! by a variable-length payload.
//!
//! ```text
//! ┌──────────────────┬───────────┬──────────────────────┐
//! │  Length (4 bytes) │ Type (1)  │  Payload (variable)  │
//! │  big-endian u32   │  u8 tag   │  up to 16 MiB        │
//! └──────────────────┴───────────┴──────────────────────┘
//! ```
//!
//! The length field covers the type byte + payload (i.e., everything after the
//! length field itself). This module handles encoding/decoding between
//! `ProtocolMessage` values and raw byte frames.

use crate::error::ProtocolError;
use crate::protocol::{FRAME_HEADER_SIZE, MAX_FRAME_PAYLOAD, MessageType, ProtocolMessage};

/// A raw frame: type tag + payload bytes.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Frame {
    /// The message type tag.
    pub message_type: MessageType,
    /// The payload bytes (JSON-encoded message body).
    pub payload: Vec<u8>,
}

impl Frame {
    /// Encode a frame into a byte buffer suitable for wire transport.
    ///
    /// Returns the complete frame including header.
    pub fn encode(&self) -> Vec<u8> {
        // Length = 1 (type tag) + payload length
        let length = 1u32 + self.payload.len() as u32;
        let mut buf = Vec::with_capacity(FRAME_HEADER_SIZE + self.payload.len());
        buf.extend_from_slice(&length.to_be_bytes());
        buf.push(self.message_type.as_byte());
        buf.extend_from_slice(&self.payload);
        buf
    }

    /// Decode a frame from a byte buffer.
    ///
    /// The buffer must contain at least the complete frame (header + payload).
    /// Returns the frame and the number of bytes consumed.
    pub fn decode(buf: &[u8]) -> std::result::Result<(Self, usize), ProtocolError> {
        if buf.len() < FRAME_HEADER_SIZE {
            return Err(ProtocolError::MalformedFrame(format!(
                "buffer too short for frame header: {} bytes",
                buf.len()
            )));
        }

        let length = u32::from_be_bytes([buf[0], buf[1], buf[2], buf[3]]);

        if length == 0 {
            return Err(ProtocolError::MalformedFrame(
                "frame length is zero".to_string(),
            ));
        }

        let payload_len = length - 1; // subtract type tag byte

        if payload_len > MAX_FRAME_PAYLOAD {
            return Err(ProtocolError::MalformedFrame(format!(
                "frame payload too large: {payload_len} bytes (max {MAX_FRAME_PAYLOAD})"
            )));
        }

        let total_frame_size = 4 + length as usize; // 4 bytes length + rest

        if buf.len() < total_frame_size {
            return Err(ProtocolError::MalformedFrame(format!(
                "buffer too short: need {total_frame_size} bytes, have {}",
                buf.len()
            )));
        }

        let type_byte = buf[4];
        let message_type = MessageType::from_byte(type_byte)
            .ok_or(ProtocolError::UnknownMessageType(type_byte))?;

        let payload = buf[5..total_frame_size].to_vec();

        Ok((
            Frame {
                message_type,
                payload,
            },
            total_frame_size,
        ))
    }
}

/// Encode a `ProtocolMessage` into a wire-format byte buffer.
pub fn encode_message(msg: &ProtocolMessage) -> std::result::Result<Vec<u8>, ProtocolError> {
    let payload =
        serde_json::to_vec(msg).map_err(|e| ProtocolError::Serialization(e.to_string()))?;

    let frame = Frame {
        message_type: msg.message_type(),
        payload,
    };

    Ok(frame.encode())
}

/// Decode a wire-format byte buffer into a `ProtocolMessage`.
///
/// Returns the decoded message and the number of bytes consumed from the buffer.
pub fn decode_message(buf: &[u8]) -> std::result::Result<(ProtocolMessage, usize), ProtocolError> {
    let (frame, consumed) = Frame::decode(buf)?;

    let msg: ProtocolMessage = serde_json::from_slice(&frame.payload)
        .map_err(|e| ProtocolError::Serialization(e.to_string()))?;

    // Verify the type tag matches the deserialized message type.
    let actual_type = msg.message_type();
    if actual_type != frame.message_type {
        return Err(ProtocolError::UnexpectedMessage {
            expected: frame.message_type.name(),
            got: actual_type.name().to_string(),
        });
    }

    Ok((msg, consumed))
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::protocol::*;
    use crate::session::{Capability, SessionId, TransferRole};

    #[test]
    fn frame_encode_decode_round_trip() {
        let frame = Frame {
            message_type: MessageType::Hello,
            payload: b"test payload".to_vec(),
        };
        let encoded = frame.encode();
        let (decoded, consumed) = Frame::decode(&encoded).unwrap();
        assert_eq!(frame, decoded);
        assert_eq!(consumed, encoded.len());
    }

    #[test]
    fn frame_header_size_correct() {
        let frame = Frame {
            message_type: MessageType::Data,
            payload: vec![0u8; 100],
        };
        let encoded = frame.encode();
        // 4 bytes length + 1 byte type + 100 bytes payload
        assert_eq!(encoded.len(), 105);
        // Length field should be 101 (1 type + 100 payload)
        let length = u32::from_be_bytes([encoded[0], encoded[1], encoded[2], encoded[3]]);
        assert_eq!(length, 101);
    }

    #[test]
    fn frame_decode_rejects_short_buffer() {
        let result = Frame::decode(&[0, 0, 0]);
        assert!(result.is_err());
    }

    #[test]
    fn frame_decode_rejects_zero_length() {
        let buf = [0u8, 0, 0, 0, 0x01];
        let result = Frame::decode(&buf);
        assert!(result.is_err());
    }

    #[test]
    fn frame_decode_rejects_unknown_type() {
        // length = 1 (just the type byte), type = 0xFF (unknown)
        let buf = [0u8, 0, 0, 1, 0xFF];
        let result = Frame::decode(&buf);
        assert!(matches!(
            result,
            Err(ProtocolError::UnknownMessageType(0xFF))
        ));
    }

    #[test]
    fn frame_decode_rejects_truncated_payload() {
        // length says 10 bytes (1 type + 9 payload) but buffer only has header
        let buf = [0u8, 0, 0, 10, 0x01];
        let result = Frame::decode(&buf);
        assert!(result.is_err());
    }

    #[test]
    fn message_encode_decode_hello() {
        let msg = ProtocolMessage::Hello(HelloMessage {
            version: ProtocolVersion::CURRENT,
            role: TransferRole::Sender,
            capabilities: vec![Capability::Resume],
            session_id: Some(SessionId::from_hex("abcdef0123456789abcdef0123456789").unwrap()),
        });
        let encoded = encode_message(&msg).unwrap();
        let (decoded, consumed) = decode_message(&encoded).unwrap();
        assert_eq!(msg, decoded);
        assert_eq!(consumed, encoded.len());
    }

    #[test]
    fn message_encode_decode_offer() {
        let msg = ProtocolMessage::Offer(OfferMessage {
            name: "test".to_string(),
            files: vec![OfferFileEntry {
                path: "file.txt".to_string(),
                size: 1024,
                is_directory: false,
                executable: false,
            }],
            total_bytes: 1024,
        });
        let encoded = encode_message(&msg).unwrap();
        let (decoded, _) = decode_message(&encoded).unwrap();
        assert_eq!(msg, decoded);
    }

    #[test]
    fn message_encode_decode_data() {
        let msg = ProtocolMessage::Data(DataMessage {
            file_index: 0,
            chunk_index: 5,
            offset: 1_310_720,
            payload: vec![1, 2, 3, 4, 5],
        });
        let encoded = encode_message(&msg).unwrap();
        let (decoded, _) = decode_message(&encoded).unwrap();
        assert_eq!(msg, decoded);
    }

    #[test]
    fn message_encode_decode_error() {
        let msg = ProtocolMessage::Error(ErrorMessage {
            code: 500,
            message: "internal error".to_string(),
            fatal: true,
        });
        let encoded = encode_message(&msg).unwrap();
        let (decoded, _) = decode_message(&encoded).unwrap();
        assert_eq!(msg, decoded);
    }

    #[test]
    fn message_encode_decode_close() {
        let msg = ProtocolMessage::Close(CloseMessage { reason: None });
        let encoded = encode_message(&msg).unwrap();
        let (decoded, _) = decode_message(&encoded).unwrap();
        assert_eq!(msg, decoded);
    }

    #[test]
    fn message_encode_decode_done() {
        let msg = ProtocolMessage::Done(DoneMessage {
            total_bytes: 999_999,
            total_files: 42,
        });
        let encoded = encode_message(&msg).unwrap();
        let (decoded, _) = decode_message(&encoded).unwrap();
        assert_eq!(msg, decoded);
    }

    #[test]
    fn message_encode_decode_ack() {
        let msg = ProtocolMessage::Ack(AckMessage {
            file_index: 2,
            chunk_index: 100,
        });
        let encoded = encode_message(&msg).unwrap();
        let (decoded, _) = decode_message(&encoded).unwrap();
        assert_eq!(msg, decoded);
    }

    #[test]
    fn message_encode_decode_accept() {
        let msg = ProtocolMessage::Accept(AcceptMessage {
            session_id: SessionId::from_hex("11111111222222223333333344444444").unwrap(),
        });
        let encoded = encode_message(&msg).unwrap();
        let (decoded, _) = decode_message(&encoded).unwrap();
        assert_eq!(msg, decoded);
    }

    #[test]
    fn message_encode_decode_reject() {
        let msg = ProtocolMessage::Reject(RejectMessage {
            session_id: SessionId::from_hex("11111111222222223333333344444444").unwrap(),
            reason: "no space".to_string(),
        });
        let encoded = encode_message(&msg).unwrap();
        let (decoded, _) = decode_message(&encoded).unwrap();
        assert_eq!(msg, decoded);
    }

    #[test]
    fn empty_payload_frame_round_trips() {
        let frame = Frame {
            message_type: MessageType::Close,
            payload: vec![],
        };
        let encoded = frame.encode();
        let (decoded, consumed) = Frame::decode(&encoded).unwrap();
        assert_eq!(frame, decoded);
        assert_eq!(consumed, encoded.len());
    }
}
