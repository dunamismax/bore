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

Build a privacy-first peer-to-peer file transfer tool where files move directly between peers with real end-to-end encryption. No accounts, no cloud, no trust required.

The architecture is **P2P-first, relay-fallback**:

- the default transfer path is a direct connection between sender and receiver
- the relay exists as a signaling server and fallback transport when direct connections fail
- end-to-end encryption (Noise XXpsk0) protects all file data regardless of transport path
- the relay never sees plaintext — it is payload-blind whether used for signaling or fallback data relay

---

## Current execution posture

bore is **active**, post-rearchitecture.

The repo shipped a working relay-based path in v0.1.0 (Phase 0). Direct transport was promoted to the default path in Phase 5. Phase 6 upgraded the direct transport to QUIC with production-quality congestion control, multi-candidate gathering, and connection quality metrics. The current work is **deepening operator surfaces and verification**.

### Architecture evolution

```
v0.1.0:  relay-only (default) → direct (opt-in, --direct flag)
v0.2.0:  direct (default)     → relay (fallback, --relay-only flag)
```

### Recommended build order unless a bug/security issue interrupts

1. ~~make relay-based transfer work~~ (Phase 0 done)
2. ~~build direct transport infrastructure~~ (Phase 1 legacy done)
3. ~~make single-file transfer resumable~~ (Phase 2 done)
4. ~~harden relay operations~~ (Phase 3 done)
5. ~~make direct transport the default path~~ (Phase 5 done)
6. ~~improve direct transport quality~~ (Phase 6 done)
7. deepen operator and browser surfaces (Phase 7 — active)

---

## Repo snapshot

bore ships a P2P-first encrypted file transfer path with QUIC direct transport and relay fallback, plus an operator dashboard, built from one root Go module and one Python frontend:

- `cmd/bore` plus `internal/client/` for the user-facing CLI, rendezvous flow, crypto, and transfer engine
- `cmd/relay` plus `internal/relay/` for the WebSocket relay, signaling server, room registry, operator endpoints, and minimal web handler
- `cmd/punchthrough` plus `internal/punchthrough/` for NAT probing and hole-punching
- `cmd/bore-admin` for minimal operator CLI status polling
- `internal/relay/ratelimit/` for per-IP token bucket rate limiting
- `internal/relay/metrics/` for atomic operator-facing counters
- `frontend/` for the FastAPI + Jinja2 + htmx operator dashboard (Python, no JS build step)

### What is working today

- **P2P direct transfer** as the default path (STUN discovery → signaling → hole-punch → transfer)
- relay-based transfer as automatic fallback when direct fails
- `--relay-only` flag to force relay path
- rendezvous code generation and parsing
- Noise `XXpsk0` handshake (works identically over direct or relay transport)
- encrypted transfer framing with SHA-256 verification
- resumable single-file transfers with on-disk checkpoint state
- relay-coordinated signaling (`/signal` endpoint) for candidate exchange
- STUN/NAT discovery and UDP hole-punching
- QUIC-based direct transport (default) with ReliableConn as fallback
- ICE-like multi-candidate gathering (host, server-reflexive)
- connection quality metrics tracking (throughput, byte counters)
- UDP reliable framing layer (`ReliableConn`) retained as legacy fallback
- per-IP rate limiting on relay `/ws` and `/signal` endpoints
- explicit HTTP server timeouts (read, write, idle, header)
- relay `/healthz`, `/status`, and `/metrics` endpoints
- relay serves minimal placeholder at `/` (operator dashboard is the Python frontend)
- `bore-admin status` against the relay status endpoint
- standalone punchthrough probing CLI
- deployment packaging (Dockerfile, systemd service unit)

### What is still not done

- TURN-style relay data channel (relay as encrypted tunnel, not just signaling)
- connection migration support for mobile/roaming scenarios
- directory transfer
- authenticated or write-capable browser workflows
- broader operator tooling beyond status snapshots and metrics
- external security review

### Architecture truth

The default transfer path is **direct P2P**. The client attempts STUN discovery, exchanges candidates through the relay's signaling channel, evaluates NAT feasibility, and attempts hole-punching. If direct fails at any step, the client falls back to the relay transport automatically. End-to-end encryption (Noise XXpsk0) protects all file data regardless of which transport path is used — the relay is always payload-blind.

