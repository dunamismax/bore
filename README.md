# bore

**Privacy-first file transfer. No accounts, no cloud, no trust required.**

bore is a command-line tool for transferring files between two computers. The sender generates a short, human-readable code. The receiver enters it. Files move directly between machines when possible, through an encrypted relay when not. The relay learns nothing about the content.

bore is not a file sharing service. It is a transfer tool — ephemeral, encrypted, peer-authenticated, and zero-knowledge by design.

> **Status: Phase 0 — scaffold only.** The workspace, CLI skeleton, and project docs exist. The transfer engine, crypto layer, and relay service are not yet implemented. See [BUILD.md](BUILD.md) for the full execution plan.

## Why bore?

Existing tools make tradeoffs that bore refuses:

| Tool | Account required | E2E encrypted | Direct P2P | Self-hostable relay | Resumable |
|------|:---:|:---:|:---:|:---:|:---:|
| Email/Slack attachment | Yes | No | No | No | No |
| WeTransfer/Dropbox | Yes | No | No | No | Partial |
| scp/rsync | No | Transport only | Yes | N/A | Yes |
| Magic Wormhole | No | Yes | Partial | Yes | No |
| croc | No | Yes | Partial | Yes | Yes |
| **bore** (goal) | **No** | **Yes** | **Yes** | **Yes** | **Yes** |

bore's design targets:

- **Zero accounts.** No signup, no login, no API key. Generate a code, share it, transfer.
- **End-to-end encryption.** The relay cannot read your files. Period.
- **Direct when possible.** LAN transfers never leave the network. WAN transfers attempt hole-punching before falling back to relay.
- **Human-friendly codes.** Short wordlist codes (e.g., `7-guitar-castle-moon`) that are easy to read aloud, type on a phone, or paste in chat.
- **Resumable.** Large transfers survive network interruptions without starting over.
- **Self-hostable relay.** Run your own relay for organizational or compliance requirements. Or use the public one.
- **Library-first.** `bore-core` is embeddable — build your own frontend, integrate into your own tools, wrap it in a GUI.

## Planned usage

```bash
# sender
bore send ./photos/
# => Code: 7-guitar-castle-moon
# => Waiting for receiver...

# receiver (on another machine)
bore receive 7-guitar-castle-moon
# => Connected to sender
# => Receiving: photos/ (3 files, 48 MB)
# => [=================>          ] 67% 12.4 MB/s
```

```bash
# send a single file
bore send report.pdf

# send to a specific relay
bore send --relay wss://relay.example.com ./data/

# resume an interrupted transfer
bore receive 7-guitar-castle-moon --resume

# check transfer history
bore history

# run your own relay
bore relay --port 8080
```

This is the target experience, not the current implementation.

## Architecture

```text
bore-core          bore-cli          bore-relay
   |                  |                  |
   |  transfer model  |  operator CLI    |  relay service
   |  session state   |  send/receive    |  room management
   |  crypto layer    |  progress UI     |  zero-knowledge store
   |  protocol codec  |  history         |  rate limiting
   |  transport trait  |  config          |  deployment config
   |                  |                  |
   +------ shared types + protocol ------+
```

- **`bore-core`** — the engine. Transfer model, session state machine, cryptographic layer, protocol codec, transport abstraction. Designed to be embedded by any frontend.
- **`bore-cli`** — the operator interface. Send, receive, history, relay management. Thin shell over `bore-core`.
- **`bore-relay`** — the optional relay server. Forwards encrypted traffic between peers that can't connect directly. Learns nothing about content. Self-hostable.

## Building from source

### Prerequisites

- Rust 1.85+ (stable)

### Build

```bash
git clone https://github.com/dunamismax/bore.git
cd bore
cargo build --release
```

### Current commands (Phase 0)

```bash
cargo run -p bore-cli                # project status (default)
cargo run -p bore-cli -- status      # project status
cargo run -p bore-cli -- components  # component map
```

### Quality checks

```bash
cargo check
cargo test
cargo fmt --check
cargo clippy --workspace --all-targets -- -D warnings
```

## Repository layout

```text
.
├── BUILD.md              # execution manual — phases, decisions, progress
├── ARCHITECTURE.md       # technical design and protocol notes
├── SECURITY.md           # threat model and security policy
├── Cargo.toml            # workspace root
├── LICENSE               # MIT
└── crates/
    ├── bore-core/        # shared library — transfer engine
    │   └── src/lib.rs
    └── bore-cli/         # binary — operator CLI
        └── src/main.rs
```

## Roadmap

| Phase | Name | Status |
|-------|------|--------|
| 0 | Truthful scaffold | **Done** |
| 1 | Protocol design and type foundations | Planned |
| 2 | Cryptographic layer | Planned |
| 3 | Local transfer engine | Planned |
| 4 | Rendezvous and code exchange | Planned |
| 5 | Direct peer-to-peer transport | Planned |
| 6 | Relay service | Planned |
| 7 | Resumable transfers and persistence | Planned |
| 8 | Hardening and security audit | Planned |
| 9 | Cross-platform polish and distribution | Planned |
| 10 | Ecosystem — library bindings, GUI, integrations | Planned |

See [BUILD.md](BUILD.md) for the full phase breakdown with tasks, exit criteria, and decisions.

## Design principles

1. **Truth over theater.** Never claim a security property that isn't proven. The CLI, docs, and code must agree.
2. **Privacy by default.** End-to-end encryption is not optional. The relay is zero-knowledge. No telemetry, no analytics, no tracking.
3. **Direct first.** Attempt peer-to-peer before relay. LAN transfers should be fast and never touch the internet.
4. **Operator control.** The user decides what relay to use, what to trust, and what to persist. No hidden magic.
5. **Composable core.** `bore-core` is a library. The CLI is one consumer. Others should be possible.

## Security

bore makes no security claims today. The cryptographic protocol, threat model, and relay trust boundaries are being designed. See [SECURITY.md](SECURITY.md) for the planned security posture and responsible disclosure policy.

## Contributing

bore is in early design. Contributions are welcome once the protocol and type foundations stabilize (Phase 1+). In the meantime, design feedback and threat model review are valuable — open an issue.

## License

MIT — see [LICENSE](LICENSE).
