# bore

**Privacy-first file transfer. No accounts, no cloud, no trust required.**

bore is a command-line tool for transferring files between two computers. The sender generates a short, human-readable code. The receiver enters it. Files move directly between machines when possible, through an encrypted relay when not. The relay learns nothing about the content.

bore is not a file sharing service. It is a transfer tool — ephemeral, encrypted, peer-authenticated, and zero-knowledge by design.

> **Status: bore works over the network.** The Rust client connects to the Go relay server over WebSocket, performs a Noise XXpsk0 handshake, and transfers encrypted files end-to-end. The CLI has working `send` and `receive` commands. The relay server handles WebSocket-based connection brokering with room management. The NAT traversal library performs STUN probing and UDP hole-punching. See [BUILD.md](BUILD.md) for the full execution plan.

## Why bore?

Existing tools make tradeoffs that bore refuses:

| Tool | Account required | E2E encrypted | Direct P2P | Self-hostable relay | Resumable |
|------|:---:|:---:|:---:|:---:|:---:|
| Email/Slack attachment | Yes | No | No | No | No |
| WeTransfer/Dropbox | Yes | No | No | No | Partial |
| scp/rsync | No | Transport only | Yes | N/A | Yes |
| Magic Wormhole | No | Yes | Partial | Yes | No |
| croc | No | Yes | Partial | Yes | Yes |
| **bore** (goal) | **No** | **Yes** | **Yes** | **Yes** | **Yes** |

bore's design targets:

- **Zero accounts.** No signup, no login, no API key. Generate a code, share it, transfer.
- **End-to-end encryption.** The relay cannot read your files. Period.
- **Direct when possible.** LAN transfers never leave the network. WAN transfers attempt hole-punching before falling back to relay.
- **Human-friendly codes.** Short wordlist codes (e.g., `7-guitar-castle-moon`) that are easy to read aloud, type on a phone, or paste in chat.
- **Resumable.** Large transfers survive network interruptions without starting over.
- **Self-hostable relay.** Run your own relay for organizational or compliance requirements. Or use the public one.
- **Library-first.** `bore-core` is embeddable — build your own frontend, integrate into your own tools, wrap it in a GUI.

## Planned usage

```bash
# sender
bore send ./photos/
# => Code: 7-guitar-castle-moon
# => Waiting for receiver...

# receiver (on another machine)
bore receive 7-guitar-castle-moon
# => Connected to sender
# => Receiving: photos/ (3 files, 48 MB)
# => [=================>          ] 67% 12.4 MB/s
```

```bash
# send a single file
bore send report.pdf

# send to a specific relay
bore send --relay wss://relay.example.com ./data/

# resume an interrupted transfer
bore receive 7-guitar-castle-moon --resume

# run your own relay server
relay serve --port 8080

# probe NAT type for debugging
punchthrough probe

# monitor relay health
bore-admin --relay http://localhost:8080
```

This is the target experience, not the current implementation.

## Architecture

bore is a monorepo with a Rust client core and Go network infrastructure. The Rust side handles encryption, file transfer, and the user-facing CLI. The Go side handles relay transport, NAT traversal, and operations monitoring.

```text
┌─────────────────────────────────────────────────────────────────────────────┐
│                              bore monorepo                                  │
│                                                                             │
│  ┌─────────────────────────────────────────────┐  ┌──────────────────────┐ │
│  │            Rust — Client Core                │  │     Go — Network     │ │
│  │                                              │  │    Infrastructure    │ │
│  │  bore-core          bore-cli                 │  │                      │ │
│  │  ┌─────────────┐   ┌──────────────┐         │  │  relay               │ │
│  │  │ crypto layer │   │ send/receive │         │  │  ┌────────────────┐  │ │
│  │  │ session mgmt │   │ progress UI  │         │  │  │ room management│  │ │
│  │  │ protocol     │   │ history      │         │  │  │ WebSocket relay│  │ │
│  │  │ codec/framing│   │ config       │         │  │  │ zero-knowledge │  │ │
│  │  │ transfer     │   └──────────────┘         │  │  └────────────────┘  │ │
│  │  │ code gen     │                            │  │                      │ │
│  │  └─────────────┘                             │  │  punchthrough        │ │
│  │                                              │  │  ┌────────────────┐  │ │
│  │                                              │  │  │ STUN client    │  │ │
│  │                                              │  │  │ NAT classify   │  │ │
│  │                                              │  │  │ UDP hole-punch │  │ │
│  │                                              │  │  └────────────────┘  │ │
│  │                                              │  │                      │ │
│  │                                              │  │  bore-admin          │ │
│  │                                              │  │  ┌────────────────┐  │ │
│  │                                              │  │  │ relay monitor  │  │ │
│  │                                              │  │  │ metrics/alerts │  │ │
│  │                                              │  │  │ TUI + web dash │  │ │
│  │                                              │  │  └────────────────┘  │ │
│  └─────────────────────────────────────────────┘  └──────────────────────┘ │
└─────────────────────────────────────────────────────────────────────────────┘
```

