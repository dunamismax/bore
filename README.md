# bore

[![CI](https://github.com/dunamismax/bore/actions/workflows/ci.yml/badge.svg)](https://github.com/dunamismax/bore/actions/workflows/ci.yml) [![Go](https://img.shields.io/badge/Go-1.26.1-00ADD8.svg)](go.mod) [![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

**Peer-to-peer encrypted file transfer. No accounts, no cloud, no trust required.**

bore moves a file between two machines with a short human-readable rendezvous code. The default transfer path is **direct peer-to-peer**: bore discovers each peer's network address via STUN, exchanges candidates through a lightweight signaling server, and establishes a direct UDP connection via hole-punching. If the direct connection fails (e.g., both peers behind symmetric NATs), bore falls back to a relay automatically.

All file data is end-to-end encrypted with Noise XXpsk0 regardless of transport path. The relay is payload-blind -- it forwards encrypted bytes without any ability to inspect file contents.

The repo also ships an operator dashboard built with Python, FastAPI, Jinja2, and htmx (no JavaScript build step):

- `/` is the Bore homepage
- `/ops/relay` is a live operator page backed by the relay's `/status` endpoint with htmx auto-refresh

## Status

**v1.0.0** -- production release. Current truth:

- the repo is one Go module rooted at `github.com/dunamismax/bore`
- binaries live under `cmd/`: `bore`, `relay`, `bore-admin`, and `punchthrough`
- shared Go packages live under `internal/`: `client`, `relay`, and `punchthrough`
- the operator dashboard lives in `frontend/` (Python + FastAPI + htmx)
- **direct P2P is the default transfer path** -- STUN discovery, signaling, hole-punching
- **QUIC-based direct transport** with production-quality congestion control (default)
- ICE-like multi-candidate gathering (host, server-reflexive candidates)
- connection quality metrics tracking (throughput, byte counters)
- relay is the automatic fallback when direct fails
- end-to-end encryption protects all data regardless of transport method
- resumable single-file transfers with on-disk checkpoint state
- the relay is hardened with per-IP rate limiting, HTTP timeouts, operational metrics, and deployment packaging

## What Ships Today

- `bore send` and `bore receive` with **direct P2P transport by default**
- automatic relay fallback when direct connection fails
- `--relay-only` flag to force relay transport
- transport method reporting (direct/relay + fallback reason)
- rendezvous code generation and parsing
- Noise `XXpsk0` handshake bound to the rendezvous code
- ChaCha20-Poly1305 encrypted transfer channel
- SHA-256 file integrity verification
- resumable single-file transfers with on-disk checkpoint state
- STUN/NAT discovery and relay-coordinated signaling for peer candidate exchange
- UDP hole-punching with QUIC transport (default) or reliable framing layer (fallback)
- multi-candidate gathering (host interfaces, STUN server-reflexive)
- self-hostable WebSocket relay with `/healthz`, `/status`, and `/metrics`
- per-IP rate limiting on relay `/ws` and `/signal` endpoints
- room ID validation on relay join/signaling paths, with signaling limited to live relay rooms
- explicit HTTP server timeouts (read, write, idle, header)
- FastAPI + Jinja2 + htmx operator dashboard at `/` and `/ops/relay`
- `bore-admin status` relay polling
- deployment packaging (Dockerfile, systemd service unit)
- standalone `punchthrough` CLI for NAT probing

## Components

| Component | Location | Status | Purpose |
| --- | --- | --- | --- |
| `bore` client | `cmd/bore`, `internal/client/` | active | P2P QUIC direct transport, relay fallback, crypto, transfer engine, CLI |
| `relay` | `cmd/relay`, `internal/relay/` | active | Signaling server for P2P connections, fallback transport, room broker |
| `frontend` | `frontend/` | active | FastAPI + Jinja2 + htmx operator dashboard |
| `punchthrough` | `cmd/punchthrough`, `internal/punchthrough/` | active, integrated | NAT probing, STUN discovery, UDP hole-punching -> QUIC transport |
| `bore-admin` | `cmd/bore-admin` | active | Minimal operator CLI for relay health and status polling |

## Data layer stance

Bore's relay path does not need a durable database today.

- `internal/relay/room` keeps live room state in memory only.
- `frontend/` is a read-only operator dashboard that fetches the relay's live `/status` snapshot.
- `bore-admin` is a stateless CLI over that same `/status` endpoint.
- resumable transfer checkpoint state is persisted as JSON on the receiver's filesystem (not in a database).
- transfer history and persisted operator history are not implemented yet.

If Bore later earns local persistence for relay observations or operator history, start with a small relational SQLite store. If the browser surface ever grows into authenticated write-heavy workflows, keep it on SQLite with handwritten SQL migrations and queries.

## Install

```bash
go install github.com/dunamismax/bore/cmd/bore@latest
go install github.com/dunamismax/bore/cmd/relay@latest
```

Or build from source:

```bash
git clone https://github.com/dunamismax/bore.git
cd bore
go build ./cmd/bore
go build ./cmd/relay
```

## Quick start

### 1. Build the relay and client

```bash
go build ./cmd/relay
go build ./cmd/bore
```

### 2. Set up the frontend

```bash
cd frontend
uv sync
```

### 3. Run a local relay

```bash
RELAY_ADDR=127.0.0.1:8080 go run ./cmd/relay
```

The relay serves as:
- **Signaling server** -- coordinates P2P candidate exchange between peers
- **Fallback transport** -- forwards encrypted bytes when direct P2P fails

### 4. Start the frontend

```bash
cd frontend && BORE_RELAY_URL=http://127.0.0.1:8080 uv run uvicorn app.main:app --host 127.0.0.1 --port 3000 --app-dir src
```

With both running:

- product page: <http://127.0.0.1:3000/>
- relay ops page: <http://127.0.0.1:3000/ops/relay>
- raw status JSON (relay): <http://127.0.0.1:8080/status>

### 5. Check relay status from the CLI

```bash
go run ./cmd/bore-admin status --relay http://127.0.0.1:8080
```

### 6. Send a file

```bash
./bore send ./report.pdf --relay http://127.0.0.1:8080
```

bore attempts a direct P2P connection by default. If direct fails, it falls back to the relay automatically.

Example output:

```text
bore send -- report.pdf (58213 bytes)

Code: Ahcj7nQZclo-j15A_xGS8w-868-outer-crane-crane
Relay: http://127.0.0.1:8080

Waiting for receiver...

Sent: report.pdf (58213 bytes, 1 chunks)
SHA-256: a1b2c3...
Transport: transport=direct
```

### 7. Receive the file on the other machine

```bash
./bore receive Ahcj7nQZclo-j15A_xGS8w-868-outer-crane-crane --relay http://127.0.0.1:8080
```

### 8. Force relay-only transport

```bash
./bore send ./report.pdf --relay http://127.0.0.1:8080 --relay-only
```

## Build and test

### Frontend

```bash
cd frontend
uv run ruff check .
uv run ruff format --check .
uv run pyright
uv run pytest
```

### Client

```bash
go test ./internal/client/... ./cmd/bore
go build ./cmd/bore
```

### Relay

```bash
go test ./internal/relay/... ./cmd/relay
go build ./cmd/relay
```

### Punchthrough

```bash
go test ./internal/punchthrough/... ./cmd/punchthrough
go build ./cmd/punchthrough
```

### bore-admin

```bash
go test ./cmd/bore-admin
go build ./cmd/bore-admin
```

## Repository Layout

```text
.
├── cmd/
│   ├── bore/
│   ├── bore-admin/
│   ├── punchthrough/
│   └── relay/
├── frontend/
│   ├── src/
│   │   └── app/
│   │       ├── routes/
│   │       ├── templates/
│   │       └── static/
│   ├── tests/
│   └── pyproject.toml
├── internal/
│   ├── client/
│   │   ├── code/
│   │   ├── crypto/
│   │   ├── engine/
│   │   ├── rendezvous/
│   │   └── transport/
│   ├── punchthrough/
│   │   ├── punch/
│   │   └── stun/
│   ├── relay/
│   │   ├── metrics/
│   │   ├── ratelimit/
│   │   ├── room/
│   │   ├── transport/
│   │   └── webui/
│   └── roomid/
├── docs/
├── ARCHITECTURE.md
├── CHANGELOG.md
└── SECURITY.md
```

## Docs

- `README.md` - current product status, quick start, and verification commands
- `ARCHITECTURE.md` - system layout, transport layering, and design notes
- `SECURITY.md` - threat model, implemented guardrails, and current limits
- `CHANGELOG.md` - release history

## Architecture

```text
                    ┌────────────────────┐
                    │ Can peers connect  │
                    │ directly?          │
                    └────────┬───────────┘
                             │
                   ┌─────────┴─────────┐
                   │                   │
                 Yes (default)        No
                   │                   │
            ┌──────┴──────┐    ┌───────┴───────┐
            │ direct path │    │ relay path    │
            │ STUN + UDP  │    │ (automatic    │
            │ hole-punch  │    │ fallback)     │
            │ -> QUIC      │    │               │
            └─────────────┘    └───────────────┘

Both paths use the same Noise XXpsk0 E2E encryption.
The relay never sees plaintext.
QUIC provides congestion control and flow management
for direct transport (~340 MB/s on loopback).
```

## Roadmap (post-v1.0)

- TURN-style relay candidate in multi-candidate gathering
- directory transfer after single-file resume semantics are proven
- connection migration for mobile/roaming scenarios
- deeper operator tooling where it solves real relay problems

## Notes

- The rendezvous code is a cryptographic input to the handshake, not just a routing token.
- The relay serves as both signaling server (P2P candidate exchange) and fallback transport.
- End-to-end encryption is identical regardless of transport path -- the relay is always payload-blind.
- Direct P2P is the default. Relay is the fallback. Use `--relay-only` to force relay transport.
- If docs and code disagree, the docs are stale. Fix both in the same change.
