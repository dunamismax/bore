# Cryptographic Design

## Purpose

This document describes bore's **current Go cryptographic design** for the relay-based transfer path and the near-term decisions still open around future expansion.

It is not a claim of formal verification or external audit. It is the implementation-level design reference for what the Go client is doing today.

---

## Overview

bore's current cryptographic path has two layers:

1. **Handshake:** Noise `XXpsk0` over Curve25519 / ChaChaPoly / SHA-256
2. **Data channel:** ChaCha20-Poly1305 authenticated encryption for post-handshake frames

The rendezvous code is not just a routing hint. It is converted into a pre-shared key and mixed into the handshake, so the code participates directly in session authentication.

Current implementation location:

- `client/internal/crypto`
- `client/internal/code`
- `client/internal/rendezvous`

---

## Handshake design

### Protocol suite

Current suite:

```text
Noise_XXpsk0_25519_ChaChaPoly_SHA256
```

What this means in practice:

- **XX** gives mutual authentication without a pre-existing identity registry
- **psk0** mixes in a rendezvous-code-derived PSK at the start of the handshake
- **25519** provides elliptic-curve Diffie-Hellman
- **ChaChaPoly** provides AEAD encryption
- **SHA-256** is used in the Noise hash / key schedule

### Why this fits bore

- sender and receiver do not know each other through a PKI or account system
- both peers need to authenticate possession of the rendezvous code
- the handshake stays compact and fits the transient transfer model
- relay infrastructure is not part of the trust root

### Rendezvous-code binding

The rendezvous code string is converted into a 32-byte PSK with HKDF-SHA256 and mixed into the Noise handshake.

Conceptually:

```text
code_string -> HKDF-SHA256 -> 32-byte PSK -> Noise XXpsk0 handshake
```

This means:

- a peer with the wrong code derives the wrong PSK
- the handshake fails if sender and receiver do not share the same code
- the code is a real authentication input, not cosmetic metadata

---

## Rendezvous-code entropy

The code format is implemented in `client/internal/code`.

Entropy sources:

- channel number: `1..999`
- 2-5 words from a 256-word list

Approximate entropy model:

| Words | Entropy (approx) |
|---|---:|
| 2 | ~26 bits |
| 3 | ~34 bits |
| 4 | ~42 bits |
| 5 | ~50 bits |

Default today:

- **3 words**
- **5 minute default expiry policy** in the code model
- **single-use session intent** in the rendezvous design

Operational implication:

- online brute force is the relevant threat model, not direct offline key guessing in the abstract
- keeping code lifetime short and adding relay-side rate limiting remain important future hardening steps

---

## Secure channel design

After the Noise handshake completes, the peers use a secure channel abstraction in `client/internal/crypto`.

Current properties:

- ChaCha20-Poly1305 authenticated encryption
- counter-based nonces
- encrypted application frames sent over the transport
- ordered delivery assumptions that fit the current relay path

The channel is transport-agnostic at the IO boundary:

- today it runs over the relay transport
- later it can run over a direct path if the client integrates `lib/punchthrough/`

---

## Transfer integrity

The transfer engine in `client/internal/engine` adds an integrity layer on top of the encrypted channel:

- sender computes SHA-256 for the file being sent
- receiver reassembles the file
- receiver verifies the final SHA-256 before reporting success

This gives bore two integrity layers:

1. authenticated encryption for the transport frames
2. final file hash verification for the delivered artifact

---

## Relay knowledge boundary

The relay is intentionally outside the trust boundary for plaintext.

The relay can know:

- room/session identifier
- connection timing
- sender/receiver network addresses
- total encrypted byte counts

The relay should not know:

- plaintext file contents
- plaintext file names
- decrypted transfer metadata exchanged inside the secure channel

This is why the relay must remain a narrow byte-forwarding service and not absorb protocol logic that requires decryption.

---

## Implementation dependencies

Current client dependencies relevant to crypto and transport:

| Dependency | Purpose |
|---|---|
| `github.com/flynn/noise` | Noise handshake implementation |
| `golang.org/x/crypto` | HKDF and related crypto support |
| `nhooyr.io/websocket` | WebSocket transport used by the relay path |

Design rule:

- keep the dependency surface small and explicit
- avoid broad, overlapping crypto stacks unless there is a strong reason

---

## What is not implemented yet

- direct-transport integration through `lib/punchthrough/`
- resumable transfer protocol state
- relay-side rate limiting and broader anti-abuse controls
- external crypto review or audit

These gaps matter because they limit what claims the repo can make today.

---

## Open follow-up questions

1. how should direct-transport negotiation be expressed without weakening the current relay-based path?
2. what resume-state format should be used once resumable transfer is implemented?
3. what hardening checks should become mandatory in CI for the crypto-relevant Go modules?
4. when does the relay need explicit key/metadata lifecycle documentation beyond the current payload-blind design?

For the broader threat model, see [`threat-model.md`](threat-model.md). For current security posture, see [`../SECURITY.md`](../SECURITY.md).
