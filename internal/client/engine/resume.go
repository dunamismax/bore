// Resume state management for bore's transfer durability.
//
// When a receiver is interrupted mid-transfer, it persists enough metadata to
// resume on the next connection:
//
//   - Resume state JSON: <outputDir>/.bore/resume-<transferID>.json
//   - Partial data file: <outputDir>/.bore/partial-<transferID>
//
// The transfer ID is a deterministic hash of (filename, size, SHA-256, chunk_size)
// so the same file always produces the same ID regardless of which relay room or
// rendezvous code was used. This means resume works across completely separate
// sessions as long as the file identity matches.
//
// Restart-vs-resume rules:
//
//   - Resume: all metadata fields match AND partial file has the expected byte count.
//   - Restart: any metadata mismatch, or partial file size does not match, or resume
//     state is corrupted. Partial data is discarded and the transfer starts from chunk 0.
//   - Final SHA-256 verification covers the entire reassembled file. If resume produced
//     corrupt data, the hash fails and the partial state is cleaned up.
package engine

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// ResumeState tracks partial progress for a single-file transfer on the
// receiver side. It is persisted as a JSON file alongside the partial data.
type ResumeState struct {
	TransferID     string `json:"transfer_id"`
	Filename       string `json:"filename"`
	Size           uint64 `json:"size"`
	SHA256Hex      string `json:"sha256"`
	ChunkSize      uint32 `json:"chunk_size"`
	ChunkCount     uint64 `json:"chunk_count"`
	ChunksReceived uint64 `json:"chunks_received"`
}

// TransferID computes a deterministic identifier for a transfer based on
// file metadata. Same file with same parameters always produces the same ID.
// Returns a 16-character hex string (8 bytes of the SHA-256).
func TransferID(filename string, size uint64, fileHash [32]byte, chunkSize uint32) string {
	h := sha256.New()
	h.Write([]byte(filename))
	_ = binary.Write(h, binary.BigEndian, size)
	h.Write(fileHash[:])
	_ = binary.Write(h, binary.BigEndian, chunkSize)
	return hex.EncodeToString(h.Sum(nil))[:16]
}

// ---------------------------------------------------------------------------
// Path helpers
// ---------------------------------------------------------------------------

// resumeDir returns the .bore subdirectory for resume state and partial files.
func resumeDir(outputDir string) string {
	return filepath.Join(outputDir, ".bore")
}

// resumeStatePath returns the path to the resume state JSON file.
func resumeStatePath(outputDir, transferID string) string {
	return filepath.Join(resumeDir(outputDir), "resume-"+transferID+".json")
}

// partialFilePath returns the path to the partial data file.
func partialFilePath(outputDir, transferID string) string {
	return filepath.Join(resumeDir(outputDir), "partial-"+transferID)
}

// ---------------------------------------------------------------------------
// Persistence
// ---------------------------------------------------------------------------

// LoadResumeState loads resume state for the given transfer ID.
// Returns (nil, nil) if no resume state file exists.
func LoadResumeState(outputDir, transferID string) (*ResumeState, error) {
	data, err := os.ReadFile(resumeStatePath(outputDir, transferID))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read resume state: %w", err)
	}
	var state ResumeState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("parse resume state: %w", err)
	}
	if state.TransferID != transferID {
		return nil, fmt.Errorf("transfer ID mismatch in resume state file")
	}
	return &state, nil
}

// SaveResumeState persists resume state for later recovery.
func SaveResumeState(outputDir string, state *ResumeState) error {
	dir := resumeDir(outputDir)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create resume dir: %w", err)
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal resume state: %w", err)
	}
	return os.WriteFile(resumeStatePath(outputDir, state.TransferID), data, 0o600)
}

// DeleteResumeState removes resume state and partial files for the transfer.
func DeleteResumeState(outputDir, transferID string) {
	os.Remove(resumeStatePath(outputDir, transferID))
	os.Remove(partialFilePath(outputDir, transferID))
	// Remove .bore directory if empty (best-effort).
	os.Remove(resumeDir(outputDir))
}

// ---------------------------------------------------------------------------
// Validation helpers
// ---------------------------------------------------------------------------

// stateMatchesHeader checks whether the resume state matches the received
// file header. All metadata fields must match for resume to be valid.
func stateMatchesHeader(state *ResumeState, hdr *FileHeader) bool {
	return state.Filename == hdr.Filename &&
		state.Size == hdr.Size &&
		state.SHA256Hex == hex.EncodeToString(hdr.SHA256[:]) &&
		state.ChunkSize == hdr.ChunkSize &&
		state.ChunkCount == hdr.ChunkCount
}

// expectedPartialSize returns the expected byte count for a partial file
// given the number of chunks received. All chunks except possibly the last
// are exactly chunkSize bytes. If all chunks are received, the file size
// equals the declared total size.
func expectedPartialSize(chunksReceived, chunkCount uint64, chunkSize uint32, totalSize uint64) int64 {
	if chunksReceived == 0 {
		return 0
	}
	if chunksReceived >= chunkCount {
		return int64(totalSize)
	}
	return int64(chunksReceived) * int64(chunkSize)
}
