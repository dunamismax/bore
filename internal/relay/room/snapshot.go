package room

import "time"

// RegistrySnapshot is an operator-facing summary of the relay room registry.
// It is intentionally limited to aggregate counts and configuration-derived
// limits so the relay can report health without exposing transfer contents.
type RegistrySnapshot struct {
	TotalRooms   int
	WaitingRooms int
	ActiveRooms  int
	MaxRooms     int
	RoomTTL      time.Duration
	ReapInterval time.Duration
}

// Snapshot returns a consistent point-in-time view of the registry.
func (reg *Registry) Snapshot() RegistrySnapshot {
	reg.mu.Lock()
	defer reg.mu.Unlock()

	snapshot := RegistrySnapshot{
		TotalRooms:   len(reg.rooms),
		MaxRooms:     reg.config.MaxRooms,
		RoomTTL:      reg.config.RoomTTL,
		ReapInterval: reg.config.ReapInterval,
	}

	for _, r := range reg.rooms {
		r.mu.Lock()
		state := r.State
		r.mu.Unlock()

		switch state {
		case Waiting:
			snapshot.WaitingRooms++
		case Active:
			snapshot.ActiveRooms++
		}
	}

	return snapshot
}
