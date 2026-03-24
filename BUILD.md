# BUILD.md

## Purpose

This is the execution manual for bore's active build lane.

Use it to answer four questions quickly:

1. what the repo actually builds now
2. what is verified and shippable today
3. what is still open, risky, or deliberately deferred
4. what the next highest-leverage move is

If this file and the code disagree, fix both in the same change.

---

## Mission

Build a privacy-first file transfer tool that makes encrypted delivery feel simple for end users while keeping the operational surface honest for relay operators.

The product line stays narrow on purpose:

- the sender and receiver should get one trustworthy path before the repo claims many
- relay transport is the shipped truth until direct transport is proven, measured, and worth promoting
- the browser surface exists to explain runtime state, not to become a separate control plane
- future durability work should make transfers more dependable without turning bore into a generic sync platform

---

## Current execution posture

bore is **active**, not archival.

Phase 0 shipped a real, usable relay-based path. That is progress, not finish line energy. The repo still has obvious product and systems work in front of it:

- direct transport is scaffolded but not yet real in the shipped client path
- transfers are still single-file and non-resumable
- relay operations are functional but not yet hardened
- the browser and operator surfaces are intentionally narrow and should stay honest while the runtime grows

Treat this document like a live program, not a retrospective. Do not let the existence of a working relay path make the repo read “done forever.”

### Recommended build order unless a bug/security issue interrupts

1. ~~make direct-path transport real and measurable~~ (Phase 1 done, pending real-world validation)
2. ~~make single-file transfer durable enough to resume cleanly~~ (Phase 2 done)
3. ~~harden relay operations and verification discipline~~ (Phase 3 done)
4. deepen operator tooling only where runtime reality justifies it

---

## Repo snapshot

bore currently ships a relay-based encrypted file transfer path plus a real in-repo browser surface built from one root Go module and one frontend workspace:

- `cmd/bore` plus `internal/client/` for the user-facing CLI, rendezvous flow, crypto, and transfer engine
- `cmd/relay` plus `internal/relay/` for the WebSocket relay, room registry, operator endpoints, and embedded web UI
- `cmd/punchthrough` plus `internal/punchthrough/` for NAT probing and hole-punching groundwork
- `cmd/bore-admin` for minimal operator CLI status polling
- `internal/relay/ratelimit/` for per-IP token bucket rate limiting
- `internal/relay/metrics/` for atomic operator-facing counters
- `web/` for the Bun + React + Vite + TypeScript browser surface

### What is working today

- relay-based encrypted file transfer
- rendezvous code generation and parsing
- Noise `XXpsk0` handshake
- encrypted transfer framing with SHA-256 verification
- resumable single-file transfers with on-disk checkpoint state
- per-IP rate limiting on relay `/ws` and `/signal` endpoints
- explicit HTTP server timeouts (read, write, idle, header)
- relay `/healthz`, `/status`, and `/metrics` endpoints
- relay-served browser surface at `/` and `/ops/relay`
- `bore-admin status` against the relay status endpoint
- standalone punchthrough probing primitives and CLI
- deployment packaging (Dockerfile, systemd service unit)

### What is still not done

- direct transport integrated end-to-end into the client path
- directory transfer
- authenticated or write-capable browser workflows
- broader operator tooling beyond status snapshots and metrics
- external security review

### Hard truth to preserve in all docs

The only verified transfer path today is **relay-based**. Direct transport exists as groundwork and integration scaffolding, not as current shipped runtime behavior.

---

## Data layer stance

Current implementation truth:

- there is no durable database in the shipped path today
- `internal/relay/room` keeps bounded room state in memory only
- `web/` reads live aggregate state from `/status`; it does not own writes or auth
- `bore-admin` fetches `/status` on demand and does not persist snapshots
- resumable transfer metadata and transfer history are future work

Doctrine for future work:

- if bore later needs local persistence, start with SQLite and a relational schema
- if the browser surface later earns authenticated write-heavy workflows, keep it on SQLite with handwritten SQL migrations and queries
- keep Go-side SQL explicit and boring before adding heavier tooling
- do not invent a document-store pivot for relay history, resume metadata, or operator state

---

## Monorepo layout

```text
bore/
├── cmd/
│   ├── bore/
│   ├── bore-admin/
│   ├── punchthrough/
│   └── relay/
├── deploy/
│   └── bore-relay.service
├── internal/
│   ├── client/
│   ├── punchthrough/
│   └── relay/
│       ├── metrics/
│       ├── ratelimit/
│       ├── room/
│       ├── transport/
│       └── webui/
├── web/
├── docs/
├── Dockerfile
├── README.md
├── BUILD.md
├── ARCHITECTURE.md
└── SECURITY.md
```

