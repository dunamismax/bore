# Security Policy

## Current security status

bore currently implements a real relay-based end-to-end encrypted transfer path in the Go client.

Implemented today:

- Noise `XXpsk0` handshake in `client/internal/crypto`
- HKDF-SHA256 derivation of a PSK from the rendezvous code
- ChaCha20-Poly1305 encrypted secure channel
- SHA-256 file integrity verification in the transfer engine
- payload-blind relay forwarding in `services/relay/`
- room expiry and bounded in-memory room tracking in the relay

Important limits on those claims:

- the currently verified path is **relay-based**, not direct peer-to-peer
- the relay is functional but not yet hardened with rate limiting, health endpoints, or metrics
- the system has **not** had an external security audit yet
- resumable transfer behavior is not implemented yet
- bore is **not** an anonymity tool

If you use bore today, treat it as an implemented encrypted transfer tool with unfinished operational hardening — not as a fully audited or production-hardened security product.

---

## Implemented security properties

### End-to-end encryption

- File data is encrypted between sender and receiver after the Noise handshake completes.
- The relay forwards encrypted bytes and does not decrypt payloads.
- The current cryptographic suite is `Noise_XXpsk0_25519_ChaChaPoly_SHA256`.

### Peer authentication via rendezvous code

- Both peers must know the rendezvous code.
- The code is converted into a PSK with HKDF-SHA256 and mixed into the handshake.
- A peer with the wrong code cannot complete the session successfully.

### Forward secrecy per session

- The handshake uses ephemeral key exchange.
- Session keys are derived for the transfer session and are not intended for long-term reuse.
- Compromise of one session should not imply compromise of unrelated sessions.

### Integrity verification

- The encrypted channel provides authenticated encryption for in-flight frames.
- The transfer engine verifies a final SHA-256 hash for the received file.
- Corrupted or modified payloads should fail verification.

### Payload-blind relay model

The relay should know only what it needs to route the session:

- room/session identifier
- connection timing
- sender/receiver IP addresses
- encrypted byte counts

The relay should **not** know:

- plaintext file content
- plaintext file names
- plaintext transfer metadata carried inside encrypted messages

### Current relay guardrails

Implemented relay guardrails are modest but real:

- waiting rooms expire after the configured TTL
- concurrent room count is bounded by registry configuration
- WebSocket message size is capped

These are baseline resource controls, not a substitute for proper abuse protection.

---

## Threat model summary

### Actors

| Actor | Trust level | Capabilities |
|---|---|---|
| Sender | trusted | Has files, generates code, initiates transfer |
| Receiver | trusted with code | Knows the rendezvous code and accepts transfer |
| Relay operator | untrusted for content | Can observe metadata and availability, cannot read encrypted payloads |
| Network observer | untrusted | Can observe endpoints, timing, and encrypted traffic |
| Active attacker | untrusted | Can intercept, modify, inject, or replay traffic |

### Assets

| Asset | Current protection |
|---|---|
| File content | End-to-end encryption after the handshake |
| File metadata in protocol messages | Encrypted within the application channel |
| Transfer integrity | AEAD + final SHA-256 verification |
| Peer identity beyond IP | Not protected; bore is not an anonymity system |
| Transfer timing / rough size | Not protected |
| Rendezvous code | Short-lived shared secret; user must keep it confidential |

### Non-goals

bore does **not** currently aim to provide:

- anonymity
- censorship resistance
- multi-party transfer
- long-term identity / accounts
- malware scanning or file-content validation
- protection against compromised endpoints

---

## Known gaps and risks

### Relay hardening is incomplete

Not yet implemented:

- rate limiting
- health endpoint
- metrics endpoint
- stronger operator controls / quotas

This means the relay should be treated as functional but not yet production-hardened against abuse or observability requirements.

### Direct transport is not active yet

`lib/punchthrough/` exists, but it is not wired into the current client flow. Security claims should stay scoped to the current relay-based path.

### No resumable transfer protocol yet

Resume-state integrity and interruption recovery are still future work. Do not claim restart-safe transfer semantics yet.

### No external security review yet

The code has local tests and design documentation, but no independent audit or formal review should be implied.

### Metadata exposure remains part of the design

The relay and network can still observe:

- who connected to the relay
- when they connected
- how long the session lasted
- roughly how much encrypted data moved

That is consistent with bore's design. It is also why bore should not be described as an anonymity system.

---

## Dependency policy

- Dependencies are tracked per Go module via `go.mod` and `go.sum`.
- Crypto-relevant client dependencies should stay small, explicit, and reviewable.
- Dependency updates should be accompanied by focused verification in the affected module.

Planned hardening work:

- add repeatable dependency review steps for the Go modules
- add broader CI/security checks as the repo stabilizes
- keep crypto and transport dependencies intentionally narrow

---

## Reporting vulnerabilities

If you discover a security vulnerability in bore, report it responsibly:

1. **Do not** open a public GitHub issue.
2. Email: `security@dunamismax.com` or use GitHub private vulnerability reporting.
3. Include a description, reproduction steps, impact, and any logs or traces that matter.
4. The project should acknowledge within 48 hours and provide a remediation timeline.

---

## Security review status

| Area | Status |
|---|---|
| Relay-based encrypted transfer | Implemented |
| Threat model documentation | Present |
| Local tests for client/relay modules | Present |
| Direct transport security review | Deferred until direct transport is integrated |
| Relay abuse controls | TODO |
| External review / audit | TODO |

For cryptographic implementation detail, see [`docs/crypto-design.md`](docs/crypto-design.md). For the broader threat model, see [`docs/threat-model.md`](docs/threat-model.md).
