# BUILD.md

## Purpose

This is the execution manual for bore's current lane.

Use it to answer four questions quickly:

1. what the repo actually builds now
2. what is shippable today
3. what is still limited or TODO
4. what the next correct move is

If this file and the code disagree, fix both in the same change.

---

## Current Truth

bore currently ships a relay-based encrypted file transfer path plus a real in-repo browser surface built from one root Go module and one frontend workspace:

- `cmd/bore` plus `internal/client/` for the user-facing CLI, rendezvous flow, crypto, and transfer engine
- `cmd/relay` plus `internal/relay/` for the WebSocket relay, room registry, operator endpoints, and embedded web UI
- `cmd/punchthrough` plus `internal/punchthrough/` for NAT probing and hole-punching groundwork
- `cmd/bore-admin` for minimal operator CLI status polling
- `web/` for the Bun + React + Vite + TypeScript browser surface

### What is working today

- relay-based encrypted file transfer
- rendezvous code generation and parsing
- Noise `XXpsk0` handshake
- encrypted transfer framing with SHA-256 verification
- relay `/healthz` and `/status` endpoints
- relay-served browser surface at `/` and `/ops/relay`
- `bore-admin status` against the relay status endpoint
- standalone punchthrough probing primitives and CLI

### What is not done yet

- direct transport integrated into the client path
- resumable transfers
- directory transfer
- relay rate limiting, quotas, and metrics
- authenticated or write-capable browser workflows
- broader operator tooling beyond status snapshots
- external security review and production hardening

---

## Data Layer Stance

Current implementation truth:

- there is no durable database in the shipped path today
- `internal/relay/room` keeps bounded room state in memory only
- `web/` reads live aggregate state from `/status`; it does not own writes or auth
- `bore-admin` fetches `/status` on demand and does not persist snapshots
- resumable transfer metadata and transfer history are future work

Doctrine for future work:

- if Bore later needs local persistence, start with SQLite and a relational schema
- if the browser surface later earns authenticated write-heavy workflows, keep it on SQLite with handwritten SQL migrations and queries
- keep Go-side SQL explicit and boring before adding heavier tooling
- do not invent a document-store pivot for relay history, resume metadata, or operator state

---

## Monorepo Layout

```text
bore/
├── cmd/
│   ├── bore/
│   ├── bore-admin/
│   ├── punchthrough/
│   └── relay/
├── internal/
│   ├── client/
│   ├── punchthrough/
│   └── relay/
├── web/
├── docs/
├── README.md
├── BUILD.md
├── ARCHITECTURE.md
└── SECURITY.md
```

---

## Component Snapshots

### `cmd/bore` + `internal/client/`

Status: working relay-based transfer path

What exists:

- rendezvous code model and parsing
- Noise `XXpsk0` handshake
- secure channel framing over arbitrary `io.ReadWriter`
- transfer engine with header, chunk, and end framing
- SHA-256 integrity verification
- relay transport plus selector wiring
- Go test coverage for code, crypto, engine, transport, and rendezvous

What is still missing:

- direct transport that works end-to-end
- resume support
- directory transfer
- richer progress and history handling

### `cmd/relay` + `internal/relay/`

Status: functional room broker

What exists:

- room registry and state machine
- WebSocket sender and receiver flow
- bidirectional encrypted frame relay
- room TTL reaper
- `/healthz` and `/status`
- embedded static web serving at `/` and `/ops/relay/`
- graceful shutdown handling
- tests for room and transport behavior

What is still missing:

- explicit rate limiting
- metrics endpoint
- stronger operator-facing resource controls
- deployment and service packaging artifacts

### `web/`

Status: active, intentionally thin

What exists:

- Bun-managed frontend workspace
- React + Vite + TypeScript SPA with product homepage and relay ops page
- TanStack Query polling of `/status`
- TanStack Router for client-side routing
- shadcn/ui + Tailwind-based component system
- production build output embedded under `internal/relay/webui/dist/`

What is still missing:

- authenticated operator workflows
- historical or persisted relay state
- control-plane mutations
- broader browser coverage beyond focused unit checks

### `cmd/punchthrough` + `internal/punchthrough/`

