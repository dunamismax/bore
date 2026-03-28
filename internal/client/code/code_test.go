package code

import (
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Wordlist invariants
// ---------------------------------------------------------------------------

func TestWordlistHas256Entries(t *testing.T) {
	if len(wordlist) != 256 {
		t.Fatalf("wordlist has %d entries, want 256", len(wordlist))
	}
}

func TestWordlistEntriesUnique(t *testing.T) {
	seen := make(map[string]bool)
	for _, w := range wordlist {
		if seen[w] {
			t.Errorf("duplicate word: %q", w)
		}
		seen[w] = true
	}
}

func TestWordlistEntriesLowercase(t *testing.T) {
	for _, w := range wordlist {
		if w != strings.ToLower(w) {
			t.Errorf("word %q is not lowercase", w)
		}
	}
}

func TestWordlistEntriesLength(t *testing.T) {
	for _, w := range wordlist {
		if len(w) < 3 || len(w) > 7 {
			t.Errorf("word %q has length %d (want 3-7)", w, len(w))
		}
	}
}

func TestWordlistEntriesAlphabetic(t *testing.T) {
	for _, w := range wordlist {
		for _, ch := range w {
			if ch < 'a' || ch > 'z' {
				t.Errorf("word %q contains non-lowercase-ascii char %q", w, ch)
				break
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Parsing
// ---------------------------------------------------------------------------

func TestParseValidThreeWordCode(t *testing.T) {
	c, err := Parse("7-apple-beach-crown")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.Channel() != 7 {
		t.Errorf("channel = %d, want 7", c.Channel())
	}
	if c.WordCount() != 3 {
		t.Errorf("word count = %d, want 3", c.WordCount())
	}
	if c.String() != "7-apple-beach-crown" {
		t.Errorf("String() = %q, want %q", c.String(), "7-apple-beach-crown")
	}
}

func TestParseValidTwoWordCode(t *testing.T) {
	c, err := Parse("42-delta-storm")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.Channel() != 42 {
		t.Errorf("channel = %d, want 42", c.Channel())
	}
	if c.WordCount() != 2 {
		t.Errorf("word count = %d, want 2", c.WordCount())
	}
}

func TestParseValidFourWordCode(t *testing.T) {
	c, err := Parse("100-apple-beach-crown-delta")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.WordCount() != 4 {
		t.Errorf("word count = %d, want 4", c.WordCount())
	}
}

func TestParseValidFiveWordCode(t *testing.T) {
	c, err := Parse("999-apple-beach-crown-delta-ember")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.WordCount() != 5 {
		t.Errorf("word count = %d, want 5", c.WordCount())
	}
}

func TestParseRejectsChannelZero(t *testing.T) {
	if _, err := Parse("0-apple-beach-crown"); err == nil {
		t.Error("expected error for channel 0")
	}
}

func TestParseRejectsChannelTooHigh(t *testing.T) {
	if _, err := Parse("1000-apple-beach-crown"); err == nil {
		t.Error("expected error for channel 1000")
	}
}

func TestParseRejectsUnknownWord(t *testing.T) {
	if _, err := Parse("7-apple-xyzzy-crown"); err == nil {
		t.Error("expected error for unknown word")
	}
}

func TestParseRejectsOneWord(t *testing.T) {
	if _, err := Parse("7-apple"); err == nil {
		t.Error("expected error for only 1 word")
	}
}

func TestParseRejectsEmptyString(t *testing.T) {
	if _, err := Parse(""); err == nil {
		t.Error("expected error for empty string")
	}
}

func TestParseCaseInsensitive(t *testing.T) {
	c, err := Parse("7-Apple-BEACH-Crown")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	words := c.Words()
	for _, w := range words {
		if w != strings.ToLower(w) {
			t.Errorf("word %q not lowercased", w)
		}
	}
}

// ---------------------------------------------------------------------------
// Code generation
// ---------------------------------------------------------------------------

func TestFromRandomBytesValid(t *testing.T) {
	random := []byte{0x00, 0x07, 0x08, 0x10, 0x20}
	c, err := FromRandomBytes(random, 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.Channel() < MinChannel || c.Channel() > MaxChannel {
		t.Errorf("channel %d out of [%d, %d]", c.Channel(), MinChannel, MaxChannel)
	}
	if c.WordCount() != 3 {
		t.Errorf("word count = %d, want 3", c.WordCount())
	}
}

func TestFromRandomBytesDeterministic(t *testing.T) {
	random := []byte{0xAB, 0xCD, 0x42, 0x99, 0x01}
	c1, err := FromRandomBytes(random, 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	c2, err := FromRandomBytes(random, 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !c1.Equal(c2) {
		t.Errorf("non-deterministic: %v != %v", c1, c2)
	}
}

func TestFromRandomBytesInsufficientBytes(t *testing.T) {
	if _, err := FromRandomBytes([]byte{0x00, 0x01}, 3); err == nil {
		t.Error("expected error for insufficient bytes")
	}
}

func TestGenerateDifferentWordCounts(t *testing.T) {
	for wc := MinWords; wc <= MaxWords; wc++ {
		c, err := Generate(wc)
		if err != nil {
			t.Fatalf("Generate(%d): %v", wc, err)
		}
		if c.WordCount() != wc {
			t.Errorf("Generate(%d): word count = %d", wc, c.WordCount())
		}
	}
}

// ---------------------------------------------------------------------------
// Entropy
// ---------------------------------------------------------------------------

func TestEntropyCalculation(t *testing.T) {
	c, err := Parse("7-apple-beach-crown")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// 10 + 3*8 = 34
	if c.EntropyBits() != 34 {
		t.Errorf("entropy = %d, want 34", c.EntropyBits())
	}
}

func TestEntropyScalesWithWords(t *testing.T) {
	random := []byte{0x00, 0x07, 0x08, 0x10, 0x20, 0x30, 0x40}
	for wc := MinWords; wc <= MaxWords; wc++ {
		c, _ := FromRandomBytes(random, wc)
		want := BitsChannel + BitsPerWord*wc
		if c.EntropyBits() != want {
			t.Errorf("wc=%d: entropy=%d, want %d", wc, c.EntropyBits(), want)
		}
	}
}

// ---------------------------------------------------------------------------
// String round-trip
// ---------------------------------------------------------------------------

func TestCodeStringRoundTrip(t *testing.T) {
	c, _ := Parse("7-apple-beach-crown")
	c2, err := Parse(c.String())
	if err != nil {
		t.Fatalf("reparse: %v", err)
	}
	if !c.Equal(c2) {
		t.Errorf("round-trip mismatch: %v != %v", c, c2)
	}
}

// ---------------------------------------------------------------------------
// CodeLifetime
// ---------------------------------------------------------------------------

func TestDefaultCodeLifetime(t *testing.T) {
	lt := DefaultCodeLifetime()
	if lt.ExpirySecs != 300 {
		t.Errorf("ExpirySecs = %d, want 300", lt.ExpirySecs)
	}
	if !lt.SingleUse {
		t.Error("SingleUse should be true")
	}
}

// ---------------------------------------------------------------------------
// FullRendezvousCode
// ---------------------------------------------------------------------------

func TestFullCodeFormat(t *testing.T) {
	pake, _ := Parse("42-apple-beach-crown")
	full := FullRendezvousCode{
		RoomID:   "abc123XYZ",
		PakeCode: pake,
		RelayURL: "http://localhost:8080",
	}
	want := "abc123XYZ-42-apple-beach-crown"
	if full.CodeString() != want {
		t.Errorf("CodeString() = %q, want %q", full.CodeString(), want)
	}
	if full.String() != want {
		t.Errorf("String() = %q, want %q", full.String(), want)
	}
}

func TestFullCodeParseSimple(t *testing.T) {
	relay := "http://localhost:8080"
	f, err := ParseFull("abc123-42-apple-beach-crown", relay)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f.RoomID != "abc123" {
		t.Errorf("RoomID = %q, want abc123", f.RoomID)
	}
	if f.PakeCode.Channel() != 42 {
		t.Errorf("channel = %d, want 42", f.PakeCode.Channel())
	}
	if f.RelayURL != relay {
		t.Errorf("RelayURL = %q, want %q", f.RelayURL, relay)
	}
}

func TestFullCodeParseBase64URLRoomID(t *testing.T) {
	relay := "http://localhost:8080"
	f, err := ParseFull("aB3_xY7z-kLm9pQrS-tUvW-100-delta-storm-noble", relay)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f.RoomID != "aB3_xY7z-kLm9pQrS-tUvW" {
		t.Errorf("RoomID = %q", f.RoomID)
	}
	if f.PakeCode.Channel() != 100 {
		t.Errorf("channel = %d, want 100", f.PakeCode.Channel())
	}
	if f.PakeCode.WordCount() != 3 {
		t.Errorf("word count = %d, want 3", f.PakeCode.WordCount())
	}
}

func TestFullCodeRoundTrip(t *testing.T) {
	relay := "http://localhost:8080"
	pake, _ := Parse("7-apple-beach-crown")
	orig := FullRendezvousCode{
		RoomID:   "testRoom123",
		PakeCode: pake,
		RelayURL: relay,
	}
	f, err := ParseFull(orig.CodeString(), relay)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if f.RoomID != orig.RoomID {
		t.Errorf("RoomID mismatch: %q != %q", f.RoomID, orig.RoomID)
	}
	if !f.PakeCode.Equal(orig.PakeCode) {
		t.Errorf("PakeCode mismatch: %v != %v", f.PakeCode, orig.PakeCode)
	}
}

func TestFullCodeParseTwoWords(t *testing.T) {
	f, err := ParseFull("room1-42-apple-beach", "http://localhost:8080")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f.RoomID != "room1" {
		t.Errorf("RoomID = %q, want room1", f.RoomID)
	}
	if f.PakeCode.WordCount() != 2 {
		t.Errorf("word count = %d, want 2", f.PakeCode.WordCount())
	}
}

func TestFullCodeParseFiveWords(t *testing.T) {
	f, err := ParseFull("room1-42-apple-beach-crown-delta-ember", "http://localhost:8080")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f.PakeCode.WordCount() != 5 {
		t.Errorf("word count = %d, want 5", f.PakeCode.WordCount())
	}
}

func TestFullCodeParseTooShort(t *testing.T) {
	if _, err := ParseFull("abc-42", "http://localhost:8080"); err == nil {
		t.Error("expected error for too-short code")
	}
}

func TestFullCodeParseNoChannel(t *testing.T) {
	if _, err := ParseFull("abc-xyz-apple-beach-crown", "http://localhost:8080"); err == nil {
		t.Error("expected error for missing channel")
	}
}

func TestFullCodeParseRejectsInvalidRoomID(t *testing.T) {
	if _, err := ParseFull("bad/id-42-apple-beach-crown", "http://localhost:8080"); err == nil {
		t.Error("expected error for invalid room ID")
	}
}
