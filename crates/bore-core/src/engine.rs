//! Transfer engine for bore.
//!
//! Implements the core file transfer protocol over [`SecureChannel`]. Handles
//! chunking, streaming, metadata exchange, and SHA-256 integrity verification.
//!
//! The engine is transport-agnostic: it operates over any async byte stream
//! wrapped in a `SecureChannel`. File data flows as a sequence of encrypted
//! messages:
//!
//! 1. **Header** — file metadata (name, size, SHA-256 hash, chunk parameters)
//! 2. **Chunks** — sequential fixed-size blocks of file data
//! 3. **End** — transfer completion signal
//!
//! The receiver verifies the SHA-256 hash of the reassembled data against
//! the hash declared in the header. Any mismatch is a hard error.
//!
//! # Wire format
//!
//! Each message is a binary payload sent as a single `SecureChannel` message
//! (encrypted with ChaCha20-Poly1305). The first byte is a type tag:
//!
//! ```text
//! Header (0x01):
//!   [1: tag] [8: size BE] [32: sha256] [4: chunk_size BE] [8: chunk_count BE] [2: name_len BE] [N: name UTF-8]
//!
//! Chunk (0x02):
//!   [1: tag] [8: index BE] [N: data]
//!
//! End (0x03):
//!   [1: tag]
//! ```
//!
//! # Example (in-process round-trip)
//!
//! ```rust,no_run
//! use bore_core::engine::{send_data, receive_data};
//! // After performing a Noise handshake to obtain two SecureChannels
//! // and a transport duplex stream, the sender calls:
//! //   send_data(&mut sender_ch, &mut writer, "report.pdf", &file_bytes).await
//! // and the receiver calls:
//! //   receive_data(&mut receiver_ch, &mut reader).await
//! ```

use sha2::{Digest, Sha256};
use tokio::io::{AsyncRead, AsyncWrite};

use crate::crypto::SecureChannel;
use crate::error::TransferError;
use crate::transfer::DEFAULT_CHUNK_SIZE;

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

/// Type tag for a header message.
const MSG_HEADER: u8 = 0x01;

/// Type tag for a chunk message.
const MSG_CHUNK: u8 = 0x02;

/// Type tag for an end message.
const MSG_END: u8 = 0x03;

/// Fixed-size portion of the header: tag(1) + size(8) + sha256(32) + chunk_size(4) + chunk_count(8) + name_len(2).
const HEADER_FIXED_LEN: usize = 1 + 8 + 32 + 4 + 8 + 2;

/// Maximum filename length in bytes (UTF-8).
const MAX_FILENAME_LEN: usize = 4096;

/// Maximum number of chunks per transfer (prevents memory exhaustion from a
/// malicious or malformed header). At 256 KiB per chunk this allows ~4 TB.
const MAX_CHUNK_COUNT: u64 = 16 * 1024 * 1024;

// ---------------------------------------------------------------------------
// Public types
// ---------------------------------------------------------------------------

/// Metadata about a file being transferred.
///
/// Sent as the first message in a transfer. The receiver uses the SHA-256 hash
/// to verify the reassembled file's integrity.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct FileHeader {
    /// Filename (without directory components).
    pub filename: String,
    /// Total file size in bytes.
    pub size: u64,
    /// SHA-256 hash of the complete file contents.
    pub sha256: [u8; 32],
    /// Chunk size in bytes used by the sender.
    pub chunk_size: u32,
    /// Total number of chunks that will follow.
    pub chunk_count: u64,
}

/// Result of a successful send operation.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct SendResult {
    /// Filename that was sent.
    pub filename: String,
    /// Total bytes of file data sent.
    pub size: u64,
    /// SHA-256 hash of the data.
    pub sha256: [u8; 32],
    /// Number of chunks sent.
    pub chunks_sent: u64,
}

/// Result of a successful receive operation.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct ReceiveResult {
    /// Filename declared by the sender.
    pub filename: String,
    /// Total bytes received.
    pub size: u64,
    /// Verified SHA-256 hash of the received data.
    pub sha256: [u8; 32],
    /// Reassembled file data.
    pub data: Vec<u8>,
    /// Number of chunks received.
    pub chunks_received: u64,
}

// ---------------------------------------------------------------------------
// Decoded message (internal)
// ---------------------------------------------------------------------------

/// A decoded transfer message.
#[derive(Debug, PartialEq, Eq)]
enum DecodedMessage {
    Header(FileHeader),
    Chunk { index: u64, data: Vec<u8> },
    End,
}

// ---------------------------------------------------------------------------
// Encoding
// ---------------------------------------------------------------------------

/// Encode a file header into a binary message.
fn encode_header(header: &FileHeader) -> Vec<u8> {
    let name_bytes = header.filename.as_bytes();
    let name_len = name_bytes.len() as u16;
    let mut buf = Vec::with_capacity(HEADER_FIXED_LEN + name_bytes.len());
    buf.push(MSG_HEADER);
    buf.extend_from_slice(&header.size.to_be_bytes());
    buf.extend_from_slice(&header.sha256);
    buf.extend_from_slice(&header.chunk_size.to_be_bytes());
    buf.extend_from_slice(&header.chunk_count.to_be_bytes());
    buf.extend_from_slice(&name_len.to_be_bytes());
    buf.extend_from_slice(name_bytes);
    buf
}

/// Encode a chunk into a binary message.
fn encode_chunk(index: u64, data: &[u8]) -> Vec<u8> {
    let mut buf = Vec::with_capacity(1 + 8 + data.len());
    buf.push(MSG_CHUNK);
    buf.extend_from_slice(&index.to_be_bytes());
    buf.extend_from_slice(data);
    buf
}

/// Encode an end signal.
fn encode_end() -> Vec<u8> {
    vec![MSG_END]
}

