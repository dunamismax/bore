package punch

import "errors"

var (
	// ErrUnpunchable indicates the NAT combination does not support hole-punching.
	// Typically both peers are behind Symmetric NATs.
	ErrUnpunchable = errors.New("punch: NAT combination is unpunchable")

	// ErrTimeout indicates the hole-punch attempt timed out without establishing
	// bidirectional communication.
	ErrTimeout = errors.New("punch: attempt timed out")

	// ErrMaxAttempts indicates the maximum number of punch attempts was reached
	// without receiving a response from the peer.
	ErrMaxAttempts = errors.New("punch: max attempts exhausted")

	// ErrHandshakeFailed indicates the punch packets were received but the
	// verification handshake did not complete successfully.
	ErrHandshakeFailed = errors.New("punch: verification handshake failed")

	// ErrInvalidPeer indicates the peer address is nil or invalid.
	ErrInvalidPeer = errors.New("punch: invalid peer address")

	// ErrConnectionClosed indicates the UDP connection was closed during the punch attempt.
	ErrConnectionClosed = errors.New("punch: connection closed")
)
