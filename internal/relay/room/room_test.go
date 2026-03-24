package room

import (
	"context"
	"net"
	"sync"
	"testing"
	"time"
)

// pipe creates a pair of connected net.Conn for testing.
func pipe() (net.Conn, net.Conn) {
	return net.Pipe()
}

// closePipe closes both ends of a pipe, ignoring errors.
func closePipe(c1, c2 net.Conn) {
	c1.Close()
	c2.Close()
}

// --- Room ID generation ---

func TestGenerateID_Length(t *testing.T) {
	id, err := GenerateID()
	if err != nil {
		t.Fatalf("GenerateID: %v", err)
	}
	// 16 bytes base64url-encoded without padding = 22 characters
	if len(id) != 22 {
		t.Errorf("expected ID length 22, got %d (%q)", len(id), id)
	}
}

func TestGenerateID_Unique(t *testing.T) {
	seen := make(map[string]bool)
	for range 1000 {
		id, err := GenerateID()
		if err != nil {
			t.Fatalf("GenerateID: %v", err)
		}
		if seen[id] {
			t.Fatalf("duplicate ID after < 1000 generations: %q", id)
		}
		seen[id] = true
	}
}

func TestGenerateID_URLSafe(t *testing.T) {
	for range 100 {
		id, err := GenerateID()
		if err != nil {
			t.Fatalf("GenerateID: %v", err)
		}
		for _, c := range id {
			if (c < 'a' || c > 'z') && (c < 'A' || c > 'Z') &&
				(c < '0' || c > '9') && c != '-' && c != '_' {
				t.Errorf("non-URL-safe character %q in ID %q", string(c), id)
			}
		}
	}
}

// --- Room state transitions ---

func TestRoomState_String(t *testing.T) {
	tests := []struct {
		state RoomState
		want  string
	}{
		{Waiting, "waiting"},
		{Active, "active"},
		{Closed, "closed"},
		{RoomState(99), "unknown"},
	}
	for _, tt := range tests {
		if got := tt.state.String(); got != tt.want {
			t.Errorf("RoomState(%d).String() = %q, want %q", tt.state, got, tt.want)
		}
	}
}

func TestRoom_CreateJoinActiveClose(t *testing.T) {
	s1, s2 := pipe()
	defer closePipe(s1, s2)
	r1, r2 := pipe()
	defer closePipe(r1, r2)

	room := NewRoom("test-1", s1)

	if room.GetState() != Waiting {
		t.Fatalf("new room state = %v, want Waiting", room.GetState())
	}

	if err := room.Join(r1); err != nil {
		t.Fatalf("Join: %v", err)
	}
	if room.GetState() != Active {
		t.Fatalf("after join, state = %v, want Active", room.GetState())
	}

	room.Close()
	if room.GetState() != Closed {
		t.Fatalf("after close, state = %v, want Closed", room.GetState())
	}

	// Done channel should be closed
	select {
	case <-room.Done():
	default:
		t.Fatal("Done() channel not closed after Close()")
	}
}

func TestRoom_JoinFullRoom(t *testing.T) {
	s1, s2 := pipe()
	defer closePipe(s1, s2)
	r1, r2 := pipe()
	defer closePipe(r1, r2)
	r3, r4 := pipe()
	defer closePipe(r3, r4)

	room := NewRoom("test-2", s1)
	if err := room.Join(r1); err != nil {
		t.Fatalf("first Join: %v", err)
	}

	if err := room.Join(r3); err != ErrRoomFull {
		t.Fatalf("second Join: got %v, want ErrRoomFull", err)
	}
}

func TestRoom_JoinClosedRoom(t *testing.T) {
	s1, s2 := pipe()
	defer closePipe(s1, s2)
	r1, r2 := pipe()
	defer closePipe(r1, r2)

	room := NewRoom("test-3", s1)
	room.Close()

	if err := room.Join(r1); err != ErrRoomClosed {
		t.Fatalf("Join closed room: got %v, want ErrRoomClosed", err)
	}
}

func TestRoom_CloseIdempotent(t *testing.T) {
	s1, s2 := pipe()
	defer closePipe(s1, s2)

	room := NewRoom("test-4", s1)
	room.Close()
	room.Close() // should not panic
	room.Close()

	if room.GetState() != Closed {
		t.Fatal("state should be Closed")
	}
}

