# BUILD.md

## Purpose

This is the execution manual for bore's current rewrite lane.

Use it to answer four questions quickly:

1. what the repo is actually building now
2. what is shippable today
3. what is still legacy baggage
4. what the next correct move is

If this file and the code disagree, update both in the same change.

---

## Current Truth

bore is no longer a Rust-first project.

The active direction is:

- **Go** for the current client, protocol implementation, relay, and network tooling
- **Zig** for future native/operator-facing tooling only if it clearly earns the complexity
- **C** only for narrow low-level or FFI cases
- **Rust** only as temporary legacy reference while the cutover settles

### What is working today

- Go client in `client/` with:
  - rendezvous code generation/parsing
  - Noise XXpsk0 handshake
  - encrypted transfer channel
  - file send/receive over relay
  - CLI with `send`, `receive`, `status`, `components`
- Go relay in `services/relay/`
- Go NAT probing / hole-punching groundwork in `lib/punchthrough/`

### What is not done yet

- direct transport integrated into the client path
- resumable transfer state
- relay hardening and metrics
- admin surface beyond scaffold
- full legacy Rust removal
- doc cleanup for `ARCHITECTURE.md` and `SECURITY.md`

---

## Monorepo Layout

```text
bore/
├── client/                  # active Go client rewrite
├── services/
│   ├── relay/              # active Go relay service
│   └── bore-admin/         # scaffolded ops surface
├── lib/
│   └── punchthrough/       # NAT probing + hole-punching primitives
├── crates/                 # legacy Rust reference only
├── README.md               # public project status
├── BUILD.md                # this file
└── REWRITE_TRACKER.md      # rewrite handoff / resume state
```

---

## Component Snapshots

### `client/` — active Go client

**Status:** working relay-based transfer path

What exists:

- `client/cmd/bore/main.go`
- rendezvous code model and parsing
- full code format: `room_id-channel-word-word-word`
- HKDF-derived PSK from rendezvous code
- Noise `XXpsk0` handshake
- secure channel framing over arbitrary `io.ReadWriter`
- transfer engine with header/chunk/end framing
- SHA-256 integrity verification
- WebSocket relay transport
- end-to-end Go test coverage for code / crypto / engine / transport / rendezvous
- integration test covering send/receive over a relay-style WebSocket server

What is still missing:

- direct-first transport selection
- resume support
- directory transfer
- transfer history
- richer progress reporting

### `services/relay/` — active Go relay

**Status:** functional

What exists:

- room registry
- room state machine
- WebSocket sender/receiver flow
- bidirectional frame relay
- graceful shutdown handling
- test coverage for room and transport behavior

What is still missing:

- rate limiting
- health endpoint
- metrics endpoint
- resource controls / quotas
- packaging/service deployment artifacts

### `lib/punchthrough/` — active Go NAT tooling

**Status:** partial, not integrated into the client flow

What exists:

- STUN probing
- NAT classification
- UDP hole-punching primitives
- CLI for probing

What is still missing:

- client integration
- coordination/signaling path
- robust real-world test matrix

### `services/bore-admin/` — scaffold

**Status:** truthful placeholder only

What exists:

- module scaffolding
- placeholder CLI entry point

What is still missing:

- relay polling
- storage
- TUI / web UI
- alerting

### `crates/` — legacy Rust reference

**Status:** frozen

Rules:

- do not add new feature work here
- use only for protocol comparison, migration confidence, or historical context
- remove once the Go path is clearly the keeper and docs are fully updated

---

## Build / Run / Verify

### Prerequisites

- Go `1.26.1`
- No top-level `go.work` currently; build and test per module

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

### Local relay-based smoke flow

Terminal 1:

```bash
cd services/relay
RELAY_ADDR=127.0.0.1:8080 go run ./cmd/relay
```

Terminal 2:

```bash
cd client
./bore send ./payload.txt --relay http://127.0.0.1:8080
```

Terminal 3:

```bash
cd client
./bore receive <code> --relay http://127.0.0.1:8080
```

Expected result:

- sender prints a full rendezvous code
- receiver completes successfully
- sender and receiver SHA-256 values match
- output file matches input bytes

---

## Verification Performed In This Recovery Pass

Completed on 2026-03-23:

```bash
cd client && go test ./...
cd client && go build ./cmd/bore
```

Manual smoke completed against the **real Go relay** on `127.0.0.1:18080`:

- `bore send <file> --relay http://127.0.0.1:18080`
- `bore receive <code> --relay http://127.0.0.1:18080`
- byte-for-byte file comparison succeeded
- SHA-256 matched on both sides

Important bug fixed during verification:

- CLI flag parsing now supports both positional-first and flag-first usage for `send` and `receive`

---

## Working Rules

1. **No new Rust architecture work.**
2. **Keep the relay payload-blind.** If it can inspect file contents, the design regressed.
3. **Treat the rendezvous code as cryptographic input, not just a locator.**
4. **Prefer direct-first eventually, but do not lie about current capability.** Right now the reliable verified path is relay-based.
5. **Keep docs honest.** Aspirational language belongs in planned sections, not current-state summaries.
6. **Run the narrowest meaningful verification first.** Broaden only when the change surface demands it.
7. **If you touch migration assumptions, update `REWRITE_TRACKER.md` in the same pass.**

---

## Immediate Next Moves

### Highest leverage path

1. settle whether `client/` is the permanent shipped CLI or an intermediate step before a Zig-facing frontend
2. integrate `lib/punchthrough/` into the client transport selection path
3. add resumable transfer state
4. add relay rate limiting + health/metrics
5. promote `bore-admin` from scaffold to useful operator surface

### If the goal is cleanup instead of features

1. rewrite `ARCHITECTURE.md` around the Go client reality
2. rewrite `SECURITY.md` to stop implying Rust is active architecture
3. remove `crates/`, `Cargo.toml`, and `Cargo.lock` once Stephen is satisfied the Rust reference is no longer needed

---

## Risks And Open Questions

### Risk: legacy Rust still confuses the repo story

Mitigation:
- keep README / BUILD / tracker explicit that Rust is frozen
- remove Rust once confidence is high enough

### Risk: punchthrough exists but is not integrated

Mitigation:
- avoid marketing bore as direct-first today
- describe direct transport as the next implementation target, not current capability

### Risk: no resumable state yet

Mitigation:
- keep transfer claims modest
- add checkpoint model before claiming interruption recovery

### Open question: where Zig actually belongs

The repo direction is Zig / Go / C only, but no Zig implementation should land just for aesthetic symmetry. Use it only where it clearly improves:

- packaging
- native UX
- local operator tools
- distribution simplicity

If the Go client keeps shipping cleanly and does not impose real pain, do not force a second frontend prematurely.

---

## Removal Gate For Legacy Rust

Do not remove the Rust reference until all are true:

- [x] Go client compiles
- [x] Go client tests pass
- [x] Go client succeeds in real relay smoke testing
- [ ] transport roadmap is stable enough that the Rust reference is no longer needed
- [ ] `ARCHITECTURE.md` and `SECURITY.md` are rewritten to match the new world
- [ ] Stephen explicitly wants the cleanup landed

---

## Resume Checklist

If you are resuming this repo later, do this in order:

1. read `README.md`
2. read this file
3. read `REWRITE_TRACKER.md`
4. inspect `git status`
5. treat `client/` as the active client
6. choose one lane only:
   - direct transport integration
   - resume support
   - relay hardening
   - legacy Rust removal
7. run focused verification before and after the change
