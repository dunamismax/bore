package transport

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dunamismax/bore/services/relay/internal/room"
	"nhooyr.io/websocket"
)

// testServer creates a relay server backed by an httptest.Server.
func testServer(t *testing.T) (*Server, *httptest.Server) {
	t.Helper()
	reg := room.NewRegistry(room.DefaultRegistryConfig())
	ctx, cancel := context.WithCancel(context.Background())
	reg.RunReaper(ctx)
	t.Cleanup(cancel)

	srv := NewServer(ServerConfig{
		Registry: reg,
	})
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)
	return srv, ts
}

// wsURL converts an httptest.Server URL to a WebSocket URL with optional query params.
func wsURL(ts *httptest.Server, path string) string {
	return "ws" + strings.TrimPrefix(ts.URL, "http") + path
}

func httpGetJSON[T any](t *testing.T, url string) T {
	t.Helper()

	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET %s: status = %d, want %d", url, resp.StatusCode, http.StatusOK)
	}

	var decoded T
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		t.Fatalf("decode %s: %v", url, err)
	}
	return decoded
}

func TestRelay_StatusEndpoints(t *testing.T) {
	_, ts := testServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	health := httpGetJSON[healthResponse](t, ts.URL+"/healthz")
	if health.Service != "bore-relay" || health.Status != "ok" {
		t.Fatalf("health = %+v, want service bore-relay / status ok", health)
	}

	status := httpGetJSON[statusResponse](t, ts.URL+"/status")
	if status.Service != "bore-relay" || status.Status != "ok" {
		t.Fatalf("status = %+v, want service bore-relay / status ok", status)
	}
	if status.Rooms.Total != 0 || status.Rooms.Waiting != 0 || status.Rooms.Active != 0 {
		t.Fatalf("empty relay status = %+v, want zero rooms", status.Rooms)
	}
	if status.Limits.MaxMessageSizeBytes != maxMessageSize {
		t.Fatalf("max message size = %d, want %d", status.Limits.MaxMessageSizeBytes, maxMessageSize)
	}

	sender, _ := dialSender(t, ctx, ts)
	defer sender.CloseNow()

	status = httpGetJSON[statusResponse](t, ts.URL+"/status")
	if status.Rooms.Total != 1 || status.Rooms.Waiting != 1 || status.Rooms.Active != 0 {
		t.Fatalf("waiting relay status = %+v, want total=1 waiting=1 active=0", status.Rooms)
	}
}

// dialSender connects as a sender and returns the WebSocket conn and the room ID.
func dialSender(t *testing.T, ctx context.Context, ts *httptest.Server) (*websocket.Conn, string) {
	t.Helper()
	conn, _, err := websocket.Dial(ctx, wsURL(ts, "/ws"), nil)
	if err != nil {
		t.Fatalf("sender dial: %v", err)
	}

	// Read the room ID (first text message from server).
	typ, data, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("sender read room ID: %v", err)
	}
	if typ != websocket.MessageText {
		t.Fatalf("expected text message for room ID, got %v", typ)
	}
	roomID := string(data)
	if len(roomID) == 0 {
		t.Fatal("got empty room ID")
	}
	return conn, roomID
}

// dialReceiver connects as a receiver with the given room ID.
func dialReceiver(t *testing.T, ctx context.Context, ts *httptest.Server, roomID string) *websocket.Conn {
	t.Helper()
	conn, _, err := websocket.Dial(ctx, wsURL(ts, "/ws?room="+roomID), nil)
	if err != nil {
		t.Fatalf("receiver dial: %v", err)
	}
	return conn
}

func TestRelay_BasicBinaryRoundTrip(t *testing.T) {
	_, ts := testServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	sender, roomID := dialSender(t, ctx, ts)
	defer sender.CloseNow()
	receiver := dialReceiver(t, ctx, ts, roomID)
	defer receiver.CloseNow()

	// Sender writes binary data, receiver reads it.
	want := []byte("hello from sender")
	if err := sender.Write(ctx, websocket.MessageBinary, want); err != nil {
		t.Fatalf("sender write: %v", err)
	}

	typ, got, err := receiver.Read(ctx)
	if err != nil {
		t.Fatalf("receiver read: %v", err)
	}
	if typ != websocket.MessageBinary {
		t.Fatalf("expected binary message, got %v", typ)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("data mismatch: got %q, want %q", got, want)
	}

	// Receiver writes back, sender reads it.
	reply := []byte("hello from receiver")
	if err := receiver.Write(ctx, websocket.MessageBinary, reply); err != nil {
		t.Fatalf("receiver write: %v", err)
	}

	typ, got, err = sender.Read(ctx)
	if err != nil {
		t.Fatalf("sender read: %v", err)
	}
	if typ != websocket.MessageBinary {
		t.Fatalf("expected binary message, got %v", typ)
	}
	if !bytes.Equal(got, reply) {
		t.Fatalf("data mismatch: got %q, want %q", got, reply)
	}

	// Clean close.
	sender.Close(websocket.StatusNormalClosure, "done")
}

