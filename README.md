# bore

[![CI](https://github.com/dunamismax/bore/actions/workflows/ci.yml/badge.svg)](https://github.com/dunamismax/bore/actions/workflows/ci.yml) [![Go](https://img.shields.io/badge/Go-1.26.1-00ADD8.svg)](go.mod) [![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

**Peer-to-peer encrypted file transfer. No accounts, no cloud, no trust required.**

bore moves a file between two machines with a short human-readable rendezvous code. The default transfer path is **direct peer-to-peer**: bore discovers each peer's network address via STUN, exchanges candidates through a lightweight signaling server, and establishes a direct UDP connection via hole-punching. If the direct connection fails (e.g., both peers behind symmetric NATs), bore falls back to a relay automatically.

All file data is end-to-end encrypted with Noise XXpsk0 regardless of transport path. The relay is payload-blind -- it forwards encrypted bytes without any ability to inspect file contents.

The relay now serves a same-origin browser surface built with Astro + Vue from `web/`:

- `/` is the Bore homepage served by the Go relay from the built web assets
- `/ops/relay` is a live read-only operator page backed by the relay's Go-owned `/status` endpoint

The primary terminal operator surface now lives in `tui/` as an OpenTUI console over that same `/status` contract.

## Status

**v1.0.1** -- current stable release with relay and browser-surface hardening. Current truth:

Rewrite note: `BUILD.md` is the active execution manual for the v2 full rewrite onto Bun + TypeScript + Astro + Vue + Elysia + Zod + PostgreSQL + Docker Compose + Caddy. This README still describes the shipped v1 codebase that exists today, while the repo now also contains a real Phase 2 backend-foundation v2 workspace under `apps/`, `packages/`, `infra/`, and `db/`.

- the repo is one Go module rooted at `github.com/dunamismax/bore`
- binaries live under `cmd/`: `bore`, `relay`, `bore-admin`, and `punchthrough`
- shared Go packages live under `internal/`: `client`, `relay`, and `punchthrough`
- the active browser surface lives in `web/` (Astro + Vue on Bun), served same-origin by `cmd/relay`
- the active terminal operator surface lives in `tui/` (OpenTUI on Bun), pointed at the relay's Go-owned `/status` endpoint
- the v2 rewrite landing zone now exists at repo root as a Bun workspace with `apps/api`, `apps/web`, `packages/contracts`, `infra/caddy`, `docker-compose.yml`, and `.env.example`
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
- Astro + Vue browser surface at `/` and `/ops/relay`, served same-origin by the relay from built static assets
- OpenTUI relay operator console in `tui/` with live refresh, room gauges, direct-vs-relay summaries, and clear stale/error states
- `bore-admin status` relay polling as a compatibility shim
- deployment packaging (Dockerfile, systemd service unit)
- standalone `punchthrough` CLI for NAT probing

## What Exists For v2 Today

The rewrite is no longer doc-only. The repo now contains a verified Phase 2 backend-foundation landing zone for the next-generation stack:

- root Bun workspace with shared `lint`, `check`, `test`, `build`, and `verify` commands
- `apps/api` Elysia service with typed env parsing, boot-time SQL migration application, `/api/health`, `/api/readiness`, `/api/sessions`, `/api/sessions/:code`, `/api/sessions/:code/join`, and `/api/ops/summary`
- `apps/web` Astro + Vue app with the early route structure for `/`, `/send`, `/receive/[code]`, and `/ops`
- `packages/contracts` with shared Zod schemas for health, readiness, session, operator-summary, and error payloads
- `db/migrations` plus checked-in Bun runners for `db:migrate` and `db:reset`
- `infra/caddy/Caddyfile`, `docker-compose.yml`, and `.env.example` so the v2 lane runs as `caddy + api + postgres + web`

This is still a build-phase scaffold, not a cutover claim. The shipped product remains the Go-first v1 described above.

## Components

| Component | Location | Status | Purpose |
| --- | --- | --- | --- |
| `bore` client | `cmd/bore`, `internal/client/` | active | P2P QUIC direct transport, relay fallback, crypto, transfer engine, CLI |
| `relay` | `cmd/relay`, `internal/relay/` | active | Signaling server for P2P connections, fallback transport, room broker |
| `web` | `web/` | active | Astro + Vue homepage and read-only relay operator surface |
| `tui` | `tui/` | active | OpenTUI relay operator console for live status, room gauges, and transport mix |
| `punchthrough` | `cmd/punchthrough`, `internal/punchthrough/` | active, integrated | NAT probing, STUN discovery, UDP hole-punching -> QUIC transport |
| `bore-admin` | `cmd/bore-admin` | compatibility shim | Minimal relay status CLI kept for terse checks alongside the OpenTUI console |

## Data layer stance

Bore's relay path does not need a durable database today.

- `internal/relay/room` keeps live room state in memory only.
- `web/` is a read-only browser surface that consumes the relay's live `/status` snapshot.
- `tui/` is the primary terminal operator surface over that same `/status` endpoint.
- `bore-admin` is a smaller stateless compatibility CLI over that same `/status` endpoint.
- resumable transfer checkpoint state is persisted as JSON on the receiver's filesystem (not in a database).
- transfer history and persisted operator history are not implemented yet.

For the shipped v1 codepath, keep relay state in memory unless a maintenance need clearly earns more. The planned v2 rewrite in `BUILD.md` moves durable application metadata and operator history to PostgreSQL.

## v2 workspace quick start

The rewrite lane now boots independently from the shipped Go runtime.

```bash
bun install
bun run verify
docker compose up -d --build
```

If port `8080` is already in use locally, override it for the v2 stack run:

```bash
BORE_V2_HTTP_PORT=18080 docker compose up -d --build
```

If local PostgreSQL is already using `5432`, override the v2 Compose mapping too:

```bash
BORE_V2_POSTGRES_PORT=15432 docker compose up -d --build
```

Once the stack is up:

- v2 web shell through Caddy: <http://127.0.0.1:8080/>
- v2 ops shell through Caddy: <http://127.0.0.1:8080/ops>
- v2 health endpoint through Caddy: <http://127.0.0.1:8080/api/health>
- alternate port example when overriding locally: <http://127.0.0.1:18080/api/health>

The v2 lane is intentionally separate from the shipped Go relay runtime. It does not replace `cmd/relay`, `web/`, or `tui/` yet.

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

### 2. Set up the web frontend

```bash
cd web
bun install
```

### 3. Build the web assets

```bash
cd web
bun run build
```

### 4. Run a local relay

```bash
RELAY_ADDR=127.0.0.1:8080 go run ./cmd/relay
```

The relay serves as:
- **Signaling server** -- coordinates P2P candidate exchange between peers
- **Fallback transport** -- forwards encrypted bytes when direct P2P fails
- **Same-origin browser surface** -- serves the built Astro output from `web/dist`

With the web build present:

- product page: <http://127.0.0.1:8080/>
- relay ops page: <http://127.0.0.1:8080/ops/relay>
- raw status JSON (relay): <http://127.0.0.1:8080/status>

For browser-only frontend iteration outside the shipped runtime path, you can also run:

```bash
cd web
bun run dev
```

### 5. Run the operator TUI

```bash
cd tui
bun install
bun run start --relay http://127.0.0.1:8080
```

### 6. Check relay status from the compatibility CLI

```bash
go run ./cmd/bore-admin status --relay http://127.0.0.1:8080
```

### 7. Send a file

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

### 8. Receive the file on the other machine

```bash
./bore receive Ahcj7nQZclo-j15A_xGS8w-868-outer-crane-crane --relay http://127.0.0.1:8080
```

### 9. Force relay-only transport

```bash
./bore send ./report.pdf --relay http://127.0.0.1:8080 --relay-only
```

## Build and test

### Frontend

```bash
cd web
bun install
bun run lint
bun run check
bun test
```

### v2 workspace

```bash
bun install
bun run lint
bun run check
bun run test
bun run build
bun run verify
docker compose up -d --build
# or: BORE_V2_HTTP_PORT=18080 docker compose up -d --build
# or: BORE_V2_POSTGRES_PORT=15432 docker compose up -d --build
```

### v2 database foundation

```bash
BORE_V2_DATABASE_URL=postgres://bore:bore@127.0.0.1:5432/bore_v2 bun run --cwd apps/api db:reset
BORE_V2_DATABASE_URL=postgres://bore:bore@127.0.0.1:5432/bore_v2 bun run --cwd apps/api db:migrate
BORE_V2_DATABASE_TEST_URL=postgres://bore:bore@127.0.0.1:5432/bore_v2 bun --cwd apps/api test tests/integration.test.ts
```

### Operator TUI

```bash
cd tui
bun install
bun run lint
bun run check
bun test
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
├── apps/
│   ├── api/
│   └── web/
├── db/
│   └── migrations/
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
├── packages/
│   └── contracts/
├── infra/
│   └── caddy/
├── docker-compose.yml
├── package.json
├── bun.lock
├── web/
│   ├── src/
│   │   ├── components/
│   │   ├── layouts/
│   │   ├── lib/
│   │   ├── pages/
│   │   └── styles/
│   ├── tests/
│   └── package.json
├── tui/
│   ├── src/
│   │   └── lib/
│   ├── tests/
│   └── package.json
├── docs/
├── ARCHITECTURE.md
├── CHANGELOG.md
└── SECURITY.md
```

## Docs

- `README.md` - current product status, quick start, and verification commands
- `ARCHITECTURE.md` - system layout, transport layering, and design notes
- `BUILD.md` - active v2 rewrite plan and phase tracker
- `SECURITY.md` - threat model, implemented guardrails, and current limits
- `docs/status-contract.md` - Go-owned `/status` contract consumed by the browser surface, `tui/`, and `bore-admin`
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
