// Package crypto implements the Noise Protocol handshake and secure channel
// for bore.
//
// Protocol: Noise_XXpsk0_25519_ChaChaPoly_SHA256
//
// The rendezvous code is derived via HKDF-SHA256 into a 32-byte PSK that is
// mixed into the Noise handshake at position 0. This means:
//   - An attacker without the code cannot complete the handshake.
//   - Both peers authenticate mutually via ephemeral static key exchange.
//   - The PSK adds code-derived entropy to session key derivation.
//
// After the 3-message XX handshake, SecureChannel provides ChaCha20-Poly1305
// AEAD encryption using the noise library's internal counter-based nonces.
//
// Wire framing:
//   - Each handshake message: [4-byte big-endian length][message bytes]
//   - Each SecureChannel message: [4-byte segment_count][per-segment: [4-byte len][ciphertext]]
package crypto

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"io"

	"crypto/sha256"
	"github.com/flynn/noise"
	"golang.org/x/crypto/hkdf"
)

// Protocol constants.
const (
	pskSalt       = "bore-pake-v0"
	pskInfo       = "bore handshake psk"
	pskLen        = 32
	maxPlaintext  = 65535 - 16 // 65519 bytes per segment
	maxCiphertext = 65535      // ciphertext = plaintext + 16-byte tag
)

// HandshakeRole identifies which side of the Noise handshake a peer plays.
type HandshakeRole int

const (
	// Initiator sends the first handshake message (bore sender).
	Initiator HandshakeRole = iota
	// Responder receives the first handshake message (bore receiver).
	Responder
)

// derivePSK derives a 32-byte PSK from a rendezvous code string using HKDF-SHA256.
// Same code always produces the same PSK (deterministic).
func derivePSK(code string) [pskLen]byte {
	r := hkdf.New(sha256.New, []byte(code), []byte(pskSalt), []byte(pskInfo))
	var psk [pskLen]byte
	if _, err := io.ReadFull(r, psk[:]); err != nil {
		// HKDF with SHA-256 to 32 bytes cannot fail.
		panic("hkdf read: " + err.Error())
	}
	return psk
}

// Handshake performs the Noise XXpsk0 handshake over rw.
// Both peers must use the same code. Returns a SecureChannel ready for use.
// Returns an error if the handshake fails (e.g. wrong code, I/O error).
func Handshake(role HandshakeRole, code string, rw io.ReadWriter) (*SecureChannel, error) {
	psk := derivePSK(code)

	cs := noise.NewCipherSuite(noise.DH25519, noise.CipherChaChaPoly, noise.HashSHA256)

	kp, err := cs.GenerateKeypair(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("handshake failed: generate keypair: %w", err)
	}

	hs, err := noise.NewHandshakeState(noise.Config{
		CipherSuite:           cs,
		Random:                rand.Reader,
		Pattern:               noise.HandshakeXX,
		Initiator:             role == Initiator,
		StaticKeypair:         kp,
		PresharedKey:          psk[:],
		PresharedKeyPlacement: 0, // XXpsk0
	})
	if err != nil {
		return nil, fmt.Errorf("handshake failed: init state: %w", err)
	}

	switch role {
	case Initiator:
		return initiatorHandshake(hs, rw)
	default:
		return responderHandshake(hs, rw)
	}
}

// initiatorHandshake executes the initiator side of the XX handshake.
// Message flow: send msg1, recv msg2, send msg3.
func initiatorHandshake(hs *noise.HandshakeState, rw io.ReadWriter) (*SecureChannel, error) {
	// msg1: -> e
	if _, _, err := sendHS(rw, hs, nil); err != nil {
		return nil, fmt.Errorf("handshake failed (msg1): %w", err)
	}
	// msg2: <- e, ee, s, es
	if _, _, _, err := recvHS(rw, hs); err != nil {
		return nil, fmt.Errorf("handshake failed (msg2): %w", err)
	}
	// msg3: -> s, se (cipher states returned after last message)
	cs0, cs1, err := sendHS(rw, hs, nil)
	if err != nil {
		return nil, fmt.Errorf("handshake failed (msg3): %w", err)
	}
	if cs0 == nil || cs1 == nil {
		return nil, fmt.Errorf("handshake failed: cipher states nil after completion")
	}
	// Initiator: send via cs0 (initiator->responder), recv via cs1 (responder->initiator).
	return &SecureChannel{sendCS: cs0, recvCS: cs1, initiator: true}, nil
}

// responderHandshake executes the responder side of the XX handshake.
// Message flow: recv msg1, send msg2, recv msg3.
func responderHandshake(hs *noise.HandshakeState, rw io.ReadWriter) (*SecureChannel, error) {
	// msg1: <- e
	if _, _, _, err := recvHS(rw, hs); err != nil {
		return nil, fmt.Errorf("handshake failed (msg1): %w", err)
	}
	// msg2: -> e, ee, s, es
	if _, _, err := sendHS(rw, hs, nil); err != nil {
		return nil, fmt.Errorf("handshake failed (msg2): %w", err)
	}
	// msg3: <- s, se (cipher states returned after last message)
	_, cs0, cs1, err := recvHS(rw, hs)
	if err != nil {
		return nil, fmt.Errorf("handshake failed (msg3): %w", err)
	}
	if cs0 == nil || cs1 == nil {
		return nil, fmt.Errorf("handshake failed: cipher states nil after completion")
	}
	// Responder: send via cs1 (responder->initiator), recv via cs0 (initiator->responder).
	return &SecureChannel{sendCS: cs1, recvCS: cs0, initiator: false}, nil
}

