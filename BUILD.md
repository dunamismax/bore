# BUILD.md

> **Required operating rule:** any agent touching Bore's rewrite plan must update this file in the same change set when scope, sequencing, architecture decisions, or verified repo truth changes. Keep the checkboxes honest. Do not mark work complete unless the repo already proves it.

Bore is in an active rewrite program. `README.md` and `ARCHITECTURE.md` describe the shipped Go-first v1 product. This file tracks the v2 cutover onto Stephen's unified Bun + TypeScript web stack.

## Current repo truth

- [x] The shipped product is still the Go-first v1 codepath rooted at `go.mod`.
- [x] `cmd/bore` and `cmd/relay` remain the primary shipped binaries.
- [x] `cmd/bore-admin` remains a compatibility operator CLI.
- [x] `web/` is the shipped Astro + Vue browser surface served by the Go relay.
- [x] `tui/` is the shipped OpenTUI operator console over the Go relay's `/status` contract.
- [x] Relay room state is still in memory for v1.
- [x] Resumable receive state is still stored as local JSON on disk for v1.
- [x] No durable PostgreSQL-backed Bore service database is part of the shipped product yet.
- [x] A real v2 landing zone now exists at repo root with `apps/api`, `apps/web`, `packages/contracts`, `infra/caddy`, `db/migrations`, `.env.example`, and `docker-compose.yml`.
- [x] The v2 workspace already exposes root `lint`, `check`, `test`, `build`, and `verify` commands.
- [ ] v2 is the shipped primary product.
- [ ] Go is retired from the primary product path.

## Rewrite guardrails

- [x] Bore v2 is web-first. The browser app is the primary product surface.
- [x] Elysia owns the future backend coordination layer instead of preserving a permanent Go-plus-TypeScript split.
- [x] Astro owns route delivery and page shells. Vue is used only where richer interaction clearly earns it.
- [x] PostgreSQL is required for durable v2 metadata and operator history.
- [x] Shared Zod contracts are required for backend and frontend payloads.
- [x] Docker Compose plus Caddy is the default local and deployment runtime shape.
- [x] End-to-end encryption and short human rendezvous codes remain non-negotiable product requirements.
- [x] v2 should ship relay-first before it spends schedule on recreating direct-path parity.
- [x] Any surviving CLI or TUI lane after cutover is secondary, compatibility-oriented, or explicitly temporary.
- [ ] Permanent dual-stack v1 and v2 operation is an acceptable steady state.

## Phase status summary

- [x] Phase 0 - Freeze the rewrite charter.
- [x] Phase 1 - Stand up the v2 workspace and runtime skeleton.
- [ ] Phase 2 - Build backend foundation in Elysia.
- [ ] Phase 3 - Build the web application foundation.
- [ ] Phase 4 - Ship a relay-first secure transfer MVP.
- [ ] Phase 5 - Recover the parity features that still matter.
- [ ] Phase 6 - Decide direct transport and fallback strategy.
- [ ] Phase 7 - Cut over and retire v1 as primary.

## Phase 0 - Freeze the rewrite charter

### Objectives

- [x] Make `BUILD.md` the source of truth for the rewrite program.
- [x] Separate shipped v1 truth from planned v2 direction in stable docs.
- [x] Document the product, stack, and deprecation stance clearly enough that future work does not guess.

### Checklist

- [x] Document that the shipped product is still the Go-first v1 line.
- [x] Document that v2 targets Bun, TypeScript, Astro, Vue, Elysia, Zod, PostgreSQL, Docker Compose, and Caddy.
- [x] Record that the browser becomes the primary product surface for v2.
- [x] Record that `bore-admin` and `tui/` are compatibility or secondary lanes unless later proven otherwise.
- [x] Record that relay-first delivery is acceptable before direct transport parity is revisited.

### Exit criteria

- [x] A future agent can answer "what ships today?" without reading code first.
- [x] A future agent can answer "what is Bore v2 supposed to become?" without guessing.
- [x] Stable docs no longer present the Go backend as the long-term architectural destination.

### Verification

- [x] Review `README.md`, `ARCHITECTURE.md`, and `BUILD.md` for consistent v1 versus v2 framing.
- [x] Confirm repo search no longer treats the old BUILD framing as current direction.
- [x] Confirm this file names the rewrite target stack and product stance explicitly.

## Phase 1 - Stand up the v2 workspace and runtime skeleton

### Objectives

- [x] Create a real repo-root landing zone for the rewrite.
- [x] Prove that Bore can boot as a Bun workspace with Caddy, API, web, and PostgreSQL services.
- [x] Establish shared contracts and basic health/readiness wiring before feature work.

