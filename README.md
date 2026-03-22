# bore

bore is a Rust workspace for a privacy-first file transfer tool. It is still a scaffold, not a working transfer system.

## Current state

- `bore-cli` prints the project status and component list
- `bore-core` holds the shared project snapshot
- encryption, relay transport, rendezvous, resumable sessions, and real transfers are not built yet

## Quick start

```bash
cargo check
cargo run -p bore-cli
cargo run -p bore-cli -- components
```
