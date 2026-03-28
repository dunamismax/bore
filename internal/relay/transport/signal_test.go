package transport

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"testing"
	"time"

	"nhooyr.io/websocket"
)

// TestSignal_CandidateExchange verifies the relay's signaling endpoint
// correctly pairs sender and receiver candidate messages.
func TestSignal_CandidateExchange(t *testing.T) {
	_, ts := testServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// First create a room so we have a room ID.
	sender, roomID := dialSender(t, ctx, ts)
	defer sender.CloseNow()

	type sigMsg struct {
		Type      string           `json:"type"`
		Candidate *json.RawMessage `json:"candidate,omitempty"`
	}

	var wg sync.WaitGroup
	wg.Add(2)

	var senderGotRemote, receiverGotRemote []byte

	// Sender signaling.
	go func() {
		defer wg.Done()
		conn, _, err := websocket.Dial(ctx, wsURL(ts, "/signal?room="+roomID+"&role=sender"), nil)
		if err != nil {
			t.Errorf("sender signal dial: %v", err)
			return
		}
		defer conn.Close(websocket.StatusNormalClosure, "done")

		senderCandidate := `{"type":"candidate","candidate":{"publicAddr":"1.2.3.4:5000","natType":"Port-Restricted Cone"}}`
		if err := conn.Write(ctx, websocket.MessageText, []byte(senderCandidate)); err != nil {
			t.Errorf("sender signal write: %v", err)
			return
		}

		_, data, err := conn.Read(ctx)
		if err != nil {
			t.Errorf("sender signal read: %v", err)
			return
		}
		senderGotRemote = data
	}()

	// Receiver signaling.
	go func() {
		defer wg.Done()
		conn, _, err := websocket.Dial(ctx, wsURL(ts, "/signal?room="+roomID+"&role=receiver"), nil)
		if err != nil {
			t.Errorf("receiver signal dial: %v", err)
			return
		}
		defer conn.Close(websocket.StatusNormalClosure, "done")

		receiverCandidate := `{"type":"candidate","candidate":{"publicAddr":"5.6.7.8:6000","natType":"Full Cone"}}`
		if err := conn.Write(ctx, websocket.MessageText, []byte(receiverCandidate)); err != nil {
			t.Errorf("receiver signal write: %v", err)
			return
		}

		_, data, err := conn.Read(ctx)
		if err != nil {
			t.Errorf("receiver signal read: %v", err)
			return
		}
		receiverGotRemote = data
	}()

	wg.Wait()

	// Sender should have received the receiver's candidate.
	if len(senderGotRemote) == 0 {
		t.Fatal("sender did not receive remote candidate")
	}
	var senderRecvMsg sigMsg
	if err := json.Unmarshal(senderGotRemote, &senderRecvMsg); err != nil {
		t.Fatalf("unmarshal sender's received msg: %v", err)
	}
	if senderRecvMsg.Type != "candidate" {
		t.Errorf("sender got type %q, want %q", senderRecvMsg.Type, "candidate")
	}

	// Receiver should have received the sender's candidate.
	if len(receiverGotRemote) == 0 {
		t.Fatal("receiver did not receive remote candidate")
	}
	var receiverRecvMsg sigMsg
	if err := json.Unmarshal(receiverGotRemote, &receiverRecvMsg); err != nil {
		t.Fatalf("unmarshal receiver's received msg: %v", err)
	}
	if receiverRecvMsg.Type != "candidate" {
		t.Errorf("receiver got type %q, want %q", receiverRecvMsg.Type, "candidate")
	}
}