---

## Component snapshots

### `cmd/bore` + `internal/client/`

Status: working relay-based transfer path with direct-path integration and resume support

What exists:

- rendezvous code model and parsing
- Noise `XXpsk0` handshake
- secure channel framing over arbitrary `io.ReadWriter`
- transfer engine with header, ResumeOffer, chunk, and end framing
- SHA-256 integrity verification
- resumable single-file transfers with on-disk checkpoint state
- ResumeOffer protocol: receiver tells sender which chunk to start from
- deterministic transfer ID from (filename, size, SHA-256, chunk_size)
- restart-vs-resume rules: metadata match + correct partial file size → resume; otherwise restart from 0
- `ReceiveFile` for disk-based resume, `ReceiveToFile` in rendezvous for full flow
- relay transport plus selector wiring
- `SelectionResult` with `Method`, `FallbackReason`, and `DirectErr` for observable transport decisions
- `Candidate` and `CandidatePair` types for relay-coordinated peer address exchange
- relay-coordinated signaling (`/signal` endpoint) for candidate exchange between peers
- STUN/NAT discovery wired into the selector via `DiscoverCandidate`
- UDP reliability/framing layer (`ReliableConn`) with sequence numbers, ACK, retransmit, and FIN
- `DirectDialer` with hole-punch integration via `internal/punchthrough/punch`
- `Selector.EnableDirect` flag for full discovery → signaling → punch → fallback path
- `--direct` CLI flag on both `bore send` and `bore receive`
- expanded `FallbackReason` set: `STUNFailed`, `NATUnfavorable`, `PunchFailed`, `SignalingFailed`
- deterministic tests verifying selector fallback reasons through unit and integration paths
- relay signaling endpoint tests for candidate exchange, no-candidate, and cleanup
- reliable framing unit tests for encode/decode, flags, and edge cases
- Go test coverage for code, crypto, engine, transport, rendezvous, and relay signaling

What is still missing:

- end-to-end direct transfer verified across real NATs (current implementation is correct but needs real-world validation)
- directory transfer
- richer progress and transfer history handling

### `cmd/relay` + `internal/relay/`

Status: hardened room broker with rate limiting, metrics, and deployment packaging

What exists:

- room registry and state machine
- WebSocket sender and receiver flow
- bidirectional encrypted frame relay with byte/frame counting
- room TTL reaper with expiry callback
- per-IP rate limiting on `/ws` and `/signal` endpoints
- explicit HTTP server timeouts (read, write, idle, header)
- operational metrics endpoint at `/metrics`
- `/healthz` and `/status`
- embedded static web serving at `/` and `/ops/relay/`
- graceful shutdown handling
- Dockerfile and systemd service unit
- tests for room, transport, rate limiting, and metrics behavior

What is still missing:

- longer-term operator observation and alerting tooling
- authenticated operator endpoints

### `web/`

Status: active, intentionally thin, already useful

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

Status: integrated into client transport selector, standalone CLI still available

What exists:

- STUN probing
- NAT classification
- UDP hole-punching primitives
- CLI for probing
- client integration via `DiscoverCandidate` and `DirectDialer.dialWithPunch`
- relay-coordinated signaling for candidate exchange

What is still missing:

- broader real-world network validation across diverse NAT types
- evidence that direct mode succeeds often enough to change product claims

### `cmd/bore-admin`

Status: active, minimal, intentionally not a control plane

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

## Build / run / verify

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

### Rate limiting and metrics

```bash
go test ./internal/relay/ratelimit/... ./internal/relay/metrics/...
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
- operational metrics: `http://127.0.0.1:8080/metrics`
- health check: `http://127.0.0.1:8080/healthz`

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

## Milestone map

These are the real milestones still in front of the repo.

### M1 — direct path is real, not just scaffolded

Success means the client can attempt a direct path, explain when it fails, and fall back to relay without hand-waving.

### M2 — transfer durability is real

Success means a single interrupted transfer can resume or cleanly restart with explicit rules and tests.

### M3 — relay ops are credibly hardened ✓

Success means the relay has rate limits, timeouts, metrics, and a clearer production posture. Done: per-IP rate limiting, HTTP timeouts, `/metrics` endpoint, deployment packaging.

### M4 — operator surfaces stay honest while gaining depth

Success means `bore-admin` and the browser surface grow only where they solve real relay/operator problems.

---

## Phase dashboard

### Phase 0 — relay-based encrypted transfer path