### Checklist

- [x] Add repo-root Bun workspace configuration.
- [x] Add `apps/api` Elysia service scaffolding.
- [x] Add `apps/web` Astro + Vue app scaffolding.
- [x] Add `packages/contracts` with initial shared Zod schemas.
- [x] Add `infra/caddy/Caddyfile`.
- [x] Add `.env.example` and environment loading guidance.
- [x] Add `docker-compose.yml` for `caddy`, `api`, `web`, and `postgres`.
- [x] Add typed health and readiness endpoints for the v2 API lane.
- [x] Add root workspace `lint`, `check`, `test`, `build`, and `verify` scripts.

### Exit criteria

- [x] The repo has one obvious v2 workspace rooted at `apps/`, `packages/`, and `infra/`.
- [x] The web shell loads through Caddy instead of bypassing the intended runtime shape.
- [x] Frontend and backend compile inside one Bun workspace.
- [x] The v2 lane is no longer doc-only.

### Verification

- [x] `bun run lint`
- [x] `bun run check`
- [x] `bun run test`
- [x] `bun run build`
- [x] `docker compose up -d --build`

## Phase 2 - Build backend foundation in Elysia

### Objectives

- [x] Move session coordination and durable metadata ownership into Elysia.
- [x] Replace Go-owned in-memory-only session truth for the v2 lane.
- [x] Make PostgreSQL-backed session recovery real.

### Checklist

- [x] Add SQL migrations for transfer sessions, participants, file metadata, and event history.
- [x] Implement session create, join, read, and operator summary endpoints.
- [ ] Add typed realtime envelopes for coordination events.
- [ ] Add structured logs, rate limits, request-size limits, and timeouts.
- [x] Keep all externally consumed payloads defined in shared Zod schemas.

### Exit criteria

- [x] Elysia can create, read, and advance transfer sessions against PostgreSQL.
- [x] Operator-visible status no longer depends on Go in-memory room state for the v2 path.
- [x] Restarting the v2 stack preserves session metadata needed for recovery and audit history.

### Verification

- [x] API unit tests for handlers and validation failures.
- [x] Integration tests against PostgreSQL in Docker Compose.
- [x] Migration up and reset smoke checks.
- [ ] Realtime contract tests for coordination message envelopes.

## Phase 3 - Build the web application foundation

### Objectives

- [ ] Make the browser flows usable before transfer streaming lands.
- [ ] Keep Astro in charge of route ownership and shell composition.
- [ ] Ensure the UI consumes typed contracts instead of ad hoc JSON.

### Checklist

- [x] Build Astro routes for `/`, `/send`, `/receive/[code]`, and `/ops`.
- [ ] Add Vue composables or state only where session creation and join flows genuinely need it.
- [ ] Add a shared typed client over `packages/contracts`.
- [ ] Add browser-visible error, loading, and validation states backed by shared schemas.
- [ ] Render operator pages from v2 APIs instead of the Go `/status` contract.

### Exit criteria

- [ ] A sender can create a session from the v2 web UI.
- [ ] A receiver can open a join route and attach to that session shell.
- [ ] Operator pages render live session data from the v2 API lane.
- [ ] No primary v2 page depends on untyped payload parsing.

### Verification

- [ ] Component and composable tests with Bun.
- [x] `astro check` passes for the v2 web app.
- [ ] Browser smoke coverage for create-session and join-session shells.
- [ ] Responsive checks for mobile and desktop widths.

## Phase 4 - Ship a relay-first secure transfer MVP

### Objectives

- [ ] Move real encrypted file transfer through the v2 stack.
- [ ] Preserve Bore's security posture while accepting a relay-first launch stance.
- [ ] Make the operator experience observe v2 sessions directly.

### Checklist

- [ ] Add TypeScript transfer and crypto primitives, likely in `packages/crypto`.
- [ ] Implement relay-session streaming through Elysia.
- [ ] Build browser send and receive flows for single-file transfer.
- [ ] Surface progress events to both participant UIs and operator pages.
- [ ] Persist lifecycle and failure events to PostgreSQL.
- [ ] Keep the server blind to plaintext file contents.

### Exit criteria

- [ ] A sender can create a code, attach a file, and complete a transfer in the v2 web UI.
- [ ] A receiver can join by code and receive that file in the v2 web UI.
- [ ] Operator pages show active sessions, completions, and failures from v2 data.
- [ ] The Compose stack runs locally without hidden manual steps.

