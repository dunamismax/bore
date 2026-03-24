package code

import (
	"testing"
)

// FuzzParse exercises the rendezvous code parser with arbitrary strings.
// The parser should never panic, even on adversarial input.
func FuzzParse(f *testing.F) {
	// Seed corpus: valid codes, edge cases, and garbage.
	f.Add("7-apple-beach-crown")
	f.Add("42-delta-storm")
	f.Add("999-apple-beach-crown-delta-ember")
	f.Add("1-acorn-adrift")
	f.Add("")
	f.Add("0-apple-beach-crown")
	f.Add("1000-apple-beach-crown")
	f.Add("7-xyzzy-beach")
	f.Add("not-a-code")
	f.Add("---")
	f.Add("7")
	f.Add("7-apple")
	f.Add("999-" + string(make([]byte, 8192)))

	f.Fuzz(func(t *testing.T, s string) {
		code, err := Parse(s)
		if err != nil {
			return // invalid input is expected
		}

		// If parsing succeeded, round-trip must also succeed.
		roundTrip, err := Parse(code.String())
		if err != nil {
			t.Fatalf("round-trip failed: Parse(%q) succeeded but Parse(%q) failed: %v",
				s, code.String(), err)
		}
		if !code.Equal(roundTrip) {
			t.Fatalf("round-trip mismatch: %v != %v", code, roundTrip)
		}
	})
}

// FuzzParseFull exercises the full rendezvous code parser.
func FuzzParseFull(f *testing.F) {
	f.Add("abc123-42-apple-beach-crown", "http://localhost:8080")
	f.Add("aB3_xY7z-kLm9pQrS-tUvW-100-delta-storm-noble", "http://relay.example.com")
	f.Add("room1-42-apple-beach", "http://localhost:8080")
	f.Add("", "http://localhost:8080")
	f.Add("abc-42", "http://localhost:8080")
	f.Add("---", "")

	f.Fuzz(func(t *testing.T, codeStr, relayURL string) {
		full, err := ParseFull(codeStr, relayURL)
		if err != nil {
			return
		}

		// If parsing succeeded, round-trip must also succeed.
		roundTrip, err := ParseFull(full.CodeString(), relayURL)
		if err != nil {
			t.Fatalf("round-trip failed: ParseFull(%q) succeeded but ParseFull(%q) failed: %v",
				codeStr, full.CodeString(), err)
		}
		if full.RoomID != roundTrip.RoomID {
			t.Fatalf("round-trip RoomID mismatch: %q != %q", full.RoomID, roundTrip.RoomID)
		}
		if !full.PakeCode.Equal(roundTrip.PakeCode) {
			t.Fatalf("round-trip PakeCode mismatch: %v != %v", full.PakeCode, roundTrip.PakeCode)
		}
	})
}

// FuzzFromRandomBytes exercises code generation from arbitrary byte slices.
func FuzzFromRandomBytes(f *testing.F) {
	f.Add([]byte{0x00, 0x07, 0x08, 0x10, 0x20}, 3)
	f.Add([]byte{0xAB, 0xCD, 0x42, 0x99, 0x01}, 3)
	f.Add([]byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}, 5)
	f.Add([]byte{}, 3)
	f.Add([]byte{0x00}, 0)

	f.Fuzz(func(t *testing.T, random []byte, wordCount int) {
		code, err := FromRandomBytes(random, wordCount)
		if err != nil {
			return
		}

		// If generation succeeded, the result must parse and round-trip.
		roundTrip, err := Parse(code.String())
		if err != nil {
			t.Fatalf("parse after FromRandomBytes failed: %v", err)
		}
		if !code.Equal(roundTrip) {
			t.Fatalf("round-trip mismatch: %v != %v", code, roundTrip)
		}
	})
}
