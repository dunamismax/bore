package roomid

import "fmt"

const (
	// MaxLen bounds accepted room IDs for relay URLs and signaling.
	// Generated relay IDs are currently 22 chars of base64url text, but the
	// validator allows longer IDs for tests and future-compatible formats.
	MaxLen = 128
)

// Validate rejects empty or malformed relay room IDs.
//
// Accepted room IDs are restricted to URL-safe ASCII used by the repo today:
// letters, digits, hyphen, and underscore.
func Validate(id string) error {
	if id == "" {
		return fmt.Errorf("room ID is empty")
	}
	if len(id) > MaxLen {
		return fmt.Errorf("room ID too long: %d bytes (max %d)", len(id), MaxLen)
	}
	for _, r := range id {
		if r >= 'a' && r <= 'z' {
			continue
		}
		if r >= 'A' && r <= 'Z' {
			continue
		}
		if r >= '0' && r <= '9' {
			continue
		}
		if r == '-' || r == '_' {
			continue
		}
		return fmt.Errorf("room ID contains invalid character %q", r)
	}
	return nil
}
