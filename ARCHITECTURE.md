# ARCHITECTURE.md

Technical architecture and design notes for `bore`.

This document describes the current repo architecture. For build and verification commands, see [README.md](README.md).

---

## System Overview

The repo is a single root Go module: `github.com/dunamismax/bore`.

bore is a **P2P-first, relay-fallback** encrypted file transfer tool. The default transfer path attempts a direct peer-to-peer connection. The relay serves as a signaling server for the P2P connection and as a fallback transport when direct fails.

```text
┌─────────────┐                           ┌─────────────┐
│   Sender    │                           │  Receiver   │
│ bore client │                           │ bore client │
│   (Go)      │                           │   (Go)      │
└──────┬──────┘                           └──────┬──────┘
       │                                         │
       │    1. STUN discovery (each peer)        │
       │    2. Candidate exchange via relay       │
       │    3. Direct UDP hole-punch              │
       │    4. E2E encrypted file transfer        │
       │◄───────────────────────────────────────►│
       │                                         │
       │    (fallback: relay forwards encrypted  │
       │     bytes if direct connection fails)    │
       │                                         │
                ┌──────────────┐
                │ Go relay     │
                │ (signaling + │
                │  fallback)   │
                └──────┬───────┘
                       │
                ┌──────▼──────┐
                │ frontend    │
                │ FastAPI +   │
                │ htmx        │
                └─────────────┘
```

### Component roles

1. **Client (`cmd/bore` + `internal/client/`)** generates or parses a rendezvous code, discovers peers via STUN, exchanges candidates through the relay's signaling channel, attempts direct hole-punching, falls back to relay if needed, performs the Noise handshake, and streams encrypted file data.
2. **Relay (`cmd/relay` + `internal/relay/`)** provides the signaling endpoint for P2P candidate exchange, serves as a fallback transport by forwarding encrypted frames over WebSockets, manages rooms, and serves the embedded browser surface.
3. **Frontend (`frontend/`)** provides the product-facing homepage and a read-only relay ops page, built with Python + FastAPI + Jinja2 + htmx (no JavaScript build step).
4. **Punchthrough (`cmd/punchthrough` + `internal/punchthrough/`)** contains STUN and UDP hole-punching primitives, integrated into the client's default transport path.
5. **bore-admin (`cmd/bore-admin`)** is a minimal operator CLI that queries relay status.

---

## Repository Map

```text
cmd/
├── bore/                    # CLI entry point
├── bore-admin/              # operator CLI entry point
├── punchthrough/            # NAT tooling CLI entry point
└── relay/                   # relay entry point

internal/
├── client/
│   ├── code/                # rendezvous code generation/parsing
│   ├── crypto/              # Noise handshake + secure channel
│   ├── engine/              # transfer framing, send/receive engine
│   ├── rendezvous/          # session orchestration
│   └── transport/           # transport selector, direct, relay, signaling
├── punchthrough/
│   ├── punch/               # NAT classification + punch engine
│   └── stun/                # STUN client and probing primitives
├── relay/
│   ├── metrics/             # atomic operator-facing counters
│   ├── ratelimit/           # per-IP token bucket rate limiting
│   ├── room/                # room lifecycle and registry
│   ├── transport/           # WebSocket server + signaling + frame forwarding
│   └── webui/               # minimal placeholder handler (dashboard served by frontend/)
└── roomid/                  # shared relay room ID validation rules

frontend/
├── src/
│   └── app/
│       ├── routes/          # FastAPI route handlers
│       ├── templates/       # Jinja2 templates with htmx
│       └── static/          # CSS (Tailwind CDN)
├── tests/                   # pytest test coverage
└── pyproject.toml           # uv, ruff, pyright, pytest config
```

---

## Client Architecture (`cmd/bore` + `internal/client/`)

### Layering

```text
CLI
  ↓
rendezvous orchestration
  ↓
transport selector (direct-first, relay-fallback)
  ↓
┌─────────────────────────┬─────────────────────────┐
│ direct transport        │ relay transport          │
│ gather -> signal -> punch │ WebSocket relay          │
│ -> QUIC (default)        │ -> wsConn                 │
│ -> ReliableConn (legacy) │                          │
└─────────────────────────┴─────────────────────────┘
  ↓
metrics tracking (MetricsConn)
  ↓
crypto + transfer engine
  ↓
io.ReadWriter (transport-agnostic)
```

### Transport selection flow

```text
1. Selector receives dial request
2. Multi-candidate gathering: host + STUN server-reflexive candidates
3. Exchange candidates via relay /signal endpoint
4. Evaluate NAT combination for hole-punch feasibility
5. Attempt UDP hole-punch
6. On success -> QUIC transport over punched socket (default)
7. On QUIC failure -> ReliableConn fallback over punched socket
8. On punch failure -> relay transport (wsConn)

All paths deliver an io.ReadWriteCloser wrapped in MetricsConn
to the crypto layer. The Noise handshake and transfer engine
are transport-agnostic.
```

### Package responsibilities

#### `cmd/bore`

Owns:

