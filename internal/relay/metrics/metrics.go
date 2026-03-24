// Package metrics provides operator-facing counters for the relay server.
//
// All counters are atomic and safe for concurrent use. The package
// intentionally avoids pulling in heavy observability dependencies —
// it provides a JSON-serializable snapshot that the relay can expose
// at /metrics.
package metrics

import (
	"sync/atomic"
	"time"
)

// Counters tracks relay-level operational counters.
type Counters struct {
	// Connections
	wsConnections      atomic.Int64
	wsConnectionsTotal atomic.Int64

	// Rooms
	roomsCreated atomic.Int64
	roomsJoined  atomic.Int64
	roomsExpired atomic.Int64
	roomsRelayed atomic.Int64

	// Transfers
	bytesRelayed atomic.Int64
	framesRelayed atomic.Int64

	// Rate limiting
	rateLimitHits atomic.Int64

	// Errors
	wsErrors atomic.Int64

	// Signaling
	signalExchanges atomic.Int64

	started time.Time
}

// NewCounters creates a new Counters instance.
func NewCounters() *Counters {
	return &Counters{
		started: time.Now(),
	}
}

// --- Increment methods ---

func (c *Counters) WSConnect()    { c.wsConnections.Add(1); c.wsConnectionsTotal.Add(1) }
func (c *Counters) WSDisconnect() { c.wsConnections.Add(-1) }
func (c *Counters) RoomCreated()  { c.roomsCreated.Add(1) }
func (c *Counters) RoomJoined()   { c.roomsJoined.Add(1) }
func (c *Counters) RoomExpired()  { c.roomsExpired.Add(1) }
func (c *Counters) RoomRelayed()  { c.roomsRelayed.Add(1) }

func (c *Counters) BytesRelayed(n int64)  { c.bytesRelayed.Add(n) }
func (c *Counters) FrameRelayed()         { c.framesRelayed.Add(1) }
func (c *Counters) RateLimitHit()         { c.rateLimitHits.Add(1) }
func (c *Counters) WSError()              { c.wsErrors.Add(1) }
func (c *Counters) SignalExchange()       { c.signalExchanges.Add(1) }

// Snapshot returns a JSON-serializable point-in-time view of all counters.
type Snapshot struct {
	UptimeSeconds         int64 `json:"uptimeSeconds"`
	ActiveWSConnections   int64 `json:"activeWsConnections"`
	TotalWSConnections    int64 `json:"totalWsConnections"`
	RoomsCreated          int64 `json:"roomsCreated"`
	RoomsJoined           int64 `json:"roomsJoined"`
	RoomsExpired          int64 `json:"roomsExpired"`
	RoomsRelayed          int64 `json:"roomsRelayed"`
	BytesRelayed          int64 `json:"bytesRelayed"`
	FramesRelayed         int64 `json:"framesRelayed"`
	RateLimitHits         int64 `json:"rateLimitHits"`
	WSErrors              int64 `json:"wsErrors"`
	SignalExchanges       int64 `json:"signalExchanges"`
}

// Snapshot returns the current counter values.
func (c *Counters) Snapshot() Snapshot {
	return Snapshot{
		UptimeSeconds:       int64(time.Since(c.started).Seconds()),
		ActiveWSConnections: c.wsConnections.Load(),
		TotalWSConnections:  c.wsConnectionsTotal.Load(),
		RoomsCreated:        c.roomsCreated.Load(),
		RoomsJoined:         c.roomsJoined.Load(),
		RoomsExpired:        c.roomsExpired.Load(),
		RoomsRelayed:        c.roomsRelayed.Load(),
		BytesRelayed:        c.bytesRelayed.Load(),
		FramesRelayed:       c.framesRelayed.Load(),
		RateLimitHits:       c.rateLimitHits.Load(),
		WSErrors:            c.wsErrors.Load(),
		SignalExchanges:     c.signalExchanges.Load(),
	}
}
