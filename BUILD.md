# BUILD.md

This file is the execution manual for Bore's full-stack rewrite.

`README.md`, `ARCHITECTURE.md`, and `docs/status-contract.md` describe the shipped v1 implementation that exists in this repo today. This file describes the planned replacement of that implementation with Stephen's unified web stack:

- Bun
- TypeScript
- Astro
- Vue
- Elysia
- Zod
- PostgreSQL
- Docker Compose
- Caddy

This is an active rewrite document. Phase 0 and Phase 1 now have real repo state under `apps/`, `packages/`, `infra/`, the root Bun workspace, and `docker-compose.yml`, but v2 is not the shipped product yet.

## Rewrite directive

**Maximum stack unification wins.**

Bore's next primary product shape is a web-first application on a single TypeScript runtime and deployment story. The current Go relay, CLI, and OpenTUI surfaces are the shipped v1 product, but they are no longer the architectural destination.

Decision summary:

- the primary destination is a unified web application, not a Go backend with companion frontends
- Elysia replaces the Go relay and API ownership for the next generation product
- Astro + Vue replaces the current browser surface as the primary user and operator experience
- PostgreSQL becomes the durable system of record for application metadata and operator history
- Docker Compose + Caddy becomes the default local and deployment runtime shape
- any retained CLI or TUI lane is transitional, compatibility-oriented, or explicitly secondary
- the rewrite is allowed to change internal architecture aggressively as long as product truth, security posture, and rollout risk stay explicit

## Current state: shipped v1

The repo currently ships a Go-first product:

- one root Go module owns transport, relay, crypto, file transfer, NAT traversal, packaging, and the main CLIs
- `cmd/bore` and `cmd/relay` are the primary shipped binaries
- `cmd/bore-admin` is a smaller compatibility operator CLI
- `web/` is an Astro + Vue browser surface served by the Go relay
- `tui/` is an OpenTUI operator console fed by the Go relay's `/status` contract
- relay room state is in memory only
- resumable receive state is local JSON on disk
- no durable service database exists today
- direct P2P plus relay fallback is a core v1 product promise

What v1 does well:

- secure file transfer already exists and ships
- operator status data already exists and has a documented contract
- the product has real deployment packaging and release discipline

What does not match the new direction:

- backend ownership is split away from Stephen's current unified TypeScript web stack
- the browser surface is a companion, not the architectural center
- operator and product surfaces are fragmented across Go CLI, OpenTUI, and browser lanes
- there is no shared TypeScript contract package across frontend and backend
- there is no PostgreSQL-backed system of record for sessions, events, or operator history
- the current runtime shape is not the target Docker Compose + Caddy full-stack layout

## Target state: shipped v2

Bore v2 should ship as a Bun workspace with one coherent full-stack architecture.

### Primary product shape

- **Web-first application** served behind Caddy
- **Astro** for route structure, shell pages, static assets, and delivery packaging
- **Vue** for interactive transfer and operator flows
- **Elysia** for HTTP APIs, WebSocket endpoints, realtime session coordination, and server-side business logic
- **Zod** for shared runtime-validated contracts across backend and frontend
- **PostgreSQL** for durable metadata, event history, operator history, and migration-backed application state
- **Docker Compose** for the default local and self-hosted runtime story

### Secondary surfaces

- any CLI kept during the rewrite is a compatibility shim, smoke-test tool, or power-user helper
- any TUI kept during the rewrite is an operator compatibility surface, not the primary UI destination
- neither the CLI nor the TUI should own product direction, core contracts, or primary UX decisions after v2 starts landing

### Target repo shape

```text
.
├── apps/
│   ├── api/                 # Elysia backend
│   └── web/                 # Astro + Vue frontend
├── packages/
│   ├── contracts/           # shared Zod schemas and typed clients
│   ├── crypto/              # TS transfer and encryption primitives
│   └── ui/                  # shared UI helpers/components if earned
├── infra/
│   └── caddy/
├── migrations/
├── docker-compose.yml
├── README.md
├── ARCHITECTURE.md
└── BUILD.md
```

During transition, the current Go tree can remain in place as `v1` reference code until cutover is complete. Do not interleave new v2 TypeScript runtime code deep inside `cmd/` or `internal/`.