- CLI argument parsing
- user-facing help and status output
- selecting `send`, `receive`, `status`, and `components` flows
- constructing the transport `Selector` with `EnableDirect: true` by default

Does not own:

- cryptographic state machine internals
- transfer framing logic
- relay room lifecycle

#### `internal/client/code`

Owns:

- human-readable rendezvous code generation
- rendezvous code parsing and validation
- entropy model for channel + word count
- formatting for the full code shown to users

Important design rule:

- the rendezvous code is not just a locator; it is also a cryptographic input to the handshake

#### `internal/client/crypto`

Owns:

- Noise `XXpsk0` handshake
- HKDF-SHA256 derivation of the PSK from the rendezvous code
- ChaCha20-Poly1305 secure channel after handshake
- frame encryption/decryption over any `io.ReadWriter`

Implementation truth today:

- protocol suite: `Noise_XXpsk0_25519_ChaChaPoly_SHA256`
- counter-based nonces
- post-handshake encrypted application frames
- **transport-agnostic**: works identically over direct UDP or WebSocket relay

#### `internal/client/engine`

Owns:

- transfer header/ResumeOffer/chunk/end framing
- sender-side file streaming with resume-aware chunk skipping
- receiver-side reassembly with on-disk checkpoint state
- SHA-256 integrity verification for the received file
- resume state persistence and validation (`ResumeState`, `TransferID`)
- restart-vs-resume decision logic (metadata match + partial file validation)

Not yet in this layer:

- directory transfer
- richer multi-transfer history

#### `internal/client/rendezvous`

Owns:

- sender/receiver session orchestration using the `transport.Dialer` interface
- room creation / room join flow delegated to the dialer
- bridging transport + crypto + engine into the user flow

This is the integration layer. It is transport-agnostic: callers supply a `Dialer`, which is typically a `Selector` that tries direct first.

#### `internal/client/transport`

Owns:

- transport abstraction: `Conn` and `Dialer`
- `Selector`: default dialer that tries direct first, falls back to relay
- `DirectDialer`: UDP direct transport with hole-punch integration and transport mode selection
- `RelayDialer`: WebSocket relay transport
- `QUICConn`: QUIC-based reliable transport over punched UDP socket (default direct transport)
- `ReliableConn`: UDP reliability/framing layer (stop-and-wait, legacy fallback)
- `MetricsConn`: transparent connection wrapper for throughput and quality tracking
- `GatherCandidates`: ICE-like multi-candidate gathering (host, server-reflexive)
- `SelectionResult`: records which transport was used and why
- `Candidate` and `CandidatePair`: peer address and NAT type exchange types
- `ExchangeCandidates`: relay-coordinated signaling for candidate exchange
- `DiscoverCandidate`: STUN discovery wrapper
- `ConnectionQuality`: transport quality metrics (throughput, byte counters, timing)
- `TransportMode`: selects QUIC (default) or ReliableUDP (legacy)

**Default behavior**: The CLI constructs a `Selector` with `EnableDirect: true`. The selector runs multi-candidate gathering, exchanges candidates through the relay's `/signal` endpoint, evaluates NAT feasibility, and attempts hole-punching. On success, QUIC is established over the punched socket. If QUIC fails, it degrades to ReliableConn. If hole-punching fails entirely, it falls back to relay. All connections are wrapped in `MetricsConn` for quality tracking. Use `--relay-only` to skip the direct attempt.

After each dial, `Selector.LastSelection` records the transport decision with fallback reasons: `STUNFailed`, `NATUnfavorable`, `PunchFailed`, `SignalingFailed`, `DialFailed`, `Timeout`. `Selector.LastMetricsConn` provides connection quality metrics after transfer.

---

## Relay Architecture (`cmd/relay` + `internal/relay/`)

The relay serves two purposes:

1. **Signaling server** -- coordinates P2P candidate exchange via `/signal` WebSocket endpoint
2. **Fallback transport** -- forwards encrypted bytes via `/ws` when direct P2P fails

It is intentionally narrow. It never inspects relay payloads. It enforces per-IP rate limits, HTTP timeouts, and tracks operational metrics.

### Layering

```text
cmd/relay
  ↓
WebSocket server / connection handling
  ↓
├── /signal endpoint (P2P signaling -- primary purpose)
├── /ws endpoint (fallback transport -- encrypted byte forwarding)
├── /healthz, /status, /metrics (operator endpoints)
└── / and /ops/relay (embedded browser surface)
  ↓
room registry + room lifecycle
```

### Package responsibilities

#### `internal/relay/transport`

Owns:

- WebSocket accept/upgrade path
- `/signal` WebSocket endpoint for relay-coordinated candidate exchange (signaling)
- sender and receiver connection handling on `/ws`
- frame relay between paired peers with byte/frame counting (fallback transport)
- `/healthz`, `/status`, and `/metrics` operator endpoints
- per-IP rate limiting on `/ws` and `/signal` endpoints
- explicit HTTP server timeouts
- same-origin serving for the embedded static web UI

Design constraints:

- do not decrypt payloads
- do not reinterpret encrypted application protocol or candidate content
- keep server state minimal and disposable

