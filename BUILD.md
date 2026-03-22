# BUILD.md

## Purpose

This file is the execution manual for `bore`.

It keeps the repo honest while the project grows from a scaffold into a real, auditable file transfer system. At any point it should answer:

- what bore is trying to become
- what exists right now
- what is explicitly not built yet
- what the next correct move is
- what must be proven before stronger claims are made

This is a living document. When code and docs disagree, fix them together in the same change.

---

## Mission

Build a Rust-native file transfer tool that moves files between two machines with:

- **Human-friendly coordination** — short wordlist codes, no accounts, no URLs
- **End-to-end encryption** — the relay cannot read content, period
- **Direct-first transport** — LAN and hole-punched WAN before relay fallback
- **Zero-knowledge relay** — optional, self-hostable, learns nothing about payload
- **Resumable transfers** — large files survive network interruptions
- **Composable core** — `bore-core` is a library, not just plumbing for the CLI

The long-term product lets two parties move files with minimal ceremony, clear trust boundaries, and honest reporting about what is local, what is relayed, and what is encrypted.

---

## Repo snapshot

**Current phase: Phase 1 — protocol design and type foundations (done)**

What exists:
- Rust workspace with `bore-core` and `bore-cli`
- CLI prints truthful project status and component state
- Documentation: README, BUILD manual, ARCHITECTURE, SECURITY, LICENSE
- Quality gates: `cargo check`, `cargo test`, `cargo fmt`, `cargo clippy`
- Core domain types with serde serialization: session, transfer, protocol, error, code
- Frame codec (`codec.rs`) for wire-format encoding/decoding
- Rendezvous code module (`code.rs`) with 256-word curated wordlist and entropy budget
- Protocol message structs with typed payloads and serde round-trip tests
- Exhaustive session state machine tests (93 total tests)
- Threat model document (`docs/threat-model.md`)
- Crypto design document (`docs/crypto-design.md`)
- `tracing` subscriber in CLI, `thiserror` derives in core errors
- Dependencies: `serde`, `serde_json`, `tracing`, `tracing-subscriber`, `thiserror`

What does **not** exist yet:
- Cryptographic protocol or key exchange (design doc exists, implementation planned for Phase 2)
- Transfer engine (chunking, streaming, integrity)
- Direct peer-to-peer transport
- Relay protocol or relay service
- Resumable session state persistence
- NAT traversal or hole-punching

---

## Source-of-truth mapping