Status: done / checked

Checklist:

- [x] client rendezvous code generation and parsing exist
- [x] Noise `XXpsk0` handshake exists
- [x] encrypted relay-based file transfer works
- [x] relay room brokering and `/healthz` + `/status` exist
- [x] `bore-admin status` exists
- [x] relay-served browser surface exists

Reality check:

- do not treat Phase 0 completion as repo completion

### Phase 1 — direct-path integration

Status: implementation complete, pending real-world NAT validation

Checklist:

- [x] transport abstraction layer with `Conn` and `Dialer`
- [x] relay transport implementing `Dialer`
- [x] direct transport stub implementing `Dialer`
- [x] selector with direct-first and relay-fallback logic
- [x] rendezvous flow wired to `Dialer`
- [x] define the relay-coordinated peer-candidate exchange needed for direct attempts
- [x] publish and consume direct-path candidate data during rendezvous
- [x] wire `internal/punchthrough/` STUN and NAT discovery into direct dial attempts
- [x] add UDP reliability/framing semantics suitable for encrypted file transfer
- [x] record why direct mode fell back so tests and operators can explain the downgrade
- [x] add deterministic verification for direct-path success and relay fallback behavior
- [x] prove the selector still lands on the existing relay path cleanly when direct mode is impossible

Exit criteria:

- direct mode implementation is complete with STUN discovery, signaling, hole-punching, and reliability framing
- relay remains the default and only production-verified path until real-world NAT testing validates direct mode
- `--direct` flag exists for opt-in experimentation

### Phase 2 — transfer durability

Status: done / checked

Checklist:

- [x] choose and document the resume-state shape before writing code blindly
- [x] persist enough sender/receiver state to resume a single-file transfer safely
- [x] define restart vs resume rules when metadata or digests do not match
- [x] add interruption-recovery tests for relay-based transfers first
- [ ] add directory transfer only after single-file resume semantics are solid and explicit

Exit criteria:

- a stopped single-file transfer can resume or restart with deterministic behavior and tests

Implementation notes:

- wire protocol extended with `ResumeOffer (0x04)` message: receiver → sender after header
- resume state persisted as `<outputDir>/.bore/resume-<transferID>.json` + partial data file
- transfer ID is a deterministic hash of (filename, size, SHA-256, chunk_size) — same file always resumes regardless of relay room or rendezvous code
- restart-vs-resume rules: metadata must match AND partial file byte count must be correct; any mismatch triggers full restart from chunk 0
- final SHA-256 covers entire reassembled file; if resumed data was corrupted, hash check fails and state is cleaned up
- `ReceiveFile` in engine handles disk-based resume; `ReceiveToFile` in rendezvous wires it into the full flow
- all existing tests updated for the new protocol; 14 new tests added for resume state persistence, protocol encoding, disk-based receive, and resume behavior

### Phase 3 — relay hardening

Status: done / checked

Checklist:

- [x] add explicit rate limiting around room creation, room joins, and connection churn
- [x] add quotas or stronger resource controls for room occupancy and message pressure
- [x] add explicit HTTP server timeouts and tighten transport guardrails
- [x] add metrics endpoint and operator-facing counters
- [x] tighten deployment and service packaging rails
- [ ] add admin-only profiling hooks only if they earn their keep operationally

Exit criteria:

- the relay reads as deliberately hardened, not merely functional

Implementation notes:

- `internal/relay/ratelimit/` provides per-IP token bucket rate limiting with automatic stale entry cleanup
- `/ws` and `/signal` endpoints enforce configurable per-IP rate limits (default: 30 req/min)
- `internal/relay/metrics/` provides atomic operator-facing counters: active/total WS connections, rooms created/joined/expired/relayed, bytes/frames relayed, rate limit hits, WS errors, signal exchanges
- `/metrics` endpoint exposes a JSON snapshot of all counters
- HTTP server now has explicit timeouts: read (30s), write (30s), idle (120s), read header (10s), max header (1 MB)
- `DefaultServerConfig()` provides production-ready defaults for all timeouts and rate limits
- room registry supports `OnExpire` callback for metrics integration
- `Dockerfile` for multi-stage relay build with health check, non-root user, and minimal image
- `deploy/bore-relay.service` systemd unit with security hardening directives
- 8 new rate limit tests, 3 new metrics tests, 2 new room callback tests added

### Phase 4 — browser and operator surface

Status: active / initial implementation landed

Checklist:

