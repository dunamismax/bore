// Package code implements rendezvous code generation and parsing for bore.
//
// Codes are human-friendly identifiers used to connect a sender and receiver.
// Format: {channel}-{word}-{word}-{word}, e.g. "7-guitar-castle-moon".
//
// The code has two entropy components:
//   - Channel: 1-999 (~10 bits)
//   - Words: 2-5 words from a 256-word list (8 bits each)
//
// Default (3 words): ~34 bits. At 1 attempt/sec with 5-min expiry, online
// brute-force is not practical.
package code

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"strings"
)

// Constants matching bore-core/src/code.rs.
const (
	MinWords          = 2
	MaxWords          = 5
	DefaultWords      = 3
	MinChannel        = 1
	MaxChannel        = 999
	DefaultExpirySecs = 300 // 5 minutes
	BitsPerWord       = 8   // 256 words = 2^8
	BitsChannel       = 10  // ~log2(999)
)

// wordlist is a 256-word curated list for rendezvous codes.
// Each word: 3-7 ASCII lowercase chars, no homophones, easy to pronounce.
// 256 words = 8 bits of entropy per word.
var wordlist = [256]string{
	"acorn", "adrift", "agent", "album", "alert", "amber", "anchor", "angel",
	"apple", "arena", "arrow", "atlas", "badge", "baker", "banjo", "basin",
	"beach", "beast", "berry", "blade", "blank", "blaze", "bloom", "board",
	"bonus", "booth", "brain", "brave", "brick", "bride", "brief", "brook",
	"brush", "cabin", "camel", "candy", "cargo", "cedar", "chalk", "charm",
	"chase", "chess", "chief", "cider", "civic", "claim", "cliff", "climb",
	"clock", "cloud", "coast", "cobra", "coral", "couch", "crane", "crash",
	"crown", "crush", "curve", "cycle", "dance", "delta", "demon", "denim",
	"depot", "diary", "disco", "diver", "dodge", "donor", "draft", "drain",
	"dream", "dress", "drift", "drink", "drive", "drone", "drum", "eagle",
	"earth", "elite", "ember", "envoy", "epoch", "event", "extra", "fable",
	"falcon", "feast", "fence", "fiber", "field", "flame", "flask", "fleet",
	"flint", "flood", "flora", "flute", "focal", "forge", "forum", "frame",
	"fresh", "frost", "fruit", "funds", "gamma", "gauge", "ghost", "giant",
	"glade", "glass", "gleam", "globe", "glyph", "goat", "grace", "grain",
	"graph", "grasp", "green", "grove", "guard", "guide", "guild", "haven",
	"hawk", "heart", "helix", "honey", "horse", "hotel", "human", "humor",
	"husky", "igloo", "index", "ivory", "jazzy", "jewel", "joint", "judge",
	"juice", "karma", "kayak", "knack", "kneel", "knife", "latch", "lemon",
	"lever", "light", "lilac", "linen", "llama", "lodge", "logic", "lotus",
	"lucky", "lunar", "magic", "major", "mango", "maple", "marsh", "melon",
	"mercy", "merit", "metal", "minor", "model", "moose", "motor", "mouse",
	"music", "noble", "north", "novel", "ocean", "olive", "onion", "opera",
	"orbit", "organ", "otter", "outer", "oxide", "ozone", "panda", "panel",
	"patch", "pearl", "phase", "piano", "pilot", "pixel", "plain", "plaza",
	"plumb", "plume", "polar", "pond", "prism", "prize", "proxy", "pulse",
	"quake", "quest", "quiet", "quota", "radar", "raven", "rebel", "reign",
	"ridge", "river", "robin", "robot", "royal", "rural", "salon", "satin",
	"scale", "scout", "shade", "shark", "shelf", "shell", "shift", "shine",
	"sigma", "silk", "siren", "slate", "sleet", "slope", "solar", "spark",
	"spice", "spoke", "squad", "stamp", "steel", "stone", "storm", "story",
	"sugar", "swift", "table", "tango", "tiger", "toast", "token", "tower",
}

