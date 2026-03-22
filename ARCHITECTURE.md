# ARCHITECTURE.md

Technical architecture and design notes for `bore`.

This document describes the intended architecture. It is a design reference, not a description of what exists today. See [BUILD.md](BUILD.md) for what is actually implemented.

---

## System overview

bore is a file transfer tool with three components:

```text
┌─────────────┐                           ┌─────────────┐
│   Sender    │                           │  Receiver   │
│  (bore-cli) │                           │  (bore-cli) │
│             │                           │             │
│  bore-core  │◄─── encrypted channel ───►│  bore-core  │
└──────┬──────┘                           └──────┬──────┘
       │                                         │
       │         ┌─────────────────┐             │
       └────────►│  bore-relay     │◄────────────┘
                 │  (optional)     │
                 │  zero-knowledge │
                 └─────────────────┘
```

1. **Sender** generates a short code, encrypts, and sends files.
2. **Receiver** enters the code, authenticates, and receives files.
3. **Relay** (optional) forwards encrypted traffic when direct connection fails. It cannot decrypt the content.

---

## Crate architecture

### bore-core

The engine. No IO, no platform-specific code in the public API. Everything behind traits.

```text
bore-core/
├── error.rs       # Typed error hierarchy (BoreError → Session/Transfer/Protocol/Crypto/Transport)
├── session.rs     # Session lifecycle: roles, state machine, capabilities, identity
├── transfer.rs    # Transfer model: intent, manifest, chunks, progress
├── protocol.rs    # Wire protocol: versioning, message types, frame constants
├── crypto.rs      # (Phase 2) Noise XX handshake, AEAD encryption, key derivation
├── transport.rs   # (Phase 5) Transport trait, direct/relay implementations
└── codec.rs       # (Phase 1) Frame encoding/decoding, message serialization
```

Design rules:
- **No `anyhow` in the library.** Typed errors only (`BoreError`, `SessionError`, etc.).
- **No direct IO.** Crypto, transport, and file access are behind traits.
- **Testable in isolation.** All state machines can be exercised without networking or filesystem.

### bore-cli

Thin shell over `bore-core`. Owns IO, user interaction, and progress display.

```text
bore-cli/
└── main.rs        # CLI entry point, clap command tree
```

Will grow to:
```text
bore-cli/
├── main.rs        # Entry point, command routing
├── send.rs        # Send command: manifest building, code display, progress
├── receive.rs     # Receive command: code input, acceptance, progress
├── relay.rs       # Relay command: server startup, config
├── history.rs     # History command: past transfers
├── config.rs      # Config file loading, CLI flag merging
└── ui.rs          # Progress bars, colored output, terminal handling
```

### bore-relay (future)

Standalone service. Minimal, deployable, self-hostable.

```text
bore-relay/
├── main.rs        # Server entry point
├── room.rs        # Room lifecycle: create, join, relay, close, timeout
├── ratelimit.rs   # Per-IP and per-room rate limiting
├── config.rs      # Server configuration
└── metrics.rs     # Operator metrics (optional)
```

---

## Transfer flow

### Happy path

```text
Sender                          Relay/Rendezvous              Receiver
  │                                    │                          │
  │  1. Create session                 │                          │
  │  2. Generate code                  │                          │
  │  3. Register code ────────────────►│                          │
  │  4. Display code to user           │                          │
  │                                    │                          │
  │                                    │   5. User enters code    │
  │                                    │◄──── Lookup code ────────│
  │                                    │────► Connection info ───►│
  │                                    │                          │
  │◄──────── 6. Noise XX handshake (PAKE-bound to code) ────────►│
  │                                    │                          │
  │──── 7. Offer (manifest) ─────────────────────────────────────►│
  │                                    │                          │
  │◄──── 8. Accept ──────────────────────────────────────────────│
  │                                    │                          │
  │──── 9. Data (chunk 0) ──────────────────────────────────────►│
  │◄──── Ack (chunk 0) ─────────────────────────────────────────│
  │──── Data (chunk 1) ─────────────────────────────────────────►│
  │◄──── Ack (chunk 1) ─────────────────────────────────────────│
  │  ... repeat ...                    │                          │
  │                                    │                          │
  │──── 10. Done ───────────────────────────────────────────────►│
  │◄──── Close ─────────────────────────────────────────────────│
  │                                    │                          │
```

### Transport selection

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
            │ Same LAN?   │    │ Relay         │
            └──────┬──────┘    │ (WebSocket)   │
                   │           └───────────────┘
            ┌──────┴──────┐
            │             │
          Yes            No
            │             │
     ┌──────┴──────┐ ┌───┴──────────┐
     │ TCP direct  │ │ QUIC +       │
     │             │ │ hole-punch   │
     └─────────────┘ └──────────────┘
