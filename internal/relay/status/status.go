// Package status defines the Go-owned relay health and status payloads consumed by shipped v1 surfaces.
package status

const (
	ServiceName = "bore-relay"
	SteadyState = "ok"
)

// HealthResponse is the Go-owned contract for the relay /healthz endpoint.
type HealthResponse struct {
	Service string `json:"service"`
	Status  string `json:"status"`
}

// Response is the Go-owned contract for the relay /status endpoint.
//
// Current consumers:
// - web/ /ops/relay live status island
// - cmd/bore-admin status
type Response struct {
	Service       string    `json:"service"`
	Status        string    `json:"status"`
	UptimeSeconds int64     `json:"uptimeSeconds"`
	Rooms         Rooms     `json:"rooms"`
	Limits        Limits    `json:"limits"`
	Transport     Transport `json:"transport"`
}

type Rooms struct {
	Total   int `json:"total"`
	Waiting int `json:"waiting"`
	Active  int `json:"active"`
}

type Transport struct {
	SignalExchanges  int64 `json:"signalExchanges"`
	SignalingStarted int64 `json:"signalingStarted"`
	RoomsRelayed     int64 `json:"roomsRelayed"`
	BytesRelayed     int64 `json:"bytesRelayed"`
	FramesRelayed    int64 `json:"framesRelayed"`
}

type Limits struct {
	MaxRooms            int   `json:"maxRooms"`
	RoomTTLSeconds      int64 `json:"roomTTLSeconds"`
	ReapIntervalSeconds int64 `json:"reapIntervalSeconds"`
	MaxMessageSizeBytes int64 `json:"maxMessageSizeBytes"`
}
