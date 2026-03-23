# bore

**Privacy-first file transfer. No accounts, no cloud, no trust in the relay.**

bore moves files between two machines with a short human-readable rendezvous code. The current shipped path is a Go client that performs an end-to-end encrypted relay-based transfer through a self-hostable Go relay. Future direct transport and any later operator tooling stay on the Go / Zig / C path only.

## Status

**Current truth:**

- the active client lives in `client/` and is implemented in **Go**
- the relay lives in `services/relay/` and is implemented in **Go**
- NAT tooling lives in `lib/punchthrough/` and is implemented in **Go**
- `services/bore-admin/` is a **Go scaffold**, not a real admin surface yet
- the tracked Rust implementation has been **removed from main**
- the repo direction is **Go now**, with **Zig** or **C** only where they later earn their keep

## What works today

- `bore send` and `bore receive` CLI flow in `client/`
- rendezvous code generation and parsing
- Noise `XXpsk0` handshake bound to the rendezvous code
- ChaCha20-Poly1305 encrypted transfer channel
- SHA-256 file integrity verification
- self-hostable WebSocket relay in `services/relay/`
- standalone NAT probing and hole-punching groundwork in `lib/punchthrough/`

## What is still TODO

- direct P2P transport wired into the client path
- resumable transfers
- directory transfer
- relay rate limiting, health endpoints, and metrics
- admin tooling beyond scaffold status
- security hardening and external review beyond current local verification

## Components

| Component | Language | Location | Status | Purpose |
|---|---|---|---|---|
| `bore` client | Go | `client/` | active | Rendezvous, handshake, encrypted transfer, CLI |
| `relay` | Go | `services/relay/` | active | WebSocket room broker for encrypted payload forwarding |
| `punchthrough` | Go | `lib/punchthrough/` | active but not integrated | NAT probing and UDP hole-punching primitives |
| `bore-admin` | Go | `services/bore-admin/` | scaffold | Future relay monitoring and operator tooling |

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

### 4. Send a file

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

### 5. Receive the file on the other machine

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

### bore-admin scaffold

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
│   └── bore-admin/          # truthful scaffold only
├── lib/
│   └── punchthrough/        # Go NAT tooling
├── docs/
│   ├── crypto-design.md
│   └── threat-model.md
├── ARCHITECTURE.md
├── BUILD.md
├── REWRITE_TRACKER.md
└── SECURITY.md
```

## Near-term roadmap

### Current lane

- keep the relay-based Go client stable and honest
- integrate `lib/punchthrough/` into transport selection
- add resumable transfer state
- harden relay operations and observability

### Later, only if justified

- Zig for local/operator-facing tooling or packaging improvements
- C only for leaf dependencies or explicit interoperability boundaries

## Notes

- The rendezvous code is a cryptographic input to the handshake, not just a routing token.
- The relay brokers connections and forwards encrypted bytes; it should stay payload-blind.
- Rust is no longer an in-tree reference. If history matters, use git history, not dead source left in `main`.
- If docs and code disagree, the docs are stale. Fix both in the same change.

For the execution manual and current migration tracker, start with [`BUILD.md`](BUILD.md) and [`REWRITE_TRACKER.md`](REWRITE_TRACKER.md).