```

The transport mode is always reported to the user. No silent fallback.

---

## Cryptographic design

### Key exchange: Noise Protocol XX

The [Noise Protocol Framework](http://noiseprotocol.org/) with the XX handshake pattern:

```text
XX:
  → e
  ← e, ee, s, es
  → s, se
```

- **XX** provides mutual authentication without pre-shared keys.
- Both peers generate ephemeral key pairs.
- Static keys are exchanged encrypted.
- After handshake: both sides have a shared symmetric key.

### PAKE binding

The rendezvous code (e.g., `7-guitar-castle-moon`) is used as a weak shared secret for PAKE. This means:

- The code isn't just a routing hint — it's a cryptographic input.
- An attacker who doesn't know the code cannot complete the handshake.
- An attacker who intercepts the code can attempt a man-in-the-middle attack, but the Noise handshake's transcript binding detects this.

### Data encryption: ChaCha20-Poly1305

After the Noise handshake completes:

- Separate send/receive keys derived from the handshake output.
- Each frame encrypted with ChaCha20-Poly1305 AEAD.
- Nonces are counter-based (monotonically increasing).
- Replayed or out-of-order frames are rejected.

### Key material lifecycle

- Ephemeral keys: generated per session, zeroized after handshake.
- Session keys: derived from Noise output, zeroized when session ends.
- No long-term keys stored on disk (unless implementing persistent identity, which is not planned).

---

## Protocol wire format

### Frame structure

```text
┌──────────────────┬───────────┬──────────────────────┐
│  Length (4 bytes) │ Type (1)  │  Payload (variable)  │
│  big-endian u32   │  u8 tag   │  up to 16 MiB        │
└──────────────────┴───────────┴──────────────────────┘
```

- Length includes the type byte but not itself.
- Maximum payload: 16 MiB (configurable down, not up).
- All multi-byte integers are big-endian.

### Message types

| Tag | Name | Direction | Description |
|-----|------|-----------|-------------|
| 0x01 | Hello | Both | Protocol version + capabilities |
| 0x02 | Offer | Sender→Receiver | Transfer manifest |
| 0x03 | Accept | Receiver→Sender | Acceptance of manifest |
| 0x04 | Reject | Receiver→Sender | Rejection with reason |
| 0x10 | Data | Sender→Receiver | File chunk |
| 0x11 | Ack | Receiver→Sender | Chunk acknowledgment |
| 0x20 | Done | Sender→Receiver | Transfer complete |
| 0xE0 | Error | Both | Error with code and message |
| 0xF0 | Close | Both | Graceful session close |

---

## Relay design

### Principles

1. **Zero-knowledge.** The relay sees encrypted bytes. It cannot read file names, content, or metadata.
2. **Minimal state.** The relay tracks rooms (code → connection pair) and nothing else.
3. **Self-hostable.** A single binary, minimal config, deployable anywhere.
4. **Not the trust root.** Authentication is end-to-end between peers. The relay is just a pipe.

### Room lifecycle

```text
Created → Active → Draining → Closed
   ↓                   ↓
 Expired            Expired
```

- **Created**: sender registers, room exists but no receiver yet.
- **Active**: both peers connected, traffic flowing.
- **Draining**: one peer disconnected, waiting for reconnection or timeout.
- **Closed**: both peers done or room expired.

### What the relay knows

| Information | Known? |
|------------|--------|
| Room code / session ID | Yes (routing) |
| Sender IP address | Yes (connection) |
| Receiver IP address | Yes (connection) |
| Total bytes forwarded | Yes (rate limiting) |
| File names | No |
| File content | No |
| Number of files | No |
| Peer identities | No |
| Protocol messages (decrypted) | No |

---

## Resumability design

### Resume token

```text
{session_id}:{manifest_hash}:{chunk_bitmap}
```

- **session_id**: identifies the session to resume.
- **manifest_hash**: ensures the file set hasn't changed (prevents bait-and-switch).
- **chunk_bitmap**: which chunks have been acknowledged.

### Resume flow

1. Receiver reconnects with resume token.
2. Sender validates session ID and manifest hash.
3. Sender sends only unacknowledged chunks.
4. Normal integrity verification continues.

Resume works across transport mode changes (e.g., started direct, resumed via relay).

---

## Error philosophy

- **Library errors are typed.** `BoreError` with specific variants for each domain.
- **CLI errors are human-readable.** `anyhow` wrapping library errors with context.
- **No panics in library code.** `Result` everywhere. Panics are bugs.
- **Every error is actionable.** The user can understand what went wrong and what to do.

---

*This document will be updated as the design evolves. See BUILD.md for what is actually implemented.*
