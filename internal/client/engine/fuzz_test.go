package engine

import (
	"crypto/sha256"
	"testing"
)

// FuzzDecodeMessage exercises the transfer frame decoder with arbitrary bytes.
// The decoder should never panic, even on adversarial input.
func FuzzDecodeMessage(f *testing.F) {
	// Seed with valid encodings.
	hdr := FileHeader{
		Filename:   "test.txt",
		Size:       12345,
		SHA256:     sha256.Sum256([]byte("hello")),
		ChunkSize:  DefaultChunkSize,
		ChunkCount: 1,
	}
	f.Add(encodeHeader(hdr))
	f.Add(encodeChunk(0, []byte("chunk data")))
	f.Add(encodeChunk(42, []byte{}))
	f.Add(encodeEnd())
	f.Add(encodeResumeOffer(0))
	f.Add(encodeResumeOffer(999))

	// Edge cases and garbage.
	f.Add([]byte{})
	f.Add([]byte{0xFF})
	f.Add([]byte{0x01})                         // truncated header
	f.Add([]byte{0x02})                         // truncated chunk
	f.Add([]byte{0x04})                         // truncated resume offer
	f.Add([]byte{0x01, 0x00, 0x00, 0x00, 0x00}) // partial header

	f.Fuzz(func(t *testing.T, data []byte) {
		msg, err := decodeMessage(data)
		if err != nil {
			return // invalid input is expected
		}

		// If decode succeeded, verify the decoded structure is consistent.
		if msg.header != nil {
			// Re-encode and check round-trip.
			reencoded := encodeHeader(*msg.header)
			msg2, err := decodeMessage(reencoded)
			if err != nil {
				t.Fatalf("re-decode header failed: %v", err)
			}
			if msg2.header == nil {
				t.Fatal("re-decoded header is nil")
			}
			h1, h2 := msg.header, msg2.header
			if h1.Filename != h2.Filename || h1.Size != h2.Size ||
				h1.SHA256 != h2.SHA256 || h1.ChunkSize != h2.ChunkSize ||
				h1.ChunkCount != h2.ChunkCount {
				t.Fatalf("header round-trip mismatch: %+v != %+v", h1, h2)
			}
		}

		if msg.chunk != nil {
			reencoded := encodeChunk(msg.chunk.index, msg.chunk.data)
			msg2, err := decodeMessage(reencoded)
			if err != nil {
				t.Fatalf("re-decode chunk failed: %v", err)
			}
			if msg2.chunk == nil {
				t.Fatal("re-decoded chunk is nil")
			}
			if msg.chunk.index != msg2.chunk.index {
				t.Fatalf("chunk index mismatch: %d != %d", msg.chunk.index, msg2.chunk.index)
			}
		}

		if msg.resumeOffer != nil {
			reencoded := encodeResumeOffer(msg.resumeOffer.startChunk)
			msg2, err := decodeMessage(reencoded)
			if err != nil {
				t.Fatalf("re-decode resume offer failed: %v", err)
			}
			if msg2.resumeOffer == nil {
				t.Fatal("re-decoded resume offer is nil")
			}
			if msg.resumeOffer.startChunk != msg2.resumeOffer.startChunk {
				t.Fatalf("resume offer mismatch: %d != %d",
					msg.resumeOffer.startChunk, msg2.resumeOffer.startChunk)
			}
		}
	})
}

// FuzzValidateFilename exercises filename validation with arbitrary strings.
func FuzzValidateFilename(f *testing.F) {
	f.Add("file.txt")
	f.Add("report.pdf")
	f.Add("")
	f.Add(".")
	f.Add("..")
	f.Add("dir/file.txt")
	f.Add("dir\\file.txt")
	f.Add("file\x00name")
	f.Add(string(make([]byte, 5000)))

	f.Fuzz(func(t *testing.T, name string) {
		// Should never panic.
		_ = validateFilename(name)
	})
}
