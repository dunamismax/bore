# BUILD.md

## Purpose

This file is the execution manual for `bore`.

It exists to keep the repo honest while the project grows from an empty idea into a real, operator-grade transfer system. It should answer four questions at all times:
- what this repo is trying to become
- what exists right now
- what the next correct move is
- what must be proven before stronger claims are made

## Mission

Build a Rust-only file transfer tool with human-friendly coordination, privacy-first defaults, and a path to relay-aware, operator-grade deployment.

The product should eventually let two parties move files with minimal ceremony and strong trust boundaries, while staying truthful about what is local, what is relayed, and what is encrypted.

## Repo snapshot

Current phase: **Phase 0**

Current state:
- workspace root is in place
- `bore-core` exists as a minimal shared library crate
- `bore-cli` exists as a minimal binary crate named `bore`
- core docs exist: `README.md`, `BUILD.md`, `LICENSE`, `.gitignore`
- `cargo check` should pass on a current stable toolchain

What does **not** exist yet:
- transfer protocol
- crypto design or implementation
- relay protocol or service
- discovery / rendezvous code generation
- file chunking, streaming, or resume logic
- persistence model
- interoperability guarantees

## Source-of-truth mapping

- `README.md`: public-facing project description and honest repo status
- `BUILD.md`: implementation map, phase tracking, decisions, and working rules
- `Cargo.toml` (root): workspace shape and shared dependency policy
- `crates/bore-core`: shared domain and protocol-adjacent logic
- `crates/bore-cli`: user-facing CLI entry point
- future `crates/bore-relay`: relay service, only after trust and protocol boundaries are defined

If docs and code disagree, the next change should make them agree immediately.

## Architecture and data flow

### Intended long-term shape

1. **CLI / local operator shell**
   - parses intent
   - gathers local file metadata
   - drives session lifecycle
   - presents codes, progress, and errors

2. **Core library**
   - owns transfer model
   - owns session state machine
   - owns capability negotiation and framing rules
   - owns protocol-safe types and validation
   - should remain usable by future desktop, service, or TUI shells

3. **Optional relay service**
   - relays traffic when direct connectivity fails or is undesirable
   - should learn as little as possible
   - should not own end-user trust decisions

### Phase-0 data flow

Current flow is intentionally tiny:
- CLI starts
- CLI reads static project metadata from `bore-core`
- CLI prints truthful status

That sounds almost insultingly small. Good. It means the repo has a real executable seam without pretending the transport layer exists.

### Planned transfer flow later

A likely shape, subject to change after design work:
- sender selects file(s)
- session intent is created locally
- rendezvous material / short code is generated
- receiver joins with code
- capability and trust checks run
- direct path is attempted first where appropriate
- relay fallback is negotiated if needed
- encrypted transfer stream begins
- integrity and completion are verified

Do not implement this from vibes. Each stage needs explicit trust, failure, and recovery semantics.

## Working rules

- Rust only for now
- keep the core crate small, explicit, and testable
- do not claim security properties before the threat model and tests exist
- prefer reversible structure over speculative complexity
- direct transfer and relay transfer must be designed as explicit modes, not hidden magic
- docs are part of the product surface; keep them current
- phase labels must stay truthful

## Tracking conventions

Use the following status language in docs, issues, and commit messages:
- **done**: implemented and checked
- **checked**: verified by command or test output
- **planned**: intentional but not started
- **blocked**: cannot proceed without a decision or dependency
- **risk**: plausible failure mode that could distort the design
- **decision**: a durable call with consequences

When new work lands, update:
- repo snapshot
- phase dashboard
- decisions if architecture changed
- progress log with date and what was actually verified

## Quality gates

Minimum gate for Phase 0 and Phase 1 work:
- `cargo fmt --check`
- `cargo clippy --workspace --all-targets -- -D warnings`
- `cargo test`
- `cargo check`

If some gate is temporarily unavailable, document why rather than silently skipping it.

## Phase dashboard

### Phase 0 — truthful scaffold
Status: **done / checked**

Goals:
- create workspace
- create core and CLI crates
- add honest docs
- pass `cargo check`

Exit criteria:
- repo structure is stable enough for real work to start
- public docs do not overclaim
- executable entry point exists

### Phase 1 — protocol and trust model
Status: **planned**

Goals:
- define actors, assets, and trust boundaries
- define session lifecycle and error model
- choose first protocol envelope and message framing
- pick initial cryptographic approach and dependency policy
- define human-friendly code/rendezvous model

