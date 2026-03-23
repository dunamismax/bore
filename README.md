# bore

**Privacy-first file transfer. No accounts, no cloud, no trust in the relay.**

bore moves a file between two machines with a short human-readable rendezvous code. The current shipped path uses a self-hostable relay to connect peers while keeping file data end-to-end encrypted.

## Status

**Current truth:**

- the active client lives in `client/` and is implemented in **Go**
- the relay lives in `services/relay/` and is implemented in **Go**
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
- `bore-admin status` relay polling in `services/bore-admin/`
- standalone NAT probing and hole-punching groundwork in `lib/punchthrough/`

## What is still next

- direct transport wired into the client path
- resumable transfers
- directory transfer
- relay rate limiting and metrics
- deeper admin tooling beyond relay status polling
- broader security hardening and external review

## Components

| Component | Location | Status | Purpose |
|---|---|---|---|
| `bore` client | `client/` | active | Rendezvous, handshake, encrypted transfer, CLI |
| `relay` | `services/relay/` | active | WebSocket room broker for encrypted payload forwarding |
| `punchthrough` | `lib/punchthrough/` | active but not integrated | NAT probing and UDP hole-punching primitives |
| `bore-admin` | `services/bore-admin/` | active | Minimal operator CLI for relay health and status polling |

## Quick start

### 1. Build the relay

```bash
cd services/relay
go build ./cmd/relay
```

### 2. Build the client

```bash
cd client
go build ./cmd/bore
```

### 3. Run a local relay

```bash
cd services/relay
RELAY_ADDR=127.0.0.1:8080 go run ./cmd/relay
```

### 4. Check relay status (optional)

```bash
cd services/bore-admin
go run ./cmd/bore-admin status --relay http://127.0.0.1:8080
```

### 5. Send a file

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

### 6. Receive the file on the other machine

```bash
cd client
./bore receive Ahcj7nQZclo-j15A_xGS8w-868-outer-crane-crane --relay http://127.0.0.1:8080
```

## Build and test

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
├── services/
│   ├── relay/               # active Go relay service
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
- integrate `lib/punchthrough/` into transport selection
- add resumable transfer state
- harden relay operations and observability
- deepen `services/bore-admin/` beyond status polling

## Notes

- The rendezvous code is a cryptographic input to the handshake, not just a routing token.
- The relay brokers connections and forwards encrypted bytes; it should stay payload-blind.
- The codebase currently ships one reliable transfer path. Treat direct transport as planned work until it is integrated and verified.
- If docs and code disagree, the docs are stale. Fix both in the same change.

For the execution manual and current TODO lane, start with [`BUILD.md`](BUILD.md).