---

## Data layer stance

Current implementation truth:

- there is no durable database in the shipped path today
- `internal/relay/room` keeps bounded room state in memory only
- `frontend/` reads live aggregate state from `/status` via the Go relay's API; it does not own writes or auth
- `bore-admin` fetches `/status` on demand and does not persist snapshots
- resumable transfer metadata is receiver-side filesystem state

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
├── frontend/
│   ├── src/
│   │   └── app/
│   │       ├── routes/
│   │       ├── templates/
│   │       └── static/
│   ├── tests/
│   └── pyproject.toml
├── internal/
│   ├── client/
│   │   ├── code/
│   │   ├── crypto/
│   │   ├── engine/
│   │   ├── rendezvous/
│   │   └── transport/
│   ├── punchthrough/
│   │   ├── punch/
│   │   └── stun/
│   └── relay/
│       ├── metrics/
│       ├── ratelimit/
│       ├── room/
│       ├── transport/
│       └── webui/
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

Status: P2P-first transfer with QUIC direct transport, relay fallback, and resume support

What exists:

- rendezvous code model and parsing
- Noise `XXpsk0` handshake (transport-agnostic, works over direct or relay)
- secure channel framing over arbitrary `io.ReadWriter`
- transfer engine with header, ResumeOffer, chunk, and end framing
- SHA-256 integrity verification
- resumable single-file transfers with on-disk checkpoint state
- ResumeOffer protocol: receiver tells sender which chunk to start from
- deterministic transfer ID from (filename, size, SHA-256, chunk_size)
- transport selector: direct-first with relay fallback (default behavior)
- `--relay-only` flag to force relay transport
- `Candidate` and `CandidatePair` types for relay-coordinated peer address exchange
- relay-coordinated signaling (`/signal` endpoint) for candidate exchange between peers
- STUN/NAT discovery wired into the selector via `DiscoverCandidate`
- **QUIC-based direct transport** (`QUICConn`) over punched UDP socket with congestion control
- QUIC client/server role assignment (sender=client, receiver=server)
- graceful QUIC→ReliableConn→relay degradation chain
- ICE-like multi-candidate gathering (`GatherCandidates`) with host and server-reflexive candidates
- candidate prioritization (host > srflx > relay)
- connection quality metrics tracking (`MetricsConn`) with throughput, byte counters, and timing
- transport mode selection (`TransportQUIC` default, `TransportReliableUDP` fallback)
- UDP reliability/framing layer (`ReliableConn`) retained as legacy fallback
- `DirectDialer` with hole-punch integration via `internal/punchthrough/punch`
- `SelectionResult` with `Method`, `FallbackReason`, and `DirectErr` for observable transport decisions
- expanded `FallbackReason` set: `STUNFailed`, `NATUnfavorable`, `PunchFailed`, `SignalingFailed`
- deterministic tests verifying selector fallback reasons
- relay signaling endpoint tests for candidate exchange
- QUIC loopback tests (bidirectional data, 1 MB transfer, benchmarks)
- reliable framing unit tests for encode/decode, flags, and edge cases
- gathering and metrics unit tests

What is still missing:

- TURN-style relay candidate support in multi-candidate gathering
- connection migration for mobile/roaming
- directory transfer
- richer progress and transfer history handling

### `cmd/relay` + `internal/relay/`

Status: signaling server and fallback transport with hardened operations

What exists:

- room registry and state machine
- WebSocket sender and receiver flow
- bidirectional encrypted frame relay with byte/frame counting (fallback transport)
- `/signal` WebSocket endpoint for relay-coordinated candidate exchange (primary signaling)
- room TTL reaper with expiry callback
- per-IP rate limiting on `/ws` and `/signal` endpoints
- explicit HTTP server timeouts (read, write, idle, header)
- operational metrics endpoint at `/metrics`
- `/healthz` and `/status`
- minimal web handler at `/` (operator dashboard served by Python frontend)
- graceful shutdown handling
- Dockerfile and systemd service unit
- tests for room, transport, rate limiting, and metrics behavior

What is still missing:

