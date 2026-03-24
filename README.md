# bore

[![CI](https://github.com/dunamismax/bore/actions/workflows/ci.yml/badge.svg)](https://github.com/dunamismax/bore/actions/workflows/ci.yml) [![Go](https://img.shields.io/badge/Go-1.26.1-00ADD8.svg)](go.mod) [![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

**Privacy-first file transfer with a real browser surface and a payload-blind relay.**

bore moves a file between two machines with a short human-readable rendezvous code. The verified transfer path today is relay-based: the relay pairs peers and forwards encrypted bytes, while the file data stays end-to-end encrypted between sender and receiver.

The repo also ships an in-repo browser surface built with Bun, React, Vite, and TypeScript:

- `/` is the Bore homepage when served by the relay
- `/ops/relay` is a read-only operator page backed by the relay's `/status` endpoint

## Status

Current truth:

- the repo is one Go module rooted at `github.com/dunamismax/bore`
- binaries live under `cmd/`: `bore`, `relay`, `bore-admin`, and `punchthrough`
- shared Go packages live under `internal/`: `client`, `relay`, and `punchthrough`
- the browser surface lives in `web/`
- the verified transfer path is relay-based with resumable single-file transfers
- direct-path transport is implemented (STUN discovery, signaling, hole-punching, reliability framing) but pending real-world NAT validation; relay remains the default
- the relay is hardened with per-IP rate limiting, HTTP timeouts, operational metrics, and deployment packaging

## What Ships Today

- `bore send` and `bore receive` for relay-based encrypted file transfer
- resumable single-file transfers with on-disk checkpoint state
- rendezvous code generation and parsing
- Noise `XXpsk0` handshake bound to the rendezvous code
- ChaCha20-Poly1305 encrypted transfer channel
- SHA-256 file integrity verification
- self-hostable WebSocket relay with `/healthz`, `/status`, and `/metrics`
- per-IP rate limiting on relay `/ws` and `/signal` endpoints
- explicit HTTP server timeouts (read, write, idle, header)
- embedded relay-served web UI at `/` and `/ops/relay`
- `bore-admin status` relay polling
- deployment packaging (Dockerfile, systemd service unit)
- STUN/NAT discovery, relay-coordinated signaling, and UDP hole-punching integrated into the client transport selector
- `--direct` CLI flag on both `bore send` and `bore receive` for opt-in direct-path attempts
- standalone `punchthrough` CLI for NAT probing

## Roadmap

- end-to-end direct transfer verified across real NATs (implementation complete, pending real-world validation)
- directory transfer
- broader operator tooling beyond status snapshots and metrics
- broader security hardening and external review

## Components

| Component | Location | Status | Purpose |
| --- | --- | --- | --- |
| `bore` client | `cmd/bore`, `internal/client/` | active | Rendezvous, handshake, encrypted transfer, CLI |
| `relay` | `cmd/relay`, `internal/relay/` | active | WebSocket room broker, `/healthz`, `/status`, and static web UI serving |
| `web` | `web/` | active | React + Vite SPA for product story and live relay ops page |
| `punchthrough` | `cmd/punchthrough`, `internal/punchthrough/` | active, integrated into client transport selector | NAT probing, STUN discovery, and UDP hole-punching |
| `bore-admin` | `cmd/bore-admin` | active | Minimal operator CLI for relay health and status polling |

## Data layer stance

Bore's relay path does not need a durable database today.

- `internal/relay/room` keeps live room state in memory only.
- `web/` is a read-only browser surface that fetches the relay's live `/status` snapshot.
- `bore-admin` is a stateless CLI over that same `/status` endpoint.
- resumable transfer checkpoint state is persisted as JSON on the receiver's filesystem (not in a database).
- transfer history and persisted operator history are not implemented yet.

If Bore later earns local persistence for relay observations or operator history, start with a small relational SQLite store. If the browser surface ever grows into authenticated write-heavy workflows, keep it on SQLite with handwritten SQL migrations and queries.

## Quick start

### 1. Build the browser surface

```bash
cd web
bun install
bun run build
```

This writes the production web output into `internal/relay/webui/dist/` so the relay can embed and serve it.

### 2. Build the relay and client

```bash
go build ./cmd/relay
go build ./cmd/bore
```

### 3. Run a local relay

```bash
RELAY_ADDR=127.0.0.1:8080 go run ./cmd/relay
```

With the relay running:

- product page: <http://127.0.0.1:8080/>
- relay ops page: <http://127.0.0.1:8080/ops/relay>
- raw status JSON: <http://127.0.0.1:8080/status>

### 4. Check relay status from the CLI

```bash
go run ./cmd/bore-admin status --relay http://127.0.0.1:8080
```

### 5. Send a file

```bash
./bore send ./report.pdf --relay http://127.0.0.1:8080
```

Example output:

```text
bore send -- report.pdf (58213 bytes)

Code: Ahcj7nQZclo-j15A_xGS8w-868-outer-crane-crane
Relay: http://127.0.0.1:8080

Waiting for receiver...
```

### 6. Receive the file on the other machine

```bash
./bore receive Ahcj7nQZclo-j15A_xGS8w-868-outer-crane-crane --relay http://127.0.0.1:8080
```

## Build and test

### Web

```bash
cd web
bun run check
bun run test
bun run build
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
│   └── relay/
│       ├── metrics/
│       ├── ratelimit/
│       ├── room/
│       ├── transport/
│       └── webui/
├── web/
├── docs/
├── ARCHITECTURE.md
├── BUILD.md
└── SECURITY.md
```

## Near-term roadmap

- keep the relay-based path stable and honest
- keep the web surface narrow, read-only, and same-origin with the relay
- validate direct transport across real-world NAT configurations
- add directory transfer after single-file resume semantics are proven
- deepen operator tooling only where it solves real relay problems

## Notes

- The rendezvous code is a cryptographic input to the handshake, not just a routing token.
- The relay brokers connections and forwards encrypted bytes; it should stay payload-blind.
- The root web surface is a product and operator layer over Bore's existing runtime, not a replacement for the CLI or transfer engine.
- The codebase ships a reliable relay-based transfer path with resumable single-file transfers. Direct transport is implemented and integrated but pending real-world NAT validation; relay remains the default.
- If docs and code disagree, the docs are stale. Fix both in the same change.

For the execution manual and current TODO lane, start with [`BUILD.md`](BUILD.md).