// ---------------------------------------------------------------------------
// Decoding
// ---------------------------------------------------------------------------

/// Decode a binary message into a typed transfer message.
fn decode_message(buf: &[u8]) -> Result<DecodedMessage, TransferError> {
    if buf.is_empty() {
        return Err(TransferError::InvalidManifest("empty message".to_string()));
    }

    match buf[0] {
        MSG_HEADER => decode_header(buf),
        MSG_CHUNK => decode_chunk(buf),
        MSG_END => Ok(DecodedMessage::End),
        tag => Err(TransferError::InvalidManifest(format!(
            "unknown message tag: 0x{tag:02x}"
        ))),
    }
}

/// Decode a header message.
fn decode_header(buf: &[u8]) -> Result<DecodedMessage, TransferError> {
    if buf.len() < HEADER_FIXED_LEN {
        return Err(TransferError::InvalidManifest(format!(
            "header too short: {} bytes (need at least {HEADER_FIXED_LEN})",
            buf.len()
        )));
    }

    let mut pos = 1; // skip tag

    let size = u64::from_be_bytes(buf[pos..pos + 8].try_into().unwrap());
    pos += 8;

    let mut sha256 = [0u8; 32];
    sha256.copy_from_slice(&buf[pos..pos + 32]);
    pos += 32;

    let chunk_size = u32::from_be_bytes(buf[pos..pos + 4].try_into().unwrap());
    pos += 4;

    let chunk_count = u64::from_be_bytes(buf[pos..pos + 8].try_into().unwrap());
    pos += 8;

    let name_len = u16::from_be_bytes(buf[pos..pos + 2].try_into().unwrap()) as usize;
    pos += 2;

    if name_len > MAX_FILENAME_LEN {
        return Err(TransferError::InvalidManifest(format!(
            "filename too long: {name_len} bytes (max {MAX_FILENAME_LEN})"
        )));
    }

    if buf.len() < pos + name_len {
        return Err(TransferError::InvalidManifest(format!(
            "header truncated: need {} bytes for filename, have {}",
            name_len,
            buf.len() - pos
        )));
    }

    let filename = String::from_utf8(buf[pos..pos + name_len].to_vec())
        .map_err(|e| TransferError::InvalidManifest(format!("filename is not valid UTF-8: {e}")))?;

    if chunk_count > MAX_CHUNK_COUNT {
        return Err(TransferError::InvalidManifest(format!(
            "chunk count too large: {chunk_count} (max {MAX_CHUNK_COUNT})"
        )));
    }

    Ok(DecodedMessage::Header(FileHeader {
        filename,
        size,
        sha256,
        chunk_size,
        chunk_count,
    }))
}

/// Decode a chunk message.
fn decode_chunk(buf: &[u8]) -> Result<DecodedMessage, TransferError> {
    // Minimum: 1 (tag) + 8 (index) = 9 bytes (data can be empty for last chunk edge case)
    if buf.len() < 9 {
        return Err(TransferError::InvalidManifest(format!(
            "chunk message too short: {} bytes (need at least 9)",
            buf.len()
        )));
    }

    let index = u64::from_be_bytes(buf[1..9].try_into().unwrap());
    let data = buf[9..].to_vec();

    Ok(DecodedMessage::Chunk { index, data })
}

// ---------------------------------------------------------------------------
// Filename validation
// ---------------------------------------------------------------------------

/// Validate a filename for transfer safety.
///
/// Rejects empty filenames, path separators, null bytes, and names that are
/// too long. This prevents directory traversal and other path injection attacks.
fn validate_filename(name: &str) -> Result<(), TransferError> {
    if name.is_empty() {
        return Err(TransferError::InvalidManifest(
            "filename is empty".to_string(),
        ));
    }

    if name.len() > MAX_FILENAME_LEN {
        return Err(TransferError::InvalidManifest(format!(
            "filename too long: {} bytes (max {MAX_FILENAME_LEN})",
            name.len()
        )));
    }

    if name.contains('/') || name.contains('\\') {
        return Err(TransferError::InvalidManifest(
            "filename contains path separator".to_string(),
        ));
    }

    if name.contains('\0') {
        return Err(TransferError::InvalidManifest(
            "filename contains null byte".to_string(),
        ));
    }

    if name == "." || name == ".." {
        return Err(TransferError::InvalidManifest(
            "filename is a relative path component".to_string(),
        ));
    }

    Ok(())
}

// ---------------------------------------------------------------------------
// Public API
// ---------------------------------------------------------------------------

/// Compute the SHA-256 hash of a byte slice.
pub fn sha256_hash(data: &[u8]) -> [u8; 32] {
    let mut hasher = Sha256::new();
    hasher.update(data);
    hasher.finalize().into()
}

/// Calculate the number of chunks needed for a given data size and chunk size.
///
/// # Panics
///
/// Panics if `chunk_size` is zero and `data_len` is non-zero.
pub fn chunk_count(data_len: u64, chunk_size: u32) -> u64 {
    if data_len == 0 {
        return 0;
    }
    assert!(chunk_size > 0, "chunk_size must be non-zero");
    data_len.div_ceil(chunk_size as u64)
}

/// Split data into chunks of the given size.
///
/// Returns an empty vec for empty input. The last chunk may be smaller than
/// `chunk_size`.
pub fn chunk_data(data: &[u8], chunk_size: usize) -> Vec<&[u8]> {
    if data.is_empty() {
        return vec![];
    }
    data.chunks(chunk_size).collect()
}