- TURN-style authenticated relay data channel
- longer-term operator observation and alerting tooling
- authenticated operator endpoints

### `frontend/`

Status: active, intentionally thin, P2P-first story

What exists:

- Python FastAPI app serving Jinja2 templates
- htmx for live-updating relay status (polling every 2 seconds)
- product homepage and relay operator dashboard
- HTTPX client fetching data from Go relay's `/status`, `/healthz`, `/metrics` endpoints
- Pydantic settings for configuration
- static CSS via Tailwind CDN (no JavaScript build step)
- ruff linting/formatting, pyright type checking, pytest tests

What is still missing:

- authenticated operator workflows
- historical or persisted relay state
- control-plane mutations
- direct transport success/failure rate visualization

### `cmd/punchthrough` + `internal/punchthrough/`

Status: integrated into client transport selector as primary connection method

What exists:

- STUN probing
- NAT classification
- UDP hole-punching primitives
- CLI for probing
- client integration via `DiscoverCandidate` and `DirectDialer.dialWithPunch`
- relay-coordinated signaling for candidate exchange

What is still missing:

- ICE-like multi-candidate gathering (multiple STUN servers, local candidates)
- relay candidate (TURN-style)
- broader real-world network validation across diverse NAT types

### `cmd/bore-admin`

Status: active, minimal, intentionally not a control plane

What exists:

- usable `status` command
- HTTP polling of the relay `/status` endpoint
- human-readable output for uptime, room counts, and relay limits

What is still missing:

- transport method breakdown in status output
- persistent storage or local history
- alerting
- configuration profiles

---

## Build / run / verify

### Prerequisites

- Go `1.26.1`
- Python `3.12+` with `uv`
- build and test from the repo root for Go, and from `frontend/` for frontend tasks

### Frontend

```bash
cd frontend
uv sync
uv run ruff check .
uv run ruff format --check .
uv run pyright
uv run pytest
```

Notes:

- the Python frontend runs as a separate process alongside the Go relay
- start with `cd frontend && uv run uvicorn app.main:app --host 127.0.0.1 --port 3000 --app-dir src`
- set `BORE_RELAY_URL` to point at the Go relay (defaults to `http://127.0.0.1:8080`)

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

### Local smoke flow

Terminal 1 — start relay (used for signaling + fallback):

```bash
RELAY_ADDR=127.0.0.1:8080 go run ./cmd/relay
```

Terminal 1b — start frontend (in a separate terminal):

```bash
cd frontend && BORE_RELAY_URL=http://127.0.0.1:8080 uv run uvicorn app.main:app --host 127.0.0.1 --port 3000 --app-dir src
```

Browser check while both are running:

- product page: `http://127.0.0.1:3000/`
- relay ops page: `http://127.0.0.1:3000/ops/relay`
- raw status JSON (relay direct): `http://127.0.0.1:8080/status`
- operational metrics (relay direct): `http://127.0.0.1:8080/metrics`
- health check (relay direct): `http://127.0.0.1:8080/healthz`

Terminal 2 — send (tries direct first, falls back to relay):

```bash
./bore send ./payload.txt --relay http://127.0.0.1:8080
```

Terminal 3 — receive:

```bash
./bore receive <code> --relay http://127.0.0.1:8080
```

Terminal 2 — send with relay-only (forces relay, skips direct):

```bash
./bore send ./payload.txt --relay http://127.0.0.1:8080 --relay-only
```

Expected result:

- sender prints a full rendezvous code and the transport method used
- receiver completes successfully and reports transport method
- sender and receiver SHA-256 values match
- output file matches input bytes
- when direct succeeds: "transport: direct" in verbose output
- when direct fails: "transport: relay (fallback: ...)" in verbose output

---

## Milestone map

### M0 — relay-based encrypted transfer ✓

Done. The relay path works, is tested, and is the proven fallback.

### M1 — direct transport infrastructure ✓

Done. STUN, signaling, hole-punching, ReliableConn all implemented and tested.

### M2 — transfer durability ✓

Done. Single-file resume with on-disk checkpoint state.

### M3 — relay hardening ✓

Done. Rate limits, timeouts, metrics, deployment packaging.

### M4 — P2P-first default ✓

