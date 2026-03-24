// Package room implements the relay's room model and connection lifecycle.
//
// A room pairs exactly two peers — a sender and a receiver — by a short,
// cryptographically random room ID. The room transitions through three
// states: Waiting (sender connected, waiting for receiver), Active (both
// peers present), and Closed (room torn down).
//
// Connections are represented as opaque interfaces so the room model is
// independent of the transport layer (WebSocket, TCP, etc.).
package room

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"net"
	"sync"
	"time"
)

// RoomState represents the lifecycle state of a room.
type RoomState int

const (
	// Waiting means one peer (the sender) has connected and the room
	// is waiting for the second peer (the receiver) to join.
	Waiting RoomState = iota

	// Active means both peers are connected and bytes can be relayed.
	Active

	// Closed means the room has been torn down. Terminal state.
	Closed
)

func (s RoomState) String() string {
	switch s {
	case Waiting:
		return "waiting"
	case Active:
		return "active"
	case Closed:
		return "closed"
	default:
		return "unknown"
	}
}

// Sentinel errors for room operations.
var (
	ErrRoomNotFound  = errors.New("room not found")
	ErrRoomFull      = errors.New("room already has two peers")
	ErrRoomClosed    = errors.New("room is closed")
	ErrRegistryFull  = errors.New("registry at capacity")
	ErrDuplicateRoom = errors.New("room ID already exists")
)

// Room pairs two peers by ID and tracks their lifecycle.
type Room struct {
	mu sync.Mutex

	ID      string
	State   RoomState
	Created time.Time

	Sender   net.Conn
	Receiver net.Conn

	// done is closed when the room transitions to Closed.
	done chan struct{}
}

// NewRoom creates a room in the Waiting state with the given sender.
func NewRoom(id string, sender net.Conn) *Room {
	return &Room{
		ID:      id,
		State:   Waiting,
		Created: time.Now(),
		Sender:  sender,
		done:    make(chan struct{}),
	}
}

// Join adds a receiver to a Waiting room, transitioning it to Active.
// Returns an error if the room is not in the Waiting state.
func (r *Room) Join(receiver net.Conn) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	switch r.State {
	case Waiting:
		r.Receiver = receiver
		r.State = Active
		return nil
	case Active:
		return ErrRoomFull
	case Closed:
		return ErrRoomClosed
	default:
		return ErrRoomClosed
	}
}

// Close transitions the room to Closed. It is safe to call multiple times.
// Close does not close the underlying connections — that is the caller's
// responsibility.
func (r *Room) Close() {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.State == Closed {
		return
	}
	r.State = Closed
	close(r.done)
}

// Done returns a channel that is closed when the room transitions to Closed.
func (r *Room) Done() <-chan struct{} {
	return r.done
}

// GetState returns the room's current state.
func (r *Room) GetState() RoomState {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.State
}

// GenerateID produces a cryptographically random, URL-safe room ID.
// The ID is base64url-encoded from 16 random bytes (128-bit entropy),
// yielding a 22-character string.
func GenerateID() (string, error) {
	b := make([]byte, 16) // 128 bits
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
