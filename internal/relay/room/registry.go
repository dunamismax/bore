package room

import (
	"context"
	"net"
	"sync"
	"time"
)

// RegistryConfig holds configuration for a Registry.
type RegistryConfig struct {
	// MaxRooms is the maximum number of concurrent rooms. Zero means no limit.
	MaxRooms int

	// RoomTTL is how long a room can stay in the Waiting state before
	// it expires. Zero means no expiry.
	RoomTTL time.Duration

	// ReapInterval is how often the background reaper checks for expired
	// rooms. Zero defaults to 10 seconds.
	ReapInterval time.Duration
}

// DefaultRegistryConfig returns a RegistryConfig with sane defaults.
func DefaultRegistryConfig() RegistryConfig {
	return RegistryConfig{
		MaxRooms:     10000,
		RoomTTL:      5 * time.Minute,
		ReapInterval: 10 * time.Second,
	}
}

// Registry is a thread-safe, in-memory store of active rooms.
type Registry struct {
	mu     sync.Mutex
	rooms  map[string]*Room
	config RegistryConfig
}

// NewRegistry creates a new Registry with the given configuration.
func NewRegistry(config RegistryConfig) *Registry {
	if config.ReapInterval == 0 {
		config.ReapInterval = 10 * time.Second
	}
	return &Registry{
		rooms:  make(map[string]*Room),
		config: config,
	}
}

// Create makes a new room with a random ID and the given sender connection.
// It returns the created room or an error if the registry is full or a
// duplicate ID is generated (astronomically unlikely with 128-bit IDs).
func (reg *Registry) Create(sender net.Conn) (*Room, error) {
	id, err := GenerateID()
	if err != nil {
		return nil, err
	}

	reg.mu.Lock()
	defer reg.mu.Unlock()

	if reg.config.MaxRooms > 0 && len(reg.rooms) >= reg.config.MaxRooms {
		return nil, ErrRegistryFull
	}

	if _, exists := reg.rooms[id]; exists {
		return nil, ErrDuplicateRoom
	}

	r := NewRoom(id, sender)
	reg.rooms[id] = r
	return r, nil
}

// CreateWithID makes a new room with a specific ID. This is primarily
// useful for testing. It returns an error if the ID is already taken
// or the registry is at capacity.
func (reg *Registry) CreateWithID(id string, sender net.Conn) (*Room, error) {
	reg.mu.Lock()
	defer reg.mu.Unlock()

	if reg.config.MaxRooms > 0 && len(reg.rooms) >= reg.config.MaxRooms {
		return nil, ErrRegistryFull
	}

	if _, exists := reg.rooms[id]; exists {
		return nil, ErrDuplicateRoom
	}

	r := NewRoom(id, sender)
	reg.rooms[id] = r
	return r, nil
}

// Join adds a receiver to an existing room identified by roomID.
// Returns the room on success or an error if the room doesn't exist,
// is already full, or is closed.
func (reg *Registry) Join(roomID string, receiver net.Conn) (*Room, error) {
	reg.mu.Lock()
	r, exists := reg.rooms[roomID]
	reg.mu.Unlock()

	if !exists {
		return nil, ErrRoomNotFound
	}

	if err := r.Join(receiver); err != nil {
		return nil, err
	}
	return r, nil
}

// Remove deletes a room from the registry and closes it.
func (reg *Registry) Remove(roomID string) {
	reg.mu.Lock()
	r, exists := reg.rooms[roomID]
	if exists {
		delete(reg.rooms, roomID)
	}
	reg.mu.Unlock()

	if exists {
		r.Close()
	}
}

// Get looks up a room by ID. Returns nil if not found.
func (reg *Registry) Get(roomID string) *Room {
	reg.mu.Lock()
	defer reg.mu.Unlock()
	return reg.rooms[roomID]
}

// Len returns the number of rooms currently in the registry.
func (reg *Registry) Len() int {
	reg.mu.Lock()
	defer reg.mu.Unlock()
	return len(reg.rooms)
}

// RunReaper starts a background goroutine that periodically removes rooms
// that have been in the Waiting state longer than the configured TTL.
// It stops when ctx is canceled. The returned channel is closed when the
// reaper goroutine exits.
func (reg *Registry) RunReaper(ctx context.Context) <-chan struct{} {
	done := make(chan struct{})

	go func() {
		defer close(done)

		ticker := time.NewTicker(reg.config.ReapInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				reg.reap()
			}
		}
	}()

	return done
}

// reap removes all rooms that are in the Waiting state and have exceeded
// the configured TTL.
func (reg *Registry) reap() {
	if reg.config.RoomTTL == 0 {
		return
	}

	now := time.Now()

	reg.mu.Lock()
	var expired []string
	for id, r := range reg.rooms {
		r.mu.Lock()
		state := r.State
		created := r.Created
		r.mu.Unlock()

		if state == Waiting && now.Sub(created) > reg.config.RoomTTL {
			expired = append(expired, id)
		}
	}

	for _, id := range expired {
		if r, exists := reg.rooms[id]; exists {
			delete(reg.rooms, id)
			r.Close()
		}
	}
	reg.mu.Unlock()
}