Done. Direct transport is the default path. `--relay-only` exists for forcing relay. Transport method is reported to the user.

### M5 — direct transport quality ✓

Done. QUIC replaces custom ReliableConn as the default direct transport. Multi-candidate gathering implemented. ~340 MB/s throughput on loopback benchmarks.

### M6 — operator surfaces grow with P2P reality

Success means the browser and admin surfaces reflect the P2P-first reality: transport method stats, direct vs relay breakdown, signaling health.

---

## Phase dashboard

### Phase 0 — relay-based encrypted transfer path (legacy, still functional)

Status: done / checked — now serves as fallback transport

Checklist:

- [x] client rendezvous code generation and parsing exist
- [x] Noise `XXpsk0` handshake exists
- [x] encrypted relay-based file transfer works
- [x] relay room brokering and `/healthz` + `/status` exist
- [x] `bore-admin status` exists
- [x] relay-served browser surface exists

Note: Phase 0 is the foundation. The relay path continues to work as the automatic fallback when direct transport fails.

### Phase 1 — direct-path infrastructure (legacy, integrated)

Status: done / checked — infrastructure integrated into client, promoted to default in Phase 5

Checklist:

- [x] transport abstraction layer with `Conn` and `Dialer`
- [x] relay transport implementing `Dialer`
- [x] direct transport implementing `Dialer`
- [x] selector with direct-first and relay-fallback logic
- [x] rendezvous flow wired to `Dialer`
- [x] relay-coordinated peer-candidate exchange
- [x] STUN and NAT discovery wired into direct dial
- [x] UDP reliability/framing layer for direct transport
- [x] fallback reason tracking for transport decisions
- [x] deterministic verification for direct-path and relay fallback

### Phase 2 — transfer durability

Status: done / checked

Checklist:

- [x] resume-state shape documented
- [x] sender/receiver state persisted for single-file resume
- [x] restart vs resume rules defined and tested
- [x] interruption-recovery tests for relay-based transfers
- [ ] directory transfer (after single-file resume is solid)

### Phase 3 — relay hardening

Status: done / checked

Checklist:

- [x] per-IP rate limiting on room creation, joins, and connections
- [x] quotas and resource controls for room occupancy
- [x] explicit HTTP server timeouts
- [x] metrics endpoint and operator counters
- [x] deployment and service packaging
- [ ] admin-only profiling hooks (deferred)

### Phase 4 — browser and operator surface

Status: active / P2P-first updates landed

Checklist:

- [x] relay-served browser surface (now `frontend/` via FastAPI + htmx)
- [x] same-origin read-only status page
- [x] product story aligned with actual runtime
- [x] update product story to reflect P2P-first architecture
- [x] surface transport method stats in operator page
- [x] show direct vs relay breakdown in `/ops/relay`
- [ ] decide whether browser surface stays static until auth story exists

### Phase 5 — P2P-first default path ★ ACTIVE

Status: **done / checked**

This is the architectural pivot. Direct transport becomes the default. Relay becomes fallback.

Checklist:

- [x] flip `Selector.EnableDirect` to true by default
- [x] remove `--direct` flag from CLI
- [x] add `--relay-only` flag to force relay transport
- [x] report transport method (direct/relay) to user after transfer
- [x] report fallback reason when relay is used
- [x] update `bore status` output to reflect P2P-first identity
- [x] update `bore components` output
- [x] increase direct transport timeout for better success rate
- [x] all existing tests pass with new defaults
- [x] new tests for default-direct behavior

Exit criteria:

- `bore send` and `bore receive` attempt direct transport by default
- relay fallback is automatic and transparent
- `--relay-only` flag exists for forcing relay
- transport method is visible to the user
- all existing relay-based tests continue to pass

### Phase 6 — direct transport quality improvements

Status: **done / checked**

Checklist:

- [x] evaluate QUIC (`quic-go`) as replacement for custom `ReliableConn`
- [x] implement QUIC-based direct transport with connection over punched UDP socket
- [ ] add sliding window or proper congestion control to `ReliableConn` (if QUIC is deferred) — QUIC chosen, not needed
- [x] implement ICE-like multi-candidate gathering (multiple STUN servers, local/relay candidates)
- [ ] add TURN-style authenticated relay data channel (deferred to future phase)
- [x] measure and optimize direct transport throughput
- [x] add connection quality metrics (RTT, loss rate, throughput)
- [ ] test across diverse real-world NAT configurations (requires external environments)
- [ ] add connection migration support for mobile/roaming scenarios (deferred)