- [x] relay-served browser surface under `web/`
- [x] same-origin read-only status page
- [x] product story aligned with the actual relay-based runtime
- [ ] decide whether the browser surface should stay static and read-only until an auth story exists
- [ ] add browser-level smoke coverage for `/` and `/ops/relay` if those pages become operationally critical
- [ ] surface direct/fallback runtime state in the UI only after the transport truth exists underneath it

Exit criteria:

- the browser surface remains truthful while still feeling intentional and useful

### Phase 5 — operator tooling depth

Status: planned

Checklist:

- [ ] decide whether relay history belongs in `bore-admin`, the browser surface, or neither
- [ ] add useful historical views only if they solve a real relay problem
- [ ] add alerting and configuration basics without turning bore into a generic control plane
- [ ] keep any persisted operator history small and relational if it is added later

Exit criteria:

- operator tooling solves real relay/operator pain instead of inventing dashboard theater

### Phase 6 — verification and release discipline

Status: active foundation, unfinished standards

Checklist:

- [x] root `.github/workflows/ci.yml` runs component verification from the consolidated module layout
- [x] add `golangci-lint run` to CI
- [x] add `govulncheck ./...` to CI
- [ ] cache Bun dependencies for the `web/` job
- [ ] add fuzz targets for rendezvous code and transfer frame parsing
- [ ] add a root task runner only if the command surface grows large enough to justify it
- [ ] keep `README.md`, `BUILD.md`, `ARCHITECTURE.md`, and `SECURITY.md` aligned whenever runtime claims change

Exit criteria:

- the repo proves its claims with repeatable checks and fewer hand-maintained assumptions

---

## Focused verification checklist

Use the narrowest verification that proves the current claim.

### Docs-only changes

- re-read the touched docs for consistency
- confirm current-state sections only describe implemented behavior
- confirm planned sections are explicitly labeled as planned or active work
- confirm the doc still reads like an active program rather than a frozen status note

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

### Rate limit or metrics changes

```bash
go test ./internal/relay/ratelimit/... ./internal/relay/metrics/... ./internal/relay/transport/...
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

## Working rules

1. Keep the relay payload-blind. If it can inspect file contents, the design regressed.
2. Treat the rendezvous code as cryptographic input, not just a locator.
3. Do not overclaim direct mode. The reliable verified path today is relay-based.
4. Keep docs honest. Aspirational language belongs in planned sections, not current-state summaries.
5. Keep the browser surface honest and narrow. New web/UI work should support the real product or operator story, not invent a control plane the runtime does not have.
6. Run the narrowest meaningful verification first. Broaden only when the change surface demands it.
7. If you change architecture or security claims, update `BUILD.md`, `ARCHITECTURE.md`, and `SECURITY.md` in the same pass.
8. If Phase 0 is the only thing that looks finished in the doc, the doc is doing its job; the repo still is not finished.

---

## Risks and open questions

### Risk: direct-path groundwork exists, but the runtime proof does not

Mitigation:

- avoid marketing bore as direct-first today
- describe direct transport as the next implementation target, not a current capability
- make fallback reasons observable before changing any README/product copy

### Risk: resume state is per-receiver only

Mitigation:

- resume state lives on the receiver's filesystem only
- the sender re-sends from the requested chunk on each connection — it does not persist progress
- directory transfer resume requires additional design work beyond single-file resume

### Risk: relay hardening is baseline, not audited

Mitigation:

- the relay now enforces per-IP rate limits, HTTP timeouts, and tracks operational metrics
- deployment packaging (Dockerfile, systemd) is available
- treat this as deliberately hardened for small-scale use, not as externally audited
- an external security review is still needed before making stronger claims

### Open question: how much operator surface bore actually wants

Useful next steps are clear:

- decide whether relay observations need persistence at all before adding a store
- add simple operator views beyond the single status summary only if they solve a real relay problem
- add alerting and configuration basics without turning bore into a control plane
- if metrics or history need local durability later, start with a small relational SQLite store

### Open question: when directory transfer becomes worth it

Current answer:

- not before single-file resume and restart semantics are trustworthy

---

## Immediate next moves

### Default next lane

If you are choosing the next substantive feature lane, pick **Phase 4 — browser and operator surface** or **Phase 6 — verification and release discipline** depending on whether the priority is user-facing polish or CI hardening.

### If the goal is cleanup instead of features

1. tighten docs around the relay-based path and current limits
2. remove claims that imply direct transport is already present
3. keep the browser and operator surface clearly scoped to what it actually does today
4. trim stale commentary that does not help a future maintainer ship the next step

---

## Resume checklist

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
8. before calling a lane done, make sure the docs still read like an active program rather than a frozen snapshot