// wordIndex maps word -> index for O(1) lookup.
var wordIndex map[string]int

func init() {
	wordIndex = make(map[string]int, len(wordlist))
	for i, w := range wordlist {
		wordIndex[w] = i
	}
}

// RendezvousCode is a parsed rendezvous code.
// Codes are single-use and expire. The code is cryptographically bound to
// the session via PAKE -- it is not just a routing hint.
type RendezvousCode struct {
	channel uint16
	words   []string
}

// New creates a RendezvousCode from a channel number and words.
// Channel must be in [MinChannel, MaxChannel]. Words must be 2-5 entries from
// the wordlist (case-insensitive; stored as lowercase).
func New(channel uint16, words []string) (RendezvousCode, error) {
	if channel < MinChannel || channel > MaxChannel {
		return RendezvousCode{}, fmt.Errorf("channel %d out of range [%d, %d]",
			channel, MinChannel, MaxChannel)
	}
	wc := len(words)
	if wc < MinWords || wc > MaxWords {
		return RendezvousCode{}, fmt.Errorf("word count %d out of range [%d, %d]",
			wc, MinWords, MaxWords)
	}
	lowers := make([]string, wc)
	for i, w := range words {
		low := strings.ToLower(w)
		if _, ok := wordIndex[low]; !ok {
			return RendezvousCode{}, fmt.Errorf("unknown word: %q", w)
		}
		lowers[i] = low
	}
	return RendezvousCode{channel: channel, words: lowers}, nil
}

// Parse parses a rendezvousCode from its string representation.
// Expected format: "{channel}-{word}-{word}-{word}"
func Parse(s string) (RendezvousCode, error) {
	if s == "" {
		return RendezvousCode{}, fmt.Errorf("empty rendezvous code")
	}
	parts := strings.Split(s, "-")
	if len(parts) < 1+MinWords {
		return RendezvousCode{}, fmt.Errorf("code must have channel and at least %d words", MinWords)
	}

	var ch uint16
	n, err := fmt.Sscanf(parts[0], "%d", &ch)
	if err != nil || n != 1 {
		return RendezvousCode{}, fmt.Errorf("invalid channel %q", parts[0])
	}

	return New(ch, parts[1:])
}

// String formats the code as "channel-word-word-word".
func (c RendezvousCode) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%d", c.channel)
	for _, w := range c.words {
		b.WriteByte('-')
		b.WriteString(w)
	}
	return b.String()
}

// Channel returns the channel number.
func (c RendezvousCode) Channel() uint16 { return c.channel }

// Words returns the words in the code.
func (c RendezvousCode) Words() []string {
	cp := make([]string, len(c.words))
	copy(cp, c.words)
	return cp
}

// WordCount returns the number of words.
func (c RendezvousCode) WordCount() int { return len(c.words) }

// EntropyBits returns the estimated entropy in bits: BitsChannel + BitsPerWord*n.
func (c RendezvousCode) EntropyBits() int {
	return BitsChannel + BitsPerWord*len(c.words)
}

// Equal returns true if two codes are identical.
func (c RendezvousCode) Equal(other RendezvousCode) bool {
	if c.channel != other.channel || len(c.words) != len(other.words) {
		return false
	}
	for i := range c.words {
		if c.words[i] != other.words[i] {
			return false
		}
	}
	return true
}

// FromRandomBytes generates a RendezvousCode deterministically from random bytes.
// Requires len(random) >= 2 + wordCount.
// Channel: big-endian u16 from random[0:2], mapped to [MinChannel, MaxChannel].
// Words: random[2:2+wordCount], each byte indexes into wordlist.
func FromRandomBytes(random []byte, wordCount int) (RendezvousCode, error) {
	if wordCount < MinWords || wordCount > MaxWords {
		return RendezvousCode{}, fmt.Errorf("word count %d out of range [%d, %d]",
			wordCount, MinWords, MaxWords)
	}
	needed := 2 + wordCount
	if len(random) < needed {
		return RendezvousCode{}, fmt.Errorf("need %d random bytes, got %d", needed, len(random))
	}

	rawCh := binary.BigEndian.Uint16(random[0:2])
	channel := uint16(rawCh%MaxChannel) + MinChannel

	words := make([]string, wordCount)
	for i := 0; i < wordCount; i++ {
		words[i] = wordlist[random[2+i]]
	}
	return RendezvousCode{channel: channel, words: words}, nil
}