/// Send data over a [`SecureChannel`] with SHA-256 integrity verification.
///
/// The transfer protocol sends a header (filename, size, hash, chunk params),
/// followed by sequential data chunks, followed by an end marker. Each message
/// is independently encrypted by the SecureChannel.
///
/// # Arguments
///
/// * `channel` — The encrypted channel from a completed Noise handshake.
/// * `writer` — The transport stream to write encrypted messages to.
/// * `filename` — Name of the file being sent (no path components allowed).
/// * `data` — Complete file contents.
///
/// # Errors
///
/// Returns `TransferError` if the filename is invalid, serialization fails,
/// or the encrypted channel encounters an error.
pub async fn send_data<W: AsyncWrite + Unpin>(
    channel: &mut SecureChannel,
    writer: &mut W,
    filename: &str,
    data: &[u8],
) -> Result<SendResult, TransferError> {
    validate_filename(filename)?;

    // Compute SHA-256 of the full file
    let sha256 = sha256_hash(data);

    let cs = DEFAULT_CHUNK_SIZE;
    let cc = chunk_count(data.len() as u64, cs);

    let header = FileHeader {
        filename: filename.to_string(),
        size: data.len() as u64,
        sha256,
        chunk_size: cs,
        chunk_count: cc,
    };

    // Send header
    let header_bytes = encode_header(&header);
    channel
        .send(&header_bytes, writer)
        .await
        .map_err(|e| TransferError::FileIo(format!("failed to send header: {e}")))?;

    // Send chunks
    let chunks = chunk_data(data, cs as usize);
    for (index, chunk) in chunks.iter().enumerate() {
        let chunk_bytes = encode_chunk(index as u64, chunk);
        channel
            .send(&chunk_bytes, writer)
            .await
            .map_err(|e| TransferError::FileIo(format!("failed to send chunk {index}: {e}")))?;
    }

    // Send end
    let end_bytes = encode_end();
    channel
        .send(&end_bytes, writer)
        .await
        .map_err(|e| TransferError::FileIo(format!("failed to send end marker: {e}")))?;

    Ok(SendResult {
        filename: filename.to_string(),
        size: data.len() as u64,
        sha256,
        chunks_sent: cc,
    })
}