func TestRelay_MultipleFrames(t *testing.T) {
	_, ts := testServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	sender, roomID := dialSender(t, ctx, ts)
	defer sender.CloseNow()
	receiver := dialReceiver(t, ctx, ts, roomID)
	defer receiver.CloseNow()

	const numFrames = 50

	// Sender sends multiple frames.
	go func() {
		for i := range numFrames {
			data := []byte{byte(i)}
			if err := sender.Write(ctx, websocket.MessageBinary, data); err != nil {
				return
			}
		}
	}()

	// Receiver reads them all.
	for i := range numFrames {
		_, got, err := receiver.Read(ctx)
		if err != nil {
			t.Fatalf("frame %d read: %v", i, err)
		}
		if len(got) != 1 || got[0] != byte(i) {
			t.Fatalf("frame %d: got %v, want [%d]", i, got, i)
		}
	}

	sender.Close(websocket.StatusNormalClosure, "done")
}

func TestRelay_LargeTransfer(t *testing.T) {
	_, ts := testServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	sender, roomID := dialSender(t, ctx, ts)
	defer sender.CloseNow()
	receiver := dialReceiver(t, ctx, ts, roomID)
	defer receiver.CloseNow()

	// Set read limits high enough for the test.
	sender.SetReadLimit(maxMessageSize)
	receiver.SetReadLimit(maxMessageSize)

	// Send 4 MB of random data in one frame.
	const size = 4 << 20
	want := make([]byte, size)
	if _, err := rand.Read(want); err != nil {
		t.Fatalf("rand.Read: %v", err)
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- sender.Write(ctx, websocket.MessageBinary, want)
	}()

	typ, got, err := receiver.Read(ctx)
	if err != nil {
		t.Fatalf("receiver read: %v", err)
	}
	if typ != websocket.MessageBinary {
		t.Fatalf("expected binary, got %v", typ)
	}
	if !bytes.Equal(got, want) {
		t.Fatal("large transfer: data mismatch")
	}

	if err := <-errCh; err != nil {
		t.Fatalf("sender write: %v", err)
	}

	sender.Close(websocket.StatusNormalClosure, "done")
}

func TestRelay_MultiMBStreaming(t *testing.T) {
	_, ts := testServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	sender, roomID := dialSender(t, ctx, ts)
	defer sender.CloseNow()
	receiver := dialReceiver(t, ctx, ts, roomID)
	defer receiver.CloseNow()

	sender.SetReadLimit(maxMessageSize)
	receiver.SetReadLimit(maxMessageSize)

	// Stream 8 MB in 1 MB chunks.
	const chunkSize = 1 << 20
	const numChunks = 8

	var wg sync.WaitGroup
	wg.Add(1)

	// Generate deterministic data.
	allChunks := make([][]byte, numChunks)
	for i := range numChunks {
		allChunks[i] = make([]byte, chunkSize)
		rand.Read(allChunks[i])
	}

	go func() {
		defer wg.Done()
		for _, chunk := range allChunks {
			if err := sender.Write(ctx, websocket.MessageBinary, chunk); err != nil {
				return
			}
		}
		sender.Close(websocket.StatusNormalClosure, "done")
	}()

	for i := range numChunks {
		_, got, err := receiver.Read(ctx)
		if err != nil {
			t.Fatalf("chunk %d read: %v", i, err)
		}
		if !bytes.Equal(got, allChunks[i]) {
			t.Fatalf("chunk %d: data mismatch", i)
		}
	}

	wg.Wait()
}

func TestRelay_SenderDisconnectMidTransfer(t *testing.T) {
	_, ts := testServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	sender, roomID := dialSender(t, ctx, ts)
	receiver := dialReceiver(t, ctx, ts, roomID)
	defer receiver.CloseNow()

	// Sender sends one frame then abruptly disconnects.
	if err := sender.Write(ctx, websocket.MessageBinary, []byte("before-drop")); err != nil {
		t.Fatalf("sender write: %v", err)
	}

	// Read the first frame.
	_, got, err := receiver.Read(ctx)
	if err != nil {
		t.Fatalf("receiver read: %v", err)
	}
	if !bytes.Equal(got, []byte("before-drop")) {
		t.Fatalf("got %q, want %q", got, "before-drop")
	}

	// Abrupt disconnect (not a clean close).
	sender.CloseNow()

	// Receiver should get an error or clean close on next read.
	_, _, err = receiver.Read(ctx)
	if err == nil {
		t.Fatal("expected error on read after sender disconnect")
	}
}

