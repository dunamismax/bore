# BUILD.md

## Purpose

This file is the unified execution manual for the bore monorepo.

It covers all components — the Rust client core (bore-core, bore-cli) and the Go network infrastructure (relay, punchthrough, bore-admin). At any point it should answer:

- what each component is trying to become
- what exists right now
- what is explicitly not built yet
- what the next correct move is for each component
- what must be proven before stronger claims are made

This is a living document. When code and docs disagree, fix them together in the same change.

---

## Mission

Build a privacy-first file transfer system with:

- **Human-friendly coordination** — short wordlist codes, no accounts, no URLs
- **End-to-end encryption** — the relay cannot read content, period
- **Direct-first transport** — LAN and hole-punched WAN before relay fallback
- **Zero-knowledge relay** — optional, self-hostable, learns nothing about payload
- **Resumable transfers** — large files survive network interruptions
- **Composable core** — `bore-core` is a library, not just plumbing for the CLI
- **Observable infrastructure** — relay monitoring, metrics, and alerting

---

## Monorepo structure

```text
bore/
├── crates/                       # Rust workspace
│   ├── bore-core/                # transfer engine, crypto, protocol
│   └── bore-cli/                 # operator CLI
├── services/                     # Go services
│   ├── relay/                    # zero-knowledge relay server
│   └── bore-admin/               # relay monitoring dashboard
├── lib/                          # Go libraries
│   └── punchthrough/             # NAT traversal and hole-punching
└── docs/                         # design documents
```

Each Go component has its own `go.mod` for independent dependency management:
- `services/relay/go.mod` — module `github.com/dunamismax/bore/services/relay`
- `services/bore-admin/go.mod` — module `github.com/dunamismax/bore/services/bore-admin`
- `lib/punchthrough/go.mod` — module `github.com/dunamismax/bore/lib/punchthrough`

---

## Component snapshots

### bore-core / bore-cli (Rust)

**Current phase: Phase 3 — transfer engine (done)**

What exists:
- Rust workspace with `bore-core` and `bore-cli`
- CLI prints truthful project status and component state
- Core domain types with serde serialization: session, transfer, protocol, error, code
- Frame codec for wire-format encoding/decoding
- Rendezvous code module with 256-word curated wordlist and entropy budget
- Noise XXpsk0 handshake with PAKE binding to rendezvous code
- SecureChannel with ChaCha20-Poly1305 AEAD encryption
- HKDF-SHA256 PSK derivation from rendezvous codes
- Counter-based nonces with replay detection
- Multi-segment framing, rekey support, key material zeroization
- Transfer engine: chunking (256 KiB default), streaming over SecureChannel, SHA-256 integrity verification
- Binary wire format for header/chunk/end messages (type-tagged, big-endian)
- Filename validation: path traversal, null bytes, relative components, length limits
- 159 total tests including transfer engine integration and error-path tests
- Design docs: threat model, crypto design

What does **not** exist yet:
- Direct peer-to-peer transport
- Relay protocol integration from Rust client
- Rendezvous code exchange over network
- Resumable session state persistence
- NAT traversal integration

### relay (Go)

**Current phase: Phase 2 — WebSocket transport (done)**

What exists:
- Room model with state machine (Waiting → Active → Closed)
- Thread-safe in-memory registry with TTL reaper
- WebSocket upgrade handler with sender/receiver routing
- Bidirectional frame relay with streaming back-pressure
- Clean close propagation and error handling
- HTTP server with SIGTERM/SIGINT handling
- 32 tests (21 room + 11 transport integration)

What does **not** exist yet:
- Rate limiting or resource management
- Configuration system (TOML file, CLI flags)
- Health endpoints or metrics
- Docker image or systemd unit

### punchthrough (Go)

**Current phase: Phase 2 — UDP hole-punching engine (done)**

