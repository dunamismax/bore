package engine

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

// ---------------------------------------------------------------------------
// TransferID
// ---------------------------------------------------------------------------

func TestTransferIDDeterministic(t *testing.T) {
	hash := sha256.Sum256([]byte("hello"))
	id1 := TransferID("test.txt", 100, hash, DefaultChunkSize)
	id2 := TransferID("test.txt", 100, hash, DefaultChunkSize)
	if id1 != id2 {
		t.Errorf("TransferID not deterministic: %q != %q", id1, id2)
	}
	if len(id1) != 16 {
		t.Errorf("TransferID length: %d, want 16", len(id1))
	}
}

func TestTransferIDDiffersWithDifferentInputs(t *testing.T) {
	hash := sha256.Sum256([]byte("hello"))
	id1 := TransferID("a.txt", 100, hash, DefaultChunkSize)
	id2 := TransferID("b.txt", 100, hash, DefaultChunkSize)
	if id1 == id2 {
		t.Error("TransferID should differ for different filenames")
	}

	id3 := TransferID("a.txt", 200, hash, DefaultChunkSize)
	if id1 == id3 {
		t.Error("TransferID should differ for different sizes")
	}
}

// ---------------------------------------------------------------------------
// ResumeState persistence
// ---------------------------------------------------------------------------

func TestSaveAndLoadResumeState(t *testing.T) {
	dir := t.TempDir()
	state := &ResumeState{
		TransferID:     "abcdef0123456789",
		Filename:       "test.txt",
		Size:           12345,
		SHA256Hex:      "aaaa",
		ChunkSize:      DefaultChunkSize,
		ChunkCount:     1,
		ChunksReceived: 0,
	}

	if err := SaveResumeState(dir, state); err != nil {
		t.Fatalf("save: %v", err)
	}

	loaded, err := LoadResumeState(dir, "abcdef0123456789")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded == nil {
		t.Fatal("loaded state is nil")
	}
	if loaded.Filename != state.Filename {
		t.Errorf("Filename: %q != %q", loaded.Filename, state.Filename)
	}
	if loaded.ChunksReceived != state.ChunksReceived {
		t.Errorf("ChunksReceived: %d != %d", loaded.ChunksReceived, state.ChunksReceived)
	}
}

func TestLoadResumeStateNonExistent(t *testing.T) {
	dir := t.TempDir()
	state, err := LoadResumeState(dir, "nonexistent1234")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state != nil {
		t.Error("expected nil state for nonexistent file")
	}
}

