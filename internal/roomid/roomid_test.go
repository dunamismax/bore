package roomid

import (
	"strings"
	"testing"
)

func TestValidateAcceptsCurrentRoomIDShapes(t *testing.T) {
	tests := []string{
		"aB3_xY7z-kLm9pQrS-tUvW",
		"room1",
		"relay_000001",
		"test-room_123",
	}

	for _, id := range tests {
		t.Run(id, func(t *testing.T) {
			if err := Validate(id); err != nil {
				t.Fatalf("Validate(%q): %v", id, err)
			}
		})
	}
}

func TestValidateRejectsInvalidRoomIDs(t *testing.T) {
	tests := []struct {
		name string
		id   string
	}{
		{name: "empty", id: ""},
		{name: "space", id: "bad id"},
		{name: "slash", id: "bad/id"},
		{name: "unicode", id: "bad🔥"},
		{name: "too long", id: strings.Repeat("a", MaxLen+1)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := Validate(tt.id); err == nil {
				t.Fatalf("Validate(%q): expected error", tt.id)
			}
		})
	}
}