func TestRelay_ReceiverDisconnectMidTransfer(t *testing.T) {
	_, ts := testServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	sender, roomID := dialSender(t, ctx, ts)
	defer sender.CloseNow()
	receiver := dialReceiver(t, ctx, ts, roomID)

	// Exchange one frame successfully.
	if err := sender.Write(ctx, websocket.MessageBinary, []byte("ping")); err != nil {
		t.Fatalf("sender write: %v", err)
	}
	_, _, err := receiver.Read(ctx)
	if err != nil {
		t.Fatalf("receiver read: %v", err)
	}

	// Receiver abruptly disconnects.
	receiver.CloseNow()

	// Sender should eventually get an error on write.
	// Write in a loop because the first write may succeed if the relay
	// hasn't detected the disconnect yet.
	for range 100 {
		err := sender.Write(ctx, websocket.MessageBinary, []byte("data"))
		if err != nil {
			return // expected
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("expected error after receiver disconnect, but writes kept succeeding")
}

func TestRelay_BinaryIntegrity(t *testing.T) {
	_, ts := testServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	sender, roomID := dialSender(t, ctx, ts)
	defer sender.CloseNow()
	receiver := dialReceiver(t, ctx, ts, roomID)
	defer receiver.CloseNow()

	// Test that all byte values survive relay (no UTF-8 mangling, no
	// frame type conversion, etc.).
	all256 := make([]byte, 256)
	for i := range 256 {
		all256[i] = byte(i)
	}

	if err := sender.Write(ctx, websocket.MessageBinary, all256); err != nil {
		t.Fatalf("sender write: %v", err)
	}

	typ, got, err := receiver.Read(ctx)
	if err != nil {
		t.Fatalf("receiver read: %v", err)
	}
	if typ != websocket.MessageBinary {
		t.Fatalf("expected binary, got %v", typ)
	}
	if !bytes.Equal(got, all256) {
		t.Fatal("all-byte-values payload was corrupted during relay")
	}
}

func TestRelay_BidirectionalConcurrent(t *testing.T) {
	_, ts := testServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	sender, roomID := dialSender(t, ctx, ts)
	defer sender.CloseNow()
	receiver := dialReceiver(t, ctx, ts, roomID)
	defer receiver.CloseNow()

	const numMsgs = 30

	var wg sync.WaitGroup
	wg.Add(4)

	// sender→receiver direction
	go func() {
		defer wg.Done()
		for i := range numMsgs {
			sender.Write(ctx, websocket.MessageBinary, []byte{byte(i), 0xAA})
		}
	}()
	go func() {
		defer wg.Done()
		for range numMsgs {
			_, _, err := receiver.Read(ctx)
			if err != nil {
				return
			}
		}
	}()

	// receiver→sender direction
	go func() {
		defer wg.Done()
		for i := range numMsgs {
			receiver.Write(ctx, websocket.MessageBinary, []byte{byte(i), 0xBB})
		}
	}()
	go func() {
		defer wg.Done()
		for range numMsgs {
			_, _, err := sender.Read(ctx)
			if err != nil {
				return
			}
		}
	}()

	wg.Wait()
}

func TestRelay_InvalidRoomID(t *testing.T) {
	_, ts := testServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Try to join a room that doesn't exist — should get an HTTP error before upgrade.
	_, resp, err := websocket.Dial(ctx, wsURL(ts, "/ws?room=nonexistent"), nil)
	if err == nil {
		t.Fatal("expected error for nonexistent room")
	}
	if resp != nil && resp.StatusCode != 404 {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestRelay_StreamingReaderWriter(t *testing.T) {
	_, ts := testServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	sender, roomID := dialSender(t, ctx, ts)
	defer sender.CloseNow()
	receiver := dialReceiver(t, ctx, ts, roomID)
	defer receiver.CloseNow()

	// Use the streaming Writer/Reader API to verify io.Copy-style
	// relay works correctly.
	go func() {
		w, err := sender.Writer(ctx, websocket.MessageBinary)
		if err != nil {
			return
		}
		// Write in small chunks to a single frame.
		for range 100 {
			w.Write([]byte("chunk"))
		}
		w.Close()
	}()

	typ, reader, err := receiver.Reader(ctx)
	if err != nil {
		t.Fatalf("receiver reader: %v", err)
	}
	if typ != websocket.MessageBinary {
		t.Fatalf("expected binary, got %v", typ)
	}

	data, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("receiver readall: %v", err)
	}

	want := bytes.Repeat([]byte("chunk"), 100)
	if !bytes.Equal(data, want) {
		t.Fatalf("streaming relay: got %d bytes, want %d", len(data), len(want))
	}
}

func TestRelay_CleanCloseHandshake(t *testing.T) {
	_, ts := testServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	sender, roomID := dialSender(t, ctx, ts)
	receiver := dialReceiver(t, ctx, ts, roomID)

	// Sender initiates clean close.
	sender.Close(websocket.StatusNormalClosure, "done")

	// Receiver should receive a close and not hang forever.
	_, _, err := receiver.Read(ctx)
	if err == nil {
		t.Fatal("expected error after peer closed")
	}

	receiver.CloseNow()
}
