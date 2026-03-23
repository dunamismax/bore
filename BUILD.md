# BUILD.md

## Purpose

This is the execution manual for bore's current lane.

Use it to answer four questions quickly:

1. what the repo is actually building now
2. what is shippable today
3. what is still limited or TODO
4. what the next correct move is

If this file and the code disagree, fix both in the same change.

---

## Current Truth

bore currently ships a **relay-based encrypted file transfer path** built from four tracked components:

- **`client/`** — user-facing CLI for rendezvous, handshake, and encrypted file transfer
- **`services/relay/`** — WebSocket relay that pairs peers, forwards encrypted frames, and exposes operator health/status endpoints
- **`lib/punchthrough/`** — NAT probing and hole-punching groundwork for a future direct path
- **`services/bore-admin/`** — minimal operator CLI for relay status polling

### What is working today

- client CLI in `client/` with:
  - rendezvous code generation/parsing
  - Noise `XXpsk0` handshake
  - encrypted transfer channel
  - file send/receive over relay
  - CLI with `send`, `receive`, `status`, and `components`
- relay in `services/relay/` with:
  - WebSocket room brokering
  - `/healthz` and `/status` operator endpoints
- NAT probing / hole-punching groundwork in `lib/punchthrough/`
- `bore-admin` CLI in `services/bore-admin/` for relay status polling

### What is not done yet

- direct transport integrated into the client path
- resumable transfers
- directory transfer
- relay rate limiting / quotas / operational controls
- metrics endpoint on the relay
- admin surface beyond status polling
- broader security review and operational hardening

---

## Monorepo Layout

```text
bore/
├── client/                  # active Go client
├── services/
│   ├── relay/               # active Go relay service
│   └── bore-admin/          # minimal operator CLI
├── lib/
│   └── punchthrough/        # NAT probing + hole-punching primitives
├── docs/                    # design and threat-model docs
├── README.md                # public project status
├── BUILD.md                 # execution manual and TODO ledger
├── ARCHITECTURE.md          # current architecture description
└── SECURITY.md              # current security posture
```

---

## Component Snapshots

### `client/` — active client

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

### `services/relay/` — active relay

**Status:** functional room broker

What exists:

- room registry
- room state machine
- WebSocket sender/receiver flow
- bidirectional frame relay
- room TTL reaper
- `/healthz` and `/status` endpoints for operator visibility
- graceful shutdown handling
- tests for room and transport behavior

What is still missing:

- explicit rate limiting
- metrics endpoint
- stronger operator-facing resource controls
- deployment/service packaging artifacts

### `lib/punchthrough/` — NAT tooling groundwork

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

### `services/bore-admin/` — operator CLI

**Status:** active, minimal

What exists:

- Go module with a usable `status` command
- HTTP polling of the relay `/status` endpoint
- human-readable output for uptime, room counts, and relay limits

What is still missing:

- persistent storage
- TUI / web UI
- alerting
- configuration profiles
- metrics/history views

---

## Build / Run / Verify

### Prerequisites

- Go `1.26.1`
- no top-level `go.work`; build and test per module

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

### bore-admin status check

