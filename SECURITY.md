# Security Policy

## Current security status

**bore does not yet implement any cryptographic protocol, transport encryption, or relay service.** The types and architecture described below are the target design, not the current implementation.

Do not use bore for sensitive transfers until the cryptographic layer is implemented, tested, and reviewed. See [BUILD.md](BUILD.md) Phase 2 and Phase 8 for the implementation and audit timeline.

---

## Planned security properties

Once implemented and verified, bore aims to provide:

### End-to-end encryption
- All file data encrypted between sender and receiver.
- Relay cannot decrypt traffic.
- Uses Noise Protocol XX for key exchange and ChaCha20-Poly1305 for data encryption.

### Peer authentication
- Both peers authenticate using the shared rendezvous code via PAKE.
- No pre-shared keys or certificates required.
- Man-in-the-middle attacks are detected by the Noise handshake transcript binding.

### Forward secrecy
- Ephemeral keys per session.
- Compromising one session does not compromise past or future sessions.
- Key material is zeroized after use.

### Zero-knowledge relay
- Relay forwards encrypted bytes only.
- Relay cannot read file names, content, or metadata.
- Relay knows: room ID, peer IP addresses, byte counts.
- Relay does not know: file content, file names, peer identities beyond IP.

### Integrity verification
- Per-chunk BLAKE3 hashes.
- Full-file hash verification on completion.
- Manifest hash prevents bait-and-switch on resume.

---

## Threat model outline

### Actors

| Actor | Trust level | Capabilities |
|-------|------------|-------------|
| Sender | Trusted | Has files, generates code |
| Receiver | Trusted (with code) | Knows the rendezvous code |
| Relay operator | Untrusted | Can observe encrypted traffic, timing, metadata |
| Network observer | Untrusted | Can observe encrypted traffic between peers/relay |
| Active attacker | Untrusted | Can intercept, modify, or inject traffic |

### Assets

| Asset | Protection |
|-------|-----------|
| File content | E2E encryption (Noise + ChaCha20-Poly1305) |
| File metadata (names, sizes) | E2E encryption (inside protocol messages) |
| Sender/receiver identity | Not protected beyond IP — bore is not an anonymity tool |
| Transfer timing | Not protected — observable by relay and network |
| Rendezvous code | Short-lived, single-use, rate-limited against brute force |

### Non-goals

bore explicitly does **not** aim to provide:

- **Anonymity.** bore does not hide who is communicating. Use Tor for that.
- **Plausible deniability.** Transfers are authenticated — both sides can prove the transfer happened.
- **Multi-party transfer.** bore is strictly two-party (sender + receiver).
- **Long-term identity.** No persistent keys, no contact lists, no trust-on-first-use.
- **Censorship resistance.** The relay can be blocked, and bore does not attempt to evade blocking.

### Known attack surfaces

| Attack | Mitigation | Status |
|--------|-----------|--------|
| Brute-force rendezvous code | Rate limiting, short expiry, single-use codes | Planned |
| Man-in-the-middle | Noise XX transcript binding + PAKE | Planned |
| Replay attack | Counter-based nonces, frame sequence validation | Planned |
| Relay as MITM | End-to-end encryption; relay cannot decrypt | Planned |
| Malicious file content | Out of scope — bore transfers bytes, not semantics | N/A |
| Denial of service on relay | Rate limiting, room limits, connection timeouts | Planned |
| Resume bait-and-switch | Manifest hash validation on resume | Planned |

---

## Reporting vulnerabilities

If you discover a security vulnerability in bore, please report it responsibly:

1. **Do not** open a public GitHub issue.
2. Email: `security@dunamismax.com` (or open a private security advisory on GitHub).
3. Include: description of the vulnerability, reproduction steps, potential impact.
4. We will acknowledge within 48 hours and provide a timeline for a fix.

If the project does not yet have a dedicated security email, use GitHub's private vulnerability reporting feature.

---

## Dependency policy

- All dependencies are tracked in `Cargo.lock`.
- `cargo audit` will be run as part of CI (Phase 8+).
- `cargo deny` will check license compliance and known advisories.
- Cryptographic dependencies (`snow`, `chacha20poly1305`, `blake3`) are chosen for their audit history and community trust.
- Transitive dependency count will be monitored and minimized.

---

## Security review timeline

| Milestone | Phase | Description |
|-----------|-------|-------------|
| Threat model written | 1 | Formal threat model document |
| Crypto implemented | 2 | Noise XX + ChaCha20-Poly1305 |
| Fuzz testing | 8 | Protocol parser, handshake, chunks |
| Dependency audit | 8 | cargo audit + cargo deny |
| External review | 8 | Independent review of crypto layer |
| Security claims published | 8 | README and docs updated with verified claims |

---

*This policy will be updated as the security posture matures. Current status: **no security properties are implemented**.*
