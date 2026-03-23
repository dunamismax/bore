package engine

import (
	"crypto/sha256"
	"io"
	"testing"

	"github.com/dunamismax/bore/client/internal/crypto"
)

// pipeConn returns two connected io.ReadWriters backed by pipe pairs.
func pipeConn() (io.ReadWriter, io.ReadWriter) {
	r1, w1 := io.Pipe()
	r2, w2 := io.Pipe()
	type rw struct {
		io.Reader
		io.Writer
	}
	return rw{r2, w1}, rw{r1, w2}
}

// channelPair performs a Noise handshake and returns two SecureChannels.
func channelPair(t *testing.T) (*crypto.SecureChannel, *crypto.SecureChannel, io.ReadWriter, io.ReadWriter) {
	t.Helper()
	a, b := pipeConn()

	type result struct {
		ch  *crypto.SecureChannel
		err error
	}
	ic := make(chan result, 1)
	rc := make(chan result, 1)

	go func() {
		ch, err := crypto.Handshake(crypto.Initiator, "test-engine-code", a)
		ic <- result{ch, err}
	}()
	go func() {
		ch, err := crypto.Handshake(crypto.Responder, "test-engine-code", b)
		rc <- result{ch, err}
	}()

	ir := <-ic
	rr := <-rc
	if ir.err != nil {
		t.Fatalf("initiator handshake: %v", ir.err)
	}
	if rr.err != nil {
		t.Fatalf("responder handshake: %v", rr.err)
	}
	return ir.ch, rr.ch, a, b
}

// ---------------------------------------------------------------------------
// Filename validation
// ---------------------------------------------------------------------------

func TestFilenameValidationEmpty(t *testing.T) {
	if err := validateFilename(""); err == nil {
		t.Error("expected error for empty filename")
	}
}

func TestFilenameValidationNullBytes(t *testing.T) {
	if err := validateFilename("file\x00name"); err == nil {
		t.Error("expected error for null byte in filename")
	}
}

func TestFilenameValidationPathSeparatorForward(t *testing.T) {
	if err := validateFilename("dir/file.txt"); err == nil {
		t.Error("expected error for forward slash")
	}
}

func TestFilenameValidationPathSeparatorBack(t *testing.T) {
	if err := validateFilename("dir\\file.txt"); err == nil {
		t.Error("expected error for backslash")
	}
}

func TestFilenameValidationDot(t *testing.T) {
	if err := validateFilename("."); err == nil {
		t.Error("expected error for '.'")
	}
}

func TestFilenameValidationDotDot(t *testing.T) {
	if err := validateFilename(".."); err == nil {
		t.Error("expected error for '..'")
	}
}

func TestFilenameValidationTooLong(t *testing.T) {
	long := make([]byte, maxFilenameLen+1)
	for i := range long {
		long[i] = 'a'
	}
	if err := validateFilename(string(long)); err == nil {
		t.Error("expected error for too-long filename")
	}
}

func TestFilenameValidationValid(t *testing.T) {
	names := []string{"file.txt", "report.pdf", "image.png", "data_2024.csv"}
	for _, name := range names {
		if err := validateFilename(name); err != nil {
			t.Errorf("validateFilename(%q): unexpected error: %v", name, err)
		}
	}
}

// ---------------------------------------------------------------------------
// Encoding/decoding round-trips
// ---------------------------------------------------------------------------

func TestHeaderDecodeRoundTrip(t *testing.T) {
	hdr := FileHeader{
		Filename:   "test.txt",
		Size:       12345,
		SHA256:     sha256.Sum256([]byte("hello")),
		ChunkSize:  DefaultChunkSize,
		ChunkCount: 1,
	}
	encoded := encodeHeader(hdr)
	msg, err := decodeMessage(encoded)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if msg.header == nil {
		t.Fatal("expected header message")
	}
	got := *msg.header
	if got.Filename != hdr.Filename {
		t.Errorf("Filename: %q != %q", got.Filename, hdr.Filename)
	}
	if got.Size != hdr.Size {
		t.Errorf("Size: %d != %d", got.Size, hdr.Size)
	}
	if got.SHA256 != hdr.SHA256 {
		t.Errorf("SHA256 mismatch")
	}
	if got.ChunkSize != hdr.ChunkSize {
		t.Errorf("ChunkSize: %d != %d", got.ChunkSize, hdr.ChunkSize)
	}
	if got.ChunkCount != hdr.ChunkCount {
		t.Errorf("ChunkCount: %d != %d", got.ChunkCount, hdr.ChunkCount)
	}
}