### Verification

- [ ] End-to-end relay transfer smoke test in Docker Compose.
- [ ] Integrity verification on completed transfer.
- [ ] Failure-path tests for expired session, duplicate join, and interrupted stream.
- [ ] Caddy reverse-proxy smoke checks for API and realtime paths.

## Phase 5 - Recover the parity features that still matter

### Objectives

- [ ] Bring back the v1 features Bore still needs after the architectural cutover.
- [ ] Refuse automatic parity work for features that no longer fit the product direction.
- [ ] Improve operator history and recovery posture in the new stack.

### Checklist

- [ ] Implement resumable transfers or document an explicit temporary replacement policy.
- [ ] Add richer operator history and filtering over PostgreSQL-backed event data.
- [ ] Decide whether `bore-admin` stays as a compatibility helper.
- [ ] Decide whether `tui/` remains useful after the web operator path reaches parity.
- [ ] Update docs, runbooks, and examples toward the v2 API and UI.

### Exit criteria

- [ ] Every meaningful v1 feature is either implemented, intentionally deferred with a reason, or explicitly dropped.
- [ ] Primary operator workflows no longer depend on the Go `/status` endpoint.
- [ ] The v2 runtime has a clear backup, restore, and migration story for PostgreSQL.

### Verification

- [ ] Resume regression suite or explicit non-resume behavior coverage.
- [ ] Operator workflow smoke tests against the v2 stack.
- [ ] Restore test from PostgreSQL backup into a fresh Compose stack.

## Phase 6 - Decide direct transport and fallback strategy

### Objectives

- [ ] Resolve the hardest v1 capability honestly instead of implying it will appear automatically.
- [ ] Either implement browser-credible direct transport deliberately or narrow the v2 promise explicitly.

### Checklist

- [ ] Decide whether browser-accessible direct peer transfer remains part of Bore v2.
- [ ] If yes, implement it deliberately with explicit fallback behavior.
- [ ] If no, document v2 as relay-first and keep any v1 direct path under a clearly labeled legacy policy.
- [ ] Surface transport selection and failure reasons in operator-visible history.

### Exit criteria

- [ ] Direct mode either works end to end with explicit fallback behavior or is explicitly removed from the v2 promise.
- [ ] No ambiguous half-implemented direct transport path remains in the shipped product story.

### Verification

- [ ] NAT-matrix or equivalent direct-transport integration testing to the extent practical.
- [ ] Transport-reporting tests for direct versus relay outcomes.
- [ ] Documentation review to confirm shipped behavior matches the stated promise.

## Phase 7 - Cut over and retire v1 as primary

### Objectives

- [ ] Finish the rewrite instead of leaving Bore in permanent dual-stack limbo.
- [ ] Make the TypeScript full-stack app the obvious default codepath.

### Checklist

- [ ] Update `README.md` and architecture docs to point to v2 as the primary product.
- [ ] Make Caddy + Elysia + PostgreSQL the documented default deploy path.
- [ ] Move Go binaries to an explicit legacy lane or remove them from the primary runtime story.
- [ ] Clearly label any surviving CLI or TUI path as secondary.
- [ ] Collapse stale v1-future wording out of stable docs.

### Exit criteria

- [ ] A new contributor immediately sees the Bun workspace as the main codepath.
- [ ] Stable docs describe one current architecture instead of a rewrite in progress.
- [ ] The repo no longer needs a build-phase tracker to explain what Bore is becoming.

### Verification

- [ ] Fresh-clone bootstrap from the final README.
- [ ] Full-stack smoke test from Docker Compose.
- [ ] Release artifact check for the shipped v2 runtime.

## Cross-phase verification gates

- [x] Completed phases update code, docs, env examples, and this file together.
- [x] Completed phases leave the repo with a current root verification surface.
- [ ] Pending phases must add targeted tests for their new boundaries instead of relying on manual hope.
- [ ] No phase is complete until its exit criteria and verification boxes are both true.

## Definition of done for any completed phase

- [x] Work items are reflected in repo reality, not just in chat.
- [x] Exit criteria are satisfied for the completed phase.
- [x] Verification expectations are checked off honestly.
- [x] `BUILD.md` reflects the current truth at the same time as the repo change.

## When to retire this file

- [ ] Retire `BUILD.md` only after Bore is no longer in an active rewrite program.
- [ ] Move enduring current-state guidance into stable docs such as `README.md`, `ARCHITECTURE.md`, and runbooks before removing this file.
- [ ] Delete the file only when the repo documents one current architecture and no longer needs a phase tracker.