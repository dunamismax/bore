package transport

import (
	"testing"
)

func TestBuildWSURLHTTP(t *testing.T) {
	u, err := BuildWSURL("http://localhost:8080", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "ws://localhost:8080/ws"
	if u != want {
		t.Errorf("got %q, want %q", u, want)
	}
}

func TestBuildWSURLHTTPS(t *testing.T) {
	u, err := BuildWSURL("https://relay.example.com", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "wss://relay.example.com/ws"
	if u != want {
		t.Errorf("got %q, want %q", u, want)
	}
}

func TestBuildWSURLWithRoom(t *testing.T) {
	u, err := BuildWSURL("http://localhost:8080", "abc123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "ws://localhost:8080/ws?room=abc123"
	if u != want {
		t.Errorf("got %q, want %q", u, want)
	}
}

func TestBuildWSURLWSScheme(t *testing.T) {
	u, err := BuildWSURL("ws://relay.local:9090", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "ws://relay.local:9090/ws"
	if u != want {
		t.Errorf("got %q, want %q", u, want)
	}
}

func TestBuildWSURLWSSScheme(t *testing.T) {
	u, err := BuildWSURL("wss://relay.example.com", "room1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "wss://relay.example.com/ws?room=room1"
	if u != want {
		t.Errorf("got %q, want %q", u, want)
	}
}

func TestBuildWSURLBadScheme(t *testing.T) {
	if _, err := BuildWSURL("ftp://relay.example.com", ""); err == nil {
		t.Error("expected error for ftp:// scheme")
	}
}

func TestBuildWSURLNoHost(t *testing.T) {
	if _, err := BuildWSURL("http://", ""); err == nil {
		t.Error("expected error for URL with no host")
	}
}

func TestBuildWSURLPortPreserved(t *testing.T) {
	u, err := BuildWSURL("http://localhost:3000", "roomX")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "ws://localhost:3000/ws?room=roomX"
	if u != want {
		t.Errorf("got %q, want %q", u, want)
	}
}