```bash
cd services/bore-admin
go run ./cmd/bore-admin status --relay http://127.0.0.1:8080
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

## Phase Dashboard

### Phase 0 — relay-based encrypted transfer path

**Status:** done / checked

Checklist:

- [x] client rendezvous code generation and parsing exist
- [x] Noise `XXpsk0` handshake exists
- [x] encrypted relay-based file transfer works
- [x] relay room brokering and `/healthz` + `/status` endpoints exist
- [x] `bore-admin status` exists
- [x] smoke flow proves sender, receiver, and SHA-256 match

### Phase 1 — direct-path integration

**Status:** planned

Checklist:

- [ ] integrate `lib/punchthrough/` into client transport selection
- [ ] add coordination/signaling needed to attempt direct paths safely
- [ ] keep relay fallback as the reliable default path
- [ ] add deterministic verification for direct-path success and relay fallback

Exit criteria:

- direct mode is real and verified, or the docs still call relay the only shipped path

### Phase 2 — transfer durability

**Status:** planned

Checklist:

- [ ] add resumable transfer state
- [ ] add interruption-recovery tests
- [ ] add directory transfer only if it stays explicit and composable
- [ ] keep file integrity guarantees obvious in operator output

### Phase 3 — relay hardening

**Status:** planned

Checklist:

- [ ] add relay rate limiting
- [ ] add quotas or resource controls
- [ ] add metrics endpoint and operator-facing counters
- [ ] tighten deployment/service packaging rails

### Phase 4 — operator surface

**Status:** planned

Checklist:

- [ ] expand `bore-admin` beyond simple status polling
- [ ] add useful historical/operator views only if they solve a real relay problem
- [ ] add alerting/config basics without turning bore into a control-plane platform

### Phase 5 — tech stack alignment

**Status:** planned

This phase closes the gap between the current codebase and the standard Go service baseline. Each item is directly motivated by a concrete gap in bore's tooling, CI, or relay operation -- not by general software idealism.

**CI pipeline:**

- [ ] add `.github/workflows/ci.yml` that runs `go test ./...` for all four modules (`client/`, `services/relay/`, `services/bore-admin/`, `lib/punchthrough/`) on push and PR
- [ ] add `golangci-lint run` to the CI workflow for each module
- [ ] add `govulncheck ./...` to the CI workflow for each module

**Linting config:**

- [ ] add a root `.golangci.yml` with `govet`, `staticcheck`, `errcheck`, and `gosec` enabled; tune `gosec` to not fire on intentional crypto use

**Relay observability:**

- [ ] add Prometheus `/metrics` endpoint to `services/relay/` using `github.com/prometheus/client_golang`
- [ ] expose relay-specific counters: active rooms, total transfers started, total bytes forwarded, and rooms reaped by TTL
- [ ] add `net/http/pprof` handler on a separate admin listener in `services/relay/` (distinct from the public WebSocket port so profiling is never reachable from the public surface)

**HTTP server hardening:**

- [ ] set explicit `ReadTimeout`, `WriteTimeout`, `IdleTimeout`, and `ReadHeaderTimeout` on the relay's `http.Server` struct; log the configured values at startup

**Security fuzz tests:**

- [ ] add a fuzz test for rendezvous code parsing in `client/internal/code/` covering malformed lengths, wrong word counts, invalid characters, and truncated inputs
- [ ] add a fuzz test for transfer frame parsing in `client/internal/` covering truncated frames, oversized length fields, and corrupted type bytes

**Task runner:**

- [ ] add a root `magefile.go` using `github.com/magefile/mage` with targets: `Test` (all modules), `Build` (all modules), `Lint` (golangci-lint on all modules), `Vuln` (govulncheck on all modules), and `Check` (all of the above in order)
- [ ] update the "Build / Run / Verify" section of this file to prefer `mage` targets over raw shell commands

Exit criteria:

- every module passes `go test ./...`, `golangci-lint run`, and `govulncheck ./...` cleanly in CI
- the relay emits Prometheus metrics at `/metrics` and `pprof` is accessible on a separate admin listener
- the relay's `http.Server` has explicit timeouts visible in the source
- fuzz targets exist for code parsing and frame parsing and run for at least one minute without a crash
- `mage check` runs the full quality bar from a single command at the repo root

---

## Focused Verification Checklist

Use the narrowest verification that proves the current claim:

### Docs-only changes

- re-read the touched docs for consistency
- confirm current-state sections only describe implemented behavior
- confirm planned sections are explicitly labeled as planned work

### Client changes

```bash
cd client && go test ./... && go build ./cmd/bore
```

### Relay changes

```bash
cd services/relay && go test ./... && go build ./cmd/relay
```

### Punchthrough changes

```bash
cd lib/punchthrough && go test ./... && go build ./cmd/punchthrough
```

### Admin CLI changes

```bash
cd services/bore-admin && go build ./cmd/bore-admin
```

### Cross-cutting changes

Run every affected module command above, then verify the docs still match the code path that actually ships.

---

## Working Rules

1. **Keep the relay payload-blind.** If it can inspect file contents, the design regressed.
2. **Treat the rendezvous code as cryptographic input, not just a locator.**
3. **Do not overclaim direct mode.** The reliable verified path today is relay-based.
4. **Keep docs honest.** Aspirational language belongs in planned sections, not current-state summaries.
5. **Avoid speculative new surfaces.** Add new tooling only when it solves a real operator or transport problem.
6. **Run the narrowest meaningful verification first.** Broaden only when the change surface demands it.
7. **If you change architecture or security claims, update `BUILD.md`, `ARCHITECTURE.md`, and `SECURITY.md` in the same pass.**

---

## Immediate Next Moves

### Highest-leverage path

1. integrate `lib/punchthrough/` into client transport selection
2. add resumable transfer state
3. add relay rate limiting + metrics
4. deepen `bore-admin` beyond status polling into a broader operator surface
5. keep documentation aligned as those features land

### If the goal is cleanup instead of features

1. tighten docs around the relay-based path and current limits
2. remove claims that imply direct transport is already present
3. keep minimal operator tooling clearly scoped to what it actually does
4. trim stale commentary that does not help a future maintainer ship the next step

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
- keep using the existing health/status endpoints for visibility, then add rate limiting, quotas, and metrics before making stronger deployment claims

### Open question: how broad the operator surface should become

Useful next steps are clear:

- persist relay observations over time
- add simple operator views beyond the single status summary
- add alerting/configuration basics
- decide whether metrics should live in the relay, the admin tool, or both

Avoid overbuilding beyond what the relay actually needs.

---

## Resume Checklist

If you are resuming this repo later, do this in order:

1. read `README.md`
2. read this file
3. read `ARCHITECTURE.md` and `SECURITY.md` if the task touches behavior or claims
4. inspect `git status`
5. treat `client/` as the active client
6. pick one lane only:
   - direct transport integration
   - resume support
   - relay hardening
   - bore-admin implementation
7. run focused verification before and after the change