## Architecture decisions for the rewrite

### 1. Web-first, not terminal-first

The rewrite centers the browser application. The send flow, receive flow, and operator experience should all be first-class in the web app.

The current CLI and TUI can stay available during transition, but they should consume or bridge to v2 contracts when useful. They are not the design center.

### 2. One backend runtime

Elysia becomes the backend owner for:

- transfer session lifecycle
- rendezvous code creation and validation
- realtime coordination and signaling
- relay-session orchestration
- operator APIs and dashboards
- auth or operator identity if Bore later adds it
- persistence and migrations

Do not preserve a permanent split where Go owns the hard parts and TypeScript owns only the skin. That defeats the point of this rewrite.

### 3. PostgreSQL is required for v2

v2 uses PostgreSQL as the durable source of truth for:

- transfer sessions
- participants and role state
- file metadata
- transfer events and failures
- operator-visible history and audits
- deploy-safe migrations and future reporting

PostgreSQL does **not** become a file blob store. File contents must remain in streaming paths or explicitly bounded temp storage. Never persist plaintext file payloads to the database.

### 4. Shared contracts are non-optional

All externally consumed v2 payloads must be defined once in Zod and shared between backend and frontend.

At minimum, create shared schemas for:

- health and readiness payloads
- transfer session create/join/read APIs
- relay and operator status APIs
- websocket message envelopes
- file metadata and progress events
- failure and validation payloads

### 5. Compose + Caddy is the default runtime story

Local and deployable runtime shape should converge early:

- `caddy` serves static assets and reverse proxies API and websocket traffic
- `api` runs the Elysia service on Bun
- `postgres` runs the application database
- optional worker or sidecar services are allowed only when clearly earned

Do not require a separate bespoke dev topology once v2 is underway.

### 6. Security posture stays explicit

The rewrite must keep Bore's existing security bar visible:

- end-to-end encryption remains a product requirement
- rendezvous codes stay security-sensitive inputs, not cosmetic join codes
- relay or coordination services must remain unable to inspect plaintext file payloads
- Caddy and Elysia boundaries must be explicit, with no accidental public admin surfaces
- rate limits, request limits, and timeouts must exist from the first public v2 build

## Data and runtime constraints

These are hard constraints for the rewrite plan.

### Durable data

- PostgreSQL is the only planned durable database for v2
- no parallel SQLite lane for the primary v2 app
- no permanent in-memory-only session model once v2 becomes the primary runtime
- all schema changes must go through migrations committed in repo

### File payload handling

- do not store plaintext transferred files in PostgreSQL
- prefer stream-through transfer paths over durable server-side storage
- if temporary disk spooling is required for resume or fallback behavior, keep it bounded, explicit, and disposable
- encryption keys and derived secrets must never be written to logs or durable metadata stores

### Runtime topology

- Caddy terminates HTTP and HTTPS and fronts all public traffic
- Elysia owns application APIs and realtime endpoints behind Caddy
- PostgreSQL is private to the Compose network by default
- local development must work through Docker Compose without hidden external dependencies

### Compatibility boundaries

- v1 Go binaries may remain runnable during migration
- v1 contracts are not automatically v2 contracts
- compatibility shims should be thin and temporary
- new feature work should target v2 unless there is a release-blocking v1 maintenance issue

## Rewrite progress snapshot

- **Phase 0: done**. Repo docs now distinguish shipped v1 truth from planned v2 direction, and this file owns the rewrite plan.
- **Phase 1: done**. The repo now has a root Bun workspace, `apps/api`, `apps/web`, `packages/contracts`, `infra/caddy`, `.env.example`, `docker-compose.yml`, and verified health/readiness plus Caddy routing.
- **Phase 2+: pending**. Persistence, typed session APIs, transfer implementation, parity recovery, and cutover work are still ahead.

## v2 launch feature matrix

