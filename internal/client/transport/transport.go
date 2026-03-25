// Package transport defines the transport abstraction for bore and provides
// concrete implementations for relay-based and (future) direct peer-to-peer
// connections.
//
// The core abstraction is [Conn] + [Dialer]:
//
//   - Conn is a bidirectional byte stream (io.ReadWriteCloser) between two peers.
//   - Dialer establishes a Conn for either the sender or receiver role.
//
// Concrete implementations:
//
//   - [RelayDialer]  -- WebSocket transport through a bore relay server.
//   - [DirectDialer] -- UDP transport for direct peer-to-peer (stub, not yet functional).
//   - [Selector]     -- tries direct first, falls back to relay.
package transport

import (
	"context"
	"io"
)

// Conn is a bidirectional byte-stream connection between two peers.
//
// Both the Noise handshake and the transfer engine operate over Conn
// without knowledge of the underlying transport mechanism.
type Conn interface {
	io.ReadWriteCloser
}

// Dialer establishes transport connections for sender and receiver roles.
//
// Each transport implementation (relay, direct, selector) implements Dialer
// so the rendezvous layer can be transport-agnostic.
type Dialer interface {
	// DialSender establishes a connection as the sending peer.
	//
	// For relay transports, sessionID is the room ID assigned by the relay.
	// For direct transports, sessionID is empty (the caller should not
	// rely on it for non-relay transports).
	DialSender(ctx context.Context) (sessionID string, conn Conn, err error)

	// DialReceiver establishes a connection as the receiving peer,
	// joining the session identified by sessionID.
	//
	// For relay transports, sessionID is the room ID.
	// For direct transports, sessionID may be ignored.
	DialReceiver(ctx context.Context, sessionID string) (Conn, error)
}
