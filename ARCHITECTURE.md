# ARCHITECTURE.md

Technical architecture and design notes for `bore`.

This document describes the current repo architecture and the near-term shape it is growing toward. For build commands and the current execution checklist, see [BUILD.md](BUILD.md).

---

## System Overview

bore currently consists of four tracked components:

```text
┌─────────────┐                           ┌─────────────┐
│   Sender    │                           │  Receiver   │
│ bore client │                           │ bore client │
│   (Go)      │                           │   (Go)      │
└──────┬──────┘                           └──────┬──────┘
       │                                         │
       │      encrypted application frames       │
       └───────────────┬─────────────────────────┘
                       │
                ┌──────▼──────┐
                │ Go relay    │
                │ (payload-   │
                │ blind)      │
                └──────┬──────┘
                       │
                ┌──────▼──────┐
                │ punchthrough│
                │ groundwork  │
                │ (future     │
                │ direct path)│
                └─────────────┘
```

1. **Client (`client/`)** generates or parses a rendezvous code, performs the Noise handshake, and streams encrypted file data.
2. **Relay (`services/relay/`)** pairs sender and receiver and forwards encrypted frames over WebSockets.
3. **Punchthrough (`lib/punchthrough/`)** contains STUN and UDP hole-punching primitives for a future direct path.
4. **bore-admin (`services/bore-admin/`)** is a minimal operator CLI that queries relay status but does not participate in transfer runtime behavior.

The current verified path is **relay-based transfer**. Direct transport is still a planned integration step, not current runtime behavior.

---

## Repository Map

```text
client/
├── cmd/bore/                 # CLI entry point
└── internal/
    ├── code/                 # rendezvous code generation/parsing
    ├── crypto/               # Noise handshake + secure channel
    ├── engine/               # transfer framing, send/receive engine
    ├── rendezvous/           # relay session orchestration
    └── transport/            # WebSocket relay transport

services/relay/
├── cmd/relay/                # relay entry point
└── internal/
    ├── room/                 # room lifecycle and registry
    └── transport/            # WebSocket server + frame forwarding

lib/punchthrough/
├── cmd/punchthrough/         # operator/dev CLI
└── pkg/
    ├── punch/                # NAT classification + punch engine
    └── stun/                 # STUN client and message handling

services/bore-admin/
└── cmd/bore-admin/           # minimal operator CLI
```

---

## Client Architecture (`client/`)

### Layering

```text
CLI
  ↓
rendezvous orchestration
  ↓
crypto + transfer engine
  ↓
transport abstraction (`io.ReadWriter`)
  ↓
WebSocket relay transport today / direct transport later
```

### Package responsibilities

#### `client/cmd/bore`

Owns:

- CLI argument parsing
- user-facing help and status output
- selecting `send`, `receive`, `status`, and `components` flows

Does not own:

- cryptographic state machine internals
- transfer framing logic
- relay room lifecycle

#### `client/internal/code`

Owns:

- human-readable rendezvous code generation
- rendezvous code parsing and validation
- entropy model for channel + word count
- formatting for the full code shown to users

Important design rule:

- the rendezvous code is not just a locator; it is also a cryptographic input to the handshake

#### `client/internal/crypto`

Owns:

- Noise `XXpsk0` handshake
- HKDF-SHA256 derivation of the PSK from the rendezvous code
- ChaCha20-Poly1305 secure channel after handshake
- frame encryption/decryption over any `io.ReadWriter`

Implementation truth today:

- protocol suite: `Noise_XXpsk0_25519_ChaChaPoly_SHA256`
- counter-based nonces
- post-handshake encrypted application frames

#### `client/internal/engine`

Owns:

- transfer header/chunk/end framing
- sender-side file streaming
- receiver-side reassembly
- SHA-256 integrity verification for the received file

Current scope:

- single file send/receive
- relay-based transport path

Not yet in this layer:

- resumable transfer bookkeeping
- directory transfer
- richer multi-transfer history

#### `client/internal/rendezvous`

Owns:

- sender/receiver session orchestration against the relay
- room creation / room join flow
- bridging transport + crypto + engine into the user flow

This is the current happy-path integration layer for the client.

#### `client/internal/transport`

Owns:

- relay WebSocket connection setup
- adapting transport IO to what the crypto layer expects

Near-term extension point:

- direct transport can later sit beside the relay transport, with selection logic above it

