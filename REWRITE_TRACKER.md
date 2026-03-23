# bore Rewrite Tracker

_Last updated: 2026-03-23_

This file is the handoff document for bore's current implementation lane.

## Executive Summary

bore now lands on a **Go / Zig / C only** repo story:

- **Go now** for the working client, relay, NAT tooling, and service logic
- **Zig later** only for operator-facing local UX or native tooling if it earns its keep
- **C only when justified** for leaf dependencies, FFI, or low-level portability work
- **Rust removed from `main`**; historical reference lives in git history only

## What This Cleanup Pass Completed

### Legacy Rust removal

Removed from the tracked tree:

- top-level `Cargo.toml`
- top-level `Cargo.lock`
- `crates/bore-cli/`
- `crates/bore-core/`

That removal also eliminates the old crate layout, Rust source files, and the last in-tree frozen-reference framing.

### Repo-truth doc rewrite

Updated to match the current repo:

- `README.md`
- `BUILD.md`
- `REWRITE_TRACKER.md`
- `ARCHITECTURE.md`
- `SECURITY.md`
- `docs/crypto-design.md`
- `docs/threat-model.md`
- `.gitignore`
- `client/internal/code/code.go` comment cleanup

## Repository Truth

### Active code that matters

- `client/` — **Go client**, including:
  - rendezvous code generation/parsing
  - Noise `XXpsk0` handshake
  - secure encrypted channel framing
  - file transfer engine
  - WebSocket relay transport
  - CLI with `send`, `receive`, `status`, and `components`
- `services/relay/` — Go WebSocket relay
- `lib/punchthrough/` — Go NAT discovery and hole-punching library/CLI
- `services/bore-admin/` — Go scaffold for future relay operations tooling

### Current status by component

| Component | Status | Truth |
|---|---|---|
| `client/` | active, functional | Working relay-based send/receive path in Go |
| `services/relay/` | active, functional | WebSocket room broker with tests |
| `lib/punchthrough/` | active, partial | NAT probing and UDP hole-punching primitives exist, not yet integrated into the client flow |
| `services/bore-admin/` | scaffold | Placeholder only |

## Verification For This Cleanup Pass

Focused verification for the Rust-removal + doc-truth pass:

```bash
cd client && go test ./... && go build ./cmd/bore
cd services/relay && go test ./... && go build ./cmd/relay
cd lib/punchthrough && go test ./... && go build ./cmd/punchthrough
cd services/bore-admin && go build ./cmd/bore-admin
git ls-files | rg '\\.(rs)$|(^|/)Cargo\\.toml$|(^|/)Cargo\\.lock$|rust-toolchain'
```

What this proves:

- the active Go modules still build/test cleanly after Rust removal
- the admin scaffold still builds
- tracked Rust source and Cargo files are gone from `main`

## Remaining TODOs

Ordered by leverage:

1. integrate `lib/punchthrough/` into the client transport flow
2. add resumable transfer state
3. harden relay operations:
   - rate limiting
   - health endpoint
   - metrics endpoint
   - resource controls
4. promote `services/bore-admin/` beyond scaffold
5. decide whether any future Zig layer is genuinely warranted

## Open Risks / Known Gaps

- **Direct P2P is not wired into the client flow yet.** Current working path is relay-based.
- **Resumable transfers are not implemented yet.**
- **bore-admin is still a placeholder.**
- **Relay hardening is incomplete.** No rate limiting or operator metrics yet.
- **Security posture is real but not externally audited.** Current claims should stay scoped to the implemented Go relay-based path.

## Resume Here Next Time

If another agent has to pick this up, do this in order:

1. read `README.md`, `BUILD.md`, and this file
2. treat `client/` as the active client implementation
3. do **not** reintroduce legacy Rust source to keep context around
4. decide whether the next chunk is:
   - direct transport integration, or
   - resumable transfer support, or
   - relay hardening, or
   - bore-admin implementation
5. run the narrowest relevant verification before changing docs again

## Short File Map

```text
client/                  active Go client
services/relay/          active Go relay
lib/punchthrough/        active Go NAT tooling
services/bore-admin/     future ops surface / current scaffold
docs/                    crypto and threat-model docs
README.md                public current-state overview
BUILD.md                 execution manual
REWRITE_TRACKER.md       rewrite handoff / resume doc
```
