# BUILD.md

This file tracks bore's frontend migration status and remaining work.

`README.md`, `ARCHITECTURE.md`, and `docs/status-contract.md` are the current implementation truth. The checkboxes below reflect what is true in the repo today.

## Decision

**Target shape: TUI-primary plus web companion.**

- Keep **Go** as the only backend and runtime owner.
- Replace the shipped Python web frontend with **TypeScript + Bun + Astro + Vue**.
- Add an **OpenTUI + TypeScript + Bun** operator console.
- Keep `bore send` and `bore receive` as plain Go CLI commands unless a later phase proves a TUI wrapper is materially better.

### Why this is the right shape

- bore is already terminal-first. The product core is `cmd/bore`, not the browser.
- `bore-admin` proves there is real operator value in a terminal surface.
- The browser surface matters, but it is still a companion: homepage plus read-only relay status.
- A browser-only migration would miss the repo's existing terminal and operator reality.
- A TUI makes sense for live relay monitoring and operator workflows. It does not make sense to force a TUI onto every file-transfer command on day one.

## Current state

- One root **Go module** owns transport, crypto, relay, NAT traversal, packaging, and CLIs.
- `web/` is the shipped **TypeScript + Bun + Astro + Vue** browser surface.
- `cmd/relay` serves the built `web/dist` output same-origin at `/` and `/ops/relay`, with an explicit fallback page when web assets are missing.
- `tui/` is now the primary terminal operator surface for relay observability.
- `bore-admin` remains as a smaller compatibility shim.
- The relay already exposes the operator data frontends need: `/status`, `/healthz`, `/metrics`.
- The `/status` contract is documented in `docs/status-contract.md` and covered by Go tests.
- There is **no durable service database** today:
  - relay state is in memory
  - receiver resume state is local JSON on disk
- `tui/` now exists as an **OpenTUI + TypeScript + Bun** operator console over the relay's Go-owned `/status` endpoint.
- CI validates `web/` and `tui/` with Bun-based lint, type-check, and test jobs.

## Target state

### Backend

- **Go stays the backend.** This repo is networking, relay, systems, and long-running runtime work.
- Go continues to own:
  - relay APIs and status payloads
  - transport and crypto logic
  - packaging and deployment shape
  - any future persistence, if it is earned

### Web frontend

- The browser surface lives at **`web/`**.
- Astro owns routes, page shells, static content, and delivery.
- Vue owns the live relay-status island and any future interactive operator widgets.
- The web surface stays aligned with current product truth:
  - homepage at `/`
  - read-only relay operator page at `/ops/relay`
- Prefer **static Astro output** plus a boring same-origin serving story from Go.
- Do **not** add a permanent Bun SSR runtime unless a later requirement clearly earns it.

### Terminal frontend

- Add an **OpenTUI + Bun** app at **`tui/`**.
- The first job of the TUI is **operator observability**, not replacing the core file-transfer CLI.
- The TUI is the intended successor to `bore-admin`, not to `bore send` and `bore receive` in the first TUI phase.

## Recommended repo shape after migration

```text
.
├── cmd/
├── internal/
├── web/
├── tui/
├── docs/
├── README.md
├── ARCHITECTURE.md
└── BUILD.md
```

## Backend notes

- Backend choice is **Go**, not Python.
- Frontends must consume **Go-owned contracts**. Start with the existing relay endpoints:
  - `/status`
  - `/healthz`
  - `/metrics`
- If the frontends need richer data, extend the Go endpoints deliberately.
- Do not move product logic into Bun apps.
- Keep JSON payload shape locked with tests as the UI layers evolve.

## Data and runtime constraints

- Do **not** add PostgreSQL for this migration.
- Do **not** invent a service database just to support the frontend rewrite.
- Keep relay state in memory unless a real history or write workflow demands persistence.
- If local persistence is later earned, prefer **SQLite in Go** with plain SQL migrations.
- Resume state stays local filesystem state owned by the Go client.
- The browser surface stays **read-only** unless auth, writes, and persistence are designed explicitly.
- Preserve the current security posture:
  - same-origin or otherwise explicit fetch boundaries
  - restrictive browser headers
  - payload-blind relay
  - no hidden control plane

## Status legend

- `[x]` means the repo already matches that step today.
- `[ ]` means the step is still open.
- When a box is unchecked, nested notes call out any partial progress that already landed.

## Phase plan

- [x] **Phase 0 - Freeze the frontend contract**
  - Goal: make the migration boring.
  - [x] Choose `web/` as the shipped web root and `tui/` as the planned terminal root.
    - `web/` and `tui/` now both exist as the active frontend lanes for this migration path.
  - [x] Inventory every `/status` field used by the legacy Python frontend and `bore-admin`.
    - `docs/status-contract.md` records the field inventory and consumer matrix.
  - [x] Document the Go-owned frontend contract in repo docs or tests.
    - Current sources: `docs/status-contract.md`, `internal/relay/status/status.go`, and `TestRelay_StatusJSONContractShape`.
  - [x] Add or tighten tests around relay status payload stability before the first UI rewrite.
  - [x] Decide exactly how Astro output will be served in production.
    - Current shipped path: `cmd/relay` serves built `web/dist` same-origin and falls back to an explicit build-missing page when needed.
  - [x] Exit criterion: future agents can point at one documented frontend contract.
  - [x] Exit criterion: frontend directory layout is fixed for the migration path.
  - [x] Exit criterion: the serving story for Astro output is explicit.

