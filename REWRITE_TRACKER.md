# bore Tracker

_Last updated: 2026-03-23_

This file is the handoff document for bore's current implementation lane.

## Executive Summary

bore currently ships a relay-based encrypted transfer flow built around four tracked components:

- `client/` — active CLI for rendezvous, handshake, and encrypted file transfer
- `services/relay/` — active WebSocket relay
- `lib/punchthrough/` — active NAT tooling groundwork, not yet integrated into the client path
- `services/bore-admin/` — scaffold for future relay operations tooling

## What This Pass Cleaned

- updated `README.md`, `BUILD.md`, `REWRITE_TRACKER.md`, `ARCHITECTURE.md`, `SECURITY.md`, `docs/crypto-design.md`, and `docs/threat-model.md`
- removed stale migration narration and repeated cleanup commentary from active docs
- tightened security and threat-model wording so current claims stay scoped to the implemented relay-based path
- kept next-work sections focused on transport integration, resume support, relay hardening, and operator tooling

## Repository Truth

### Active code that matters

- `client/` — working relay-based send/receive path with:
  - rendezvous code generation/parsing
  - Noise `XXpsk0` handshake
  - secure encrypted channel framing
  - file transfer engine
  - WebSocket relay transport
  - CLI with `send`, `receive`, `status`, and `components`
- `services/relay/` — WebSocket room broker
- `lib/punchthrough/` — NAT discovery and hole-punching library/CLI
- `services/bore-admin/` — scaffold for future relay operations work

### Current status by component

| Component | Status | Truth |
|---|---|---|
| `client/` | active, functional | Working relay-based send/receive path |
| `services/relay/` | active, functional | WebSocket room broker with tests |
| `lib/punchthrough/` | active, partial | NAT probing and UDP hole-punching primitives exist, not yet integrated into the client flow |
| `services/bore-admin/` | scaffold | Placeholder only |

## Verification For This Pass

Focused verification:

```bash
cd client && go test ./... && go build ./cmd/bore
cd services/relay && go test ./... && go build ./cmd/relay
cd lib/punchthrough && go test ./... && go build ./cmd/punchthrough
cd services/bore-admin && go build ./cmd/bore-admin
```

What this proves:

- the active modules still build and test cleanly
- the admin scaffold still builds
- the docs are aligned with the implementation that currently ships

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
5. keep docs aligned as transport and operator work land

## Open Risks / Known Gaps

- **Direct transport is not wired into the client flow yet.** Current working path is relay-based.
- **Resumable transfers are not implemented yet.**
- **bore-admin is still a placeholder.**
- **Relay hardening is incomplete.** No rate limiting or operator metrics yet.
- **Security posture is real but not externally audited.** Current claims should stay scoped to the implemented path.

## Resume Here Next Time

If another agent has to pick this up, do this in order:

1. read `README.md`, `BUILD.md`, and this file
2. treat `client/` as the active client implementation
3. decide whether the next chunk is:
   - direct transport integration, or
   - resumable transfer support, or
   - relay hardening, or
   - bore-admin implementation
4. run the narrowest relevant verification before changing docs again

## Short File Map

```text
client/                  active client
services/relay/          active relay
lib/punchthrough/        active NAT tooling groundwork
services/bore-admin/     future ops surface / current scaffold
docs/                    crypto and threat-model docs
README.md                public current-state overview
BUILD.md                 execution manual
REWRITE_TRACKER.md       handoff / resume doc
```
