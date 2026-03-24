package room

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestRegistry_OnExpireCallback(t *testing.T) {
	var expired atomic.Int64

	cfg := RegistryConfig{
		MaxRooms:     100,
		RoomTTL:      50 * time.Millisecond,
		ReapInterval: 20 * time.Millisecond,
		OnExpire: func(_ string) {
			expired.Add(1)
		},
	}
	reg := NewRegistry(cfg)

	s1, s2 := pipe()
	defer closePipe(s1, s2)

	_, err := reg.Create(s1)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	reaperDone := reg.RunReaper(ctx)

	time.Sleep(150 * time.Millisecond)

	cancel()
	<-reaperDone

	if expired.Load() != 1 {
		t.Fatalf("OnExpire called %d times, want 1", expired.Load())
	}
}

func TestRegistry_OnExpireNotCalledForActive(t *testing.T) {
	var expired atomic.Int64

	cfg := RegistryConfig{
		MaxRooms:     100,
		RoomTTL:      50 * time.Millisecond,
		ReapInterval: 20 * time.Millisecond,
		OnExpire: func(_ string) {
			expired.Add(1)
		},
	}
	reg := NewRegistry(cfg)

	s1, s2 := pipe()
	defer closePipe(s1, s2)
	r1, r2 := pipe()
	defer closePipe(r1, r2)

	rm, err := reg.Create(s1)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := reg.Join(rm.ID, r1); err != nil {
		t.Fatalf("Join: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	reaperDone := reg.RunReaper(ctx)

	time.Sleep(150 * time.Millisecond)

	cancel()
	<-reaperDone

	if expired.Load() != 0 {
		t.Fatalf("OnExpire called %d times for active room, want 0", expired.Load())
	}
}