Exit criteria:

- direct transport throughput is competitive with relay for large files — **achieved: ~340 MB/s on QUIC loopback benchmark**
- NAT traversal success rate is measured and documented — **multi-candidate gathering implemented; real-world measurement requires external testing**
- fallback to relay is faster and more graceful — **QUIC failure gracefully degrades to ReliableConn, then to relay**

### Phase 7 — operator and browser surfaces for P2P reality

Status: active / initial transport stats landed

Checklist:

- [x] show transport method breakdown (direct vs relay) in `/ops/relay`
- [x] add signaling health metrics to `/metrics`
- [ ] add direct transport success/failure rates to operator view
- [ ] decide whether relay history needs persistence
- [ ] add useful historical views only if they solve real problems
- [ ] add alerting basics without turning bore into a control plane

Exit criteria:

- operator surfaces reflect the actual P2P-first runtime
- signaling and direct transport health are observable

### Phase 8 — verification and release discipline

Status: active / fuzz targets and CI caching landed

Checklist:

- [x] root `.github/workflows/ci.yml` runs component verification
- [x] `golangci-lint run` in CI
- [x] `govulncheck ./...` in CI
- [x] CI job for `frontend/` (uv sync → ruff → pyright → pytest)
- [x] add fuzz targets for rendezvous code and transfer frame parsing
- [ ] add integration test for full direct transfer (loopback STUN mock)
- [ ] add integration test for direct-fails-relay-succeeds path
- [ ] keep all docs aligned when runtime claims change

Exit criteria:

- the repo proves its claims with repeatable checks

### Phase 9 — frontend rewrite: FastAPI + htmx

Status: **active**

Replace the React + Vite + TypeScript SPA (`web/`) with a Python server-rendered frontend (`frontend/`). The Python frontend runs as a separate lightweight process, fetches data from the Go relay's existing API, and serves HTML pages with htmx for live updates. No JavaScript build step.

Checklist:

