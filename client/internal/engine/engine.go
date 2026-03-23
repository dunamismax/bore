// Package engine implements the bore file transfer protocol over a SecureChannel.
//
// The engine is transport-agnostic: it operates over any SecureChannel backed
// by an io.Reader/io.Writer. Files flow as a sequence of encrypted messages:
//
//  1. Header -- file metadata (name, size, SHA-256, chunk parameters)
//  2. Chunks -- sequential fixed-size blocks of file data
//  3. End -- transfer completion signal
//
// The receiver verifies the SHA-256 hash of reassembled data against the hash
// declared in the header. Any mismatch is a hard error.
//
// Wire format (each message sent as one SecureChannel message):
//
//	Header (0x01):
//	  [1: tag=0x01][8: size BE][32: sha256][4: chunk_size BE][8: chunk_count BE]
//	  [2: name_len BE][name_len: filename UTF-8]
//
//	Chunk (0x02):
//	  [1: tag=0x02][8: index BE][variable: data]
//
//	End (0x03):
//	  [1: tag=0x03]
package engine

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io"
	"strings"

	"github.com/dunamismax/bore/client/internal/crypto"
)

// Wire format type tags.
const (
	msgHeader byte = 0x01
	msgChunk  byte = 0x02
	msgEnd    byte = 0x03
)

// headerFixedLen is the byte count of the fixed portion of a header message:
// tag(1) + size(8) + sha256(32) + chunk_size(4) + chunk_count(8) + name_len(2) = 55
const headerFixedLen = 1 + 8 + 32 + 4 + 8 + 2

const (
	maxFilenameLen = 4096             // maximum filename byte length
	maxChunkCount  = 16 * 1024 * 1024 // ~4 TB at 256 KiB/chunk
	// DefaultChunkSize matches bore-core DEFAULT_CHUNK_SIZE = 256 KiB.
	DefaultChunkSize = 256 * 1024
)

// FileHeader is the metadata sent before file data.
type FileHeader struct {
	Filename   string
	Size       uint64
	SHA256     [32]byte
	ChunkSize  uint32
	ChunkCount uint64
}

// SendResult is returned after a successful send operation.
type SendResult struct {
	Filename   string
	Size       uint64
	SHA256     [32]byte
	ChunksSent uint64
}

// ReceiveResult is returned after a successful receive operation.
type ReceiveResult struct {
	Filename       string
	Size           uint64
	SHA256         [32]byte
	Data           []byte
	ChunksReceived uint64
}

// ---------------------------------------------------------------------------
// Validation
// ---------------------------------------------------------------------------

// validateFilename rejects filenames that could cause path traversal or other issues.
func validateFilename(name string) error {
	if name == "" {
		return fmt.Errorf("filename is empty")
	}
	if len(name) > maxFilenameLen {
		return fmt.Errorf("filename too long: %d bytes (max %d)", len(name), maxFilenameLen)
	}
	if strings.ContainsRune(name, 0) {
		return fmt.Errorf("filename contains null byte")
	}
	if strings.ContainsRune(name, '/') || strings.ContainsRune(name, '\\') {
		return fmt.Errorf("filename contains path separator")
	}
	if name == "." || name == ".." {
		return fmt.Errorf("filename is a relative path component")
	}
	return nil
}

// ---------------------------------------------------------------------------
// Encoding
// ---------------------------------------------------------------------------

func encodeHeader(h FileHeader) []byte {
	nameBytes := []byte(h.Filename)
	buf := make([]byte, 0, headerFixedLen+len(nameBytes))
	buf = append(buf, msgHeader)
	buf = binary.BigEndian.AppendUint64(buf, h.Size)
	buf = append(buf, h.SHA256[:]...)
	buf = binary.BigEndian.AppendUint32(buf, h.ChunkSize)
	buf = binary.BigEndian.AppendUint64(buf, h.ChunkCount)
	buf = binary.BigEndian.AppendUint16(buf, uint16(len(nameBytes)))
	buf = append(buf, nameBytes...)
	return buf
}

