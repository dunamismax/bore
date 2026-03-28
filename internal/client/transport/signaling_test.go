package transport

import (
	"testing"
)

func TestBuildSignalURL(t *testing.T) {
	tests := []struct {
		name    string
		relay   string
		room    string
		role    string
		want    string
		wantErr bool
	}{
		{
			name:  "http sender",
			relay: "http://localhost:8080",
			room:  "abc123",
			role:  "sender",
			want:  "ws://localhost:8080/signal?role=sender&room=abc123",
		},
		{
			name:  "https receiver",
			relay: "https://relay.example.com",
			room:  "room42",
			role:  "receiver",
			want:  "wss://relay.example.com/signal?role=receiver&room=room42",
		},
		{
			name:  "ws passthrough",
			relay: "ws://relay.local:9090",
			room:  "r1",
			role:  "sender",
			want:  "ws://relay.local:9090/signal?role=sender&room=r1",
		},
		{
			name:  "wss passthrough",
			relay: "wss://relay.example.com",
			room:  "r2",
			role:  "receiver",
			want:  "wss://relay.example.com/signal?role=receiver&room=r2",
		},
		{
			name:    "bad scheme",
			relay:   "ftp://relay.example.com",
			room:    "r1",
			role:    "sender",
			wantErr: true,
		},
		{
			name:    "no host",
			relay:   "http://",
			room:    "r1",
			role:    "sender",
			wantErr: true,
		},
		{
			name:    "invalid room",
			relay:   "http://localhost:8080",
			room:    "bad/id",
			role:    "sender",
			wantErr: true,
		},
		{
			name:    "invalid role",
			relay:   "http://localhost:8080",
			room:    "r1",
			role:    "admin",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := buildSignalURL(tt.relay, tt.room, tt.role)
			if (err != nil) != tt.wantErr {
				t.Errorf("buildSignalURL() err=%v, wantErr=%v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("buildSignalURL() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSignalingMessageTypes(t *testing.T) {
	// Verify constants are as expected.
	if sigTypeCandidate != "candidate" {
		t.Errorf("sigTypeCandidate = %q, want %q", sigTypeCandidate, "candidate")
	}
	if sigTypeNone != "no-candidate" {
		t.Errorf("sigTypeNone = %q, want %q", sigTypeNone, "no-candidate")
	}
}
