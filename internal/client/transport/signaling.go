package transport

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"

	"github.com/dunamismax/bore/internal/roomid"
	"nhooyr.io/websocket"
)

// SignalingMessage is the envelope used for the relay-coordinated candidate exchange.
// Both peers publish their candidate through the relay's WebSocket signaling
// channel. The relay forwards the message to the other peer without inspection.
type SignalingMessage struct {
	// Type discriminates the signaling message.
	Type string `json:"type"`

	// Candidate carries the peer's direct-path candidate when Type is "candidate".
	Candidate *Candidate `json:"candidate,omitempty"`
}

const (
	sigTypeCandidate = "candidate"
	sigTypeNone      = "no-candidate"
)

// ExchangeCandidates performs the relay-coordinated candidate exchange.
//
// Both peers:
//  1. Connect to the relay's signaling endpoint: /signal?room=ROOM_ID&role=sender|receiver
//  2. Publish their local candidate (or a "no-candidate" message if STUN failed).
//  3. Read the remote peer's candidate.
//
// The returned remote candidate is nil if the peer published "no-candidate".
func ExchangeCandidates(ctx context.Context, relayURL, roomID, role string, local *Candidate) (*Candidate, error) {
	wsURL, err := buildSignalURL(relayURL, roomID, role)
	if err != nil {
		return nil, fmt.Errorf("build signal URL: %w", err)
	}

	conn, _, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{
		CompressionMode: websocket.CompressionDisabled,
	})
	if err != nil {
		return nil, fmt.Errorf("connect signaling: %w", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "done")

	conn.SetReadLimit(64 * 1024) // signaling messages are small

	// Publish local candidate.
	var msg SignalingMessage
	if local != nil {
		msg = SignalingMessage{Type: sigTypeCandidate, Candidate: local}
	} else {
		msg = SignalingMessage{Type: sigTypeNone}
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("marshal signaling message: %w", err)
	}

	if err := conn.Write(ctx, websocket.MessageText, data); err != nil {
		return nil, fmt.Errorf("publish candidate: %w", err)
	}

	slog.Debug("signaling: published candidate", "role", role, "has_candidate", local != nil)

	// Read remote candidate.
	_, remoteData, err := conn.Read(ctx)
	if err != nil {
		return nil, fmt.Errorf("read remote candidate: %w", err)
	}

	var remoteMsg SignalingMessage
	if err := json.Unmarshal(remoteData, &remoteMsg); err != nil {
		return nil, fmt.Errorf("unmarshal remote candidate: %w", err)
	}

	if remoteMsg.Type == sigTypeNone || remoteMsg.Candidate == nil {
		slog.Debug("signaling: remote peer has no candidate", "role", role)
		return nil, nil
	}

	slog.Debug("signaling: received remote candidate",
		"role", role,
		"remote_addr", remoteMsg.Candidate.PublicAddr,
		"remote_nat", remoteMsg.Candidate.NATType.String(),
	)

	return remoteMsg.Candidate, nil
}

// buildSignalURL constructs the WebSocket URL for the signaling endpoint.
func buildSignalURL(relayURL, roomID, role string) (string, error) {
	if err := roomid.Validate(roomID); err != nil {
		return "", fmt.Errorf("invalid room ID: %w", err)
	}
	if role != "sender" && role != "receiver" {
		return "", fmt.Errorf("invalid role %q", role)
	}

	u, err := url.Parse(relayURL)
	if err != nil {
		return "", fmt.Errorf("parse relay URL: %w", err)
	}
	if u.Host == "" {
		return "", fmt.Errorf("relay URL has no host: %q", relayURL)
	}

	switch u.Scheme {
	case "http", "ws":
		u.Scheme = "ws"
	case "https", "wss":
		u.Scheme = "wss"
	default:
		return "", fmt.Errorf("unsupported relay URL scheme: %q", u.Scheme)
	}

	u.Path = "/signal"
	q := url.Values{}
	q.Set("room", roomID)
	q.Set("role", role)
	u.RawQuery = q.Encode()

	return u.String(), nil
}
