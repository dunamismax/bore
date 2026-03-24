package transport

import (
	"context"
	"errors"
	"io"

	"github.com/dunamismax/bore/internal/relay/metrics"
	"nhooyr.io/websocket"
)

// Relay bidirectionally forwards WebSocket frames between two connections
// until one side disconnects or the context is canceled. Frames are
// forwarded as-is with no inspection or transformation.
//
// Back-pressure is handled naturally: each direction runs in a goroutine
// that reads a single frame, writes it to the peer, and only then reads
// the next frame. If the receiver is slow, the reader blocks on write,
// which backs up into the read, which applies TCP-level back-pressure.
// No intermediate buffering beyond a single frame per direction.
func Relay(ctx context.Context, a, b *websocket.Conn, counters *metrics.Counters) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	errc := make(chan error, 2)

	go func() {
		errc <- forward(ctx, a, b, counters)
		cancel() // if one direction ends, cancel the other
	}()

	go func() {
		errc <- forward(ctx, b, a, counters)
		cancel()
	}()

	// Wait for both goroutines. Return the first meaningful error.
	var firstErr error
	for range 2 {
		if err := <-errc; err != nil && firstErr == nil {
			firstErr = err
		}
	}

	// Attempt clean close handshake on both connections.
	// Use StatusNormalClosure if we ended cleanly.
	code := websocket.StatusNormalClosure
	reason := "relay closed"
	if firstErr != nil {
		code = websocket.StatusGoingAway
		reason = "peer disconnected"
	}
	a.Close(code, reason)
	b.Close(code, reason)

	return firstErr
}

// forward reads frames from src and writes them to dst until the context
// is canceled or an error occurs. It handles binary and text frames
// identically — forwarding as-is.
func forward(ctx context.Context, src, dst *websocket.Conn, counters *metrics.Counters) error {
	for {
		typ, reader, err := src.Reader(ctx)
		if err != nil {
			return classifyError(err)
		}

		w, err := dst.Writer(ctx, typ)
		if err != nil {
			return classifyError(err)
		}

		// io.Copy streams the frame content from reader to writer.
		// This handles back-pressure: Copy blocks on write if the
		// destination is slow, which backs up into the read.
		n, err := io.Copy(w, reader)
		if err != nil {
			w.Close()
			return classifyError(err)
		}

		if err := w.Close(); err != nil {
			return classifyError(err)
		}

		if counters != nil {
			counters.BytesRelayed(n)
			counters.FrameRelayed()
		}
	}
}

// classifyError converts WebSocket close errors into nil for normal
// closures, preserving actual errors.
func classifyError(err error) error {
	if err == nil {
		return nil
	}

	// Normal close or context cancellation are not errors.
	var closeErr websocket.CloseError
	if errors.As(err, &closeErr) {
		if closeErr.Code == websocket.StatusNormalClosure ||
			closeErr.Code == websocket.StatusGoingAway {
			return nil
		}
	}

	if errors.Is(err, context.Canceled) {
		return nil
	}

	if errors.Is(err, io.EOF) {
		return nil
	}

	return err
}
