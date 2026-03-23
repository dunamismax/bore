package rendezvous

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dunamismax/bore/client/internal/code"
	"nhooyr.io/websocket"
)

// These tests cover the parsing/formatting logic of FullRendezvousCode
// (which lives in the code package but is exercised through rendezvous).

type relayTestServer struct {
	t      *testing.T
	srv    *httptest.Server
	nextID uint64

	mu      sync.Mutex
	pending map[string]*relayPeer
}

type relayPeer struct {
	recvCh chan *websocket.Conn
	doneCh chan struct{}
}

func newRelayTestServer(t *testing.T) *relayTestServer {
	t.Helper()

	rts := &relayTestServer{
		t:       t,
		pending: make(map[string]*relayPeer),
	}
	rts.srv = httptest.NewServer(http.HandlerFunc(rts.handleWS))
	t.Cleanup(rts.srv.Close)
	return rts
}

func (r *relayTestServer) URL() string {
	return r.srv.URL
}

func (r *relayTestServer) handleWS(w http.ResponseWriter, req *http.Request) {
	roomID := req.URL.Query().Get("room")
	if roomID == "" {
		r.handleSender(w, req)
		return
	}
	r.handleReceiver(w, req, roomID)
}

func (r *relayTestServer) handleSender(w http.ResponseWriter, req *http.Request) {
	conn, err := websocket.Accept(w, req, nil)
	if err != nil {
		r.t.Logf("sender accept: %v", err)
		return
	}
	defer conn.CloseNow()

	roomID := fmt.Sprintf("relay_%06x", atomic.AddUint64(&r.nextID, 1))
	peer := &relayPeer{
		recvCh: make(chan *websocket.Conn, 1),
		doneCh: make(chan struct{}),
	}

	r.mu.Lock()
	r.pending[roomID] = peer
	r.mu.Unlock()
	defer func() {
		r.mu.Lock()
		delete(r.pending, roomID)
		r.mu.Unlock()
	}()

	if err := conn.Write(req.Context(), websocket.MessageText, []byte(roomID)); err != nil {
		r.t.Logf("sender room id write: %v", err)
		return
	}

	var recvConn *websocket.Conn
	select {
	case recvConn = <-peer.recvCh:
	case <-req.Context().Done():
		return
	}

	if err := relayMessages(req.Context(), conn, recvConn); err != nil {
		r.t.Logf("relay %s: %v", roomID, err)
	}
	close(peer.doneCh)
}

func (r *relayTestServer) handleReceiver(w http.ResponseWriter, req *http.Request, roomID string) {
	r.mu.Lock()
	peer, ok := r.pending[roomID]
	r.mu.Unlock()
	if !ok {
		http.Error(w, "room not found", http.StatusNotFound)
		return
	}

	conn, err := websocket.Accept(w, req, nil)
	if err != nil {
		r.t.Logf("receiver accept: %v", err)
		return
	}
	defer conn.CloseNow()

	select {
	case peer.recvCh <- conn:
	case <-req.Context().Done():
		return
	}

	select {
	case <-peer.doneCh:
	case <-req.Context().Done():
	}
}

func relayMessages(ctx context.Context, a, b *websocket.Conn) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	errCh := make(chan error, 2)
	go func() {
		errCh <- forwardMessages(ctx, a, b)
		cancel()
	}()
	go func() {
		errCh <- forwardMessages(ctx, b, a)
		cancel()
	}()

	var firstErr error
	for range 2 {
		if err := <-errCh; err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func forwardMessages(ctx context.Context, src, dst *websocket.Conn) error {
	for {
		msgType, data, err := src.Read(ctx)
		if err != nil {
			return classifyRelayError(err)
		}
		if err := dst.Write(ctx, msgType, data); err != nil {
			return classifyRelayError(err)
		}
	}
}

func classifyRelayError(err error) error {
	if err == nil {
		return nil
	}

	var closeErr websocket.CloseError
	if errors.As(err, &closeErr) {
		if closeErr.Code == websocket.StatusNormalClosure || closeErr.Code == websocket.StatusGoingAway {
			return nil
		}
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, io.EOF) {
		return nil
	}
	return err
}

func TestGeneratePakeCodeValid(t *testing.T) {
	c, err := code.Generate(code.DefaultWords)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if c.WordCount() != code.DefaultWords {
		t.Errorf("word count = %d, want %d", c.WordCount(), code.DefaultWords)
	}
	if c.Channel() < code.MinChannel || c.Channel() > code.MaxChannel {
		t.Errorf("channel %d out of range", c.Channel())
	}
}

func TestGeneratePakeCodeWordCounts(t *testing.T) {
	for wc := code.MinWords; wc <= code.MaxWords; wc++ {
		c, err := code.Generate(wc)
		if err != nil {
			t.Fatalf("Generate(%d): %v", wc, err)
		}
		if c.WordCount() != wc {
			t.Errorf("Generate(%d): word count = %d", wc, c.WordCount())
		}
	}
}

func TestGeneratePakeCodeInvalidCount(t *testing.T) {
	if _, err := code.Generate(1); err == nil {
		t.Error("expected error for word count 1")
	}
	if _, err := code.Generate(6); err == nil {
		t.Error("expected error for word count 6")
	}
}

func TestDefaultRelayURL(t *testing.T) {
	if DefaultRelayURL == "" {
		t.Error("DefaultRelayURL should not be empty")
	}
}

func TestSendReceiveViaRelay(t *testing.T) {
	relay := newRelayTestServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	payload := []byte("relay integration test payload")
	codeCh := make(chan code.FullRendezvousCode, 1)
	senderDone := make(chan struct {
		result SenderResult
		err    error
	}, 1)

	go func() {
		result, err := SendWithCodeCallback(ctx, relay.URL(), "payload.txt", payload, code.DefaultWords, func(full code.FullRendezvousCode) {
			codeCh <- full
		})
		senderDone <- struct {
			result SenderResult
			err    error
		}{result: result, err: err}
	}()

	var fullCode code.FullRendezvousCode
	select {
	case fullCode = <-codeCh:
	case <-ctx.Done():
		t.Fatalf("timed out waiting for sender code: %v", ctx.Err())
	}

	recvResult, err := Receive(ctx, fullCode.CodeString(), relay.URL())
	if err != nil {
		t.Fatalf("Receive: %v", err)
	}

	var sender struct {
		result SenderResult
		err    error
	}
	select {
	case sender = <-senderDone:
	case <-ctx.Done():
		t.Fatalf("timed out waiting for sender result: %v", ctx.Err())
	}
	if sender.err != nil {
		t.Fatalf("SendWithCodeCallback: %v", sender.err)
	}

	if sender.result.Code.CodeString() != fullCode.CodeString() {
		t.Fatalf("sender code = %q, want %q", sender.result.Code.CodeString(), fullCode.CodeString())
	}
	if recvResult.Transfer.Filename != "payload.txt" {
		t.Fatalf("received filename = %q, want payload.txt", recvResult.Transfer.Filename)
	}
	if string(recvResult.Transfer.Data) != string(payload) {
		t.Fatalf("received data = %q, want %q", recvResult.Transfer.Data, payload)
	}
	if sender.result.Transfer.SHA256 != recvResult.Transfer.SHA256 {
		t.Fatal("sender and receiver SHA-256 hashes differ")
	}
	if sender.result.Transfer.Size != recvResult.Transfer.Size {
		t.Fatalf("size mismatch: sender=%d receiver=%d", sender.result.Transfer.Size, recvResult.Transfer.Size)
	}
}