| File | Owns |
|------|------|
| `README.md` | Public-facing project description, honest status |
| `BUILD.md` | Implementation map, phase tracking, decisions, working rules |
| `ARCHITECTURE.md` | Technical design, protocol notes, data flow |
| `SECURITY.md` | Threat model, security policy, disclosure process |
| `Cargo.toml` (root) | Workspace shape, shared dependency policy |
| `crates/bore-core/src/lib.rs` | Domain types, transfer model, protocol layer |
| `crates/bore-core/src/codec.rs` | Frame encoding/decoding for wire protocol |
| `crates/bore-core/src/code.rs` | Rendezvous code generation, parsing, wordlist |
| `crates/bore-cli/src/main.rs` | Operator-facing CLI surface |
| `docs/threat-model.md` | Threat model: actors, assets, attack scenarios |
| `docs/crypto-design.md` | Cryptographic design: Noise XX, PAKE, ChaCha20-Poly1305 |
| [relay](https://github.com/dunamismax/relay) (Go, separate repo) | Relay fallback transport — zero-knowledge stream broker |
| [punchthrough](https://github.com/dunamismax/punchthrough) (Go, separate repo) | NAT traversal and UDP hole-punching for direct connections |

**Invariant:** If docs, types, and CLI output ever disagree, the next change must reconcile all three.

---

## Working rules

1. **Rust only.** No Python, no Node, no shell scripts in the critical path.
2. **Core stays small, explicit, testable.** No IO in the library crate unless behind a trait.
3. **CLI and docs stay aligned.** The CLI prints what exists, not what's aspirational.
4. **No security claims before proof.** Don't claim E2E encryption until the crypto is implemented, tested, and reviewed.
5. **Direct and relay are explicit modes.** No hidden fallback magic — the user knows which transport is active.
6. **Docs are product surface.** Keep them current. Stale docs are bugs.
7. **Phase labels stay truthful.** A phase is "done" only when exit criteria are met and verified.
8. **Prefer reversible structure over speculative complexity.** Don't build abstractions for hypothetical futures.
9. **Every dependency must justify itself.** Prefer the Rust ecosystem's battle-tested crates. No YOLO deps.
10. **Tests prove claims.** If a property matters, it has a test. If it doesn't have a test, don't claim it.

---

## Tracking conventions

Use this language consistently in docs, commits, and issues:

| Term | Meaning |
|------|---------|
| **done** | Implemented and verified |
| **checked** | Verified by command or test output |
| **planned** | Intentional, not started |
| **in-progress** | Actively being worked on |
| **blocked** | Cannot proceed without a decision or dependency |
| **risk** | Plausible failure mode that could distort the design |
| **decision** | A durable call with consequences |

When new work lands, update: repo snapshot, phase dashboard, decisions (if architecture changed), and progress log with date and what was verified.

---

## Quality gates

### Minimum gate (all phases)

```bash
cargo check
cargo test
cargo fmt --check
cargo clippy --workspace --all-targets -- -D warnings
```

### Phase 2+ additions

```bash
cargo test -- --ignored          # slow / integration tests
cargo bench                      # performance regression checks
```

### Phase 8+ additions

```bash
cargo audit                      # dependency vulnerability scan
cargo deny check                 # license + advisory checks
cargo fuzz                       # fuzz protocol parsing
```

For docs-only changes, verify wording consistency and that repo state matches documented commands.

If a gate is temporarily unavailable, document why. Never silently skip.

---

## Dependency strategy

### Current dependencies

| Crate | Purpose | Phase |
|-------|---------|-------|
| `anyhow` | Error handling (CLI) | 0 |
| `clap` | CLI argument parsing | 0 |
| `thiserror` | Typed errors in core | 1 |
| `serde` + `serde_json` | Serialization (protocol messages, types) | 1 |
| `tracing` | Structured observability | 1 |
| `tracing-subscriber` | Log output in CLI | 1 |

### Planned dependencies (subject to design decisions)

| Crate | Purpose | Phase | Notes |
|-------|---------|-------|-------|
| `tokio` | Async runtime | 2 | Required for network IO |
| `snow` | Noise Protocol (XX) | 2 | E2E key exchange. Alternative: `noise-protocol` |
| `chacha20poly1305` | AEAD encryption | 2 | Post-handshake symmetric encryption |
| `blake3` | Hashing / integrity | 3 | File chunk verification |
| `indicatif` | Progress bars | 3 | Transfer progress UI |
| `quinn` / `s2n-quic` | QUIC transport | 5 | Direct P2P + relay transport |
| `stun-rs` | STUN/TURN | 5 | NAT traversal |
| `axum` or `hyper` | Relay HTTP/WebSocket | 6 | Relay server framework |
| `directories` | XDG paths | 7 | Config + state persistence |
| `toml` | Config file format | 7 | User/relay configuration |
| `bip39` or custom | Wordlist codes | 4 | Human-friendly rendezvous codes |

Every dependency addition must be justified in the decisions log with: what it replaces, what it costs (compile time, binary size, audit surface), and whether a simpler alternative exists.

---

## Phase dashboard

### Phase 0 — Truthful scaffold
**Status: done / checked**

Goals:
- [x] Create workspace with `bore-core` and `bore-cli`
- [x] CLI prints truthful project status and component state
- [x] Docs: README, BUILD manual, LICENSE, .gitignore
- [x] Pass all quality gates
- [x] Foundation domain types in bore-core

Exit criteria:
- Repo structure is stable enough for real design work
- Public docs do not overclaim
- Executable entry point exists and prints truth
- CLI and docs tell the same story

---

### Phase 1 — Protocol design and type foundations
**Status: done / checked**

This phase is about making decisions, not writing transfer code. The output is types, design docs, and tests — not a working transfer.

Goals:
- [x] Write threat model document (`docs/threat-model.md`)
  - Define actors: sender, receiver, relay operator, network observer, malicious relay
  - Define assets: file content, file metadata, sender/receiver identity, transfer timing
  - Define non-goals: anonymity (Tor-level), plausible deniability, multi-party transfer
- [x] Define core domain types in `bore-core`:
  - `SessionId` — unique per transfer session, cryptographically random
  - `TransferIntent` — what the sender wants to send (files, directories, metadata)
  - `TransferRole` — `Sender` | `Receiver`
  - `TransportMode` — `Direct` | `Relayed` | `Unknown`
  - `SessionState` — state machine: `Created → Waiting → Connected → Transferring → Complete | Failed`
  - `ProtocolVersion` — for future compatibility negotiation
  - `Capability` — feature flags for negotiation (compression, resume, etc.)
- [x] Design the session lifecycle and error model
  - Every state transition has an explicit error variant
  - Timeouts are first-class, not afterthoughts
  - Cancellation is clean from any state
- [x] Choose protocol envelope format
  - Length-prefixed binary frames with a type tag
  - Messages: `Hello`, `Offer`, `Accept`, `Reject`, `Data`, `Ack`, `Done`, `Error`, `Close`
  - Versioned from the start
- [x] Design human-friendly code model
  - Wordlist-based (e.g., `7-guitar-castle-moon`)
  - Entropy budget: ~34 bits default (3 words + channel)
  - Code lifetime and single-use semantics
  - Code-to-session binding mechanism (PAKE via Noise PSK)
- [x] Add `tracing` for structured observability
- [x] Add `thiserror` for typed error variants in core
- [x] Unit tests for all state transitions and type invariants (93 tests)

Exit criteria:
- [x] Design docs exist under `docs/` (threat-model.md, crypto-design.md)
- [x] Core types model the session honestly and completely
- [x] State machine has tests for every valid transition and every invalid one (exhaustive matrix)
- [x] Threat model has been written and reviewed (even if self-reviewed)
- [x] Protocol message types are defined with serde serialization

Risks:
- **risk (mitigated):** over-designing the protocol before any transfer code exists — kept types minimal and evolvable
- **risk (mitigated):** choosing crypto primitives too early without understanding the trust model — threat model written first, crypto design is a plan not implementation

---

### Phase 2 — Cryptographic layer
**Status: planned**

This phase implements the cryptographic handshake and encrypted channel. No file transfer yet — just the secure pipe.

Goals:
- [ ] Implement Noise Protocol XX handshake via `snow`
  - Mutual authentication: both sides prove identity without pre-shared keys
  - PAKE (Password-Authenticated Key Exchange) binding to the rendezvous code
  - The short code serves as the weak shared secret for PAKE
- [ ] Implement post-handshake AEAD symmetric encryption
  - ChaCha20-Poly1305 for the data channel
  - Unique nonces per frame, counter-based
  - Reject replayed or out-of-order frames
- [ ] Define key derivation for session keys
  - Derive from Noise handshake output
  - Separate keys for send/receive directions
  - Key rotation strategy for long transfers
- [ ] Implement `CryptoTransport` trait in bore-core
  - `async fn handshake(stream, role, code) -> Result<SecureChannel>`
  - `async fn send_frame(data) -> Result<()>`
  - `async fn recv_frame() -> Result<Vec<u8>>`
- [ ] Zeroize sensitive key material on drop
- [ ] Unit tests for handshake success, handshake failure (wrong code), frame encryption/decryption, replay rejection
- [ ] Property tests for frame codec round-tripping

Exit criteria:
- Two in-process peers can establish an encrypted channel using a shared code
- Wrong code produces a clear, non-panicking authentication failure
- Key material is zeroized on drop
- No plaintext file data is ever transmitted after handshake completes
- Tests prove all of the above

Risks:
- **risk:** implementing crypto from scratch instead of using audited primitives — use `snow` and `chacha20poly1305`, do not roll our own
- **risk:** PAKE binding to short codes may have entropy issues — quantify and document

---

### Phase 3 — Local transfer engine
**Status: planned**

Files actually move in this phase. Initially over an in-process or loopback channel, not real networking.

Goals:
- [ ] Define file manifest model
  - `FileEntry`: path, size, modified time, permissions (optional), blake3 hash
  - `TransferManifest`: ordered list of files, total size, entry count
  - Handle: single files, directories (recursive), symlinks (skip or follow — configurable)
- [ ] Implement chunking strategy
  - Fixed-size chunks (e.g., 256 KiB default, configurable)
  - Each chunk: index, offset, length, blake3 hash
  - Final chunk may be short
- [ ] Implement sender state machine
  - Build manifest from filesystem
  - Send manifest to receiver
  - Wait for receiver acceptance
  - Stream chunks in order
  - Handle back-pressure
  - Report progress
- [ ] Implement receiver state machine
  - Receive and validate manifest
  - Accept or reject (e.g., disk space check)
  - Receive chunks, verify integrity per-chunk
  - Write to temporary location, atomic rename on completion
  - Report progress
- [ ] Implement integrity verification
  - Per-chunk blake3 verification
  - Full-file hash verification on completion
  - Manifest hash for tamper detection
- [ ] Progress reporting trait
  - `on_transfer_start(manifest)`
  - `on_chunk_sent/received(index, bytes)`
  - `on_file_complete(path, hash)`
  - `on_transfer_complete(stats)`
  - `on_error(error)`
- [ ] Integration tests: in-process sender/receiver transferring real files through the crypto layer

Exit criteria:
- Files transfer correctly through an encrypted in-process channel
- Directory transfers preserve structure
- Corrupted chunks are detected and rejected
- Progress callbacks fire at the right moments
- Temporary files are cleaned up on failure

Risks:
- **risk:** large file handling (>4GB) needs explicit testing — don't assume everything fits in memory
- **risk:** filesystem edge cases (permissions, symlinks, long paths, unicode names) — test on macOS and Linux

---

### Phase 4 — Rendezvous and code exchange
**Status: planned**

This phase adds the human-facing coordination layer — the codes that connect sender and receiver.

Goals:
- [ ] Implement wordlist-based code generation
  - Format: `{n}-{word}-{word}-{word}` (e.g., `7-guitar-castle-moon`)
  - Wordlist: curated for pronounceability and low ambiguity (no homophones)
  - Configurable word count (2-5 words) for entropy/usability tradeoff
  - Default: 3 words + channel number = ~40 bits of entropy
- [ ] Implement code-to-session binding
  - Code is bound to session identity during PAKE handshake
  - Code is single-use: second attempt with same code fails
  - Code expires after configurable timeout (default: 5 minutes)
- [ ] Design rendezvous protocol
  - Sender registers code with rendezvous server (relay, or direct)
  - Receiver looks up code and gets sender's connection info
  - Rendezvous server learns the code and connection metadata, not file content
  - Rendezvous can be relay-hosted or a separate lightweight service
- [ ] Implement rendezvous client in bore-core
- [ ] Implement basic rendezvous server (can be part of relay, or standalone)
- [ ] CLI: `bore send <path>` generates code and waits
- [ ] CLI: `bore receive <code>` connects to sender
- [ ] Integration test: full send/receive flow over loopback with code exchange

Exit criteria:
- `bore send` generates a code and waits for a receiver
- `bore receive <code>` connects, authenticates, and receives files
- Wrong code produces a clear error, not a hang or crash
- Code expiry works correctly
- Rendezvous server is minimal and tested

---

### Phase 5 — Direct peer-to-peer transport
**Status: planned**

This phase adds real network transport — direct connections between peers.

Goals:
- [ ] Implement TCP transport for LAN transfers
  - mDNS or local broadcast for same-network discovery (optional, not required)
  - Direct TCP when both peers are on the same network
- [ ] Implement QUIC transport for WAN transfers
  - Using `quinn` or `s2n-quic`
  - QUIC provides: multiplexing, built-in TLS, connection migration, 0-RTT
  - bore's Noise handshake runs inside the QUIC connection (defense in depth)
- [ ] Implement NAT traversal
  - STUN for discovering public address
  - UDP hole-punching for direct WAN connections
  - ICE-lite for connectivity checking
  - Relay fallback when hole-punching fails (Phase 6)
- [ ] Implement transport abstraction in bore-core
  - `Transport` trait: `connect()`, `accept()`, `send()`, `recv()`
  - Implementations: `TcpTransport`, `QuicTransport`, `RelayTransport` (Phase 6)
  - Transport selection is explicit and reported to the user
- [ ] Connection quality reporting
  - Measure and report: latency, throughput, packet loss
  - Display transport mode in CLI progress output
- [ ] Integration tests over real network interfaces (loopback at minimum)

Exit criteria:
- Two machines on the same LAN can transfer files directly over TCP
- QUIC transport works over WAN (tested with two different networks)
- NAT hole-punching succeeds in common NAT configurations
- Transport mode is visible to the user at all times
- Fallback path to relay is clear (even if relay isn't built yet)

Risks:
- **risk:** NAT traversal is inherently unreliable — measure success rates, don't assume hole-punching works
- **risk:** QUIC implementations vary in maturity — evaluate `quinn` vs `s2n-quic` carefully
- **risk:** mDNS/Bonjour behavior differs across OS — start with explicit IP, add discovery later

---

### Phase 6 — Relay service
**Status: planned**

The relay is a dumb pipe. It forwards encrypted bytes between peers that can't connect directly. It cannot read, modify, or selectively drop content.

Goals:
- [ ] Create `bore-relay` crate
- [ ] Implement relay protocol
  - WebSocket-based for firewall friendliness
  - Room-based routing: sender creates room, receiver joins by code
  - Relay sees: room ID, connection metadata, encrypted byte counts
  - Relay does NOT see: file names, file content, sender/receiver identity beyond IP
- [ ] Implement relay server
  - Framework: `axum` + `tokio`
  - Room lifecycle: create, join, relay, close, timeout
  - Rate limiting per IP and per room
  - Configurable max room count, max transfer size, max connection duration
  - Health check endpoint
  - Metrics endpoint (optional, for operators)
- [ ] Implement relay client in bore-core
  - `RelayTransport` implements the `Transport` trait
  - Automatic reconnection on transient failures
  - Relay URL configurable via CLI flag or config file
- [ ] Operator documentation
  - Deployment guide (Docker, systemd, bare metal)
  - Configuration reference
  - Monitoring and alerting guidance
  - Privacy and legal considerations for relay operators
- [ ] Integration tests: full send/receive through relay
- [ ] Load testing: concurrent transfers, large files, connection churn

Exit criteria:
- Relay forwards encrypted traffic without being able to decrypt it
- Relay operator can configure limits and monitor usage
- Relay is deployable as a Docker container or systemd service
- Self-hosted relay works identically to public relay
- Relay cannot become the trust root — authentication is always end-to-end

Risks:
- **risk:** relay convenience becomes relay dependence — direct mode must always be attempted first
- **risk:** WebSocket framing overhead for large transfers — measure and optimize
- **risk:** relay abuse (spam rooms, bandwidth abuse) — rate limiting must be in place from day one

---

### Phase 7 — Resumable transfers and persistence
**Status: planned**

Large transfers over unreliable connections need to survive interruptions without starting over.

Goals:
- [ ] Design resume protocol
  - Sender and receiver track which chunks have been acknowledged
  - On reconnection, receiver reports its state, sender resumes from the gap
  - Resume token: session ID + manifest hash + chunk bitmap
  - Resume works across transport changes (e.g., started direct, resumed via relay)
- [ ] Implement persistent session state
  - State stored in XDG-compliant location (`~/.local/share/bore/`)
  - State includes: session ID, manifest, chunk progress, peer identity, transport mode
  - State is encrypted at rest (derived from session key or a local passphrase)
  - State expires after configurable timeout (default: 24 hours)
  - `bore history` shows past and in-progress transfers
- [ ] Implement `bore receive --resume <code>`
  - Reconnects to sender (or relay) and resumes from last acknowledged chunk
  - Validates manifest hasn't changed (no bait-and-switch)
- [ ] Implement sender-side resume support
  - Sender keeps state until transfer is complete or expired
  - Sender can resume from a different network/IP
- [ ] Configuration file support
  - TOML format at `~/.config/bore/config.toml`
  - Settings: default relay, word count, chunk size, state directory, timeouts
  - CLI flags override config file
- [ ] Tests for resume scenarios: network drop mid-transfer, transport change, manifest mismatch, expired state

Exit criteria:
- A 1GB transfer interrupted at 50% resumes and completes without retransmitting the first half
- Resume works across transport mode changes
- State is encrypted at rest
- Config file is loaded and respected
- `bore history` shows accurate transfer records

Risks:
- **risk:** resume protocol creates a "resume with different file" attack surface — manifest hash must be verified on reconnection
- **risk:** persistent state on disk introduces a new data-at-rest concern — encrypt state with session-derived key
- **risk:** config file introduces a new surface for surprising behavior — CLI flags always override, config is additive only

---

### Phase 8 — Hardening and security audit
**Status: planned**

This phase is about proving the security claims before making them publicly.

Goals:
- [ ] Formal threat model review
  - Review against STRIDE framework
  - Document accepted risks and mitigations
  - Enumerate trust boundaries with data flow diagrams
- [ ] Fuzz testing
  - Fuzz protocol parser with `cargo-fuzz`
  - Fuzz handshake with malformed messages
  - Fuzz chunk processing with corrupted data
  - Fuzz relay room management with adversarial clients
- [ ] Dependency audit
  - `cargo audit` for known vulnerabilities
  - `cargo deny` for license compliance
  - Review transitive dependency count and minimize
  - Document audit results
- [ ] Static analysis
  - `cargo clippy` with all lints
  - `cargo semver-checks` for API compatibility
  - Consider `cargo careful` for undefined behavior detection
- [ ] Performance profiling
  - Benchmark: throughput, latency, memory usage, CPU usage
  - Profile large file transfers (1GB, 10GB, 100GB)
  - Profile many-file transfers (10,000+ files)
  - Identify and fix bottlenecks
- [ ] Error handling audit
  - Every error path produces a clear, actionable message
  - No panics in library code (except truly unrecoverable states)
  - No error swallowing
- [ ] External security review (if possible)
  - Engage external reviewers for the crypto layer
  - Focus: key exchange, PAKE binding, nonce handling, frame authentication
  - Publish findings and fixes

Exit criteria:
- Threat model is documented and reviewed
- Fuzz testing has run for 24+ hours with no crashes
- All dependencies are audited and compliant
- Performance baselines are documented
- Security review findings are addressed
- SECURITY.md is updated with accurate claims

Risks:
- **risk:** fuzz testing finds issues late that require protocol-level changes — start fuzzing earlier in an ad-hoc capacity
- **risk:** external security review may not happen — self-review with published methodology as fallback
- **risk:** performance optimization breaks correctness — benchmark suite must include correctness assertions

---

### Phase 9 — Cross-platform polish and distribution
**Status: planned**

bore should work reliably on Linux, macOS, and Windows. Releases should be trivial to install.

Goals:
- [ ] Cross-platform testing
  - CI matrix: Linux (x86_64, aarch64), macOS (x86_64, aarch64), Windows (x86_64)
  - Test filesystem edge cases per platform (permissions, long paths, unicode, symlinks)
  - Test NAT traversal on real consumer networks
- [ ] Release engineering
  - GitHub Actions CI/CD pipeline
  - Automated builds for all target platforms
  - Binary releases with checksums and signatures
  - `cargo install bore-cli` works from crates.io
  - Homebrew formula (`brew install bore`)
  - AUR package
  - `.deb` and `.rpm` packages via `cargo-deb` / `cargo-generate-rpm`
  - Static Linux binary (musl)
- [ ] Shell completions
  - Generate completions for bash, zsh, fish, PowerShell via clap
  - Include in release packages
- [ ] Man pages
  - Generate from clap metadata or hand-write
  - Include in release packages
- [ ] User documentation
  - Quick start guide
  - Configuration reference
  - Relay operator guide
  - FAQ / troubleshooting
  - Protocol specification (for interoperability)
- [ ] CLI polish
  - Colored output with graceful degradation (no color in pipes)
  - Progress bars with ETA and speed
  - Quiet mode (`--quiet`)
  - JSON output mode (`--json`) for scripting
  - Verbose mode (`--verbose`) for debugging

Exit criteria:
- `bore send` / `bore receive` works reliably on Linux, macOS, and Windows
- Installation from Homebrew, crates.io, and GitHub releases works
- Shell completions and man pages are included
- CLI output is polished and accessible
- Documentation covers all user-facing features

Risks:
- **risk:** Windows filesystem semantics differ significantly (permissions, symlinks, path length) — may need platform-specific code paths
- **risk:** packaging for every distribution is a maintenance burden — prioritize crates.io and GitHub releases, add others on demand
- **risk:** NAT traversal success rates vary wildly across consumer networks — document expected success rates honestly

---

### Phase 10 — Ecosystem: library bindings, GUI, integrations
**Status: planned / aspirational**

bore-core is a library. Other frontends and integrations should be possible.

Goals:
- [ ] Stabilize `bore-core` public API
  - Document every public type and function
  - Semantic versioning with `cargo semver-checks`
  - API stability guarantees start here
- [ ] C FFI bindings
  - `bore-ffi` crate exposing core operations via C ABI
  - Header generation via `cbindgen`
  - Enables: Python, Ruby, Swift, Kotlin bindings
- [ ] Python bindings
  - `bore-python` via PyO3
  - Publish to PyPI
  - `pip install bore` for scripting and automation
- [ ] GUI exploration
  - Desktop: Tauri-based GUI wrapping bore-core
  - Mobile: feasibility study for iOS/Android via FFI
  - Web: feasibility study for WASM compilation of bore-core
- [ ] Integration patterns
  - Webhook notifications on transfer complete
  - Pipe mode: `cat file | bore send --stdin` / `bore receive <code> --stdout | tar xz`
  - Programmatic API for embedding in other tools
- [ ] Protocol specification
  - Formal specification document for interoperability
  - Version negotiation rules
  - Reference test vectors

Exit criteria:
- bore-core has a stable, documented public API
- At least one non-Rust binding exists and works
- Pipe mode works for streaming use cases
- Protocol specification exists for third-party implementations

Risks:
- **risk:** stabilizing the public API too early locks in design mistakes — wait until the protocol and transport layers are proven
- **risk:** FFI bindings expand the maintenance surface significantly — start with one binding (Python) and validate demand
- **risk:** WASM compilation may require significant refactoring of async/networking code — treat as research, not commitment

---

## Decisions

### decision-0001: Rust workspace from the start
**Phase:** 0

Supports growth without forcing a monolith. Clean separation between CLI, core logic, and future relay. Enables independent versioning later.

### decision-0002: Phase 0 is intentionally minimal
**Phase:** 0

Avoids fake-finished transfer code. Puts honesty and compile health ahead of premature implementation. The scaffold exists to hang real work on.

### decision-0003: CLI-first entry point
**Phase:** 0

Fastest way to exercise architecture and developer workflow. Easiest operator surface for early experiments. GUI and library consumers come later.

### decision-0004: No security claims without proof
**Phase:** 0

The project's credibility depends on disciplined truth-telling. Protocol and crypto are the hard parts — they must be earned, not assumed.

### decision-0005: Noise Protocol XX for key exchange
**Phase:** 1 (pending)

Noise XX provides mutual authentication without pre-shared keys. Well-studied, audited implementations exist in Rust (`snow`). XX pattern allows both sides to authenticate without prior contact. Alternative considered: plain Diffie-Hellman + SAS verification — rejected because Noise provides a more complete framework.

### decision-0006: PAKE binding to rendezvous code
**Phase:** 1 (pending)

The short human-readable code serves as the weak shared secret for PAKE. This means the code isn't just a routing hint — it's a cryptographic input. This decision has entropy implications: shorter codes are easier to use but more vulnerable to online brute-force. Mitigation: rate limiting on the rendezvous/relay side, single-use codes, and short expiry.

### decision-0007: QUIC as the primary WAN transport
**Phase:** 5 (pending)

QUIC provides multiplexing, connection migration, 0-RTT resumption, and built-in congestion control. bore's Noise handshake runs inside QUIC as defense-in-depth (QUIC's TLS is for the transport; Noise is for end-to-end peer authentication). Alternative considered: raw TCP + custom framing — rejected because QUIC solves too many transport problems to ignore.

### decision-0008: WebSocket relay transport
**Phase:** 6 (pending)

WebSockets are firewall-friendly (port 443), widely supported, and easy to deploy behind reverse proxies. The relay is a dumb pipe — it doesn't need sophisticated protocol support. Alternative considered: QUIC relay — possible future optimization, but WebSocket is the pragmatic starting point.

---

## Risks

### Active risks

- **risk:** naming the product goal too aggressively before protocol work pushes toward theater instead of proof
- **risk:** human-friendly codes can become a UX win and a security failure if trust semantics are vague — entropy budget must be explicit
- **risk:** relay convenience can quietly become relay dependence if transport boundaries are not explicit early
- **risk:** Noise Protocol + PAKE integration is non-trivial — may need custom protocol composition, which increases audit surface
- **risk:** QUIC implementations are still maturing in Rust — may hit edge cases in NAT traversal and connection migration
- **risk:** cross-platform filesystem handling (permissions, symlinks, long paths, unicode) is a minefield
- **risk:** resume protocol needs careful design to prevent a "resume with different file" attack

### Mitigated risks

- **risk (mitigated):** overbuilding Phase 0 — kept intentionally minimal with only truthful scaffold
- **risk (mitigated):** docs drifting from reality — established sync discipline in working rules

---

## Open questions

These questions need answers before or during the indicated phase:

| Question | Phase | Impact |
|----------|-------|--------|
| Exact entropy budget for rendezvous codes (bits vs. usability)? | 1 | Security vs. UX tradeoff |
| Should the relay also serve as the rendezvous server, or separate? | 4 | Architecture simplicity vs. separation of concerns |
| Support for streaming transfers (stdin/stdout pipe mode)? | 3 | API design, chunking model |
| Should bore support multi-file transfers as a single atomic operation? | 3 | UX and failure semantics |
| Protocol extensibility: how to add features without breaking compat? | 1 | Long-term protocol health |
| Relay federation: should relays be able to chain? | 10 | Scale architecture |
| Mobile support: is FFI sufficient or does bore need a daemon mode? | 10 | Architecture |

---

## Immediate next moves

Recommended order for starting Phase 1:

1. Write threat model document under `docs/threat-model.md`
2. Add `tracing` and `thiserror` to workspace dependencies
3. Define core domain types: `SessionId`, `TransferIntent`, `TransferRole`, `SessionState`
4. Implement session state machine with exhaustive tests
5. Design protocol message types with serde serialization
6. Design the rendezvous code format and entropy budget
7. Write design note for crypto approach (Noise XX + PAKE)
8. Only then move to Phase 2 implementation

---

## Progress log

### 2026-03-22

- Initialized repo as Rust workspace
- Added `bore-core` and `bore-cli`
- Added executable `bore` binary printing current scaffold status
- Added README, BUILD manual, MIT license, .gitignore
- Verified scaffold with `cargo check`
- Restored richer README / BUILD project-tracking notes after docs-tightening regression
- Realigned docs around CLI behavior and `bore-core` runtime snapshot
- Verified with `git diff --check`, `cargo check`, `cargo test`, `cargo run -p bore-cli -- status`, `cargo run -p bore-cli -- components`
- Expanded project scope: 10-phase roadmap, technical architecture doc, security policy, dependency strategy, detailed protocol design notes
- Added foundational domain types to bore-core: error types, transfer model, session state, protocol version, transport mode
- Expanded CLI with planned command structure
- **Phase 1 complete:**
  - Wrote threat model document (`docs/threat-model.md`): actors, assets, attack scenarios, trust boundaries, metadata exposure matrix
  - Wrote crypto design document (`docs/crypto-design.md`): Noise XX + PAKE, ChaCha20-Poly1305, key derivation, entropy analysis, key lifecycle
  - Added `thiserror` to bore-core, migrated all error types from manual Display/Error impls to derive macros
  - Added `tracing` + `tracing-subscriber` to workspace, initialized subscriber in bore-cli
  - Added `serde` + `serde_json` to workspace, added Serialize/Deserialize to all domain types
  - Created `codec.rs`: frame encoding/decoding with round-trip tests
  - Created `code.rs`: rendezvous code generation/parsing, 256-word curated wordlist, entropy budget documentation
  - Added `TransportMode::Unknown` variant
  - Added concrete protocol message structs (`HelloMessage`, `OfferMessage`, `AcceptMessage`, `RejectMessage`, `DataMessage`, `AckMessage`, `DoneMessage`, `ErrorMessage`, `CloseMessage`) with serde serialization
  - Added `CodeError` variant to error taxonomy
  - Exhaustive session state machine tests (full transition matrix)
  - 93 total tests, all passing
  - All quality gates pass: `cargo check`, `cargo test`, `cargo fmt --check`, `cargo clippy -- -D warnings`
  - Updated `project_snapshot()` to reflect Phase 1 state
  - Updated README, BUILD.md, source-of-truth mapping, dependency table

---

*Update this log only with things that actually happened.*