func TestRoom_DoneBlocksUntilClose(t *testing.T) {
	s1, s2 := pipe()
	defer closePipe(s1, s2)

	room := NewRoom("test-5", s1)

	select {
	case <-room.Done():
		t.Fatal("Done() should block before Close()")
	default:
	}

	room.Close()

	select {
	case <-room.Done():
	case <-time.After(time.Second):
		t.Fatal("Done() should be closed after Close()")
	}
}

// --- Registry ---

func TestRegistry_CreateAndGet(t *testing.T) {
	reg := NewRegistry(DefaultRegistryConfig())
	s1, s2 := pipe()
	defer closePipe(s1, s2)

	room, err := reg.Create(s1)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if room == nil {
		t.Fatal("Create returned nil room")
	}
	if room.GetState() != Waiting {
		t.Fatalf("new room state = %v, want Waiting", room.GetState())
	}
	if reg.Len() != 1 {
		t.Fatalf("registry length = %d, want 1", reg.Len())
	}

	got := reg.Get(room.ID)
	if got != room {
		t.Fatal("Get did not return the same room")
	}
}

func TestRegistry_CreateAndJoin(t *testing.T) {
	reg := NewRegistry(DefaultRegistryConfig())
	s1, s2 := pipe()
	defer closePipe(s1, s2)
	r1, r2 := pipe()
	defer closePipe(r1, r2)

	room, err := reg.Create(s1)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	joined, err := reg.Join(room.ID, r1)
	if err != nil {
		t.Fatalf("Join: %v", err)
	}
	if joined.GetState() != Active {
		t.Fatalf("after join, state = %v, want Active", joined.GetState())
	}
}

func TestRegistry_JoinNonexistent(t *testing.T) {
	reg := NewRegistry(DefaultRegistryConfig())
	r1, r2 := pipe()
	defer closePipe(r1, r2)

	_, err := reg.Join("does-not-exist", r1)
	if err != ErrRoomNotFound {
		t.Fatalf("Join nonexistent: got %v, want ErrRoomNotFound", err)
	}
}

func TestRegistry_Remove(t *testing.T) {
	reg := NewRegistry(DefaultRegistryConfig())
	s1, s2 := pipe()
	defer closePipe(s1, s2)

	room, err := reg.Create(s1)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	reg.Remove(room.ID)

	if reg.Len() != 0 {
		t.Fatalf("registry length = %d, want 0 after Remove", reg.Len())
	}
	if room.GetState() != Closed {
		t.Fatalf("removed room state = %v, want Closed", room.GetState())
	}
	if reg.Get(room.ID) != nil {
		t.Fatal("Get should return nil after Remove")
	}
}

func TestRegistry_RemoveNonexistent(t *testing.T) {
	reg := NewRegistry(DefaultRegistryConfig())
	reg.Remove("nope") // should not panic
}

func TestRegistry_DuplicateID(t *testing.T) {
	reg := NewRegistry(DefaultRegistryConfig())
	s1, s2 := pipe()
	defer closePipe(s1, s2)
	s3, s4 := pipe()
	defer closePipe(s3, s4)

	_, err := reg.CreateWithID("dup-test", s1)
	if err != nil {
		t.Fatalf("first CreateWithID: %v", err)
	}

	_, err = reg.CreateWithID("dup-test", s3)
	if err != ErrDuplicateRoom {
		t.Fatalf("duplicate CreateWithID: got %v, want ErrDuplicateRoom", err)
	}
}

func TestRegistry_CapacityLimit(t *testing.T) {
	cfg := RegistryConfig{
		MaxRooms:     2,
		RoomTTL:      time.Minute,
		ReapInterval: time.Second,
	}
	reg := NewRegistry(cfg)

	conns := make([]net.Conn, 0, 6)
	defer func() {
		for _, c := range conns {
			c.Close()
		}
	}()

	for i := range 2 {
		c1, c2 := pipe()
		conns = append(conns, c1, c2)
		_, err := reg.CreateWithID("cap-"+string(rune('a'+i)), c1)
		if err != nil {
			t.Fatalf("Create room %d: %v", i, err)
		}
	}

	c1, c2 := pipe()
	conns = append(conns, c1, c2)
	_, err := reg.Create(c1)
	if err != ErrRegistryFull {
		t.Fatalf("Create at capacity: got %v, want ErrRegistryFull", err)
	}

	// CreateWithID should also fail
	c3, c4 := pipe()
	conns = append(conns, c3, c4)
	_, err = reg.CreateWithID("cap-overflow", c3)
	if err != ErrRegistryFull {
		t.Fatalf("CreateWithID at capacity: got %v, want ErrRegistryFull", err)
	}
}