- [x] **Phase 1 - Migrate the web surface to Astro + Vue**
  - Goal: remove the Python web stack from the shipped browser path without changing product truth.
  - [x] Recreate `/` in Astro.
  - [x] Recreate `/ops/relay` in Astro with a small Vue island for live status refresh.
  - [x] Keep the web surface read-only.
  - [x] Keep current routing and product messaging aligned with repo docs.
  - [x] Remove htmx, Jinja2, Tailwind CDN coupling, and the Python frontend from the active runtime path.
    - `frontend/` cleanup and CI cutover landed in Phase 3.
  - [x] Exit criterion: `/` and `/ops/relay` run on Astro + Vue with parity or better clarity.
  - [x] Exit criterion: the web frontend reads relay data cleanly from the Go backend.
  - [x] Exit criterion: the shipped browser surface no longer depends on Python.

- [x] **Phase 2 - Add the OpenTUI operator console**
  - Goal: give bore a real terminal operator surface instead of a thin polling CLI.
  - [x] Build an OpenTUI app focused on relay observability first.
  - [x] Cover current `bore-admin status` scope plus live refresh, room gauges, direct vs relay counts, and clear failure states.
  - [x] Keep the boundary boring: HTTP to relay endpoints or a thin local wrapper around them.
  - [x] Keep `bore-admin` as a compatibility shim until the TUI proves stable.
    - `bore-admin` stays intentionally smaller and points operators at `tui/` for the richer live surface.
  - [x] Exit criterion: there is a usable TUI for relay operators.
  - [x] Exit criterion: `bore-admin` is either a thin wrapper into the TUI or clearly marked for deprecation.
  - [x] Exit criterion: the TUI owns presentation, not backend logic.

- [x] **Phase 3 - Consolidate packaging, CI, and docs**
  - Goal: finish the stack cutover cleanly.
  - [x] Remove `frontend/` and its Python tooling once the legacy reference is no longer needed.
  - [x] Update CI to check `web/` and `tui/`.
  - [x] Update README, ARCHITECTURE, and deployment docs to reflect the final frontend lanes.
  - [x] Decide whether relay `/` serves Astro-built assets directly or redirects to a separately hosted web frontend.
    - Current decision: direct same-origin serving from `cmd/relay`.
  - [x] Remove stale commands, paths, and implementation claims in the same change.
  - [x] Exit criterion: no active runtime path depends on the Python frontend.
  - [x] Exit criterion: docs describe Go + Astro/Vue + OpenTUI accurately.
  - [x] Exit criterion: CI covers the new frontend lanes.

- [ ] **Phase 4 - Optional product-facing TUI expansion**
  - Goal: only do this if it earns itself.
  - [ ] Decide whether operator usage actually earns product-facing TUI expansion.
    - Nothing in the repo today shows that this phase has been earned yet.
  - [ ] If it is earned, scope guided receive and send workflows.
  - [ ] If it is earned, scope transfer progress dashboards.
  - [ ] If it is earned, scope local transfer history or resume inspection.
  - [ ] Exit criterion: either there is a concrete expansion plan with real operator value, or this phase is explicitly closed as unnecessary.

## Recommended execution order

- [x] Phase 0
- [x] Phase 1
- [x] Phase 2
- [x] Phase 3
- [ ] Phase 4 only if operator usage proves the need

### Why this order

- The Python web frontend was the clearest mismatch with the current stack contract.
- The web scope is small enough to migrate without touching transport or crypto logic.
- The TUI is justified, but it should land against a stable Go contract, not during backend churn.
- This order keeps current product behavior intact while the frontend lanes change.

## Risks

- overbuilding the web surface into a control plane
- forcing OpenTUI onto `bore send` and `bore receive` too early
- adding a Bun SSR runtime when static output or simple Go-side serving is enough
- letting frontend claims drift from security docs or transport reality
- breaking operator parity during the Python-to-Astro cutover
- spreading the repo across too many app roots without one obvious build and test story

## Acceptance criteria for this BUILD

This BUILD is in good shape when future agents can use it to continue the migration without guessing the repo direction.

- [x] Frontend target is unambiguous: **Go backend + TUI-primary + web companion**.
- [x] The first migration target is unambiguous: **replace the Python web frontend with Astro + Vue**.
- [x] The terminal plan is unambiguous: **add OpenTUI for operator workflows, do not replace the core transfer CLI by default**.
- [x] Backend and data constraints are explicit.
- [x] Risks and non-goals are explicit.
- [x] Phase boundaries and exit criteria are concrete.
