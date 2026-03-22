# bore

`bore` is a Rust workspace for a privacy-first, human-friendly file transfer tool.

Today it is still **Phase 0 / scaffold only**: the repo has a truthful CLI, a shared core crate, and the project-shaping docs, but it does **not** yet move files.

## Current state

Implemented now:
- Rust workspace scaffold
- `bore-core` shared library crate
- `bore-cli` package that builds the `bore` binary
- CLI commands that print the current project snapshot and planned component map
- repo docs that define scope, constraints, and the next build phases

Not implemented yet:
- cryptographic protocol
- direct peer-to-peer transfer
- relay transport/service
- rendezvous code exchange
- resumable sessions
- persistent transfer state
- protocol compatibility guarantees

The point of the current repo is to stay honest while the real transfer model, trust boundaries, and operator workflow are designed.

## Source of truth

- `README.md` is the public-facing project summary.
- `BUILD.md` is the execution manual: phase tracking, architecture notes, decisions, and next work.
- `crates/bore-core/src/lib.rs` is the current runtime snapshot that the CLI prints.
- `crates/bore-cli/src/main.rs` is the operator-facing seam.

If the docs, `project_snapshot()`, and CLI output ever disagree, fix them together.

## Near-term architecture

Planned shape:
- **`bore-core`**: transfer model, session state, protocol-safe types, validation, and trust boundaries
- **`bore-cli`**: operator-facing CLI and local workflow
- **future `bore-relay`**: optional relay service once protocol and trust semantics are clear

The intended growth path is direct transfer when possible, relay-assisted delivery when necessary, and a core crate that can support more than one frontend over time.

## Quick start

### Prerequisites

- stable Rust toolchain with Cargo

### Check the scaffold

```bash
cargo check
cargo run -p bore-cli
cargo run -p bore-cli -- status
cargo run -p bore-cli -- components
```

Notes:
- `cargo run -p bore-cli` defaults to `status`
- the current CLI reports project truth; it does not start a transfer session

## Repository layout

```text
.
├── BUILD.md
├── Cargo.toml
├── LICENSE
├── README.md
└── crates
    ├── bore-cli
    │   └── src/main.rs
    └── bore-core
        └── src/lib.rs
```

## Security and truthfulness

Do **not** market or treat this repository as a secure transfer tool yet. There is no implemented crypto design, transport protocol, relay contract, or interoperability story. The current value is an honest scaffold with a clean path into those harder parts.

## Build discipline

`BUILD.md` is the repo's working manual. It tracks phase/state, architecture direction, decisions, and the next correct move so the project can grow without pretending to be finished.

## License

MIT