What exists:
- STUN client with NAT type classification via pion/stun v3
- NAT type detection (Full Cone, Restricted Cone, Port-Restricted Cone, Symmetric)
- UDP hole-punching engine with simultaneous open strategy
- 24-byte binary punch protocol (ping/pong/ack)
- Strategy selection based on NAT type pairs
- CLI with `probe` and `version` commands

What does **not** exist yet:
- Coordination protocol (signaling via relay)
- Probe cache (SQLite)
- CLI `punch` command
- Real-world NAT testing beyond loopback

### bore-admin (Go)

**Current phase: Phase 0 — truthful scaffold (done)**

What exists:
- Go module and placeholder entry point

What does **not** exist yet:
- Relay health polling
- SQLite metrics storage
- Terminal UI or web dashboard
- Alerting engine

---

## Quality gates

### Rust (all phases)

```bash
cargo check
cargo test
cargo fmt --check
cargo clippy --workspace --all-targets -- -D warnings
```

### Go — relay

```bash
cd services/relay
go build ./...
go test ./...
go vet ./...
```

### Go — punchthrough

```bash
cd lib/punchthrough
go build ./...
go test ./...
go vet ./...
```

### Go — bore-admin

```bash
cd services/bore-admin
go build ./...
go vet ./...
```

---

## Working rules

1. **Rust for the client, Go for the network layer.** Each language where it fits best.
2. **Core stays small, explicit, testable.** No IO in the library crate unless behind a trait.
3. **CLI and docs stay aligned.** The CLI prints what exists, not what's aspirational.
4. **No security claims before proof.** Don't claim E2E encryption until the crypto is implemented, tested, and reviewed.
5. **The relay is a dumb pipe.** It forwards bytes. Any logic that inspects payload content is a bug.
6. **Docs are product surface.** Keep them current. Stale docs are bugs.
7. **Phase labels stay truthful.** A phase is "done" only when exit criteria are met and verified.
8. **Tests prove claims.** If a property matters, it has a test. If it doesn't have a test, don't claim it.
9. **Go modules stay independent.** Each Go component manages its own dependencies.
10. **Monorepo coherence.** Components are developed together, documented together, released together.

---

## Tracking conventions

| Term | Meaning |
|------|---------|
| **done** | Implemented and verified |
| **checked** | Verified by command or test output |
| **planned** | Intentional, not started |
| **in-progress** | Actively being worked on |
| **blocked** | Cannot proceed without a decision or dependency |
| **risk** | Plausible failure mode that could distort the design |
| **decision** | A durable call with consequences |

---

## Source-of-truth mapping

| File | Owns |
|------|------|
| `README.md` | Public-facing project description, honest status |
| `BUILD.md` | Unified execution manual, phase tracking, decisions |
| `ARCHITECTURE.md` | Technical design, protocol notes, data flow |
| `SECURITY.md` | Threat model, security policy, disclosure process |
| `Cargo.toml` | Rust workspace shape, shared dependency policy |
| `crates/bore-core/` | Domain types, transfer model, crypto layer, protocol |
| `crates/bore-cli/` | Operator-facing CLI surface |
| `services/relay/` | Relay server: room model, WebSocket transport |
| `services/bore-admin/` | Relay monitoring dashboard |
| `lib/punchthrough/` | NAT traversal: STUN client, hole-punch engine |
| `docs/threat-model.md` | Threat model: actors, assets, attack scenarios |
| `docs/crypto-design.md` | Cryptographic design document |

**Invariant:** If docs, types, and CLI output ever disagree, the next change must reconcile all three.

---

## Dependency strategy

### Rust dependencies (current)