Exit criteria:
- design docs exist
- core types model the session honestly
- tests exist for parsing and state transitions

### Phase 2 — local transfer engine
Status: **planned**

Goals:
- file metadata model
- chunking / streaming design
- sender / receiver roles
- integrity verification
- local happy-path transfer prototype

Exit criteria:
- transfer works in a controlled local scenario
- failures are represented cleanly
- tests cover core state transitions

### Phase 3 — connectivity strategy
Status: **planned**

Goals:
- direct path strategy
- NAT / connectivity assumptions
- relay fallback contract
- transport abstraction boundary

Exit criteria:
- direct and relay modes are explicit in code
- fallback behavior is deterministic and observable

### Phase 4 — relay implementation
Status: **planned**

Goals:
- implement relay crate/service
- constrain relay visibility and retained state
- add deployment and operator docs

Exit criteria:
- relay can support transfers without becoming the trust root
- operator responsibilities are documented

### Phase 5 — hardening and productization
Status: **planned**

Goals:
- test matrix
- resumability design
- performance profiling
- packaging and release discipline
- security review and documentation

Exit criteria:
- behavior is reproducible
- operational guidance exists
- security claims have evidence behind them

## Detailed phase plan

### Phase 0 tasks
- [x] initialize workspace
- [x] add `bore-core`
- [x] add `bore-cli`
- [x] make CLI print truthful project status
- [x] add README
- [x] add BUILD manual
- [x] add MIT license
- [x] add `.gitignore`
- [x] verify with `cargo check`

### Phase 1 tasks
- [ ] write design note for threat model and non-goals
- [ ] define `SessionId`, `TransferIntent`, `TransferRole`, and `TransportMode` in `bore-core`
- [ ] design code-entry / rendezvous UX
- [ ] choose cryptographic primitives and supporting crates
- [ ] decide whether relay is blind, metadata-aware, or mixed
- [ ] add unit tests around state transitions

### Phase 2 tasks
- [ ] define file manifest model
- [ ] define chunking strategy
- [ ] implement a local sender/receiver state machine
- [ ] prove integrity verification path
- [ ] add failure injection tests

### Phase 3 tasks
- [ ] design transport trait boundary
- [ ] prototype direct mode
- [ ] prototype relay negotiation
- [ ] define observability events and tracing fields

### Phase 4 tasks
- [ ] create `bore-relay` crate
- [ ] define relay config surface
- [ ] define operator deployment story
- [ ] add integration tests between CLI, core, and relay

### Phase 5 tasks
- [ ] benchmark large-file transfer behavior
- [ ] define resume semantics
- [ ] package releases
- [ ] document supported compatibility window
- [ ] run explicit security review before strong security marketing

## Risks

- **risk:** naming the product goal too aggressively before protocol work can push the code toward theater instead of proof
- **risk:** human-friendly codes can become a UX win and a security failure if trust semantics are vague
- **risk:** relay convenience can quietly become relay dependence if transport boundaries are not explicit early
- **risk:** overbuilding now will waste time before the transfer model is clear
- **risk:** under-documenting decisions will make later protocol work inconsistent

## Decisions

### decision-0001: start as a Rust workspace
Reason:
- keeps growth path open without forcing monolith-only structure
- supports a clean separation between CLI, core logic, and future relay work

### decision-0002: keep Phase 0 intentionally minimal
Reason:
- avoids fake-finished transfer code
- puts honesty and compile health ahead of premature implementation

### decision-0003: CLI-first entry point
Reason:
- fastest way to exercise architecture and developer workflow
- easiest operator surface for early experiments

### decision-0004: no security claims without proof
Reason:
- the project's credibility depends on disciplined truth-telling
- protocol and crypto are the hard part, so they must be earned

## Immediate next moves

Recommended order:
1. write a short protocol/threat-model design note in `docs/`
2. add first real domain types to `bore-core`
3. decide the first rendezvous-code UX and validation rules
4. add `tracing`, `thiserror`, and tests once the first stateful logic lands
5. only then consider a transport prototype

## Progress log

### 2026-03-22
- initialized repo as a Rust workspace
- added `bore-core` and `bore-cli`
- added an executable `bore` binary that prints current scaffold status
- added README, BUILD manual, MIT license, and `.gitignore`
- intended verification target: `cargo check`

Update this log only with things that actually happened.