func TestChunkDecodeRoundTrip(t *testing.T) {
	data := []byte("chunk data here")
	encoded := encodeChunk(42, data)
	msg, err := decodeMessage(encoded)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if msg.chunk == nil {
		t.Fatal("expected chunk message")
	}
	if msg.chunk.index != 42 {
		t.Errorf("index: %d != 42", msg.chunk.index)
	}
	if string(msg.chunk.data) != string(data) {
		t.Errorf("data mismatch")
	}
}

func TestDecodeUnknownTag(t *testing.T) {
	if _, err := decodeMessage([]byte{0xFF}); err == nil {
		t.Error("expected error for unknown tag")
	}
}

func TestDecodeEmptyBuffer(t *testing.T) {
	if _, err := decodeMessage([]byte{}); err == nil {
		t.Error("expected error for empty buffer")
	}
}

func TestEndDecodeRoundTrip(t *testing.T) {
	encoded := encodeEnd()
	msg, err := decodeMessage(encoded)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !msg.isEnd {
		t.Error("expected isEnd=true")
	}
}

// ---------------------------------------------------------------------------
// Full send/receive
// ---------------------------------------------------------------------------

func TestSendRecvSmallFile(t *testing.T) {
	initCh, respCh, a, b := channelPair(t)

	fileData := []byte("hello, bore transfer!")
	filename := "hello.txt"

	type recvResult struct {
		res ReceiveResult
		err error
	}
	done := make(chan recvResult, 1)
	go func() {
		res, err := ReceiveData(respCh, b)
		done <- recvResult{res, err}
	}()

	sendRes, err := SendData(initCh, a, filename, fileData)
	if err != nil {
		t.Fatalf("send: %v", err)
	}

	rr := <-done
	if rr.err != nil {
		t.Fatalf("receive: %v", rr.err)
	}

	if string(rr.res.Data) != string(fileData) {
		t.Errorf("data mismatch: got %q, want %q", rr.res.Data, fileData)
	}
	if rr.res.Filename != filename {
		t.Errorf("filename: %q != %q", rr.res.Filename, filename)
	}
	if rr.res.SHA256 != sendRes.SHA256 {
		t.Error("SHA256 mismatch between send and receive")
	}
	if rr.res.Size != sendRes.Size {
		t.Errorf("size: %d != %d", rr.res.Size, sendRes.Size)
	}
}

func TestSendRecvLargeFile(t *testing.T) {
	initCh, respCh, a, b := channelPair(t)

	// 2 MB -- exercises multiple chunks
	fileData := make([]byte, 2*1024*1024)
	for i := range fileData {
		fileData[i] = byte(i % 251) // prime to avoid alignment patterns
	}

	type recvResult struct {
		res ReceiveResult
		err error
	}
	done := make(chan recvResult, 1)
	go func() {
		res, err := ReceiveData(respCh, b)
		done <- recvResult{res, err}
	}()

	sendRes, err := SendData(initCh, a, "large.bin", fileData)
	if err != nil {
		t.Fatalf("send: %v", err)
	}

	rr := <-done
	if rr.err != nil {
		t.Fatalf("receive: %v", rr.err)
	}

	if sendRes.SHA256 != rr.res.SHA256 {
		t.Error("SHA256 mismatch")
	}
	if len(rr.res.Data) != len(fileData) {
		t.Errorf("length: %d != %d", len(rr.res.Data), len(fileData))
	}
}

func TestSendRecvEmptyFile(t *testing.T) {
	initCh, respCh, a, b := channelPair(t)

	type recvResult struct {
		res ReceiveResult
		err error
	}
	done := make(chan recvResult, 1)
	go func() {
		res, err := ReceiveData(respCh, b)
		done <- recvResult{res, err}
	}()

	_, err := SendData(initCh, a, "empty.txt", []byte{})
	if err != nil {
		t.Fatalf("send: %v", err)
	}

	rr := <-done
	if rr.err != nil {
		t.Fatalf("receive: %v", rr.err)
	}
	if len(rr.res.Data) != 0 {
		t.Errorf("expected empty data, got %d bytes", len(rr.res.Data))
	}
}

func TestChunkCountLimitEnforced(t *testing.T) {
	// Build a header with chunk_count exceeding maxChunkCount.
	hdr := FileHeader{
		Filename:   "test.txt",
		Size:       100,
		SHA256:     [32]byte{},
		ChunkSize:  1,
		ChunkCount: maxChunkCount + 1,
	}
	encoded := encodeHeader(hdr)
	_, err := decodeMessage(encoded)
	if err == nil {
		t.Error("expected error for excessive chunk count")
	}
}