| Area | v2 launch stance | Notes |
| --- | --- | --- |
| Web-first send and receive flows | keep | Core product path for v2 |
| Operator web surface | keep | Must move to v2 APIs, not Go `/status` |
| End-to-end encryption | keep | Non-negotiable product requirement |
| Short human rendezvous codes | keep | Security-sensitive join primitive |
| Relay-first transfer path | keep | Explicit Phase 4 MVP direction |
| PostgreSQL-backed metadata and event history | keep | Required v2 system of record |
| Browser-capable direct transport parity | defer | Decide only after relay-first v2 is stable |
| Resume semantics | defer | Recover after relay-first MVP, if earned |
| `cmd/bore` as a primary product lane | drop | Keep only as legacy or compatibility if retained at all |
| `cmd/relay` as the long-term backend owner | drop | Elysia replaces Go backend ownership in v2 |
| `web/` Go-served browser surface as the primary UI | drop | Replaced by `apps/web` over time |
| `tui/` as the primary operator surface | drop | Secondary compatibility lane only if still useful |

## Transitional surface stance

- `cmd/bore`: keep runnable during migration for legacy direct-transfer workflows, but do not let it drive v2 product design.
- `cmd/relay`: keep runnable for shipped v1 operations only. It is not the v2 backend destination.
- `cmd/bore-admin`: keep only as a terse compatibility shim while the web and operator APIs mature. Remove or freeze once v2 ops coverage is adequate.
- `tui/`: keep as a secondary operator compatibility surface while the v2 web ops shell grows. Do not treat it as the primary operator destination.

## Proposed v2 persistence model

Start with these tables and keep them boring:

- `transfer_sessions`
  - id, human_code, status, transport_mode, created_at, expires_at, completed_at, failure_reason
- `transfer_participants`
  - id, session_id, role, client_id, state, connected_at, disconnected_at, last_seen_at
- `transfer_files`
  - id, session_id, file_name, size_bytes, sha256, content_type, chunk_size, total_chunks
- `transfer_events`
  - id, session_id, event_type, event_payload, created_at
- `operator_events`
  - id, service_node, event_type, event_payload, created_at
- `schema_migrations`
  - migration bookkeeping if not owned by the migration tool itself

Do not over-model before the relay-first MVP exists. Session state, participant state, file metadata, and append-only events are enough to start.

## Rewrite phases

### Phase 0: freeze the v2 charter

Purpose: remove ambiguity before code starts moving.

Status: **done**. The repo docs now distinguish shipped v1 truth from the active v2 plan, and the feature and deprecation stance is explicit in this file.

Deliverables:

- this `BUILD.md` becomes the rewrite source of truth
- repo docs clearly distinguish shipped v1 truth from planned v2 direction
- explicit feature matrix: keep, defer, or drop for v2 launch
- deprecation stance for `cmd/bore`, `cmd/relay`, `bore-admin`, and `tui/`

Acceptance criteria:

- future work can answer "what is v1?" and "what is v2?" without guessing
- no repo doc still presents the Go backend as the long-term destination
- the target stack is stated consistently everywhere that speaks about future direction

Verification expectations:

- doc review only
- `git diff --check`
- repo search confirms the old BUILD framing is gone

### Phase 1: stand up the v2 workspace and runtime skeleton

Purpose: create a real landing zone for the rewrite.

Status: **done**. The root Bun workspace, `apps/api`, `apps/web`, `packages/contracts`, `infra/caddy`, `.env.example`, `docker-compose.yml`, and Caddy-routed health shell now exist in repo and verify locally.

Deliverables:

- Bun workspace at repo root
- `apps/api` booting an Elysia service
- `apps/web` booting an Astro + Vue app
- `packages/contracts` with first shared Zod schemas
- `docker-compose.yml` bringing up Caddy, api, and PostgreSQL
- environment loading and `.env.example` for local setup
- base health endpoints and readiness checks

Acceptance criteria:

- `docker compose up --build` brings up Caddy, API, and PostgreSQL locally
- `/api/health` returns a typed health payload
- the web app loads through Caddy, not by bypassing the target runtime shape
- frontend and backend compile in one Bun workspace

Verification expectations:

By the end of this phase, the repo must expose and keep green:

- `bun run lint`
- `bun run check`
- `bun run test`
- `bun run build`
- `docker compose up -d --build`

### Phase 2: build backend foundation in Elysia

Purpose: replace Go-owned application coordination with Elysia-owned v2 services.

Deliverables:

- typed environment config and startup validation
- PostgreSQL migrations for session, participant, file, and event tables
- session creation and join endpoints
- session lookup and operator summary endpoints
- websocket or SSE coordination channel with typed envelopes
- structured logs, rate limits, request size limits, and timeouts