#### Other relay packages

- `internal/relay/room` -- room creation, lookup, pairing, lifecycle, TTL
- `internal/relay/ratelimit` -- per-IP token bucket rate limiting
- `internal/relay/metrics` -- atomic operator-facing counters
- `internal/relay/webui` -- minimal placeholder handler (dashboard served by frontend/)

---

## Punchthrough Architecture (`cmd/punchthrough` + `internal/punchthrough/`)

This component provides the NAT discovery and hole-punching primitives used by the client's **default** transport path.

### Integration into the default path

```text
bore send/receive
  -> Selector (EnableDirect: true by default)
    -> DiscoverCandidate (STUN probe)
    -> ExchangeCandidates (relay signaling)
    -> DirectDialer.dialWithPunch (hole-punching)
    -> QUICConn (QUIC over punched socket, default)
    -> or ReliableConn (legacy fallback)
    -> MetricsConn (quality tracking wrapper)
    -> Noise handshake + transfer engine
```

### Package responsibilities

- `internal/punchthrough/stun` -- STUN requests/responses, network probing, external address discovery
- `internal/punchthrough/punch` -- NAT classification, UDP hole-punching flow, config/types/errors
- `cmd/punchthrough` -- operator/dev CLI entry point for probing and testing

---

## Transfer Flow

### Default flow (direct P2P)

```text
Sender                     Relay (signaling)              Receiver
  │                            │                              │
  │ 1. Create room             │                              │
  │────────────────────────────►│                              │
  │                            │                              │
  │ 2. STUN probe              │  3. STUN probe               │
  │ (discover public addr)     │  (discover public addr)      │
  │                            │                              │
  │ 4. Exchange candidates via /signal                        │
  │────────────────────────────►│◄─────────────────────────────│
  │◄────────────────────────────│──────────────────────────────►│
  │                            │                              │
  │ 5. Direct UDP hole-punch (bypasses relay)                 │
  │◄─────────────────────────────────────────────────────────►│
  │                            │                              │
  │ 6. Noise XXpsk0 handshake (direct)                       │
  │◄─────────────────────────────────────────────────────────►│
  │                            │                              │
  │ 7. Encrypted file transfer (direct)                      │
  │─────────────────────────────────────────────────────────►│
  │                            │                              │
```

### Fallback flow (relay transport)

When direct connection fails at any step, the transfer proceeds through the relay:

```text
Sender                               Relay                         Receiver
  │                                    │                              │
  │ 1. Create room via /ws             │                              │
  │───────────────────────────────────►│                              │
  │                                    │ 2. Receiver joins via /ws    │
  │                                    │◄─────────────────────────────│
  │                                    │                              │
  │ 3. Noise XXpsk0 handshake over relay                              │
  │◄─────────────────────────────────────────────────────────────────►│
  │                                    │                              │
  │ 4. Encrypted file transfer via relay                              │
  │─────────────────────────────────────────────────────────────────►│
  │                                    │                              │
```

Both flows use identical encryption. The relay forwards encrypted bytes without decryption.

---

## Frontend Architecture (`frontend/`)

The frontend is a separate Python process (FastAPI + Jinja2 + htmx) that fetches data from the Go relay's API and renders server-side HTML with live updates.

- `frontend/src/app/routes` -- FastAPI route handlers for homepage, relay ops, 404
- `frontend/src/app/templates` -- Jinja2 templates with htmx for live-updating relay status
- `frontend/src/app/static` -- static CSS via Tailwind CDN (no JavaScript build step)
- `frontend/tests` -- pytest test coverage

Design constraints:

- keep the web surface read-only
- fetch relay data from the Go relay's `/status`, `/healthz`, `/metrics` endpoints
- keep the browser story aligned with the actual P2P-first product truth
- no JavaScript build step

---

## Admin Surface (`cmd/bore-admin`)

A small operator CLI that queries the relay `/status` endpoint. It is not an operational dependency of the relay or client.

---

## Data And Persistence Posture

Bore's current architecture uses **filesystem-based resume state** on the receiver side and **no durable data layer** on the relay.

If Bore later needs local durable state, the default path is:

1. keep the data **relational**
2. start with **SQLite**
3. use handwritten SQL migrations and queries
4. keep Go-side queries plain SQL first

---

## Design Rules

1. **Direct P2P is the default. Relay is the fallback.**
2. **Docs describe the code that exists, not the code we wish existed.**
3. **Relay stays payload-blind.**
4. **Rendezvous code is cryptographic input, not cosmetic metadata.**
5. **E2E encryption is transport-agnostic -- works identically over direct or relay.**
6. **Minimal tools stay clearly labeled until they carry broader workload.**

---

## Open Architectural Work

- TURN-style relay candidate in multi-candidate gathering
- connection migration for mobile/roaming scenarios
- add directory transfer (after single-file resume is proven)
- surface QUIC metrics and connection quality in operator view
- decide how much operator surface `bore-admin` actually needs
- external security review and formal audit

For verification commands and local run instructions, see [README.md](README.md). For security claims and limits, see [SECURITY.md](SECURITY.md).
