# BUILD.md

## Purpose

This is the execution manual for bore's current lane.

Use it to answer four questions quickly:

1. what the repo is actually building now
2. what is shippable today
3. what is still scaffold or TODO
4. what the next correct move is

If this file and the code disagree, fix both in the same change.

---

## Current Truth

bore is now a **Go-first** repo with an explicitly limited future stack:

- **Go** for the current client, relay, NAT tooling, and service logic
- **Zig** only for future native/operator tooling if it clearly improves UX or packaging
- **C** only for narrow low-level, FFI, or portability cases
- **Rust is removed from the tracked tree on `main`**

Historical Rust code lives in git history only. Do not reintroduce it as “reference material” in-tree.

### What is working today

- Go client in `client/` with:
  - rendezvous code generation/parsing
  - Noise `XXpsk0` handshake
  - encrypted transfer channel
  - file send/receive over relay
  - CLI with `send`, `receive`, `status`, and `components`
- Go relay in `services/relay/`
- Go NAT probing / hole-punching groundwork in `lib/punchthrough/`
- truthful scaffold for `services/bore-admin/`

### What is not done yet

- direct transport integrated into the client path
- resumable transfers
- directory transfer
- relay rate limiting / quotas / timeouts hardening
- health and metrics endpoints on the relay
- admin surface beyond scaffold
- broader security review and operational hardening

---

## Monorepo Layout

```text
bore/
├── client/                  # active Go client
├── services/
│   ├── relay/              # active Go relay service
│   └── bore-admin/         # truthful scaffold only
├── lib/
│   └── punchthrough/       # NAT probing + hole-punching primitives
├── docs/                   # design and threat-model docs
├── README.md               # public project status
├── BUILD.md                # this file
├── REWRITE_TRACKER.md      # rewrite handoff / resume state
├── ARCHITECTURE.md         # current architecture description
└── SECURITY.md             # current security posture
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
- Go test coverage for code / crypto / engine / transport / rendezvous
- relay-style rendezvous integration test

What is still missing:

- direct-first transport selection
- resume support
- directory transfer
- transfer history
- richer progress/reporting polish

### `services/relay/` — active Go relay

**Status:** functional room broker

What exists:

- room registry
- room state machine
- WebSocket sender/receiver flow
- bidirectional frame relay
- graceful shutdown handling
- tests for room and transport behavior

What is still missing:

- explicit rate limiting
- health endpoint
- metrics endpoint
- operator-facing resource controls / quotas
- deployment/service packaging artifacts

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
- broader real-world network validation

### `services/bore-admin/` — scaffold

**Status:** truthful placeholder only

What exists:

- Go module scaffolding
- placeholder CLI entry point
- explicit statement of what is not built yet

What is still missing:

- relay polling
- storage
- TUI / web UI
- alerting
- configuration system

---

## Build / Run / Verify

### Prerequisites

- Go `1.26.1`
- No top-level `go.work`; build and test per module

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

## Verification Performed In This Rust-Removal Pass

Cleanup date: 2026-03-23

Focused verification for this pass:

```bash
cd client && go test ./... && go build ./cmd/bore
cd services/relay && go test ./... && go build ./cmd/relay
cd lib/punchthrough && go test ./... && go build ./cmd/punchthrough
cd services/bore-admin && go build ./cmd/bore-admin
git ls-files | rg '\\.(rs)$|(^|/)Cargo\\.toml$|(^|/)Cargo\\.lock$|rust-toolchain'
```

Success criteria for this cleanup:

- active Go modules still build and test cleanly
- `services/bore-admin` still builds as an honest scaffold
- tracked Rust source / Cargo files are gone from `main`
- docs consistently describe Go / Zig / C only

---

## Working Rules

1. **No new Rust work in-tree.** If historical context matters, use git history.
2. **Keep the relay payload-blind.** If it can inspect file contents, the design regressed.
3. **Treat the rendezvous code as cryptographic input, not just a locator.**
4. **Do not overclaim direct mode.** The reliable verified path today is relay-based.
5. **Keep docs honest.** Aspirational language belongs in planned sections, not current-state summaries.
6. **Run the narrowest meaningful verification first.** Broaden only when the change surface demands it.
7. **If you change architecture or security claims, update `REWRITE_TRACKER.md`, `ARCHITECTURE.md`, and `SECURITY.md` in the same pass.**

---

## Immediate Next Moves

### Highest leverage path

1. integrate `lib/punchthrough/` into client transport selection
2. add resumable transfer state
3. add relay rate limiting + health/metrics
4. promote `bore-admin` from scaffold to useful operator surface
5. decide whether any future Zig layer is actually warranted by packaging or UX pain

### If the goal is cleanup instead of features

1. keep docs aligned with the Go implementation as features land
2. tighten relay operational controls and security posture
3. avoid leaving dead scaffolding or language migrations half-finished

---

## Risks And Open Questions

### Risk: punchthrough exists but is not integrated

Mitigation:
- avoid marketing bore as direct-first today
- describe direct transport as the next implementation target, not a current capability

### Risk: no resumable state yet

Mitigation:
- keep transfer claims modest
- add checkpoint / resume state before claiming interruption recovery

### Risk: relay hardening is incomplete

Mitigation:
- treat the current relay as functional, not production-hardened
- add rate limiting, quotas, health, and metrics before making stronger deployment claims

### Open question: where Zig actually belongs

The repo direction is Go / Zig / C only, but no Zig implementation should land just for symmetry. Use it only where it clearly improves:

- packaging
- native UX
- local operator tooling
- distribution simplicity

If the Go client keeps shipping cleanly and does not impose real pain, do not force a second frontend prematurely.

---

## Resume Checklist

If you are resuming this repo later, do this in order:

1. read `README.md`
2. read this file
3. read `REWRITE_TRACKER.md`
4. inspect `git status`
5. treat `client/` as the active client
6. pick one lane only:
   - direct transport integration
   - resume support
   - relay hardening
   - bore-admin implementation
7. run focused verification before and after the change