Backend rewrite stage exit criteria:

- Elysia can create, read, and advance transfer sessions against PostgreSQL
- all request and response payloads are validated by shared Zod schemas
- operator-visible status can be produced without reading Go in-memory state
- the app can recover session metadata after restart because state lives in PostgreSQL

Verification expectations:

- API unit tests for handlers and validation failures
- integration tests against PostgreSQL in Docker Compose
- migration up/down smoke check
- websocket contract tests for message envelopes

### Phase 3: build the web application foundation

Purpose: make v2 usable from the browser before transfer data starts moving.

Deliverables:

- Astro route structure for `/`, `/send`, `/receive/[code]`, and `/ops`
- Vue state and composables for session create/join flows
- shared typed client generated or wrapped from `packages/contracts`
- form validation and error states using Zod-backed contracts
- operator dashboard shell that consumes live API data from Elysia

Frontend rewrite stage exit criteria:

- a user can create a transfer session from the web UI
- a receiver can open a join route and attach to that session
- operator pages render live session data from v2 APIs
- no page depends on ad hoc untyped JSON parsing

Verification expectations:

- component and composable tests with Bun
- route/type validation with `astro check`
- browser smoke tests for create and join flow shells
- responsive checks for mobile and desktop widths

### Phase 4: ship a relay-first secure transfer MVP

Purpose: get the rewritten stack moving real encrypted files before chasing direct-path parity.

This phase is intentionally opinionated: **v2 should ship relay-first before it tries to recreate v1's direct transport story.** A relay-first MVP is acceptable if it preserves end-to-end encryption, short-code rendezvous, and operational clarity.

Deliverables:

- TypeScript transfer and crypto primitives in `packages/crypto`
- relay-session streaming path through Elysia for sender and receiver
- browser send and receive flows for single-file transfer
- progress events surfaced to both participant UIs and operator surfaces
- PostgreSQL event recording for lifecycle and failure analysis
- Caddy routing for API, websocket, and frontend assets

Acceptance criteria:

- a sender can create a code, attach a file, and complete a transfer from the web UI
- a receiver can join by code and receive that file from the web UI
- the server cannot read plaintext payload contents
- operator pages show active sessions, completions, and failures from v2 data
- the compose stack works locally without hidden manual steps

Verification expectations:

- end-to-end relay transfer smoke test in Docker Compose
- integrity verification on completed transfer
- failure-path tests for expired session, duplicate join, and interrupted stream
- Caddy reverse-proxy smoke check for websocket and API paths

### Phase 5: recover parity features that actually matter

Purpose: bring back the v1 features that Bore still needs after the architectural cutover.

Deliverables:

- resumable transfers or an explicit replacement policy if resume is not yet viable
- better operator history and filtering from PostgreSQL-backed event data
- compatibility helpers for CLI-driven smoke checks if still needed
- deprecation decision for `bore-admin` and `tui/`
- migration of documentation, examples, and runbooks to the v2 API and UI

Acceptance criteria:

- every meaningful v1 feature is in one of three states: implemented, deferred with reason, or intentionally dropped
- operator workflows no longer depend on the Go `/status` endpoint for the primary product path
- the v2 runtime has a clear backup, restore, and migration story for PostgreSQL

Verification expectations:

- regression suite for resume or explicit non-resume behavior
- operator workflow smoke tests
- restore test from a PostgreSQL backup into a fresh Compose stack

### Phase 6: decide direct transport and fallback strategy

Purpose: handle the hardest v1 capability honestly instead of hand-waving it.

Decision rule:

- if Bore still needs browser-accessible direct peer transfer, implement it deliberately as a v2 feature after the relay-first system is stable
- if browser and Bun runtime constraints make that impractical, document v2 as relay-first and keep v1 direct mode as a legacy lane until a real replacement exists

Likely implementation direction if this phase is pursued:

- browser-capable direct transport via WebRTC data channels with Elysia-based signaling
- relay fallback remains available when direct negotiation fails
- transport selection and failure reasons are surfaced in the operator UI and persisted in event history

Acceptance criteria:

- direct mode either works end-to-end with explicit fallback behavior or is explicitly removed from the v2 promise
- there is no ambiguous half-implemented direct path in the shipped product

Verification expectations:

- NAT-matrix integration testing to the extent practical
- transport reporting tests for direct vs relay outcomes
- documentation that matches the actual shipped behavior

### Phase 7: cut over and retire v1 as primary

Purpose: finish the rewrite instead of living in permanent dual-stack limbo.

Deliverables:

- README and architecture docs point to v2 as the primary product
- release pipeline builds and ships the Bun workspace runtime by default
- Go binaries are either removed from the default path or moved under an explicit legacy policy
- any remaining compatibility CLI or TUI surface is clearly labeled secondary

Acceptance criteria:

- a new contributor sees the TypeScript full-stack app as the main codepath immediately
- Caddy + Elysia + PostgreSQL is the documented deploy path
- v1 is either archived, sunset, or constrained to a clearly named legacy lane

Verification expectations:

- fresh-clone bootstrap from README
- full-stack smoke test from Docker Compose
- release artifact check for the shipped v2 runtime

## Backend rewrite stages summary

These stages should be completed in order:

1. **Workspace and config foundation**
   - Bun workspace, env validation, shared lint/test/build scripts
2. **Persistence and contracts**
   - PostgreSQL migrations, Zod schemas, typed API envelopes
3. **Session and coordination APIs**
   - create/join/read session flows, operator summaries, realtime message channels
4. **Encrypted transfer implementation**
   - TypeScript crypto and relay streaming path
5. **Operability and hardening**
   - logs, rate limits, timeouts, backup/restore, deployment polish
6. **Direct transport decision**
   - either implement intentionally or remove from the v2 promise

Do not skip from workspace setup straight to transfer plumbing. Shared contracts, migrations, and runtime topology must exist first.

## Frontend rewrite stages summary

These stages should be completed in order:

1. **Route and shell foundation**
   - landing, send, receive, and ops routes in Astro
2. **Typed client and state layer**
   - Vue composables wired to shared contracts
3. **Transfer UX**
   - create code, join session, upload/select file, progress, completion, failure states
4. **Operator UX**
   - active sessions, history, failure reasons, transport visibility
5. **Compatibility cleanup**
   - remove dependence on the current Go-served web surface as v2 becomes real

Do not rebuild the current `/status` page and call the rewrite done. The web app must own the core transfer flow.

## Risks

### Product and architecture risks

- rewriting a networking-heavy Go system into Bun + TypeScript can lose transport maturity, throughput, and operational simplicity
- a web-first destination may change the feel of a product that currently excels in terminal and binary form
- direct P2P parity may be substantially harder in browser-compatible tooling than in the current Go implementation
- PostgreSQL adds operational weight that v1 intentionally avoids today

### Security risks

- accidental weakening of end-to-end encryption during the rewrite
- treating rendezvous codes as simple routing tokens instead of security-sensitive material
- leaking file metadata, secrets, or session details into logs or overly broad operator views
- exposing operator endpoints through Caddy without a deliberate access model

### Delivery risks

- dual-running v1 and v2 for too long and letting docs drift
- rebuilding surface polish before core secure transfer works
- over-designing the schema before the relay-first MVP proves the actual data needs
- shipping a half-finished direct transport story that confuses users and operators

## Non-goals

These are explicitly out of scope unless a later phase earns them:

- durable storage of plaintext file payloads in PostgreSQL
- keeping Go as a permanent co-equal backend for the rewritten product
- preserving OpenTUI as the primary operator destination
- adding a second database technology to hedge on commitment
- introducing microservices before the monolithic Elysia app proves insufficient

## Done means

The rewrite plan is complete only when all of the following are true:

- Bore's primary documented architecture is Bun + TypeScript + Astro + Vue + Elysia + Zod + PostgreSQL + Docker Compose + Caddy
- the main product flows run through the web application, not through legacy companion surfaces
- backend contracts are shared through Zod and validated at runtime
- PostgreSQL owns durable metadata and operator history with migrations in repo
- local and deployment runtime stories both go through Compose + Caddy
- v1 Go binaries are either retired or clearly constrained to legacy compatibility work

Until then, treat the current Go codebase as shipped v1 and this file as the authoritative execution plan for replacing it.