# bore

`bore` is a Rust-first file transfer project aimed at privacy-respecting, human-friendly exchange.

## Current status

**Phase 0 / scaffold only.**

This repository currently contains:
- a Rust workspace
- a tiny `bore-core` library crate
- a tiny `bore` CLI binary crate
- repository docs that define the shape of the project and the next build phases

This repository does **not** yet implement:
- end-to-end encryption
- peer-to-peer transfer
- relay transport
- rendezvous code exchange
- resumable sessions
- protocol compatibility guarantees

The current goal is to establish a truthful base that can grow into a serious operator-grade tool without pretending the hard parts are already done.

## Project intent

`bore` is meant to become a local-first, privacy-first file transfer system with:
- human-friendly transfer codes
- direct transfer when possible
- relay-assisted delivery when necessary
- a clean Rust core that can support multiple frontends or operating modes over time

Think: practical encrypted transfer with clean UX and room to grow into Wormhole-like capabilities, but built honestly from first principles.

## Phase 0 goals

- establish repo structure
- define the architectural direction
- create a minimal CLI entry point
- create a shared core crate
- document what is in and out of scope right now
- make the scaffold compile cleanly

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

## Quick start

### Prerequisites

- stable Rust toolchain with Cargo

### Check the scaffold

```bash
cargo check
cargo run -p bore-cli
cargo run -p bore-cli -- components
```

## Current CLI behavior

The CLI currently reports project status and planned component state. That is intentional: it gives the repo a real entry point without lying about implemented transfer behavior.

## Near-term architecture

Planned growth path:
- `bore-core`: protocol framing, state machines, transfer metadata, trust boundaries
- `bore-cli`: operator-facing CLI and local UX
- `bore-relay`: optional relay service crate once protocol and trust model are clear

## Security note

This scaffold should be treated as **non-secure and non-functional for real file transfer**. Until protocol, cryptography, identity model, and testing exist, `bore` is a design scaffold, not a secure tool.

## Build discipline

`BUILD.md` is the execution manual for this repository. Treat it as the source of truth for phase tracking, decisions, and implementation sequencing.

## License

MIT
