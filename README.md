# bore

**Privacy-first file transfer. No accounts, no cloud, no trust required.**

bore is a direct-first file transfer tool for moving files between two machines with a short human-readable code. When a direct path is not available, it falls back to a self-hostable encrypted relay that cannot read the payload.

## Status

**Current truth:** the active client rewrite lives in `client/` and is implemented in **Go**. The relay and NAT tooling are also in **Go**. The long-term stack for this repo is **Zig / Go / C only**.

What works today:

- relay-based end-to-end encrypted transfer
- working `bore send` and `bore receive` CLI flow in `client/`
- Noise `XXpsk0` handshake bound to the rendezvous code
- SHA-256 integrity verification
- self-hostable Go relay
- standalone NAT probing / hole-punching groundwork in `lib/punchthrough/`

What is not wired end-to-end yet:

- direct P2P transfer path in the client
- resumable transfers
- admin dashboard beyond scaffold state

## Architecture Now

bore currently has one active implementation lane:

- **Go client** in `client/`
- **Go relay** in `services/relay/`
- **Go NAT tooling** in `lib/punchthrough/`
- **Go admin scaffold** in `services/bore-admin/`

Planned architecture direction:

- **Go** for protocol, client implementation, relay, NAT, and service logic
- **Zig** for future local/operator-facing native tooling if it clearly improves the UX or packaging story
- **C** only for leaf dependencies, FFI boundaries, or portability cases that justify it

Legacy Rust crates remain in `crates/` strictly as migration/reference material. They are **not** the target architecture and should not receive new feature work.

## Components

| Component | Language | Location | Status | Purpose |
|---|---|---|---|---|
| `bore` client | Go | `client/` | active | Rendezvous, handshake, encrypted transfer, CLI |
| `relay` | Go | `services/relay/` | active | WebSocket room broker for encrypted payload forwarding |
| `punchthrough` | Go | `lib/punchthrough/` | active but not integrated | NAT probing and UDP hole-punching primitives |
| `bore-admin` | Go | `services/bore-admin/` | scaffold | Relay monitoring / operations surface |
| legacy client reference | Rust | `crates/` | frozen | Protocol/reference history during migration |

## Quick Start

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

## Build And Test

### Go client

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

## Repository Layout

```text
.
├── client/                  # active Go client rewrite
│   ├── cmd/bore/
│   └── internal/
│       ├── code/
│       ├── crypto/
│       ├── engine/
│       ├── rendezvous/
│       └── transport/
├── services/
│   ├── relay/               # active Go relay service
│   └── bore-admin/          # future ops/admin surface
├── lib/
│   └── punchthrough/        # NAT probing and hole-punching tooling
├── crates/                  # legacy Rust reference implementation
├── ARCHITECTURE.md          # design notes (needs rewrite follow-up)
├── BUILD.md                 # execution manual / current-state truth
└── REWRITE_TRACKER.md       # migration and resume tracker
```

## Migration Plan

### Phase A — current

- Keep the Go client as the active implementation
- Keep legacy Rust only as a frozen reference
- Make docs truthful
- Verify the relay-based Go path end-to-end

### Phase B — next

- integrate direct transport via `lib/punchthrough/`
- add resumable transfer state
- harden relay operations and observability

### Phase C — cleanup

- remove the Rust crates and Cargo files once the Go path is unquestionably the keeper
- rewrite `ARCHITECTURE.md` and `SECURITY.md` around the new reality

## Notes

- The rendezvous code is a cryptographic input to the Noise handshake, not just a routing token.
- The relay brokers connections and forwards encrypted bytes; it should remain payload-blind.
- If docs and code disagree, the docs are stale. Fix both in the same change.

For the detailed execution plan and rewrite handoff, start with [`BUILD.md`](BUILD.md) and [`REWRITE_TRACKER.md`](REWRITE_TRACKER.md).