// sendHS writes one handshake message to w with a 4-byte big-endian length prefix.
// Returns (cs0, cs1, err). cs0 and cs1 are non-nil only after the last message.
func sendHS(w io.Writer, hs *noise.HandshakeState, payload []byte) (*noise.CipherState, *noise.CipherState, error) {
	msg, cs0, cs1, err := hs.WriteMessage(nil, payload)
	if err != nil {
		return nil, nil, err
	}
	var lenBuf [4]byte
	binary.BigEndian.PutUint32(lenBuf[:], uint32(len(msg)))
	if _, err := w.Write(lenBuf[:]); err != nil {
		return nil, nil, fmt.Errorf("write length: %w", err)
	}
	if _, err := w.Write(msg); err != nil {
		return nil, nil, fmt.Errorf("write message: %w", err)
	}
	return cs0, cs1, nil
}

// recvHS reads one handshake message from r (4-byte length prefix + message).
// Returns (payload, cs0, cs1, err). cs0/cs1 non-nil only after the last message.
func recvHS(r io.Reader, hs *noise.HandshakeState) ([]byte, *noise.CipherState, *noise.CipherState, error) {
	var lenBuf [4]byte
	if _, err := io.ReadFull(r, lenBuf[:]); err != nil {
		return nil, nil, nil, fmt.Errorf("read length: %w", err)
	}
	msgLen := binary.BigEndian.Uint32(lenBuf[:])
	if msgLen > maxCiphertext {
		return nil, nil, nil, fmt.Errorf("handshake message too large: %d bytes", msgLen)
	}
	msg := make([]byte, msgLen)
	if _, err := io.ReadFull(r, msg); err != nil {
		return nil, nil, nil, fmt.Errorf("read message: %w", err)
	}
	payload, cs0, cs1, err := hs.ReadMessage(nil, msg)
	if err != nil {
		return nil, nil, nil, err
	}
	return payload, cs0, cs1, nil
}

// SecureChannel is an encrypted bidirectional channel established after a
// successful Noise handshake. ChaCha20-Poly1305 AEAD with counter-based nonces
// (managed by the noise library) protects all messages.
type SecureChannel struct {
	sendCS    *noise.CipherState
	recvCS    *noise.CipherState
	initiator bool
}

// IsInitiator returns true if this channel was the handshake initiator.
func (c *SecureChannel) IsInitiator() bool { return c.initiator }

// Send encrypts data and writes it to w.
//
// Payloads larger than maxPlaintext bytes are split into multiple segments,
// each independently encrypted. Wire format:
//
//	[4-byte big-endian segment_count]
//	for each segment:
//	  [4-byte big-endian ciphertext_len]
//	  [ciphertext_len bytes: ciphertext + 16-byte AEAD tag]
//
// Empty data: sent as 1 segment with a 16-byte ciphertext (just the tag).
func (c *SecureChannel) Send(w io.Writer, data []byte) error {
	// Split data into segments.
	var chunks [][]byte
	if len(data) == 0 {
		chunks = [][]byte{{}}
	} else {
		for len(data) > 0 {
			n := len(data)
			if n > maxPlaintext {
				n = maxPlaintext
			}
			chunks = append(chunks, data[:n])
			data = data[n:]
		}
	}

	// Write segment count.
	var countBuf [4]byte
	binary.BigEndian.PutUint32(countBuf[:], uint32(len(chunks)))
	if _, err := w.Write(countBuf[:]); err != nil {
		return fmt.Errorf("write segment count: %w", err)
	}

	// Write each encrypted segment.
	for _, chunk := range chunks {
		encrypted, encErr := c.sendCS.Encrypt(nil, nil, chunk)
		if encErr != nil {
			return fmt.Errorf("encrypt segment: %w", encErr)
		}
		var lenBuf [4]byte
		binary.BigEndian.PutUint32(lenBuf[:], uint32(len(encrypted)))
		if _, err := w.Write(lenBuf[:]); err != nil {
			return fmt.Errorf("write segment length: %w", err)
		}
		if _, err := w.Write(encrypted); err != nil {
			return fmt.Errorf("write segment data: %w", err)
		}
	}
	return nil
}

// Recv reads and decrypts one message from r.
// Handles multi-segment messages transparently, reassembling the full plaintext.
func (c *SecureChannel) Recv(r io.Reader) ([]byte, error) {
	var countBuf [4]byte
	if _, err := io.ReadFull(r, countBuf[:]); err != nil {
		return nil, fmt.Errorf("read segment count: %w", err)
	}
	segCount := binary.BigEndian.Uint32(countBuf[:])
	if segCount == 0 {
		return nil, fmt.Errorf("invalid segment count: 0")
	}
	// Cap to prevent memory exhaustion from a malicious peer (~4 GB at maxPlaintext/segment).
	if segCount > 65536 {
		return nil, fmt.Errorf("segment count too large: %d", segCount)
	}

	var result []byte
	for i := uint32(0); i < segCount; i++ {
		var lenBuf [4]byte
		if _, err := io.ReadFull(r, lenBuf[:]); err != nil {
			return nil, fmt.Errorf("read segment length: %w", err)
		}
		segLen := binary.BigEndian.Uint32(lenBuf[:])
		if segLen > maxCiphertext {
			return nil, fmt.Errorf("segment too large: %d bytes", segLen)
		}
		encrypted := make([]byte, segLen)
		if _, err := io.ReadFull(r, encrypted); err != nil {
			return nil, fmt.Errorf("read segment data: %w", err)
		}
		plaintext, err := c.recvCS.Decrypt(nil, nil, encrypted)
		if err != nil {
			return nil, fmt.Errorf("decrypt segment %d: %w", i, err)
		}
		result = append(result, plaintext...)
	}
	return result, nil
}
