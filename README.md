# bore

**Privacy-first file transfer with a real browser surface and a payload-blind relay.**

bore moves a file between two machines with a short human-readable rendezvous code. The current shipped transfer path is relay-based: the relay connects peers and forwards encrypted bytes, while the file data stays end-to-end encrypted between sender and receiver.

The repo now also ships an in-repo browser surface built with **Bun + TypeScript + Astro + Alpine.js**. It lives alongside the existing CLI/runtime story instead of replacing it:

- `/` is the product-facing Bore homepage when served by the relay
- `/ops/relay/` is a read-only operator page backed by the relay's existing `/status` endpoint

## Status

**Current truth:**

- the active client lives in `client/` and is implemented in **Go**
- the relay lives in `services/relay/` and is implemented in **Go**
- the browser surface lives in `web/` and is implemented with **Bun + TypeScript + Astro + Alpine.js**
- NAT tooling lives in `lib/punchthrough/` and is implemented in **Go**
- `services/bore-admin/` is a minimal operator CLI for relay health and status checks
- the verified transfer path today is **relay-based**, not direct peer-to-peer

## What works today

- `bore send` and `bore receive` CLI flow in `client/`
- rendezvous code generation and parsing
- Noise `XXpsk0` handshake bound to the rendezvous code
- ChaCha20-Poly1305 encrypted transfer channel
- SHA-256 file integrity verification
- self-hostable WebSocket relay in `services/relay/`
- relay `/healthz` and `/status` operator endpoints
- embedded relay-served web UI at `/` and `/ops/relay/`
- `bore-admin status` relay polling in `services/bore-admin/`
- standalone NAT probing and hole-punching groundwork in `lib/punchthrough/`

## What is still next

- direct transport wired into the client path
- resumable transfers
- directory transfer
- relay rate limiting and metrics
- broader operator tooling beyond status snapshots
- broader security hardening and external review

## Components

| Component | Location | Status | Purpose |
| --- | --- | --- | --- |
| `bore` client | `client/` | active | Rendezvous, handshake, encrypted transfer, CLI |
| `relay` | `services/relay/` | active | WebSocket room broker, `/healthz`, `/status`, and static web UI serving |
| `web` | `web/` | active | Astro/Alpine browser surface for product story and live relay ops page |
| `punchthrough` | `lib/punchthrough/` | active but not integrated | NAT probing and UDP hole-punching primitives |
| `bore-admin` | `services/bore-admin/` | active | Minimal operator CLI for relay health and status polling |

## Data layer stance

Bore's shipped path does **not** need a durable database today.

- `services/relay/` keeps live room state in memory only.
- `web/` is a read-only browser surface that fetches the relay's live `/status` snapshot.
- `services/bore-admin/` is a stateless CLI over that same `/status` endpoint.
- resumable transfers, transfer history, and persisted operator history are **not implemented yet**.

If Bore later earns local persistence for resume metadata, relay observations, or operator history, start with a small **relational SQLite** store. Only move to **PostgreSQL** if Bore becomes a genuinely multi-node control plane with write/concurrency pressure SQLite no longer fits. If the browser surface ever grows into authenticated write-heavy workflows, **Drizzle** is the default web-side fit; otherwise keep Go-side SQL explicit and boring. Do not pivot Bore's future data story toward MongoDB or a document-store control plane.

## Quick start

### 1. Build the browser surface

```bash
cd web
bun install
bun run build
```

This writes the production web output into `services/relay/internal/webui/dist/` so the relay can embed and serve it.

### 2. Build the relay

```bash
cd services/relay
go build ./cmd/relay
```

### 3. Build the client

```bash
cd client
go build ./cmd/bore
```

### 4. Run a local relay

```bash
cd services/relay
RELAY_ADDR=127.0.0.1:8080 go run ./cmd/relay
```

With the relay running:

- product page: <http://127.0.0.1:8080/>
- relay ops page: <http://127.0.0.1:8080/ops/relay/>
- raw status JSON: <http://127.0.0.1:8080/status>

### 5. Check relay status from the CLI (optional)

```bash
cd services/bore-admin
go run ./cmd/bore-admin status --relay http://127.0.0.1:8080
```

### 6. Send a file

```bash
cd client
./bore send ./report.pdf --relay http://127.0.0.1:8080
```

Example output:

```text
bore send -- report.pdf (58213 bytes)

Code: Ahcj7nQZclo-j15A_xGS8w-868-outer-crane-crane
Relay: http://127.0.0.1:8080

Waiting for receiver...
```

### 7. Receive the file on the other machine

```bash
cd client
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
cd client
go test ./...
go build ./cmd/bore
```

### Relay

```bash
cd services/relay
go test ./...
go build ./cmd/relay
```

### Punchthrough

```bash
cd lib/punchthrough
go test ./...
go build ./cmd/punchthrough
```

### bore-admin

```bash
cd services/bore-admin
go build ./cmd/bore-admin
```

## Repository layout

```text
.
├── client/                  # active Go client
│   ├── cmd/bore/
│   └── internal/
│       ├── code/
│       ├── crypto/
│       ├── engine/
│       ├── rendezvous/
│       └── transport/
├── web/                     # Astro + Alpine browser surface
├── services/
│   ├── relay/               # active Go relay service + embedded web UI dist
│   └── bore-admin/          # minimal operator CLI
├── lib/
│   └── punchthrough/        # NAT tooling
├── docs/
│   ├── crypto-design.md
│   └── threat-model.md
├── ARCHITECTURE.md
├── BUILD.md
└── SECURITY.md
```

## Near-term roadmap

- keep the relay-based path stable and honest
- keep the web surface narrow, read-only, and same-origin with the relay
- integrate `lib/punchthrough/` into transport selection
- add resumable transfer state
- harden relay operations and observability
- deepen operator tooling only where it solves real relay problems

## Notes

- The rendezvous code is a cryptographic input to the handshake, not just a routing token.
- The relay brokers connections and forwards encrypted bytes; it should stay payload-blind.
- The root web surface is a product/operator layer over Bore's existing runtime, not a replacement for the CLI or transfer engine.
- The codebase currently ships one reliable transfer path. Treat direct transport as planned work until it is integrated and verified.
- If docs and code disagree, the docs are stale. Fix both in the same change.

For the execution manual and current TODO lane, start with [`BUILD.md`](BUILD.md).