func TestLoadResumeStateIDMismatch(t *testing.T) {
	dir := t.TempDir()
	state := &ResumeState{
		TransferID: "abcdef0123456789",
		Filename:   "test.txt",
	}
	if err := SaveResumeState(dir, state); err != nil {
		t.Fatalf("save: %v", err)
	}

	// Rename the file to simulate ID mismatch.
	oldPath := resumeStatePath(dir, "abcdef0123456789")
	newPath := resumeStatePath(dir, "different0123456")
	if err := os.MkdirAll(filepath.Dir(newPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(oldPath, newPath); err != nil {
		t.Fatal(err)
	}

	_, err := LoadResumeState(dir, "different0123456")
	if err == nil {
		t.Error("expected error for ID mismatch")
	}
}

func TestDeleteResumeState(t *testing.T) {
	dir := t.TempDir()
	tid := "abcdef0123456789"
	state := &ResumeState{
		TransferID: tid,
		Filename:   "test.txt",
	}
	if err := SaveResumeState(dir, state); err != nil {
		t.Fatalf("save: %v", err)
	}

	// Also write a partial file.
	boreDir := resumeDir(dir)
	partial := partialFilePath(dir, tid)
	if err := os.WriteFile(partial, []byte("partial data"), 0o600); err != nil {
		t.Fatal(err)
	}

	DeleteResumeState(dir, tid)

	if _, err := os.Stat(resumeStatePath(dir, tid)); !os.IsNotExist(err) {
		t.Error("resume state file should be deleted")
	}
	if _, err := os.Stat(partial); !os.IsNotExist(err) {
		t.Error("partial file should be deleted")
	}
	// .bore dir should also be removed if empty.
	if _, err := os.Stat(boreDir); !os.IsNotExist(err) {
		t.Error(".bore directory should be removed when empty")
	}
}

// ---------------------------------------------------------------------------
// stateMatchesHeader
// ---------------------------------------------------------------------------

func TestStateMatchesHeader(t *testing.T) {
	hash := sha256.Sum256([]byte("data"))
	hdr := &FileHeader{
		Filename:   "test.txt",
		Size:       100,
		SHA256:     hash,
		ChunkSize:  DefaultChunkSize,
		ChunkCount: 1,
	}
	state := &ResumeState{
		TransferID:     "test",
		Filename:       "test.txt",
		Size:           100,
		SHA256Hex:      hex.EncodeToString(hash[:]),
		ChunkSize:      DefaultChunkSize,
		ChunkCount:     1,
		ChunksReceived: 0,
	}

	if !stateMatchesHeader(state, hdr) {
		t.Error("matching state should return true")
	}

	// Mismatched filename.
	state2 := *state
	state2.Filename = "other.txt"
	if stateMatchesHeader(&state2, hdr) {
		t.Error("different filename should not match")
	}

	// Mismatched size.
	state3 := *state
	state3.Size = 999
	if stateMatchesHeader(&state3, hdr) {
		t.Error("different size should not match")
	}

	// Mismatched hash.
	state4 := *state
	state4.SHA256Hex = "0000"
	if stateMatchesHeader(&state4, hdr) {
		t.Error("different hash should not match")
	}
}

// ---------------------------------------------------------------------------
// expectedPartialSize
// ---------------------------------------------------------------------------

func TestExpectedPartialSize(t *testing.T) {
	cs := uint32(256 * 1024)

	// No chunks received.
	if s := expectedPartialSize(0, 10, cs, 10*256*1024); s != 0 {
		t.Errorf("0 chunks: got %d, want 0", s)
	}

	// 3 of 10 chunks.
	if s := expectedPartialSize(3, 10, cs, 10*256*1024); s != 3*256*1024 {
		t.Errorf("3 of 10: got %d, want %d", s, 3*256*1024)
	}

	// All chunks received.
	totalSize := uint64(10*256*1024 - 100) // not aligned
	if s := expectedPartialSize(10, 10, cs, totalSize); s != int64(totalSize) {
		t.Errorf("all chunks: got %d, want %d", s, totalSize)
	}
}

// ---------------------------------------------------------------------------
// ResumeOffer encoding/decoding
// ---------------------------------------------------------------------------

func TestResumeOfferRoundTrip(t *testing.T) {
	encoded := encodeResumeOffer(42)
	msg, err := decodeMessage(encoded)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if msg.resumeOffer == nil {
		t.Fatal("expected resume offer message")
	}
	if msg.resumeOffer.startChunk != 42 {
		t.Errorf("startChunk: %d != 42", msg.resumeOffer.startChunk)
	}
}

func TestResumeOfferZero(t *testing.T) {
	encoded := encodeResumeOffer(0)
	msg, err := decodeMessage(encoded)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if msg.resumeOffer == nil {
		t.Fatal("expected resume offer message")
	}
	if msg.resumeOffer.startChunk != 0 {
		t.Errorf("startChunk: %d != 0", msg.resumeOffer.startChunk)
	}
}

func TestResumeOfferTooShort(t *testing.T) {
	// Tag only, missing the 8-byte start chunk.
	if _, err := decodeResumeOffer([]byte{msgResumeOffer}); err == nil {
		t.Error("expected error for too-short resume offer")
	}
}

// ---------------------------------------------------------------------------
// Full send/receive with resume (in-memory, no resume state)
// ---------------------------------------------------------------------------

func TestSendRecvResumeProtocol(t *testing.T) {
	// Verifies the resume protocol works for fresh transfers (startChunk=0).
	initCh, respCh, a, b := channelPair(t)

	fileData := []byte("resume protocol test data!")
	filename := "resume.txt"

	type recvResult struct {
		res ReceiveResult
		err error
	}
	done := make(chan recvResult, 1)
	go func() {
		res, err := ReceiveData(respCh, b)
		done <- recvResult{res, err}
	}()

	sendRes, err := SendData(initCh, a, a, filename, fileData)
	if err != nil {
		t.Fatalf("send: %v", err)
	}

	rr := <-done
	if rr.err != nil {
		t.Fatalf("receive: %v", rr.err)
	}

	if string(rr.res.Data) != string(fileData) {
		t.Errorf("data mismatch")
	}
	if rr.res.SHA256 != sendRes.SHA256 {
		t.Error("SHA256 mismatch")
	}
}

// ---------------------------------------------------------------------------
// Disk-based ReceiveFile
// ---------------------------------------------------------------------------

func TestReceiveFileFresh(t *testing.T) {
	initCh, respCh, a, b := channelPair(t)
	outDir := t.TempDir()

	fileData := []byte("disk-based receive test data!")
	filename := "disk.txt"

	type recvResult struct {
		res ReceiveResult
		err error
	}
	done := make(chan recvResult, 1)
	go func() {
		res, err := ReceiveFile(respCh, b, outDir)
		done <- recvResult{res, err}
	}()

	_, err := SendData(initCh, a, a, filename, fileData)
	if err != nil {
		t.Fatalf("send: %v", err)
	}

	rr := <-done
	if rr.err != nil {
		t.Fatalf("receive: %v", rr.err)
	}

	// Verify file on disk.
	outPath := filepath.Join(outDir, filename)
	diskData, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if string(diskData) != string(fileData) {
		t.Errorf("disk data mismatch")
	}

	// Verify no resume state lingering.
	hash := sha256.Sum256(fileData)
	tid := TransferID(filename, uint64(len(fileData)), hash, DefaultChunkSize)
	if _, err := os.Stat(resumeStatePath(outDir, tid)); !os.IsNotExist(err) {
		t.Error("resume state should be cleaned up after successful transfer")
	}
}

func TestReceiveFileResume(t *testing.T) {
	// Simulate: first transfer was interrupted after 1 chunk of a 2-chunk file.
	// Second transfer should resume from chunk 1.
	chunkSize := uint32(16) // small chunks for testing
	fileData := make([]byte, 30) // 2 chunks at 16 bytes each
	for i := range fileData {
		fileData[i] = byte('A' + (i % 26))
	}
	filename := "resume-test.bin"

	hash := sha256.Sum256(fileData)
	chunkCount := uint64(2) // ceil(30/16) = 2

	tid := TransferID(filename, uint64(len(fileData)), hash, chunkSize)
	outDir := t.TempDir()

	// Pre-seed resume state: 1 chunk received.
	state := &ResumeState{
		TransferID:     tid,
		Filename:       filename,
		Size:           uint64(len(fileData)),
		SHA256Hex:      hex.EncodeToString(hash[:]),
		ChunkSize:      chunkSize,
		ChunkCount:     chunkCount,
		ChunksReceived: 1,
	}
	if err := SaveResumeState(outDir, state); err != nil {
		t.Fatalf("save resume state: %v", err)
	}

	// Write partial file (first chunk = first 16 bytes).
	partial := fileData[:16]
	pPath := partialFilePath(outDir, tid)
	if err := os.MkdirAll(filepath.Dir(pPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pPath, partial, 0o600); err != nil {
		t.Fatal(err)
	}

	// Now do a transfer with the custom chunk size. We need to use the engine
	// directly since SendData uses DefaultChunkSize. We'll test the resume
	// offer/response at the protocol level.
	initCh, respCh, a, b := channelPair(t)

	type recvResult struct {
		res ReceiveResult
		err error
	}
	done := make(chan recvResult, 1)
	go func() {
		res, err := ReceiveFile(respCh, b, outDir)
		done <- recvResult{res, err}
	}()

	// Sender side: manually send with custom chunk size.
	hdr := FileHeader{
		Filename:   filename,
		Size:       uint64(len(fileData)),
		SHA256:     hash,
		ChunkSize:  chunkSize,
		ChunkCount: chunkCount,
	}

	// Send header.
	if err := initCh.Send(a, encodeHeader(hdr)); err != nil {
		t.Fatalf("send header: %v", err)
	}

	// Wait for ResumeOffer.
	offerBytes, err := initCh.Recv(a)
	if err != nil {
		t.Fatalf("recv resume offer: %v", err)
	}
	offerMsg, err := decodeMessage(offerBytes)
	if err != nil {
		t.Fatalf("decode resume offer: %v", err)
	}
	if offerMsg.resumeOffer == nil {
		t.Fatal("expected resume offer")
	}
	startChunk := offerMsg.resumeOffer.startChunk
	if startChunk != 1 {
		t.Fatalf("expected startChunk=1 (resume), got %d", startChunk)
	}

	// Send only chunk 1 (the remaining chunk).
	chunk1Data := fileData[16:] // bytes 16-29
	if err := initCh.Send(a, encodeChunk(1, chunk1Data)); err != nil {
		t.Fatalf("send chunk 1: %v", err)
	}

	// Send end.
	if err := initCh.Send(a, encodeEnd()); err != nil {
		t.Fatalf("send end: %v", err)
	}

	rr := <-done
	if rr.err != nil {
		t.Fatalf("receive: %v", rr.err)
	}

	// Verify complete file on disk.
	outPath := filepath.Join(outDir, filename)
	diskData, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if string(diskData) != string(fileData) {
		t.Errorf("disk data mismatch: got %q, want %q", diskData, fileData)
	}

	// Verify resume state cleaned up.
	if _, err := os.Stat(resumeStatePath(outDir, tid)); !os.IsNotExist(err) {
		t.Error("resume state should be cleaned up")
	}
}

func TestReceiveFileInvalidResumeStateFallsBackToFresh(t *testing.T) {
	// Pre-seed resume state with wrong SHA256 — should fall back to chunk 0.
	chunkSize := uint32(DefaultChunkSize)
	fileData := []byte("fresh fallback test data")
	filename := "fallback.txt"
	hash := sha256.Sum256(fileData)

	// Compute TID with the real hash.
	tid := TransferID(filename, uint64(len(fileData)), hash, chunkSize)
	outDir := t.TempDir()

	// Save state with wrong hash.
	state := &ResumeState{
		TransferID:     tid,
		Filename:       filename,
		Size:           uint64(len(fileData)),
		SHA256Hex:      "0000000000000000000000000000000000000000000000000000000000000000",
		ChunkSize:      chunkSize,
		ChunkCount:     1,
		ChunksReceived: 1,
	}
	if err := SaveResumeState(outDir, state); err != nil {
		t.Fatalf("save: %v", err)
	}

	initCh, respCh, a, b := channelPair(t)

	type recvResult struct {
		res ReceiveResult
		err error
	}
	done := make(chan recvResult, 1)
	go func() {
		res, err := ReceiveFile(respCh, b, outDir)
		done <- recvResult{res, err}
	}()

	_, err := SendData(initCh, a, a, filename, fileData)
	if err != nil {
		t.Fatalf("send: %v", err)
	}

	rr := <-done
	if rr.err != nil {
		t.Fatalf("receive: %v", rr.err)
	}

	// Verify file on disk.
	outPath := filepath.Join(outDir, filename)
	diskData, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if string(diskData) != string(fileData) {
		t.Errorf("data mismatch")
	}
}