- [x] create `frontend/` directory with FastAPI + Jinja2 + htmx app
- [x] `pyproject.toml` with uv, ruff, pyright, pytest
- [x] `.python-version` pinning Python 3.12+
- [x] FastAPI app that proxies to Go relay for `/status`, `/healthz`, `/metrics`
- [x] Jinja2 base template with navigation (product page + relay ops)
- [x] product homepage template (equivalent to SPA's `/` route)
- [x] relay ops template (equivalent to SPA's `/ops/relay` route) with htmx live polling
- [x] 404 template
- [x] static CSS (Tailwind CDN, no build step)
- [x] htmx `hx-get` with `hx-trigger="every 2s"` for live-updating relay status
- [x] pytest tests for API routes
- [x] `ruff check .` and `ruff format --check .` pass
- [x] pyright passes
- [x] delete `web/` directory entirely (React, Vite, Bun, TypeScript, node_modules)
- [x] remove `web/` references from CI (`.github/workflows/ci.yml`)
- [x] update Go relay to no longer embed SPA assets
- [x] update `internal/relay/webui/` to serve a redirect or minimal placeholder
- [x] add `frontend/` CI job (uv sync → ruff → pyright → pytest)
- [x] update `BUILD.md` repo snapshot, monorepo layout, component snapshots
- [x] update `README.md` to reflect new frontend stack
- [ ] commit and push

Exit criteria:

- `cd frontend && uv sync && ruff check . && ruff format --check . && pytest` passes
- Go code still compiles and passes tests: `gofmt -w . && go vet ./... && go test ./... && go build ./cmd/bore`
- the relay ops page shows the same information the SPA showed, with htmx live updates
- no Node, Bun, TypeScript, React, or JavaScript build step remains in the repo

---

## Focused verification checklist

Use the narrowest verification that proves the current claim.

### Docs-only changes

- re-read the touched docs for consistency
- confirm current-state sections describe implemented behavior
- confirm planned sections are labeled as planned or active

### Frontend changes

```bash
cd frontend && uv run ruff check . && uv run ruff format --check . && uv run pyright && uv run pytest
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

### Full pre-commit verification

```bash
gofmt -w .
go vet ./...
go test ./...
go build ./cmd/bore
```

---

## Working rules

1. Keep the relay payload-blind. If it can inspect file contents, the design regressed.
2. Treat the rendezvous code as cryptographic input, not just a locator.
3. Direct transport is the default. Relay is the fallback. Docs should reflect this.
4. End-to-end encryption works identically regardless of transport method.
5. Keep docs honest. The browser surface and operator tools should match runtime reality.
6. Run the narrowest meaningful verification first. Broaden only when the change surface demands it.
7. If you change architecture or security claims, update `BUILD.md`, `ARCHITECTURE.md`, and `SECURITY.md` in the same pass.
8. The relay is a signaling server first, fallback transport second. Design decisions should respect this hierarchy.

---

## Risks and open questions

### Risk: direct transport may fail across many real-world NATs

Mitigation:

- relay fallback is automatic and transparent
- fallback reasons are observable for debugging
- the user always gets their file transferred — the question is which path
- measuring real-world success rate is a priority for Phase 6

### Risk: QUIC transport adds dependency complexity

Mitigation:

- `quic-go` is a well-maintained, widely-used QUIC implementation
- ReliableConn is retained as fallback if QUIC fails to establish
- the degradation chain is: QUIC → ReliableConn → relay
- QUIC throughput is ~340 MB/s on loopback, a massive improvement over stop-and-wait

### Risk: resume state is per-receiver only

Mitigation:

- resume state lives on the receiver's filesystem only
- the sender re-sends from the requested chunk on each connection
- directory transfer resume requires additional design work

### Risk: relay hardening is baseline, not audited

Mitigation:

- the relay enforces per-IP rate limits, HTTP timeouts, and tracks metrics
- deployment packaging is available
- external security review still needed

### Risk: signaling depends on relay availability

Mitigation:

- the relay is lightweight and self-hostable
- signaling is a brief WebSocket exchange, not a sustained connection
- future work: support alternative signaling mechanisms (shared secret + known endpoint)

### Resolved: QUIC chosen over improved ReliableConn

Decision:

- QUIC (`quic-go`) was evaluated and implemented in Phase 6
- provides production-quality congestion control, flow management, and reliability
- ~340 MB/s throughput on loopback benchmarks (vs ReliableConn's stop-and-wait bottleneck)
- ReliableConn retained as fallback for environments where QUIC fails to establish
- QUIC is now the default direct transport layer

### Open question: when directory transfer becomes worth it

Current answer:

- not before single-file resume and restart semantics are trustworthy
- not before direct transport quality is proven

---

## Immediate next moves

### Current lane: Phase 6 complete

Phase 6 (direct transport quality) is done. QUIC transport, multi-candidate gathering, connection metrics, and throughput benchmarks are all in place. The next highest-leverage moves are:

1. **Phase 8 — integration tests**: loopback STUN mock for direct transfer, direct-fails-relay-succeeds path
2. **Phase 7 — deeper operator stats**: direct transport success/failure rates, QUIC metrics in operator view
3. **TURN-style relay candidate**: add relay as a candidate type in multi-candidate gathering

### If the goal is cleanup instead of features

1. add integration tests for the full QUIC transfer path
2. improve the QUIC-to-ReliableConn fallback behavior under adverse conditions
3. add more comprehensive tests for NAT combinations
4. decide whether browser surface stays static until auth story exists

---

## Resume checklist

If you are resuming this repo later, do this in order:

1. read `README.md`
2. read this file
3. read `ARCHITECTURE.md` and `SECURITY.md` if the task touches behavior or claims
4. inspect `git status`
5. treat the repo root as the source of truth for Go builds
6. pick one lane:
   - direct transport quality (Phase 6)
   - frontend/operator surface updates (Phase 4/7/9)
   - verification and CI (Phase 8)
   - directory transfer
7. run focused verification before and after the change
8. before calling a lane done, make sure the docs still read like an active program
