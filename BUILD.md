# BUILD.md

This file is the execution plan for bore's frontend migration.

`README.md` and `ARCHITECTURE.md` remain the source of current implementation truth until each phase here lands.

## Decision

**Target shape: TUI-primary plus web companion.**

- Keep **Go** as the only backend and runtime owner.
- Replace the Python web frontend with **TypeScript + Bun + Astro + Vue**.
- Add an **OpenTUI + TypeScript + Bun** operator console.
- Keep `bore send` and `bore receive` as plain Go CLI commands unless a future phase proves a TUI wrapper is materially better.

### Why this is the right shape

- bore is already terminal-first. The product core is `cmd/bore`, not the browser.
- `bore-admin` proves there is real operator value in a terminal surface.
- The browser surface matters, but today it is a companion: homepage plus read-only relay status.
- A browser-only migration would miss the repo's existing terminal/operator reality.
- A TUI makes sense for live relay monitoring and operator workflows. It does not make sense to force a TUI onto every file-transfer command on day one.

## Current state

- One root **Go module** owns transport, crypto, relay, NAT traversal, packaging, and CLIs.
- `frontend/` is a separate **Python + FastAPI + Jinja2 + htmx** process.
- The browser surface is intentionally thin:
  - `/` is a product homepage
  - `/ops/relay` is a read-only relay status page
- `bore-admin` is a minimal Go CLI that polls relay `/status`.
- The relay already exposes the operator data frontends need: `/status`, `/healthz`, `/metrics`.
- There is **no durable service database** today:
  - relay state is in memory
  - receiver resume state is local JSON on disk
- The relay root handler is a placeholder because the real browser surface lives in the separate Python process.

## Target state

### Backend

- **Go stays the backend.** This repo is networking, relay, systems, and long-running runtime work.
- Go continues to own:
  - relay APIs and status payloads
  - transport and crypto logic
  - packaging and deployment shape
  - any future persistence, if it is earned

### Web frontend

- Add a **Bun + Astro + Vue** app at **`web/`**.
- Astro owns routes, page shells, static content, and delivery.
- Vue owns the live relay-status island and any future interactive operator widgets.
- Keep the web surface aligned with current product truth:
  - homepage at `/`
  - read-only relay operator page at `/ops/relay`
- Prefer **static Astro output** plus a boring same-origin serving story from Go.
- Do **not** add a permanent Bun SSR runtime unless a later requirement clearly earns it.

### Terminal frontend

- Add an **OpenTUI + Bun** app at **`tui/`**.
- The first job of the TUI is **operator observability**, not replacing the core file-transfer CLI.
- The TUI is the intended successor to `bore-admin`, not to `bore send` and `bore receive` in phase 1.

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

Delete `frontend/` after the Astro web cutover is complete.

## Backend notes

- Backend choice is **Go**, not Python.
- Frontends must consume **Go-owned contracts**. Start with the existing relay endpoints:
  - `/status`
  - `/healthz`
  - `/metrics`
- If the frontends need richer data, extend the Go endpoints deliberately.
- Do not move product logic into Bun apps.
- Lock JSON payload shape with tests before the UI rewrite starts.

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

## Phase plan

### Phase 0 - Freeze the frontend contract

Goal: make the migration boring.

Work:
- choose `web/` and `tui/` as the permanent frontend roots
- inventory every `/status` field used by the current Python frontend and `bore-admin`
- document the Go-owned frontend contract in repo docs or tests
- add or tighten tests around relay status payload stability before the first UI rewrite
- decide exactly how Astro output will be served in production

Exit criteria:
- future agents can point at one documented frontend contract
- frontend directory layout is fixed
- the serving story for Astro output is explicit

### Phase 1 - Migrate the web surface to Astro + Vue

Goal: remove the Python web stack without changing product truth.

Work:
- recreate `/` in Astro
- recreate `/ops/relay` in Astro with a small Vue island for live status refresh
- keep the web surface read-only
- keep current routing and product messaging unless a repo-wide docs update is planned in the same change
- remove htmx, Jinja2, Tailwind CDN coupling, and the Python frontend from the active runtime path

Exit criteria:
- `/` and `/ops/relay` run on Astro + Vue with parity or better clarity
- the web frontend reads relay data cleanly from the Go backend
- the repo no longer depends on Python for the shipped browser surface

### Phase 2 - Add the OpenTUI operator console

Goal: give bore a real terminal operator surface instead of a thin polling CLI.

Work:
- build an OpenTUI app focused on relay observability first
- cover current `bore-admin status` scope plus live refresh, room gauges, direct vs relay counts, and clear failure states
- keep the boundary boring: HTTP to relay endpoints or a thin local wrapper around them
- keep `bore-admin` as a compatibility shim until the TUI proves stable

Exit criteria:
- there is a usable TUI for relay operators
- `bore-admin` is either a thin wrapper into the TUI or clearly marked for deprecation
- the TUI owns presentation, not backend logic

### Phase 3 - Consolidate packaging, CI, and docs

Goal: finish the stack cutover cleanly.

Work:
- remove `frontend/` and its Python tooling once the Astro web frontend is live
- update CI to check `web/` and `tui/`
- update README, ARCHITECTURE, and deployment docs to reflect the new lanes
- decide whether relay `/` serves Astro-built assets directly or redirects to a separately hosted web frontend
- remove stale commands, paths, and implementation claims in the same change

Exit criteria:
- no active runtime path depends on the Python frontend
- docs describe Go + Astro/Vue + OpenTUI accurately
- CI covers the new frontend lanes

### Phase 4 - Optional product-facing TUI expansion

Goal: only do this if it earns itself.

Possible scope:
- guided receive/send workflows
- transfer progress dashboards
- local transfer history or resume inspection

Rules:
- do not do this by default
- a plain CLI is still the right tool for many bore flows
- require an explicit user/operator benefit before broadening the TUI scope

Exit criteria:
- either there is a concrete expansion plan with real operator value, or this phase is closed out as unnecessary

## Recommended execution order

1. Phase 0
2. Phase 1
3. Phase 2
4. Phase 3
5. Phase 4 only if operator usage proves the need

### Why this order

- The Python web frontend is the clearest mismatch with the new stack contract.
- The web scope is small enough to migrate without touching transport or crypto logic.
- The TUI is justified, but it should land against a stable Go contract, not during backend churn.
- This order keeps current product behavior intact while the frontend lanes change.

## Risks

- overbuilding the web surface into a control plane
- forcing OpenTUI onto `bore send` and `bore receive` too early
- adding a Bun SSR runtime when static output or simple Go-side serving is enough
- letting frontend claims drift from security docs or transport reality
- breaking operator parity during the Python-to-Astro cutover
- spreading the repo across too many app roots without one obvious build/test story

## Acceptance criteria

This BUILD is complete when future agents can use it to execute the migration without guessing the repo direction.

That means:
- frontend target is unambiguous: **Go backend + TUI-primary + web companion**
- the first migration target is unambiguous: **replace the Python web frontend with Astro + Vue**
- the terminal plan is unambiguous: **add OpenTUI for operator workflows, do not replace the core transfer CLI by default**
- backend and data constraints are explicit
- risks and non-goals are explicit
- phase boundaries and exit criteria are concrete