| Crate | Purpose | Phase |
|-------|---------|-------|
| `snow` | Noise Protocol XXpsk0 handshake + ChaCha20-Poly1305 transport | 2 |
| `hkdf` + `sha2` | HKDF-SHA256 PSK derivation; SHA-256 transfer integrity | 2, 3 |
| `zeroize` | Key material cleanup on drop | 2 |
| `rand` | CSPRNG for keypair generation | 2 |
| `tokio` | Async runtime for handshake IO | 2 |
| `serde` + `serde_json` | Serialization for protocol messages | 1 |
| `tracing` + `tracing-subscriber` | Structured observability | 1 |
| `thiserror` | Typed errors in core | 1 |
| `anyhow` | Error handling in CLI | 0 |
| `clap` | CLI argument parsing | 0 |

### Go dependencies (current)

| Module | Component | Purpose |
|--------|-----------|---------|
| `nhooyr.io/websocket` | relay | WebSocket transport |
| `github.com/pion/stun/v3` | punchthrough | STUN binding request/response |
| `github.com/pion/transport/v4` | punchthrough | Transitive dep of pion/stun |

---

## Immediate next moves

### bore-core — Phase 4: rendezvous and code exchange
1. Rendezvous code exchange over network transport
2. Peer discovery and connection coordination
3. Integration with relay for code-based matchmaking
4. Session lifecycle: code generation → exchange → handshake → transfer

### relay — Phase 3: rate limiting
1. Per-IP rate limiting for room creation
2. Max concurrent rooms with 503 on limit
3. Max transfer size per room
4. Connection TTLs and idle timeouts

### punchthrough — Phase 3: coordination protocol
1. Define minimal signaling protocol
2. Run signaling over WebSocket to relay
3. Integration test with mock signaling server

### bore-admin — Phase 1: relay health polling
1. Define metrics model (RelaySnapshot, HealthStatus)
2. Implement HTTP poller for relay's /health endpoint
3. Initialize SQLite storage with schema creation
4. Wire into CLI with --relay flag

---

## Decisions

### decision: Rust + Go monorepo
Rust handles the hard problems: crypto, file chunking, integrity verification, transport abstraction. Go handles the network infrastructure: relay server (goroutine-per-room concurrency), NAT traversal (concurrent UDP probes), monitoring (bubbletea TUI). Each language where it fits best, in one repo for coherent development.

### decision: Independent Go modules
Each Go component has its own `go.mod`. This keeps dependency trees independent (relay doesn't need pion/stun, punchthrough doesn't need websocket), enables independent builds, and avoids a top-level `go.work` that would couple versioning.

### decision: WebSocket relay transport
Firewall-friendly (port 443), works behind reverse proxies, bidirectional binary framing. The relay is a dumb pipe — it doesn't need sophisticated protocol support.

### decision: Noise XXpsk0 for key exchange
Mutual authentication without pre-shared keys, PAKE binding to rendezvous codes via HKDF-derived PSK. The short code is a cryptographic input, not just a routing hint.

---

## Progress log

### 2026-03-22

- Monorepo established with bore-core (Rust Phase 2), bore-cli (Rust Phase 2)
- Phases 0-2 complete for bore-core: domain types, protocol design, cryptographic layer
- Relay merged from standalone repo: Phases 0-2 complete (room model, WebSocket transport, 32 tests)
- Punchthrough merged from standalone repo: Phases 0-2 complete (STUN client, hole-punch engine)
- bore-admin merged from standalone repo: Phase 0 complete (scaffold)
- All components compile: `cargo check` ✓, `go build ./...` ✓ for all Go modules
- Module paths updated to monorepo scheme
- Unified README and BUILD manual written
- Phase 3 complete for bore-core: transfer engine with chunking, streaming, SHA-256 integrity verification
- Transfer engine: binary wire format (header/chunk/end), 256 KiB chunks, filename validation
- 159 total tests (12 new error-path and boundary-condition tests for the transfer engine)
- All quality gates pass: cargo test ✓, clippy ✓, fmt ✓, cargo check for bore-cli ✓
- Updated lib.rs project snapshot, BUILD.md, README.md to reflect Phase 3 completion

---

*Update this log only with things that actually happened.*