### Components

| Component | Language | Location | Phase | What it does |
|-----------|----------|----------|-------|-------------|
| **bore-core** | Rust | `crates/bore-core/` | Phase 4 | Transfer engine, session state, cryptographic layer, protocol codec, transport abstraction, relay integration. Designed to be embedded by any frontend. |
| **bore-cli** | Rust | `crates/bore-cli/` | Phase 4 | Operator interface with working send/receive commands. Thin shell over bore-core. |
| **relay** | Go | `services/relay/` | Phase 2 | Zero-knowledge encrypted stream broker. Pairs connections by room ID and forwards encrypted bytes. Self-hostable. |
| **punchthrough** | Go | `lib/punchthrough/` | Phase 2 | NAT traversal library. STUN-based NAT discovery and UDP hole-punching for direct peer connections. |
| **bore-admin** | Go | `services/bore-admin/` | Phase 0 | Monitoring and administration dashboard for relay infrastructure. TUI and web interface. |

### How they connect

bore-cli attempts a direct connection (via punchthrough) first. If the NAT configuration allows it, files transfer peer-to-peer with no intermediary. If hole-punching fails, bore-cli falls back to relay, which forwards encrypted bytes without being able to read them. bore-admin connects to relay's health and metrics endpoints to provide real-time monitoring.

```text
bore-cli (sender)                                         bore-cli (receiver)
     |                                                          |
     |  1. discover NAT type (punchthrough)                     |
     |  2. attempt direct UDP connection                        |
     |  ======================================================>|
     |               direct transfer (encrypted)                |
     |                                                          |
     |  --- if direct fails ---                                 |
     |                                                          |
     |  3. connect to relay                    relay            |
     |  ==================================>   |   <============ |
     |     encrypted bytes (opaque to relay)   |                |
     |  <==================================   |   ============>|
     |                                                          |
     |                              bore-admin                  |
     |                              (monitors relay health)     |
```

## Building from source

### Prerequisites

- Rust 1.85+ (stable) — for bore-core and bore-cli
- Go 1.22+ — for relay, punchthrough, and bore-admin

### Rust components

```bash
git clone https://github.com/dunamismax/bore.git
cd bore
cargo build --release
```

```bash
# Project info
cargo run -p bore-cli                # project status
cargo run -p bore-cli -- status      # project status
cargo run -p bore-cli -- components  # component map

# File transfer (requires relay running: cd services/relay && go run ./cmd/relay)
cargo run -p bore-cli -- send report.pdf                         # send a file
cargo run -p bore-cli -- receive <code>                          # receive using code
cargo run -p bore-cli -- send --relay http://relay.example.com report.pdf  # custom relay
```

### Go components

```bash
# Relay server
cd services/relay && go build ./cmd/relay

# NAT traversal library + CLI
cd lib/punchthrough && go build ./cmd/punchthrough

# Admin dashboard
cd services/bore-admin && go build ./cmd/bore-admin
```

### Quality checks

```bash
# Rust
cargo check
cargo test
cargo fmt --check
cargo clippy --workspace --all-targets -- -D warnings

# Go — relay
cd services/relay && go build ./... && go test ./... && go vet ./...

# Go — punchthrough
cd lib/punchthrough && go build ./... && go test ./... && go vet ./...

# Go — bore-admin
cd services/bore-admin && go build ./... && go vet ./...
```

## Repository layout

