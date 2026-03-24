# Security Policy

## Current security status

bore currently implements a real relay-based end-to-end encrypted transfer path in the Go client.

Implemented today:

- Noise `XXpsk0` handshake in `internal/client/crypto`
- HKDF-SHA256 derivation of a PSK from the rendezvous code
- ChaCha20-Poly1305 encrypted secure channel
- SHA-256 file integrity verification in the transfer engine
- payload-blind relay forwarding in `internal/relay/`
- room expiry and bounded in-memory room tracking in the relay
- relay `/healthz` and `/status` endpoints that expose aggregate operator data only
- same-origin relay-served web pages at `/` and `/ops/relay/`, with the ops page reading aggregate data from `/status`

Important limits on those claims:

- the currently verified path is relay-based, not direct peer-to-peer
- the relay is functional but not yet hardened with rate limiting or metrics
- the browser surface is read-only and intentionally narrow; it is not an authenticated control plane
- the system has not had an external security audit yet
- resumable transfer behavior is not implemented yet
- bore is not an anonymity tool

If you use bore today, treat it as an implemented encrypted transfer tool with unfinished operational hardening, not as a fully audited or production-hardened security product.

---

## Implemented security properties

### End-to-end encryption

- file data is encrypted between sender and receiver after the Noise handshake completes
- the relay forwards encrypted bytes and does not decrypt payloads
- the current cryptographic suite is `Noise_XXpsk0_25519_ChaChaPoly_SHA256`

### Peer authentication via rendezvous code

- both peers must know the rendezvous code
- the code is converted into a PSK with HKDF-SHA256 and mixed into the handshake
- a peer with the wrong code cannot complete the session successfully

### Forward secrecy per session

- the handshake uses ephemeral key exchange
- session keys are derived for the transfer session and are not intended for long-term reuse
- compromise of one session should not imply compromise of unrelated sessions

### Integrity verification

- the encrypted channel provides authenticated encryption for in-flight frames
- the transfer engine verifies a final SHA-256 hash for the received file
- corrupted or modified payloads should fail verification

### Payload-blind relay model

The relay should know only what it needs to route the session:

- room or session identifier
- connection timing
- sender and receiver IP addresses
- encrypted byte counts

The relay should not know:

- plaintext file content
- plaintext file names
- plaintext transfer metadata carried inside encrypted messages

### Current relay guardrails

Implemented relay guardrails:

- waiting rooms expire after the configured TTL
- concurrent room count is bounded by registry configuration
- WebSocket message size is capped at 64 MB per frame
- per-IP rate limiting on `/ws` and `/signal` endpoints (default: 30 requests/minute)
- explicit HTTP server timeouts: read (30s), write (30s), idle (120s), read header (10s)
- max header size limited to 1 MB
- operational metrics tracked via atomic counters and exposed at `/metrics`

These controls provide meaningful abuse resistance for the relay's threat profile.

### Operator endpoints and browser surface

The relay exposes `/healthz`, `/status`, and `/metrics` for operator visibility and serves a same-origin browser surface at `/` and `/ops/relay`.
Those surfaces are intended to reveal only aggregate service state such as:

- process health
- relay uptime
- room counts by state
- configured room and transport limits
- static product and operator copy that matches the shipped runtime

They should not expose plaintext payloads, rendezvous codes, per-transfer decrypted metadata, or control-plane mutations.

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

bore does not currently aim to provide:

- anonymity
- censorship resistance
- multi-party transfer
- long-term identity or accounts
- malware scanning or file-content validation
- protection against compromised endpoints

---

## Known gaps and risks

### Relay hardening

Implemented:

- per-IP rate limiting on WebSocket and signaling endpoints
- explicit HTTP server timeouts
- operational metrics endpoint at `/metrics`
- room expiry tracking via callback
- deployment artifacts (Dockerfile, systemd unit)

Still not implemented:

- longer-term relay observation tooling and alerting
- external security audit

### Browser surface is intentionally thin

The web layer is intentionally read-only. It does not add auth, persistent operator state, or mutation endpoints. Treat it as a convenience view over aggregate relay state, not a security boundary or control plane. If Bore later adds local durable operator state or resumable-transfer metadata, start with a small relational SQLite store by default.

### Direct transport is not active yet

`internal/punchthrough/` exists, but it is not wired into the current client flow. Security claims should stay scoped to the current relay-based path.

### Resume state is filesystem-based

Resume state is persisted as plaintext JSON + partial file data under `<outputDir>/.bore/`. Security notes:

- resume state files are created with mode 0600 (owner-read/write only)
- the `.bore/` directory is created with mode 0700
- partial data on disk is unencrypted — it represents the plaintext file content as received
- resume state is validated against the file header on each connection; mismatched metadata triggers a full restart
- the final SHA-256 covers the entire reassembled file, not just the resumed portion
- on successful transfer, all resume state and partial files are deleted
- on SHA-256 verification failure after resume, resume state is cleaned up to prevent retrying corrupt data

Threat consideration: an attacker with filesystem access to the receiver can read partial file content. This is consistent with bore's non-goal of protecting against compromised endpoints.

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

- Dependencies are tracked in the root `go.mod` and `go.sum`.
- Crypto-relevant client dependencies should stay small, explicit, and reviewable.
- Dependency updates should be accompanied by focused verification in the affected package set.

Planned hardening work:

- add repeatable dependency review steps for the consolidated Go module
- add broader CI and security checks as the repo stabilizes
- keep crypto and transport dependencies intentionally narrow

---

## Reporting vulnerabilities

If you discover a security vulnerability in bore, report it responsibly:

1. Do not open a public GitHub issue.
2. Email `security@dunamismax.com` or use GitHub private vulnerability reporting.
3. Include a description, reproduction steps, impact, and any logs or traces that matter.
4. The project should acknowledge within 48 hours and provide a remediation timeline.

---

## Security review status

| Area | Status |
|---|---|
| Relay-based encrypted transfer | Implemented |
| Resumable transfer with integrity verification | Implemented |
| Threat model documentation | Present |
| Local tests for client and relay packages | Present |
| Direct transport security review | Deferred until direct transport is integrated |
| Relay abuse controls | Implemented (rate limiting, timeouts, metrics) |
| External review / audit | TODO |

For cryptographic implementation detail, see [`docs/crypto-design.md`](docs/crypto-design.md). For the broader threat model, see [`docs/threat-model.md`](docs/threat-model.md).