func TestRegistry_PeerDisconnectClosesRoom(t *testing.T) {
	reg := NewRegistry(DefaultRegistryConfig())
	s1, s2 := pipe()
	r1, r2 := pipe()
	defer func() {
		s1.Close()
		s2.Close()
		r1.Close()
		r2.Close()
	}()

	room, err := reg.Create(s1)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	_, err = reg.Join(room.ID, r1)
	if err != nil {
		t.Fatalf("Join: %v", err)
	}

	if room.GetState() != Active {
		t.Fatalf("state = %v, want Active", room.GetState())
	}

	// Simulate peer disconnect: close the room and remove it
	reg.Remove(room.ID)

	if room.GetState() != Closed {
		t.Fatalf("after disconnect, state = %v, want Closed", room.GetState())
	}
	if reg.Len() != 0 {
		t.Fatal("registry should be empty after disconnect")
	}
}

// --- Reaper ---

func TestRegistry_ReaperExpiresWaitingRooms(t *testing.T) {
	cfg := RegistryConfig{
		MaxRooms:     100,
		RoomTTL:      50 * time.Millisecond,
		ReapInterval: 20 * time.Millisecond,
	}
	reg := NewRegistry(cfg)

	s1, s2 := pipe()
	defer closePipe(s1, s2)

	room, err := reg.Create(s1)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	reaperDone := reg.RunReaper(ctx)

	// Wait for TTL + a couple reap intervals
	time.Sleep(150 * time.Millisecond)

	cancel()
	<-reaperDone

	if reg.Len() != 0 {
		t.Fatalf("registry length = %d, want 0 after reap", reg.Len())
	}
	if room.GetState() != Closed {
		t.Fatalf("expired room state = %v, want Closed", room.GetState())
	}
}

func TestRegistry_ReaperDoesNotExpireActiveRooms(t *testing.T) {
	cfg := RegistryConfig{
		MaxRooms:     100,
		RoomTTL:      50 * time.Millisecond,
		ReapInterval: 20 * time.Millisecond,
	}
	reg := NewRegistry(cfg)

	s1, s2 := pipe()
	defer closePipe(s1, s2)
	r1, r2 := pipe()
	defer closePipe(r1, r2)

	room, err := reg.Create(s1)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := reg.Join(room.ID, r1); err != nil {
		t.Fatalf("Join: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	reaperDone := reg.RunReaper(ctx)

	time.Sleep(150 * time.Millisecond)

	cancel()
	<-reaperDone

	if reg.Len() != 1 {
		t.Fatalf("registry length = %d, want 1 (active room should survive)", reg.Len())
	}
	if room.GetState() != Active {
		t.Fatalf("active room state = %v, want Active", room.GetState())
	}
}

func TestRegistry_ReaperStopsOnCancel(t *testing.T) {
	cfg := RegistryConfig{
		MaxRooms:     100,
		RoomTTL:      time.Hour, // won't expire
		ReapInterval: 10 * time.Millisecond,
	}
	reg := NewRegistry(cfg)

	ctx, cancel := context.WithCancel(context.Background())
	reaperDone := reg.RunReaper(ctx)

	cancel()

	select {
	case <-reaperDone:
	case <-time.After(time.Second):
		t.Fatal("reaper did not stop within 1 second after cancel")
	}
}

// --- Concurrency ---

func TestRegistry_ConcurrentAccess(t *testing.T) {
	cfg := RegistryConfig{
		MaxRooms:     1000,
		RoomTTL:      time.Minute,
		ReapInterval: time.Second,
	}
	reg := NewRegistry(cfg)

	const goroutines = 50
	const opsPerGoroutine = 20

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for range goroutines {
		go func() {
			defer wg.Done()
			for range opsPerGoroutine {
				s1, s2 := pipe()
				room, err := reg.Create(s1)
				if err != nil {
					closePipe(s1, s2)
					continue
				}

				r1, r2 := pipe()
				_, _ = reg.Join(room.ID, r1)

				reg.Remove(room.ID)
				closePipe(s1, s2)
				closePipe(r1, r2)
			}
		}()
	}

	wg.Wait()

	if reg.Len() != 0 {
		t.Fatalf("registry length = %d after concurrent ops, want 0", reg.Len())
	}
}
