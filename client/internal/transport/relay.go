package transport

import (
	"context"
	"fmt"
	"net/url"

	"nhooyr.io/websocket"
)

const maxMessageBytes = 64 * 1024 * 1024 // 64 MB

// RelayDialer implements [Dialer] using a WebSocket relay server.
//
// The relay protocol:
//   - Sender connects to ws://relay/ws (no query params), relay sends room ID as
//     the first text message.
//   - Receiver connects to ws://relay/ws?room=ROOM_ID, relay pairs them.
//   - After pairing, the relay forwards WebSocket frames bidirectionally
//     (zero-knowledge: the relay never sees plaintext).
type RelayDialer struct {
	// RelayURL is the HTTP(S) URL of the relay server.
	RelayURL string
}

// DialSender connects to the relay as a sender.
//
// Creates a new room on the relay. The relay sends the room ID as the first
// text message after the WebSocket connection is established.
// Returns the room ID and a [Conn] backed by the WebSocket connection.
func (d *RelayDialer) DialSender(ctx context.Context) (string, Conn, error) {
	wsURL, err := BuildWSURL(d.RelayURL, "")
	if err != nil {
		return "", nil, fmt.Errorf("build sender URL: %w", err)
	}

	conn, _, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{
		CompressionMode: websocket.CompressionDisabled,
	})
	if err != nil {
		return "", nil, fmt.Errorf("connect to relay: %w", err)
	}
	conn.SetReadLimit(maxMessageBytes)

	// The relay sends the room ID as the first text message.
	msgType, data, err := conn.Read(ctx)
	if err != nil {
		conn.Close(websocket.StatusInternalError, "read room ID failed")
		return "", nil, fmt.Errorf("read room ID: %w", err)
	}
	if msgType != websocket.MessageText {
		conn.Close(websocket.StatusUnsupportedData, "expected text room ID")
		return "", nil, fmt.Errorf("expected text room ID message, got type %v", msgType)
	}

	roomID := string(data)
	return roomID, &wsConn{conn: conn, ctx: ctx}, nil
}

// DialReceiver connects to the relay as a receiver, joining an existing room.
// Returns a [Conn] backed by the WebSocket connection.
func (d *RelayDialer) DialReceiver(ctx context.Context, roomID string) (Conn, error) {
	wsURL, err := BuildWSURL(d.RelayURL, roomID)
	if err != nil {
		return nil, fmt.Errorf("build receiver URL: %w", err)
	}

	conn, _, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{
		CompressionMode: websocket.CompressionDisabled,
	})
	if err != nil {
		return nil, fmt.Errorf("connect to relay: %w", err)
	}
	conn.SetReadLimit(maxMessageBytes)

	return &wsConn{conn: conn, ctx: ctx}, nil
}

// wsConn adapts a *websocket.Conn to [Conn] (io.ReadWriteCloser).
// Each Write becomes a single binary WebSocket message.
// Read buffers incoming messages and serves bytes.
type wsConn struct {
	conn *websocket.Conn
	ctx  context.Context
	buf  []byte // unread bytes from the most recently received message
	pos  int    // current read position in buf
}

// Read implements io.Reader.
// Drains the internal buffer first; if empty, receives the next WebSocket message.
func (c *wsConn) Read(p []byte) (int, error) {
	for c.pos >= len(c.buf) {
		_, data, err := c.conn.Read(c.ctx)
		if err != nil {
			return 0, err
		}
		if len(data) == 0 {
			continue // skip empty messages, loop for next
		}
		c.buf = data
		c.pos = 0
	}
	n := copy(p, c.buf[c.pos:])
	c.pos += n
	return n, nil
}

// Write implements io.Writer.
// Sends p as a single binary WebSocket message.
func (c *wsConn) Write(p []byte) (int, error) {
	if err := c.conn.Write(c.ctx, websocket.MessageBinary, p); err != nil {
		return 0, fmt.Errorf("websocket write: %w", err)
	}
	return len(p), nil
}

// Close implements io.Closer.
// Sends a normal closure to the WebSocket peer.
func (c *wsConn) Close() error {
	return c.conn.Close(websocket.StatusNormalClosure, "done")
}

// BuildWSURL converts a relay HTTP(S) URL to a WebSocket URL and optionally
// appends a room query parameter.
//
// Conversions:
//
//	http://  ->  ws://
//	https:// ->  wss://
//	ws://    ->  ws://    (pass-through)
//	wss://   ->  wss://   (pass-through)
//
// Always appends "/ws" path. Appends "?room=roomID" if roomID is non-empty.
func BuildWSURL(relayURL, roomID string) (string, error) {
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

	u.Path = "/ws"

	if roomID != "" {
		q := url.Values{}
		q.Set("room", roomID)
		u.RawQuery = q.Encode()
	} else {
		u.RawQuery = ""
	}

	return u.String(), nil
}

// ConnectAsSender is a convenience wrapper for callers that don't use [Dialer].
// Deprecated: prefer [RelayDialer.DialSender] for new code.
func ConnectAsSender(ctx context.Context, relayURL string) (string, Conn, error) {
	d := &RelayDialer{RelayURL: relayURL}
	return d.DialSender(ctx)
}

// ConnectAsReceiver is a convenience wrapper for callers that don't use [Dialer].
// Deprecated: prefer [RelayDialer.DialReceiver] for new code.
func ConnectAsReceiver(ctx context.Context, relayURL, roomID string) (Conn, error) {
	d := &RelayDialer{RelayURL: relayURL}
	return d.DialReceiver(ctx, roomID)
}