---

## Relay Architecture (`services/relay/`)

The relay is intentionally narrow. It should act like a room broker and encrypted byte pipe, not an application-layer participant.

### Layering

```text
cmd/relay
  ↓
WebSocket server / connection handling
  ↓
room registry + room lifecycle
  ↓
encrypted frame forwarding
```

### Package responsibilities

#### `services/relay/cmd/relay`

Owns:

- process startup
- bind address configuration
- wiring the transport server to the room registry
- shutdown orchestration

#### `services/relay/internal/room`

Owns:

- room creation and lookup
- sender/receiver pairing
- room lifecycle transitions
- expiration / cleanup behavior

#### `services/relay/internal/transport`

Owns:

- WebSocket accept/upgrade path
- sender and receiver connection handling
- frame relay between paired peers
- lightweight `/healthz` and `/status` operator endpoints

Design constraints:

- do not decrypt payloads
- do not reinterpret the encrypted application protocol
- keep server state minimal and disposable

Current limitations:

- no explicit rate limiting yet
- no metrics endpoint yet
- operator visibility is still limited to lightweight health/status summaries

---

## Punchthrough Architecture (`lib/punchthrough/`)

This module is groundwork for a future direct path. It is not yet integrated into the client runtime.

### Package responsibilities

#### `pkg/stun`

Owns:

- STUN requests/responses
- network probing support
- external address discovery and related typing/config

#### `pkg/punch`

Owns:

- NAT classification
- UDP hole-punching flow primitives
- related config/types/errors

#### `cmd/punchthrough`

Owns:

- operator/dev CLI entry point for probing and testing the NAT tooling

Near-term architectural goal:

- feed punchthrough results into the client's transport selection so the client can attempt direct transport before falling back to relay

---

## Admin Surface (`services/bore-admin/`)

This module is now a small but real operator CLI.

What it is:

- a Go CLI that queries the relay `/status` endpoint
- a human-readable status summary for relay uptime, room counts, and limits
- a place to grow additional relay monitoring and operator workflows later

What it is not:

- a dashboard
- a metrics/history system
- a storage layer
- an operational dependency of the relay or client

Keep docs honest: treat this module as minimal operator tooling until it grows beyond status polling.

---

## Transfer Flow

### Current verified relay flow

```text
Sender                               Relay                         Receiver
  │                                    │                              │
  │ 1. Create/send room                │                              │
  │───────────────────────────────────►│                              │
  │                                    │                              │
  │ 2. Display full rendezvous code    │                              │
  │                                    │                              │
  │                                    │ 3. Receiver joins room       │
  │                                    │◄─────────────────────────────│
  │                                    │                              │
  │ 4. Noise XXpsk0 handshake over encrypted transport path           │
  │◄─────────────────────────────────────────────────────────────────►│
  │                                    │                              │
  │ 5. Sender streams encrypted header/chunks/end                    │
  │─────────────────────────────────────────────────────────────────►│
  │                                    │                              │
  │ 6. Receiver reassembles and verifies SHA-256                     │
  │                                    │                              │
```

### Planned transport selection

```text
                    ┌────────────────────┐
                    │ Can peers connect  │
                    │ directly?          │
                    └────────┬───────────┘
                             │
                   ┌─────────┴─────────┐
                   │                   │
                 Yes                  No
                   │                   │
            ┌──────┴──────┐    ┌───────┴───────┐
            │ direct path │    │ relay path    │
            │ via         │    │ via WebSocket │
            │ punchthrough│    │ broker        │
            └─────────────┘    └───────────────┘
```

This selection logic is planned, not current behavior.

---

## Design Rules

1. **Docs describe the code that exists, not the code we wish existed.**
2. **Relay stays payload-blind.**
3. **Rendezvous code is cryptographic input, not cosmetic metadata.**
4. **Direct transport is optional architecture, not a precondition for shipping the relay path.**
5. **Minimal tools stay clearly labeled until they carry broader workload.**

---

## Open Architectural Work

- integrate punchthrough into client transport selection
- add resumable transfer state and resume protocol rules
- harden relay operations with rate limiting and metrics
- decide how much operator surface bore-admin actually needs beyond relay status polling

For the current execution plan and verification commands, see [BUILD.md](BUILD.md). For security claims and limits, see [SECURITY.md](SECURITY.md).