```text
.
├── Cargo.toml                # Rust workspace root
├── Cargo.lock                # Rust dependency lock
├── BUILD.md                  # unified execution manual
├── ARCHITECTURE.md           # technical design and protocol notes
├── SECURITY.md               # threat model and security policy
├── LICENSE                   # MIT
├── crates/
│   ├── bore-core/            # Rust — transfer engine, crypto, and transport
│   │   └── src/
│   │       ├── lib.rs        # domain types, session state, transfer model
│   │       ├── crypto.rs     # Noise XXpsk0 handshake, SecureChannel
│   │       ├── engine.rs     # transfer engine: chunking, streaming, SHA-256
│   │       ├── transport.rs  # Transport trait, WebSocket and duplex adapters
│   │       ├── rendezvous.rs # rendezvous coordination: code ↔ relay mapping
│   │       ├── codec.rs      # frame encoding/decoding
│   │       ├── code.rs       # rendezvous code generation/parsing
│   │       ├── protocol.rs   # protocol message types
│   │       ├── session.rs    # session state machine
│   │       ├── transfer.rs   # transfer model types
│   │       └── error.rs      # typed error variants
│   └── bore-cli/             # Rust — operator CLI
│       └── src/main.rs
├── services/
│   ├── relay/                # Go — zero-knowledge relay server
│   │   ├── go.mod
│   │   ├── cmd/relay/main.go
│   │   └── internal/
│   │       ├── room/         # room model, registry, state machine
│   │       └── transport/    # WebSocket handler, bidirectional relay
│   └── bore-admin/           # Go — relay monitoring dashboard
│       ├── go.mod
│       └── cmd/bore-admin/main.go
├── lib/
│   └── punchthrough/         # Go — NAT traversal library
│       ├── go.mod
│       ├── cmd/punchthrough/main.go
│       └── pkg/
│           ├── stun/         # STUN client and NAT classification
│           └── punch/        # UDP hole-punching engine
└── docs/
    ├── threat-model.md       # threat model: actors, assets, attack scenarios
    └── crypto-design.md      # cryptographic design: Noise XX, PAKE, ChaCha20-Poly1305
```

## Component status

### bore-core — Phase 4 (network transport) ✓ / bore-cli — Phase 4 ✓

The Rust client core is through its fourth major phase. What exists:

- **Working send/receive** over the Go relay server via WebSocket
- **Transport trait** abstracting WebSocket, TCP, and in-process streams
- **WebSocket client transport** connecting to the relay via `tokio-tungstenite`
- **Rendezvous coordination** mapping codes to relay room IDs and PAKE secrets
- **Transfer engine** with chunking (256 KiB default), streaming over SecureChannel, SHA-256 integrity verification
- **Binary wire format** for header/chunk/end messages with type tags and big-endian encoding
- **Filename validation** against path traversal, null bytes, relative components, and length limits
- **Noise XXpsk0 handshake** with PAKE binding to rendezvous codes
- **SecureChannel** with ChaCha20-Poly1305 AEAD encryption
- **HKDF-SHA256** PSK derivation from rendezvous codes
- **Counter-based nonces** with replay detection
- **Multi-segment framing** for payloads exceeding 64KB
- **Rekey support** for long-running transfers
- **Key material zeroization** on drop
- **180 tests** including transport, rendezvous, transfer integration, error-path, and crypto tests
- Core domain types: session state machine, protocol messages, frame codec, rendezvous codes

What's next: direct peer-to-peer transport (Phase 5), then relay hardening (Phase 6).

### relay — Phase 2 (WebSocket transport) ✓

The relay server handles the core job: pair two connections by room ID and forward encrypted bytes between them. What exists:

- **Room model** with state machine (Waiting → Active → Closed)
- **In-memory registry** with TTL reaper for expired rooms
- **WebSocket transport** with bidirectional frame relay
- **Streaming back-pressure** via `io.Copy` between Reader/Writer pairs
- **Clean close propagation** between peers
- **32 tests** (21 room model + 11 transport integration)

What's next: rate limiting (Phase 3), configuration and deployment (Phase 4).

### punchthrough — Phase 2 (UDP hole-punching engine) ✓

The NAT traversal library handles direct connection establishment. What exists:

- **STUN client** with NAT type classification via `pion/stun` v3
- **NAT type detection**: Full Cone, Restricted Cone, Port-Restricted Cone, Symmetric
- **UDP hole-punching engine** with simultaneous open strategy
- **24-byte binary punch protocol** with ping/pong/ack handshake
- **Strategy selection** based on both peers' NAT types
- **CLI** with `probe` and `version` commands

