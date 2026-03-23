# bore Rewrite Tracker

_Last updated: 2026-03-23_

This file is the handoff document for bore's active rewrite lane.

## Executive Summary

bore is moving off the legacy Rust client onto a Zig / Go / C trajectory:

- **Go now** for the working client rewrite and all network services
- **Zig later** for operator-facing local UX or native tooling where it earns its keep
- **C only when justified** for leaf dependencies, FFI, or low-level portability work
- **No new Rust work** in this repo's target architecture

### What was rescued in this recovery pass

- Rescued the previously interrupted, untracked **Go client module** in `client/`
- Rescued and replaced the stale placeholder rewrite notes with this tracker
- Inspected leftover agent/worktree artifacts under `.claude/`
  - `.claude/worktrees/inspiring-mahavira/` was just a clean legacy Rust snapshot
  - `.claude/worktrees/peaceful-spence/` contained no useful implementation work
  - No unique code needed to be copied out of those worktrees
- Added `.claude/` and `.claire/` to `.gitignore` so future local agent debris does not keep the repo dirty

## Repository Truth

### Active code that matters

- `client/` — **Go client rewrite**, including:
  - rendezvous code generation/parsing
  - Noise XXpsk0 handshake
  - secure encrypted channel framing
  - file transfer engine
  - WebSocket relay transport
  - end-user CLI with `send`, `receive`, `status`, and `components`
- `services/relay/` — Go WebSocket relay
- `lib/punchthrough/` — Go NAT discovery and hole-punching library/CLI
- `services/bore-admin/` — Go scaffold for relay operations tooling

### Legacy code still present

- `crates/bore-core/`
- `crates/bore-cli/`
- `Cargo.toml`
- `Cargo.lock`

These Rust artifacts are now **reference material only**. They are kept temporarily so parity and migration cleanup can be verified without losing protocol history. They should not receive feature work.

## Current Status By Component

| Component | Status | Truth |
|---|---|---|
| `client/` | active, functional | Working relay-based send/receive path in Go |
| `services/relay/` | active, functional | WebSocket room broker with tests |
| `lib/punchthrough/` | active, partial | NAT probing and UDP hole-punching primitives exist, not yet integrated into the client flow |
| `services/bore-admin/` | scaffold | Placeholder only |
| legacy Rust crates | frozen | Compatibility/reference only; not the target architecture |

## Recovery Checks Completed

### 1. Working tree inspection

Checked:

- `git status`
- `client/` untracked module contents
- `.claude/` worktree leftovers
- root docs (`README.md`, `BUILD.md`)

Result:

- The only valuable unreconciled rewrite work was the Go client under `client/`
- The `.claude/` worktrees did not contain newer or divergent rewrite code
- Existing public docs were still describing the old Rust-first world and needed replacement

### 2. Rewrite code audit (`client/`)

Implemented and present:

- `client/cmd/bore/main.go`
- `client/internal/code/`
- `client/internal/crypto/`
- `client/internal/engine/`
- `client/internal/transport/`
- `client/internal/rendezvous/`
- `client/go.mod`
- `client/go.sum`

### 3. Verification completed in this pass

Checked on 2026-03-23:

```bash
cd client && go test ./...
cd client && go build ./cmd/bore
```

Additional smoke verification completed against the **real Go relay**:

- started `services/relay/cmd/relay` on `127.0.0.1:18080`
- ran `client/bore send <file> --relay http://127.0.0.1:18080`
- captured the generated full rendezvous code
- ran `client/bore receive <code> --relay http://127.0.0.1:18080`
- verified sender/receiver SHA-256 matched and output file matched the input file byte-for-byte

## Changes Made In This Pass

### Code / tests

- Fixed the broken crypto tests that could deadlock on mismatched handshakes
- Added a real relay-style rendezvous integration test in `client/internal/rendezvous/rendezvous_test.go`
- Fixed CLI flag handling so these now both work:
  - `bore send ./file --relay http://host:port`
  - `bore send --relay http://host:port ./file`
- Aligned `client/go.mod` with the repo's Go toolchain version

### Repo hygiene

- Updated `.gitignore` to ignore:
  - `client/bore`
  - `.claude/`
  - `.claire/`

### Documentation

- Rewrote `README.md` to describe the current rewrite state truthfully
- Rewrote `BUILD.md` as the current execution manual for the Zig / Go / C lane
- Replaced the stale checklist tracker with this file

## Immediate Next Moves

Ordered by leverage:

1. **Decide whether the Go client is now the permanent shipped CLI**
   - If yes, remove the legacy Rust crates and Cargo files in one cleanup pass
   - If no, define the boundary between Go client core and future Zig-facing operator UX
2. **Integrate `lib/punchthrough/` into the client flow**
   - direct-first transport selection
   - relay fallback when direct path fails
3. **Add resumable transfer state**
   - sender/receiver checkpoints
   - interrupted transfer recovery
4. **Harden relay operations**
   - rate limiting
   - idle timeouts
   - health endpoints / metrics
5. **Promote bore-admin beyond scaffold**
   - relay health polling
   - storage
   - operator surface

## Recommended Cleanup Gate For Removing Rust

Only remove `crates/` + Cargo files after all of the following are true:

- [x] Go client builds locally
- [x] Go client tests pass
- [x] Go client can send/receive through the real Go relay
- [ ] Direct transport strategy is settled
- [ ] README / BUILD / ARCHITECTURE / SECURITY all stop treating Rust as active architecture
- [ ] Stephen is comfortable dropping the Rust reference implementation

## Open Risks / Known Gaps

- **Direct P2P is not wired into the client flow yet.** Current working path is relay-based.
- **Resumable transfers are not implemented yet.**
- **bore-admin is still a placeholder.**
- **ARCHITECTURE.md and SECURITY.md still contain legacy framing and need a rewrite pass** to fully match the current direction.
- **Legacy Rust remains in-tree**, which is useful for comparison but still creates conceptual drag.

## Resume Here Next Time

If another agent has to pick this up, do this in order:

1. Read `README.md`, `BUILD.md`, and this file
2. Treat `client/` as the active client implementation
3. Do **not** add new Rust feature work
4. Decide whether the next chunk is:
   - legacy Rust removal, or
   - direct transport integration, or
   - resumable transfer support
5. Run the narrowest relevant verification before changing docs again

## Short File Map

```text
client/                  active Go client rewrite
services/relay/          active Go relay
lib/punchthrough/        active Go NAT tooling
services/bore-admin/     future ops surface
crates/                  legacy Rust reference only
README.md                public current-state overview
BUILD.md                 execution manual
REWRITE_TRACKER.md       rewrite handoff / resume doc
```