func encodeChunk(index uint64, data []byte) []byte {
	buf := make([]byte, 0, 1+8+len(data))
	buf = append(buf, msgChunk)
	buf = binary.BigEndian.AppendUint64(buf, index)
	buf = append(buf, data...)
	return buf
}

func encodeEnd() []byte {
	return []byte{msgEnd}
}

// ---------------------------------------------------------------------------
// Decoding
// ---------------------------------------------------------------------------

type decodedMessage struct {
	header *FileHeader
	chunk  *chunkMsg
	isEnd  bool
}

type chunkMsg struct {
	index uint64
	data  []byte
}

func decodeMessage(buf []byte) (decodedMessage, error) {
	if len(buf) == 0 {
		return decodedMessage{}, fmt.Errorf("empty message")
	}
	switch buf[0] {
	case msgHeader:
		h, err := decodeHeader(buf)
		if err != nil {
			return decodedMessage{}, err
		}
		return decodedMessage{header: &h}, nil
	case msgChunk:
		c, err := decodeChunk(buf)
		if err != nil {
			return decodedMessage{}, err
		}
		return decodedMessage{chunk: &c}, nil
	case msgEnd:
		return decodedMessage{isEnd: true}, nil
	default:
		return decodedMessage{}, fmt.Errorf("unknown message tag: 0x%02x", buf[0])
	}
}

func decodeHeader(buf []byte) (FileHeader, error) {
	if len(buf) < headerFixedLen {
		return FileHeader{}, fmt.Errorf("header too short: %d bytes (need %d)", len(buf), headerFixedLen)
	}

	pos := 1 // skip tag
	size := binary.BigEndian.Uint64(buf[pos : pos+8])
	pos += 8

	var hash [32]byte
	copy(hash[:], buf[pos:pos+32])
	pos += 32

	chunkSize := binary.BigEndian.Uint32(buf[pos : pos+4])
	pos += 4

	chunkCount := binary.BigEndian.Uint64(buf[pos : pos+8])
	pos += 8

	if chunkCount > maxChunkCount {
		return FileHeader{}, fmt.Errorf("chunk count too large: %d (max %d)", chunkCount, maxChunkCount)
	}

	nameLen := int(binary.BigEndian.Uint16(buf[pos : pos+2]))
	pos += 2

	if nameLen > maxFilenameLen {
		return FileHeader{}, fmt.Errorf("filename too long: %d bytes (max %d)", nameLen, maxFilenameLen)
	}
	if len(buf) < pos+nameLen {
		return FileHeader{}, fmt.Errorf("header truncated: need %d bytes for filename, have %d",
			nameLen, len(buf)-pos)
	}

	name := string(buf[pos : pos+nameLen])

	return FileHeader{
		Filename:   name,
		Size:       size,
		SHA256:     hash,
		ChunkSize:  chunkSize,
		ChunkCount: chunkCount,
	}, nil
}

func decodeChunk(buf []byte) (chunkMsg, error) {
	// tag(1) + index(8) + data(variable, may be 0)
	if len(buf) < 9 {
		return chunkMsg{}, fmt.Errorf("chunk too short: %d bytes (need at least 9)", len(buf))
	}
	index := binary.BigEndian.Uint64(buf[1:9])
	data := make([]byte, len(buf)-9)
	copy(data, buf[9:])
	return chunkMsg{index: index, data: data}, nil
}

// ---------------------------------------------------------------------------
// Transfer functions
// ---------------------------------------------------------------------------