/// Receive data over a [`SecureChannel`] with SHA-256 integrity verification.
///
/// Reads the transfer protocol: header, sequential chunks, end marker.
/// Verifies the reassembled data against the SHA-256 hash declared in the
/// header. Returns a hard error on hash mismatch.
///
/// # Arguments
///
/// * `channel` — The encrypted channel from a completed Noise handshake.
/// * `reader` — The transport stream to read encrypted messages from.
///
/// # Errors
///
/// Returns `TransferError` if the protocol is violated (unexpected messages,
/// out-of-order chunks, size mismatch) or integrity verification fails.
pub async fn receive_data<R: AsyncRead + Unpin>(
    channel: &mut SecureChannel,
    reader: &mut R,
) -> Result<ReceiveResult, TransferError> {
    // Receive header
    let header_bytes = channel
        .recv(reader)
        .await
        .map_err(|e| TransferError::FileIo(format!("failed to receive header: {e}")))?;

    let header = match decode_message(&header_bytes)? {
        DecodedMessage::Header(h) => h,
        other => {
            return Err(TransferError::InvalidManifest(format!(
                "expected header, got {other:?}"
            )));
        }
    };

    validate_filename(&header.filename)?;

    // Pre-allocate buffer for the full file
    let mut received_data = Vec::with_capacity(header.size as usize);
    let mut chunks_received: u64 = 0;

    // Receive chunks until End
    loop {
        let msg_bytes = channel
            .recv(reader)
            .await
            .map_err(|e| TransferError::FileIo(format!("failed to receive message: {e}")))?;

        match decode_message(&msg_bytes)? {
            DecodedMessage::Chunk { index, data } => {
                // Verify sequential ordering
                if index != chunks_received {
                    return Err(TransferError::ChunkIntegrityFailure { chunk_index: index });
                }
                received_data.extend_from_slice(&data);
                chunks_received += 1;
            }
            DecodedMessage::End => break,
            DecodedMessage::Header(_) => {
                return Err(TransferError::InvalidManifest(
                    "unexpected header message during transfer".to_string(),
                ));
            }
        }
    }

    // Verify chunk count
    if chunks_received != header.chunk_count {
        return Err(TransferError::InvalidManifest(format!(
            "chunk count mismatch: expected {}, received {chunks_received}",
            header.chunk_count
        )));
    }

    // Verify size
    if received_data.len() as u64 != header.size {
        return Err(TransferError::InvalidManifest(format!(
            "size mismatch: expected {} bytes, received {}",
            header.size,
            received_data.len()
        )));
    }

    // Verify SHA-256 integrity
    let actual_sha256 = sha256_hash(&received_data);
    if actual_sha256 != header.sha256 {
        return Err(TransferError::InvalidManifest(
            "SHA-256 integrity verification failed: hash mismatch".to_string(),
        ));
    }

    Ok(ReceiveResult {
        filename: header.filename,
        size: received_data.len() as u64,
        sha256: actual_sha256,
        data: received_data,
        chunks_received,
    })
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

#[cfg(test)]
mod tests {
    use super::*;
    use crate::crypto::{HandshakeRole, handshake};
    use tokio::io::duplex;

    // -----------------------------------------------------------------------
    // Utility: SHA-256
    // -----------------------------------------------------------------------

    #[test]
    fn sha256_empty_input() {
        let hash = sha256_hash(&[]);
        // Known SHA-256 of empty input
        let expected: [u8; 32] = [
            0xe3, 0xb0, 0xc4, 0x42, 0x98, 0xfc, 0x1c, 0x14, 0x9a, 0xfb, 0xf4, 0xc8, 0x99, 0x6f,
            0xb9, 0x24, 0x27, 0xae, 0x41, 0xe4, 0x64, 0x9b, 0x93, 0x4c, 0xa4, 0x95, 0x99, 0x1b,
            0x78, 0x52, 0xb8, 0x55,
        ];
        assert_eq!(hash, expected);
    }

    #[test]
    fn sha256_known_value() {
        let hash = sha256_hash(b"hello");
        // SHA-256("hello") is well-known
        let hex: String = hash.iter().map(|b| format!("{b:02x}")).collect();
        assert_eq!(
            hex,
            "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"
        );
    }

    #[test]
    fn sha256_deterministic() {
        let h1 = sha256_hash(b"test data");
        let h2 = sha256_hash(b"test data");
        assert_eq!(h1, h2);
    }

    #[test]
    fn sha256_different_input_different_hash() {
        let h1 = sha256_hash(b"hello");
        let h2 = sha256_hash(b"world");
        assert_ne!(h1, h2);
    }

    // -----------------------------------------------------------------------
    // Utility: chunk_count
    // -----------------------------------------------------------------------

    #[test]
    fn chunk_count_empty() {
        assert_eq!(chunk_count(0, DEFAULT_CHUNK_SIZE), 0);
    }

    #[test]
    fn chunk_count_smaller_than_chunk_size() {
        assert_eq!(chunk_count(100, DEFAULT_CHUNK_SIZE), 1);
    }

    #[test]
    fn chunk_count_exact_fit() {
        let cs = DEFAULT_CHUNK_SIZE;
        assert_eq!(chunk_count(cs as u64, cs), 1);
        assert_eq!(chunk_count(cs as u64 * 3, cs), 3);
    }

    #[test]
    fn chunk_count_with_remainder() {
        let cs = DEFAULT_CHUNK_SIZE;
        assert_eq!(chunk_count(cs as u64 + 1, cs), 2);
        assert_eq!(chunk_count(cs as u64 * 2 + 100, cs), 3);
    }

    #[test]
    fn chunk_count_one_byte() {
        assert_eq!(chunk_count(1, DEFAULT_CHUNK_SIZE), 1);
    }

    // -----------------------------------------------------------------------
    // Utility: chunk_data
    // -----------------------------------------------------------------------

    #[test]
    fn chunk_data_empty() {
        let chunks = chunk_data(&[], 1024);
        assert!(chunks.is_empty());
    }

    #[test]
    fn chunk_data_smaller_than_chunk_size() {
        let data = vec![1, 2, 3, 4, 5];
        let chunks = chunk_data(&data, 1024);
        assert_eq!(chunks.len(), 1);
        assert_eq!(chunks[0], &[1, 2, 3, 4, 5]);
    }

    #[test]
    fn chunk_data_exact_fit() {
        let data = vec![0u8; 1024];
        let chunks = chunk_data(&data, 512);
        assert_eq!(chunks.len(), 2);
        assert_eq!(chunks[0].len(), 512);
        assert_eq!(chunks[1].len(), 512);
    }

    #[test]
    fn chunk_data_with_remainder() {
        let data = vec![0u8; 1000];
        let chunks = chunk_data(&data, 300);
        assert_eq!(chunks.len(), 4); // 300 + 300 + 300 + 100
        assert_eq!(chunks[0].len(), 300);
        assert_eq!(chunks[1].len(), 300);
        assert_eq!(chunks[2].len(), 300);
        assert_eq!(chunks[3].len(), 100);
    }

    #[test]
    fn chunk_data_preserves_content() {
        let data: Vec<u8> = (0..=255).collect();
        let chunks = chunk_data(&data, 100);
        let reassembled: Vec<u8> = chunks.into_iter().flatten().copied().collect();
        assert_eq!(data, reassembled);
    }

    // -----------------------------------------------------------------------
    // Encoding/decoding: header
    // -----------------------------------------------------------------------

    #[test]
    fn header_encode_decode_roundtrip() {
        let header = FileHeader {
            filename: "report.pdf".to_string(),
            size: 1_048_576,
            sha256: sha256_hash(b"test"),
            chunk_size: DEFAULT_CHUNK_SIZE,
            chunk_count: 4,
        };
        let encoded = encode_header(&header);
        let decoded = decode_message(&encoded).unwrap();
        assert_eq!(decoded, DecodedMessage::Header(header));
    }

    #[test]
    fn header_encode_decode_empty_file() {
        let header = FileHeader {
            filename: "empty.txt".to_string(),
            size: 0,
            sha256: sha256_hash(&[]),
            chunk_size: DEFAULT_CHUNK_SIZE,
            chunk_count: 0,
        };
        let encoded = encode_header(&header);
        let decoded = decode_message(&encoded).unwrap();
        assert_eq!(decoded, DecodedMessage::Header(header));
    }

    #[test]
    fn header_encode_decode_unicode_filename() {
        let header = FileHeader {
            filename: "日本語ファイル.txt".to_string(),
            size: 42,
            sha256: [0xAB; 32],
            chunk_size: DEFAULT_CHUNK_SIZE,
            chunk_count: 1,
        };
        let encoded = encode_header(&header);
        let decoded = decode_message(&encoded).unwrap();
        assert_eq!(decoded, DecodedMessage::Header(header));
    }

    // -----------------------------------------------------------------------
    // Encoding/decoding: chunk
    // -----------------------------------------------------------------------

    #[test]
    fn chunk_encode_decode_roundtrip() {
        let data = vec![0xDE, 0xAD, 0xBE, 0xEF, 0x01, 0x02, 0x03];
        let encoded = encode_chunk(42, &data);
        let decoded = decode_message(&encoded).unwrap();
        assert_eq!(
            decoded,
            DecodedMessage::Chunk {
                index: 42,
                data: data.clone()
            }
        );
    }

    #[test]
    fn chunk_encode_decode_empty_data() {
        let encoded = encode_chunk(0, &[]);
        let decoded = decode_message(&encoded).unwrap();
        assert_eq!(
            decoded,
            DecodedMessage::Chunk {
                index: 0,
                data: vec![]
            }
        );
    }

    #[test]
    fn chunk_encode_decode_large_index() {
        let encoded = encode_chunk(u64::MAX, &[1, 2, 3]);
        let decoded = decode_message(&encoded).unwrap();
        assert_eq!(
            decoded,
            DecodedMessage::Chunk {
                index: u64::MAX,
                data: vec![1, 2, 3]
            }
        );
    }

    // -----------------------------------------------------------------------
    // Encoding/decoding: end
    // -----------------------------------------------------------------------

    #[test]
    fn end_encode_decode_roundtrip() {
        let encoded = encode_end();
        assert_eq!(encoded, vec![MSG_END]);
        let decoded = decode_message(&encoded).unwrap();
        assert_eq!(decoded, DecodedMessage::End);
    }

    // -----------------------------------------------------------------------
    // Decoding: error cases
    // -----------------------------------------------------------------------

    #[test]
    fn decode_rejects_empty_buffer() {
        let result = decode_message(&[]);
        assert!(result.is_err());
    }

    #[test]
    fn decode_rejects_unknown_tag() {
        let result = decode_message(&[0xFF]);
        assert!(result.is_err());
        let msg = result.unwrap_err().to_string();
        assert!(msg.contains("unknown message tag"));
    }

    #[test]
    fn decode_rejects_truncated_header() {
        // Tag + partial data — not enough for a complete header
        let mut buf = vec![MSG_HEADER];
        buf.extend_from_slice(&[0u8; 10]); // way too short
        let result = decode_message(&buf);
        assert!(result.is_err());
    }

    #[test]
    fn decode_rejects_truncated_chunk() {
        // Tag only, no index
        let result = decode_message(&[MSG_CHUNK, 0, 0]);
        assert!(result.is_err());
    }

    // -----------------------------------------------------------------------
    // Filename validation
    // -----------------------------------------------------------------------

    #[test]
    fn validate_filename_accepts_normal() {
        assert!(validate_filename("report.pdf").is_ok());
        assert!(validate_filename("my file (1).txt").is_ok());
        assert!(validate_filename("日本語.txt").is_ok());
        assert!(validate_filename("a").is_ok());
    }

    #[test]
    fn validate_filename_rejects_empty() {
        assert!(validate_filename("").is_err());
    }

    #[test]
    fn validate_filename_rejects_forward_slash() {
        assert!(validate_filename("path/file.txt").is_err());
    }

    #[test]
    fn validate_filename_rejects_backslash() {
        assert!(validate_filename("path\\file.txt").is_err());
    }

    #[test]
    fn validate_filename_rejects_null_byte() {
        assert!(validate_filename("file\0.txt").is_err());
    }

    #[test]
    fn validate_filename_rejects_dot() {
        assert!(validate_filename(".").is_err());
        assert!(validate_filename("..").is_err());
    }

    #[test]
    fn validate_filename_rejects_too_long() {
        let long_name = "a".repeat(MAX_FILENAME_LEN + 1);
        assert!(validate_filename(&long_name).is_err());
    }

    // -----------------------------------------------------------------------
    // Integration: handshake helper
    // -----------------------------------------------------------------------

    /// Perform a Noise handshake between two in-process peers, returning
    /// a pair of SecureChannels.
    async fn handshake_pair(code: &str) -> (SecureChannel, SecureChannel) {
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

        (init_result.unwrap(), resp_result.unwrap())
    }

    /// Helper: perform a send-receive round-trip and return the result.
    ///
    /// Creates a fresh transport stream, runs sender and receiver concurrently,
    /// and returns both results.
    async fn round_trip(
        sender_ch: &mut SecureChannel,
        receiver_ch: &mut SecureChannel,
        filename: &str,
        data: &[u8],
    ) -> (SendResult, ReceiveResult) {
        let fname = filename.to_string();
        let send_data_copy = data.to_vec();

        let (tx_stream, rx_stream) = duplex(4 * 1024 * 1024);
        let (_, mut tx_writer) = tokio::io::split(tx_stream);
        let (mut rx_reader, _) = tokio::io::split(rx_stream);

        let (sr, rr) = tokio::join!(
            send_data(sender_ch, &mut tx_writer, &fname, &send_data_copy),
            receive_data(receiver_ch, &mut rx_reader),
        );

        (sr.unwrap(), rr.unwrap())
    }

    // -----------------------------------------------------------------------
    // Integration: encrypted transfer round-trips
    // -----------------------------------------------------------------------

    #[tokio::test]
    async fn transfer_empty_file() {
        let (mut sender_ch, mut receiver_ch) = handshake_pair("transfer-empty").await;
        let (send_res, recv_res) =
            round_trip(&mut sender_ch, &mut receiver_ch, "empty.txt", &[]).await;

        assert_eq!(send_res.filename, "empty.txt");
        assert_eq!(send_res.size, 0);
        assert_eq!(send_res.chunks_sent, 0);

        assert_eq!(recv_res.filename, "empty.txt");
        assert_eq!(recv_res.size, 0);
        assert_eq!(recv_res.chunks_received, 0);
        assert!(recv_res.data.is_empty());
        assert_eq!(send_res.sha256, recv_res.sha256);
    }

    #[tokio::test]
    async fn transfer_small_file() {
        let (mut sender_ch, mut receiver_ch) = handshake_pair("transfer-small").await;
        let data = b"hello, encrypted world!";
        let (send_res, recv_res) =
            round_trip(&mut sender_ch, &mut receiver_ch, "hello.txt", data).await;

        assert_eq!(send_res.filename, "hello.txt");
        assert_eq!(send_res.size, data.len() as u64);
        assert_eq!(send_res.chunks_sent, 1);

        assert_eq!(recv_res.filename, "hello.txt");
        assert_eq!(recv_res.data, data);
        assert_eq!(recv_res.chunks_received, 1);
        assert_eq!(send_res.sha256, recv_res.sha256);
    }

    #[tokio::test]
    async fn transfer_exactly_one_chunk() {
        let (mut sender_ch, mut receiver_ch) = handshake_pair("transfer-1chunk").await;
        let data = vec![0xAB; DEFAULT_CHUNK_SIZE as usize];
        let (send_res, recv_res) =
            round_trip(&mut sender_ch, &mut receiver_ch, "exact.bin", &data).await;

        assert_eq!(send_res.chunks_sent, 1);
        assert_eq!(recv_res.chunks_received, 1);
        assert_eq!(recv_res.data, data);
    }

    #[tokio::test]
    async fn transfer_multi_chunk() {
        let (mut sender_ch, mut receiver_ch) = handshake_pair("transfer-multi").await;
        // 2.5 chunks worth of data
        let data: Vec<u8> = (0..DEFAULT_CHUNK_SIZE as usize * 2 + DEFAULT_CHUNK_SIZE as usize / 2)
            .map(|i| (i % 256) as u8)
            .collect();
        let (send_res, recv_res) =
            round_trip(&mut sender_ch, &mut receiver_ch, "multi.bin", &data).await;

        assert_eq!(send_res.chunks_sent, 3);
        assert_eq!(recv_res.chunks_received, 3);
        assert_eq!(recv_res.data, data);
        assert_eq!(send_res.sha256, recv_res.sha256);
    }

    #[tokio::test]
    async fn transfer_large_file() {
        let (mut sender_ch, mut receiver_ch) = handshake_pair("transfer-large").await;
        // 1 MB file
        let data: Vec<u8> = (0..1_000_000).map(|i| (i % 256) as u8).collect();
        let expected_chunks = chunk_count(data.len() as u64, DEFAULT_CHUNK_SIZE);

        let (send_res, recv_res) =
            round_trip(&mut sender_ch, &mut receiver_ch, "large.bin", &data).await;

        assert_eq!(send_res.size, 1_000_000);
        assert_eq!(send_res.chunks_sent, expected_chunks);
        assert_eq!(recv_res.data.len(), 1_000_000);
        assert_eq!(recv_res.data, data);
        assert_eq!(send_res.sha256, recv_res.sha256);
    }

    #[tokio::test]
    async fn transfer_preserves_binary_data() {
        let (mut sender_ch, mut receiver_ch) = handshake_pair("transfer-binary").await;
        // Every possible byte value
        let data: Vec<u8> = (0..=255).collect();
        let (_, recv_res) = round_trip(&mut sender_ch, &mut receiver_ch, "bytes.bin", &data).await;
        assert_eq!(recv_res.data, data);
    }

    #[tokio::test]
    async fn transfer_preserves_filename() {
        let (mut sender_ch, mut receiver_ch) = handshake_pair("transfer-fname").await;
        let filename = "My Report (Final v2).pdf";
        let (send_res, recv_res) =
            round_trip(&mut sender_ch, &mut receiver_ch, filename, b"content").await;
        assert_eq!(send_res.filename, filename);
        assert_eq!(recv_res.filename, filename);
    }

    #[tokio::test]
    async fn transfer_sha256_matches() {
        let (mut sender_ch, mut receiver_ch) = handshake_pair("transfer-sha").await;
        let data = b"integrity check data";
        let expected_hash = sha256_hash(data);

        let (send_res, recv_res) =
            round_trip(&mut sender_ch, &mut receiver_ch, "check.txt", data).await;

        assert_eq!(send_res.sha256, expected_hash);
        assert_eq!(recv_res.sha256, expected_hash);
    }

    #[tokio::test]
    async fn send_rejects_invalid_filename() {
        let (mut sender_ch, _receiver_ch) = handshake_pair("transfer-badname").await;
        let (tx, _rx) = duplex(65536);
        let (_, mut writer) = tokio::io::split(tx);

        let result = send_data(&mut sender_ch, &mut writer, "", b"data").await;
        assert!(result.is_err());
        assert!(result.unwrap_err().to_string().contains("empty"));

        // Need fresh channels since the previous send may have partially consumed nonces
        let (mut sender_ch2, _) = handshake_pair("transfer-badname2").await;
        let (tx2, _rx2) = duplex(65536);
        let (_, mut writer2) = tokio::io::split(tx2);

        let result2 = send_data(&mut sender_ch2, &mut writer2, "path/file.txt", b"data").await;
        assert!(result2.is_err());
        assert!(result2.unwrap_err().to_string().contains("path separator"));
    }

    // -----------------------------------------------------------------------
    // Decoding: additional error cases
    // -----------------------------------------------------------------------

    #[test]
    fn decode_rejects_header_with_excessive_chunk_count() {
        // Build a header with chunk_count > MAX_CHUNK_COUNT
        let header = FileHeader {
            filename: "test.bin".to_string(),
            size: u64::MAX,
            sha256: [0u8; 32],
            chunk_size: 1,
            chunk_count: MAX_CHUNK_COUNT + 1,
        };
        let encoded = encode_header(&header);
        let result = decode_message(&encoded);
        assert!(result.is_err());
        assert!(
            result
                .unwrap_err()
                .to_string()
                .contains("chunk count too large")
        );
    }

    #[test]
    fn decode_rejects_header_with_filename_truncated() {
        // Build a header that claims a filename length longer than what follows
        let mut buf = vec![MSG_HEADER];
        buf.extend_from_slice(&100u64.to_be_bytes()); // size
        buf.extend_from_slice(&[0u8; 32]); // sha256
        buf.extend_from_slice(&256u32.to_be_bytes()); // chunk_size
        buf.extend_from_slice(&1u64.to_be_bytes()); // chunk_count
        buf.extend_from_slice(&50u16.to_be_bytes()); // name_len = 50
        buf.extend_from_slice(b"short"); // only 5 bytes of name
        let result = decode_message(&buf);
        assert!(result.is_err());
        assert!(result.unwrap_err().to_string().contains("truncated"));
    }

    #[test]
    fn decode_rejects_header_with_invalid_utf8_filename() {
        let mut buf = vec![MSG_HEADER];
        buf.extend_from_slice(&100u64.to_be_bytes());
        buf.extend_from_slice(&[0u8; 32]);
        buf.extend_from_slice(&256u32.to_be_bytes());
        buf.extend_from_slice(&1u64.to_be_bytes());
        let bad_utf8 = &[0xFF, 0xFE, 0xFD];
        buf.extend_from_slice(&(bad_utf8.len() as u16).to_be_bytes());
        buf.extend_from_slice(bad_utf8);
        let result = decode_message(&buf);
        assert!(result.is_err());
        assert!(result.unwrap_err().to_string().contains("UTF-8"));
    }

    #[test]
    fn decode_rejects_header_with_filename_too_long() {
        let mut buf = vec![MSG_HEADER];
        buf.extend_from_slice(&100u64.to_be_bytes());
        buf.extend_from_slice(&[0u8; 32]);
        buf.extend_from_slice(&256u32.to_be_bytes());
        buf.extend_from_slice(&1u64.to_be_bytes());
        // name_len > MAX_FILENAME_LEN (4096)
        buf.extend_from_slice(&(MAX_FILENAME_LEN as u16 + 1).to_be_bytes());
        // Don't need to add actual bytes — the length check fires first
        let result = decode_message(&buf);
        assert!(result.is_err());
        assert!(
            result
                .unwrap_err()
                .to_string()
                .contains("filename too long")
        );
    }

    // -----------------------------------------------------------------------
    // chunk_count: edge cases
    // -----------------------------------------------------------------------

    #[test]
    fn chunk_count_zero_chunk_size_with_zero_data() {
        // Zero data with zero chunk_size is fine — short-circuits
        assert_eq!(chunk_count(0, 0), 0);
    }

    #[test]
    #[should_panic(expected = "chunk_size must be non-zero")]
    fn chunk_count_zero_chunk_size_with_data_panics() {
        chunk_count(100, 0);
    }

    // -----------------------------------------------------------------------
    // Integration: receiver error paths
    // -----------------------------------------------------------------------

    #[tokio::test]
    async fn receive_rejects_out_of_order_chunks() {
        let (mut sender_ch, mut receiver_ch) = handshake_pair("ooo-chunks").await;

        let (tx_stream, rx_stream) = duplex(1024 * 1024);
        let (_, mut tx_writer) = tokio::io::split(tx_stream);
        let (mut rx_reader, _) = tokio::io::split(rx_stream);

        let data = vec![0u8; DEFAULT_CHUNK_SIZE as usize * 2];
        let sha256 = sha256_hash(&data);

        let send_task = async {
            // Send header
            let header = FileHeader {
                filename: "test.bin".to_string(),
                size: data.len() as u64,
                sha256,
                chunk_size: DEFAULT_CHUNK_SIZE,
                chunk_count: 2,
            };
            sender_ch
                .send(&encode_header(&header), &mut tx_writer)
                .await
                .unwrap();
            // Send chunk index 1 first (out of order — should be 0)
            sender_ch
                .send(
                    &encode_chunk(1, &data[..DEFAULT_CHUNK_SIZE as usize]),
                    &mut tx_writer,
                )
                .await
                .unwrap();
        };

        let recv_task = async { receive_data(&mut receiver_ch, &mut rx_reader).await };

        let (_, recv_result) = tokio::join!(send_task, recv_task);
        assert!(recv_result.is_err());
        assert!(
            recv_result
                .unwrap_err()
                .to_string()
                .contains("integrity check")
        );
    }

    #[tokio::test]
    async fn receive_rejects_chunk_count_mismatch() {
        let (mut sender_ch, mut receiver_ch) = handshake_pair("cc-mismatch").await;

        let (tx_stream, rx_stream) = duplex(1024 * 1024);
        let (_, mut tx_writer) = tokio::io::split(tx_stream);
        let (mut rx_reader, _) = tokio::io::split(rx_stream);

        let data = b"hello";
        let sha256 = sha256_hash(data);

        let send_task = async {
            // Header claims 2 chunks but we only send 1 + End
            let header = FileHeader {
                filename: "test.bin".to_string(),
                size: data.len() as u64,
                sha256,
                chunk_size: DEFAULT_CHUNK_SIZE,
                chunk_count: 2,
            };
            sender_ch
                .send(&encode_header(&header), &mut tx_writer)
                .await
                .unwrap();
            sender_ch
                .send(&encode_chunk(0, data), &mut tx_writer)
                .await
                .unwrap();
            sender_ch.send(&encode_end(), &mut tx_writer).await.unwrap();
        };

        let recv_task = async { receive_data(&mut receiver_ch, &mut rx_reader).await };

        let (_, recv_result) = tokio::join!(send_task, recv_task);
        assert!(recv_result.is_err());
        assert!(
            recv_result
                .unwrap_err()
                .to_string()
                .contains("chunk count mismatch")
        );
    }

    #[tokio::test]
    async fn receive_rejects_size_mismatch() {
        let (mut sender_ch, mut receiver_ch) = handshake_pair("sz-mismatch").await;

        let (tx_stream, rx_stream) = duplex(1024 * 1024);
        let (_, mut tx_writer) = tokio::io::split(tx_stream);
        let (mut rx_reader, _) = tokio::io::split(rx_stream);

        let data = b"hello world";
        let sha256 = sha256_hash(data);

        let send_task = async {
            // Header claims size is 5 but we send 11 bytes
            let header = FileHeader {
                filename: "test.bin".to_string(),
                size: 5, // wrong
                sha256,
                chunk_size: DEFAULT_CHUNK_SIZE,
                chunk_count: 1,
            };
            sender_ch
                .send(&encode_header(&header), &mut tx_writer)
                .await
                .unwrap();
            sender_ch
                .send(&encode_chunk(0, data), &mut tx_writer)
                .await
                .unwrap();
            sender_ch.send(&encode_end(), &mut tx_writer).await.unwrap();
        };

        let recv_task = async { receive_data(&mut receiver_ch, &mut rx_reader).await };

        let (_, recv_result) = tokio::join!(send_task, recv_task);
        assert!(recv_result.is_err());
        assert!(
            recv_result
                .unwrap_err()
                .to_string()
                .contains("size mismatch")
        );
    }

    #[tokio::test]
    async fn receive_rejects_sha256_mismatch() {
        let (mut sender_ch, mut receiver_ch) = handshake_pair("sha-mismatch").await;

        let (tx_stream, rx_stream) = duplex(1024 * 1024);
        let (_, mut tx_writer) = tokio::io::split(tx_stream);
        let (mut rx_reader, _) = tokio::io::split(rx_stream);

        let data = b"hello";

        let send_task = async {
            // Header has a bogus SHA-256
            let header = FileHeader {
                filename: "test.bin".to_string(),
                size: data.len() as u64,
                sha256: [0xAA; 32], // wrong hash
                chunk_size: DEFAULT_CHUNK_SIZE,
                chunk_count: 1,
            };
            sender_ch
                .send(&encode_header(&header), &mut tx_writer)
                .await
                .unwrap();
            sender_ch
                .send(&encode_chunk(0, data), &mut tx_writer)
                .await
                .unwrap();
            sender_ch.send(&encode_end(), &mut tx_writer).await.unwrap();
        };

        let recv_task = async { receive_data(&mut receiver_ch, &mut rx_reader).await };

        let (_, recv_result) = tokio::join!(send_task, recv_task);
        assert!(recv_result.is_err());
        assert!(
            recv_result
                .unwrap_err()
                .to_string()
                .contains("SHA-256 integrity verification failed")
        );
    }

    #[tokio::test]
    async fn receive_rejects_unexpected_header_mid_transfer() {
        let (mut sender_ch, mut receiver_ch) = handshake_pair("dup-header").await;

        let (tx_stream, rx_stream) = duplex(1024 * 1024);
        let (_, mut tx_writer) = tokio::io::split(tx_stream);
        let (mut rx_reader, _) = tokio::io::split(rx_stream);

        let data = b"hello";
        let sha256 = sha256_hash(data);

        let send_task = async {
            let header = FileHeader {
                filename: "test.bin".to_string(),
                size: data.len() as u64,
                sha256,
                chunk_size: DEFAULT_CHUNK_SIZE,
                chunk_count: 1,
            };
            // Send header
            sender_ch
                .send(&encode_header(&header), &mut tx_writer)
                .await
                .unwrap();
            // Send another header instead of a chunk
            sender_ch
                .send(&encode_header(&header), &mut tx_writer)
                .await
                .unwrap();
        };

        let recv_task = async { receive_data(&mut receiver_ch, &mut rx_reader).await };

        let (_, recv_result) = tokio::join!(send_task, recv_task);
        assert!(recv_result.is_err());
        assert!(
            recv_result
                .unwrap_err()
                .to_string()
                .contains("unexpected header")
        );
    }

    #[tokio::test]
    async fn receive_rejects_non_header_as_first_message() {
        let (mut sender_ch, mut receiver_ch) = handshake_pair("no-header").await;

        let (tx_stream, rx_stream) = duplex(1024 * 1024);
        let (_, mut tx_writer) = tokio::io::split(tx_stream);
        let (mut rx_reader, _) = tokio::io::split(rx_stream);

        let send_task = async {
            // Send a chunk as the first message (should be header)
            sender_ch
                .send(&encode_chunk(0, b"data"), &mut tx_writer)
                .await
                .unwrap();
        };

        let recv_task = async { receive_data(&mut receiver_ch, &mut rx_reader).await };

        let (_, recv_result) = tokio::join!(send_task, recv_task);
        assert!(recv_result.is_err());
        assert!(
            recv_result
                .unwrap_err()
                .to_string()
                .contains("expected header")
        );
    }

    // -----------------------------------------------------------------------
    // Integration: encrypted transfer round-trips (continued)
    // -----------------------------------------------------------------------

    #[tokio::test]
    async fn multiple_sequential_transfers() {
        let (mut sender_ch, mut receiver_ch) = handshake_pair("transfer-seq").await;

        // First transfer
        let (tx1, rx1) = duplex(1024 * 1024);
        let (_, mut w1) = tokio::io::split(tx1);
        let (mut r1, _) = tokio::io::split(rx1);

        let (sr1, rr1) = tokio::join!(
            send_data(&mut sender_ch, &mut w1, "file1.txt", b"first file"),
            receive_data(&mut receiver_ch, &mut r1),
        );
        let sr1 = sr1.unwrap();
        let rr1 = rr1.unwrap();
        assert_eq!(rr1.data, b"first file");
        assert_eq!(sr1.sha256, rr1.sha256);

        // Second transfer over the same channels (nonces continue incrementing)
        let (tx2, rx2) = duplex(1024 * 1024);
        let (_, mut w2) = tokio::io::split(tx2);
        let (mut r2, _) = tokio::io::split(rx2);

        let (sr2, rr2) = tokio::join!(
            send_data(&mut sender_ch, &mut w2, "file2.txt", b"second file"),
            receive_data(&mut receiver_ch, &mut r2),
        );
        let sr2 = sr2.unwrap();
        let rr2 = rr2.unwrap();
        assert_eq!(rr2.data, b"second file");
        assert_eq!(sr2.sha256, rr2.sha256);
    }
}