Status: partial, not integrated into the client flow

What exists:

- STUN probing
- NAT classification
- UDP hole-punching primitives
- CLI for probing

What is still missing:

- client integration
- coordination and signaling
- broader real-world network validation

### `cmd/bore-admin`

Status: active, minimal

What exists:

- usable `status` command
- HTTP polling of the relay `/status` endpoint
- human-readable output for uptime, room counts, and relay limits

What is still missing:

- persistent storage or local history
- alerting
- configuration profiles
- deeper coordination with the browser operator surface

---

## Build / Run / Verify

### Prerequisites

- Go `1.26.1`
- Bun `1.3.x`
- build and test from the repo root for Go, and from `web/` for frontend tasks

### Web

```bash
cd web
bun install
bun run check
bun run test
bun run build
```

Notes:

- `bun run build` writes the SPA output into `internal/relay/webui/dist/`
- rebuild the web surface before shipping relay changes that depend on updated embedded assets
- `bun run dev` proxies `/status` to `http://127.0.0.1:8080` for local development against a running relay

### Client

```bash
go test ./internal/client/... ./cmd/bore
go build ./cmd/bore
```

### Relay

```bash
go test ./internal/relay/... ./cmd/relay
go build ./cmd/relay
```

### Punchthrough

```bash
go test ./internal/punchthrough/... ./cmd/punchthrough
go build ./cmd/punchthrough
```

### bore-admin

```bash
go test ./cmd/bore-admin
go build ./cmd/bore-admin
```

### bore-admin status check

```bash
go run ./cmd/bore-admin status --relay http://127.0.0.1:8080
```

### Local relay-based smoke flow

Terminal 1:

```bash
RELAY_ADDR=127.0.0.1:8080 go run ./cmd/relay
```

Browser check while Terminal 1 is running:

- product page: `http://127.0.0.1:8080/`
- relay ops page: `http://127.0.0.1:8080/ops/relay`
- raw status JSON: `http://127.0.0.1:8080/status`

Terminal 2:

```bash
./bore send ./payload.txt --relay http://127.0.0.1:8080
```

Terminal 3:

```bash
./bore receive <code> --relay http://127.0.0.1:8080
```

Expected result:

- sender prints a full rendezvous code
- receiver completes successfully
- sender and receiver SHA-256 values match
- output file matches input bytes
- `/` and `/ops/relay` render from the relay with no broken static assets
- `/ops/relay` successfully reads aggregate data from `/status`

---

## Phase Dashboard

### Phase 0 — relay-based encrypted transfer path

Status: done / checked

Checklist:

- [x] client rendezvous code generation and parsing exist
- [x] Noise `XXpsk0` handshake exists
- [x] encrypted relay-based file transfer works
- [x] relay room brokering and `/healthz` + `/status` exist
- [x] `bore-admin status` exists
- [x] relay-served browser surface exists

### Phase 1 — direct-path integration

Status: in progress

Checklist:

- [x] transport abstraction layer with `Conn` and `Dialer`
- [x] relay transport implementing `Dialer`
- [x] direct transport stub implementing `Dialer`
- [x] selector with direct-first and relay-fallback logic
- [x] rendezvous flow wired to `Dialer`
- [ ] integrate `internal/punchthrough/` into direct transport for NAT hole-punching
- [ ] add relay-coordinated signaling to exchange peer addresses
- [ ] add reliability and framing over UDP for direct transport
- [ ] add deterministic verification for direct-path success and relay fallback

Exit criteria:

- direct mode is real and verified, or the docs still call relay the only shipped path

### Phase 2 — transfer durability

Status: planned

Checklist:

- [ ] add resumable transfer state
- [ ] add interruption-recovery tests
- [ ] add directory transfer only if it stays explicit and composable

### Phase 3 — relay hardening

Status: planned

Checklist:

- [ ] add relay rate limiting
- [ ] add quotas or resource controls
- [ ] add metrics endpoint and operator-facing counters
- [ ] tighten deployment and service packaging rails

### Phase 4 — browser and operator surface

Status: active / initial implementation landed

Checklist:

- [x] relay-served browser surface under `web/`
- [x] same-origin read-only status page
- [x] product story aligned with the actual relay-based runtime
- [ ] decide whether the browser surface should stay static and read-only or grow authenticated workflows later
- [ ] add browser-level smoke coverage only if the page surface becomes operationally critical

### Phase 5 — operator tooling depth

Status: planned

Checklist:

- [ ] expand `bore-admin` only when the operator story truly needs it
- [ ] add useful historical views only if they solve a real relay problem
- [ ] add alerting and configuration basics without turning bore into a control plane

### Phase 6 — tech stack alignment

Status: planned

Checklist:

- [x] root `.github/workflows/ci.yml` runs Go verification from the consolidated module layout
- [ ] add `golangci-lint run` to CI
- [ ] add `govulncheck ./...` to CI
- [ ] cache Bun dependencies for the `web/` job
- [ ] add relay metrics and admin-only profiling endpoints
- [ ] add explicit HTTP server timeouts to the relay
- [ ] add fuzz targets for rendezvous code and transfer frame parsing
- [ ] add a root task runner if the command surface grows large enough

---

## Focused Verification Checklist

Use the narrowest verification that proves the current claim.

### Docs-only changes

- re-read the touched docs for consistency
- confirm current-state sections only describe implemented behavior
- confirm planned sections are explicitly labeled as planned work

### Web changes

```bash
cd web && bun run check && bun run test && bun run build
```

### Client changes

```bash
go test ./internal/client/... ./cmd/bore
go build ./cmd/bore
```

### Relay changes

```bash
go test ./internal/relay/... ./cmd/relay
go build ./cmd/relay
```

### Punchthrough changes

```bash
go test ./internal/punchthrough/... ./cmd/punchthrough
go build ./cmd/punchthrough
```

### Admin CLI changes

```bash
go test ./cmd/bore-admin
go build ./cmd/bore-admin
```

### Cross-cutting changes

Run every affected command above, then verify the docs still match the code path that actually ships.

---

## Working Rules

1. Keep the relay payload-blind. If it can inspect file contents, the design regressed.
2. Treat the rendezvous code as cryptographic input, not just a locator.
3. Do not overclaim direct mode. The reliable verified path today is relay-based.
4. Keep docs honest. Aspirational language belongs in planned sections, not current-state summaries.
5. Keep the browser surface honest and narrow. New web/UI work should support the real product or operator story, not invent a control plane the runtime does not have.
6. Run the narrowest meaningful verification first. Broaden only when the change surface demands it.
7. If you change architecture or security claims, update `BUILD.md`, `ARCHITECTURE.md`, and `SECURITY.md` in the same pass.

---

## Immediate Next Moves

### Highest-leverage path

1. integrate `internal/punchthrough/` into client transport selection
2. add resumable transfer state
3. add relay rate limiting and metrics
4. decide how much operator depth belongs in `bore-admin` versus the browser surface
5. keep documentation and embedded web assets aligned as those features land

### If the goal is cleanup instead of features

1. tighten docs around the relay-based path and current limits
2. remove claims that imply direct transport is already present
3. keep the browser and operator surface clearly scoped to what it actually does today
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
- add checkpoint and resume state before claiming interruption recovery

### Risk: relay hardening is incomplete

Mitigation:

- treat the current relay as functional, not production-hardened
- keep using the existing health and status endpoints for visibility, then add rate limiting, quotas, and metrics before making stronger deployment claims

### Open question: how broad the operator surface should become

Useful next steps are clear:

- decide whether relay observations need persistence at all before adding a store
- add simple operator views beyond the single status summary only if they solve a real relay problem
- add alerting and configuration basics without turning Bore into a control plane
- if metrics or history need local durability later, start with a small relational SQLite store

Avoid overbuilding beyond what the relay actually needs.

---

## Resume Checklist

If you are resuming this repo later, do this in order:

1. read `README.md`
2. read this file
3. read `ARCHITECTURE.md` and `SECURITY.md` if the task touches behavior or claims
4. inspect `git status`
5. treat the repo root as the source of truth for Go builds
6. pick one lane only:
   - direct transport integration
   - resume support
   - relay hardening
   - browser and operator surface work
   - bore-admin implementation
7. run focused verification before and after the change