// TestSignal_NoCandidateExchange verifies the relay handles "no-candidate" messages.
func TestSignal_NoCandidateExchange(t *testing.T) {
	_, ts := testServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	sender, roomID := dialSender(t, ctx, ts)
	defer sender.CloseNow()

	var wg sync.WaitGroup
	wg.Add(2)

	var senderGotRemote []byte

	// Sender sends no-candidate.
	go func() {
		defer wg.Done()
		conn, _, err := websocket.Dial(ctx, wsURL(ts, "/signal?room="+roomID+"&role=sender"), nil)
		if err != nil {
			t.Errorf("sender signal dial: %v", err)
			return
		}
		defer conn.Close(websocket.StatusNormalClosure, "done")

		if err := conn.Write(ctx, websocket.MessageText, []byte(`{"type":"no-candidate"}`)); err != nil {
			t.Errorf("sender signal write: %v", err)
			return
		}

		_, data, err := conn.Read(ctx)
		if err != nil {
			t.Errorf("sender signal read: %v", err)
			return
		}
		senderGotRemote = data
	}()

	// Receiver also sends no-candidate.
	go func() {
		defer wg.Done()
		conn, _, err := websocket.Dial(ctx, wsURL(ts, "/signal?room="+roomID+"&role=receiver"), nil)
		if err != nil {
			t.Errorf("receiver signal dial: %v", err)
			return
		}
		defer conn.Close(websocket.StatusNormalClosure, "done")

		if err := conn.Write(ctx, websocket.MessageText, []byte(`{"type":"no-candidate"}`)); err != nil {
			t.Errorf("receiver signal write: %v", err)
			return
		}

		_, _, err = conn.Read(ctx)
		if err != nil {
			t.Errorf("receiver signal read: %v", err)
			return
		}
	}()

	wg.Wait()

	// Sender should receive "no-candidate" from receiver.
	if len(senderGotRemote) == 0 {
		t.Fatal("sender did not receive remote message")
	}
	type sigMsg struct {
		Type string `json:"type"`
	}
	var msg sigMsg
	if err := json.Unmarshal(senderGotRemote, &msg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if msg.Type != "no-candidate" {
		t.Errorf("got type %q, want %q", msg.Type, "no-candidate")
	}
}

// TestSignal_MissingParams verifies the /signal endpoint rejects bad requests.
func TestSignal_MissingParams(t *testing.T) {
	_, ts := testServer(t)

	tests := []struct {
		name string
		path string
	}{
		{"no room", "/signal?role=sender"},
		{"no role", "/signal?room=abc"},
		{"bad role", "/signal?room=abc&role=admin"},
		{"invalid room format", "/signal?room=bad/id&role=sender"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := http.Get(ts.URL + tt.path)
			if err != nil {
				t.Fatalf("GET %s: %v", tt.path, err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusBadRequest {
				t.Errorf("GET %s: status = %d, want %d", tt.path, resp.StatusCode, http.StatusBadRequest)
			}
		})
	}
}

// TestSignal_Cleanup verifies signaling state is cleaned up after exchange.
func TestSignal_UnknownRoom(t *testing.T) {
	_, ts := testServer(t)

	resp, err := http.Get(ts.URL + "/signal?room=missing-room&role=sender")
	if err != nil {
		t.Fatalf("GET /signal unknown room: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

func TestSignal_Cleanup(t *testing.T) {
	srv, ts := testServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	sender, roomID := dialSender(t, ctx, ts)
	defer sender.CloseNow()

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		conn, _, err := websocket.Dial(ctx, wsURL(ts, "/signal?room="+roomID+"&role=sender"), nil)
		if err != nil {
			return
		}
		defer conn.Close(websocket.StatusNormalClosure, "done")
		conn.Write(ctx, websocket.MessageText, []byte(`{"type":"no-candidate"}`))
		conn.Read(ctx)
	}()

	go func() {
		defer wg.Done()
		conn, _, err := websocket.Dial(ctx, wsURL(ts, "/signal?room="+roomID+"&role=receiver"), nil)
		if err != nil {
			return
		}
		defer conn.Close(websocket.StatusNormalClosure, "done")
		conn.Write(ctx, websocket.MessageText, []byte(`{"type":"no-candidate"}`))
		conn.Read(ctx)
	}()

	wg.Wait()

	// After both peers complete, the signal state for this room should be cleaned up.
	time.Sleep(100 * time.Millisecond) // allow goroutines to complete deferred cleanup

	srv.sigMu.Lock()
	_, exists := srv.signals[roomID]
	srv.sigMu.Unlock()

	if exists {
		t.Error("signal state was not cleaned up after exchange")
	}
}