// Generate creates a random RendezvousCode with wordCount words using crypto/rand.
func Generate(wordCount int) (RendezvousCode, error) {
	needed := 2 + wordCount
	buf := make([]byte, needed)
	if _, err := rand.Read(buf); err != nil {
		return RendezvousCode{}, fmt.Errorf("generate random bytes: %w", err)
	}
	return FromRandomBytes(buf, wordCount)
}

// CodeLifetime describes the lifetime policy for a rendezvous code.
// Enforcement happens in the relay and session management layers.
type CodeLifetime struct {
	ExpirySecs uint64
	SingleUse  bool
}

// DefaultCodeLifetime returns the default lifetime: 5 minutes, single-use.
func DefaultCodeLifetime() CodeLifetime {
	return CodeLifetime{ExpirySecs: DefaultExpirySecs, SingleUse: true}
}

// FullRendezvousCode encodes relay connection parameters.
// Format: "<room_id>-<channel>-<word>-<word>-<word>"
// The relay URL is not in the code string; it is provided separately.
type FullRendezvousCode struct {
	// RoomID is the relay-assigned room identifier (base64url, typically 22 chars).
	RoomID string
	// PakeCode is the PAKE code (channel + words) for the Noise handshake.
	PakeCode RendezvousCode
	// RelayURL is the relay server URL (not encoded in CodeString).
	RelayURL string
}

// CodeString formats the code for display: "room_id-channel-word-word-word".
func (f FullRendezvousCode) CodeString() string {
	return f.RoomID + "-" + f.PakeCode.String()
}

// String is an alias for CodeString.
func (f FullRendezvousCode) String() string { return f.CodeString() }

// ParseFull parses a full rendezvous code from its string representation.
// relayURL is the relay server URL to store in the returned struct.
//
// Parsing strategy: scan parts left-to-right, find the first part that
// parses as a valid channel number (1-999) with enough remaining parts for
// 2-5 words. Everything before that part is the room ID.
func ParseFull(codeStr, relayURL string) (FullRendezvousCode, error) {
	if codeStr == "" {
		return FullRendezvousCode{}, fmt.Errorf("empty code")
	}
	parts := strings.Split(codeStr, "-")
	// Minimum: 1 room_id part + 1 channel + 2 words = 4 parts
	if len(parts) < 4 {
		return FullRendezvousCode{}, fmt.Errorf("rendezvous code too short: need at least 4 parts, got %d", len(parts))
	}

	splitIdx := -1
	for i := 0; i < len(parts); i++ {
		var ch uint16
		n, err := fmt.Sscanf(parts[i], "%d", &ch)
		if err != nil || n != 1 {
			continue
		}
		if ch < MinChannel || ch > MaxChannel {
			continue
		}
		remaining := len(parts) - i - 1
		if remaining >= MinWords && remaining <= MaxWords {
			splitIdx = i
			break
		}
	}

	if splitIdx < 0 {
		return FullRendezvousCode{}, fmt.Errorf("could not find channel number in rendezvous code")
	}

	roomID := strings.Join(parts[:splitIdx], "-")
	if roomID == "" {
		return FullRendezvousCode{}, fmt.Errorf("rendezvous code has empty room ID")
	}

	pakeStr := strings.Join(parts[splitIdx:], "-")
	pakeCode, err := Parse(pakeStr)
	if err != nil {
		return FullRendezvousCode{}, fmt.Errorf("invalid PAKE code: %w", err)
	}

	return FullRendezvousCode{
		RoomID:   roomID,
		PakeCode: pakeCode,
		RelayURL: relayURL,
	}, nil
}