// SendData sends filename and data over the SecureChannel.
// It computes the SHA-256 hash, builds the header, and streams chunks.
func SendData(ch *crypto.SecureChannel, w io.Writer, filename string, data []byte) (SendResult, error) {
	if err := validateFilename(filename); err != nil {
		return SendResult{}, fmt.Errorf("invalid filename: %w", err)
	}

	hash := sha256.Sum256(data)
	size := uint64(len(data))

	chunkSize := uint32(DefaultChunkSize)
	var chunkCount uint64
	if size == 0 {
		chunkCount = 0
	} else {
		chunkCount = (size + uint64(chunkSize) - 1) / uint64(chunkSize)
	}

	hdr := FileHeader{
		Filename:   filename,
		Size:       size,
		SHA256:     hash,
		ChunkSize:  chunkSize,
		ChunkCount: chunkCount,
	}

	// Send header.
	if err := ch.Send(w, encodeHeader(hdr)); err != nil {
		return SendResult{}, fmt.Errorf("send header: %w", err)
	}

	// Send chunks.
	var chunksSent uint64
	remaining := data
	for chunksSent < chunkCount {
		n := len(remaining)
		if n > int(chunkSize) {
			n = int(chunkSize)
		}
		if err := ch.Send(w, encodeChunk(chunksSent, remaining[:n])); err != nil {
			return SendResult{}, fmt.Errorf("send chunk %d: %w", chunksSent, err)
		}
		remaining = remaining[n:]
		chunksSent++
	}

	// Send end.
	if err := ch.Send(w, encodeEnd()); err != nil {
		return SendResult{}, fmt.Errorf("send end: %w", err)
	}

	return SendResult{
		Filename:   filename,
		Size:       size,
		SHA256:     hash,
		ChunksSent: chunksSent,
	}, nil
}

// ReceiveData receives a file from the SecureChannel.
// It validates the header, reassembles chunks, and verifies SHA-256 integrity.
func ReceiveData(ch *crypto.SecureChannel, r io.Reader) (ReceiveResult, error) {
	// Receive and decode header.
	hdrBytes, err := ch.Recv(r)
	if err != nil {
		return ReceiveResult{}, fmt.Errorf("recv header: %w", err)
	}
	msg, err := decodeMessage(hdrBytes)
	if err != nil {
		return ReceiveResult{}, fmt.Errorf("decode header: %w", err)
	}
	if msg.header == nil {
		return ReceiveResult{}, fmt.Errorf("expected header message, got other")
	}
	hdr := *msg.header

	if err := validateFilename(hdr.Filename); err != nil {
		return ReceiveResult{}, fmt.Errorf("invalid filename from sender: %w", err)
	}

	// Receive chunks.
	assembled := make([]byte, 0, hdr.Size)
	for i := uint64(0); i < hdr.ChunkCount; i++ {
		chunkBytes, err := ch.Recv(r)
		if err != nil {
			return ReceiveResult{}, fmt.Errorf("recv chunk %d: %w", i, err)
		}
		cmsg, err := decodeMessage(chunkBytes)
		if err != nil {
			return ReceiveResult{}, fmt.Errorf("decode chunk %d: %w", i, err)
		}
		if cmsg.chunk == nil {
			return ReceiveResult{}, fmt.Errorf("expected chunk message at index %d, got other", i)
		}
		if cmsg.chunk.index != i {
			return ReceiveResult{}, fmt.Errorf("chunk index mismatch: got %d, want %d",
				cmsg.chunk.index, i)
		}
		assembled = append(assembled, cmsg.chunk.data...)
	}

	// Receive end signal.
	endBytes, err := ch.Recv(r)
	if err != nil {
		return ReceiveResult{}, fmt.Errorf("recv end: %w", err)
	}
	endMsg, err := decodeMessage(endBytes)
	if err != nil {
		return ReceiveResult{}, fmt.Errorf("decode end: %w", err)
	}
	if !endMsg.isEnd {
		return ReceiveResult{}, fmt.Errorf("expected end message, got other")
	}

	// Verify SHA-256 integrity.
	actualHash := sha256.Sum256(assembled)
	if actualHash != hdr.SHA256 {
		return ReceiveResult{}, fmt.Errorf("SHA-256 integrity check failed: data corrupted in transit")
	}

	return ReceiveResult{
		Filename:       hdr.Filename,
		Size:           hdr.Size,
		SHA256:         hdr.SHA256,
		Data:           assembled,
		ChunksReceived: hdr.ChunkCount,
	}, nil
}