What's next: coordination protocol (Phase 3), library API polish (Phase 4).

### bore-admin — Phase 0 (scaffold) ✓

The monitoring dashboard is in its earliest stage. What exists:

- Project scaffold with Go module and entry point
- Planned architecture for relay health polling, SQLite metrics storage, TUI dashboard, web dashboard, and configurable alerting

What's next: relay health polling and SQLite storage (Phase 1).

## Roadmap

### Rust client (bore-core / bore-cli)

| Phase | Name | Status |
|-------|------|--------|
| 0 | Truthful scaffold | **Done** |
| 1 | Protocol design and type foundations | **Done** |
| 2 | Cryptographic layer | **Done** |
| 3 | Local transfer engine | **Done** |
| 4 | Rendezvous and network transport | **Done** |
| 5 | Direct peer-to-peer transport | Planned |
| 6 | Relay service hardening | Planned |
| 7 | Resumable transfers and persistence | Planned |
| 8 | Hardening and security audit | Planned |
| 9 | Cross-platform polish and distribution | Planned |
| 10 | Ecosystem — library bindings, GUI, integrations | Planned |

### Relay server

| Phase | Name | Status |
|-------|------|--------|
| 0 | Truthful scaffold | **Done** |
| 1 | Room model and connection lifecycle | **Done** |
| 2 | WebSocket transport | **Done** |
| 3 | Rate limiting and resource management | Planned |
| 4 | Configuration and deployment | Planned |
| 5 | Observability | Planned |
| 6 | Integration testing with bore | Planned |
| 7 | Hardening | Planned |

### Punchthrough (NAT traversal)

| Phase | Name | Status |
|-------|------|--------|
| 0 | Truthful scaffold | **Done** |
| 1 | STUN client and NAT discovery | **Done** |
| 2 | UDP hole-punching engine | **Done** |
| 3 | Coordination protocol | Planned |
| 4 | Library API | Planned |
| 5 | CLI wrapper | Planned |
| 6 | SQLite probe cache | Planned |
| 7 | Hardening and real-world testing | Planned |

### bore-admin (monitoring)

| Phase | Name | Status |
|-------|------|--------|
| 0 | Truthful scaffold | **Done** |
| 1 | Relay health polling and SQLite storage | Planned |
| 2 | Terminal UI dashboard | Planned |
| 3 | Alerting engine | Planned |
| 4 | Web dashboard | Planned |
| 5 | Multi-instance support | Planned |
| 6 | Configuration and deployment | Planned |
| 7 | Hardening and real-world testing | Planned |

See [BUILD.md](BUILD.md) for the full phase breakdown with tasks, exit criteria, and decisions.

## Design principles

1. **Truth over theater.** Never claim a security property that isn't proven. The CLI, docs, and code must agree.
2. **Privacy by default.** End-to-end encryption is not optional. The relay is zero-knowledge. No telemetry, no analytics, no tracking.
3. **Direct first.** Attempt peer-to-peer before relay. LAN transfers should be fast and never touch the internet.
4. **Operator control.** The user decides what relay to use, what to trust, and what to persist. No hidden magic.
5. **Composable core.** `bore-core` is a library. The CLI is one consumer. Others should be possible.
6. **Dumb pipe relay.** The relay forwards bytes. It doesn't parse them, validate them, transform them, or store them. Complexity belongs in the client.
7. **Fail fast, fall back clean.** If hole-punching can't work for a given NAT configuration, detect that quickly and report clearly.
8. **Honest scope.** Never describe future work as present capability.

## Security

bore's Noise XXpsk0 handshake with PAKE binding to rendezvous codes and ChaCha20-Poly1305 AEAD encryption is implemented and tested. The relay is zero-knowledge by design — it forwards encrypted bytes without being able to decrypt them.

bore makes no security claims beyond what has been implemented and tested. See [SECURITY.md](SECURITY.md) for the full threat model, security posture, and responsible disclosure policy. See [docs/crypto-design.md](docs/crypto-design.md) for the cryptographic protocol design.

## Contributing

bore is in active development across all components. Contributions are welcome — especially:

- **Protocol and security review** for the cryptographic layer
- **NAT traversal testing** across different network configurations
- **Relay load testing** for concurrent connection scenarios

Open an issue to start a conversation.

## License

MIT — see [LICENSE](LICENSE).
