# Threat Model

## Purpose

This document defines the threat model for bore: who the actors are, what the system protects, what attacks matter, and what the current design still leaves exposed. It is a practical reference for design decisions, not an academic exercise.

bore is a two-party file transfer tool with a P2P-first, relay-fallback architecture. The default transport is direct peer-to-peer via STUN discovery and UDP hole-punching. When direct fails, bore falls back to a relay automatically. End-to-end encryption protects all file data regardless of transport path. The threat model applies to both paths.

---

## Actors

### Sender

**Trust level: trusted (by definition)**

The party initiating the transfer. They select files, generate a rendezvous code, and wait for a receiver. bore does not protect against a malicious sender sending harmful files; it only protects the transport.

**Capabilities:**
- chooses files to send
- generates the rendezvous code
- controls the session lifetime
- can cancel at any time

### Receiver

**Trust level: trusted (possesses the code)**

The party accepting the transfer. They know the rendezvous code, use it to authenticate into the session, and decide whether to keep the received file.

**Capabilities:**
- enters the rendezvous code
- accepts or rejects the offered transfer
- can cancel at any time
- decides where to store received files

### Relay operator

**Trust level: untrusted for content, semi-trusted for availability**

The relay serves two roles: signaling server for P2P candidate exchange, and fallback transport when direct fails. It is not trusted with file content or transfer metadata carried inside the encrypted channel.

**Capabilities:**
- sees connection metadata such as IP addresses, timing, and encrypted byte counts
- during signaling, sees both peers' STUN-discovered public addresses and NAT types
- when used as fallback transport, can throttle, delay, or drop traffic
- can refuse to relay or refuse to coordinate signaling
- cannot decrypt the protected application data

### Network observer

**Trust level: untrusted**

Any party able to observe traffic between the sender, receiver, and relay.

**Capabilities:**
- can observe endpoints, timing, and encrypted traffic volume
- can correlate sender/receiver activity at the network layer
- cannot read encrypted file contents

### Active attacker

**Trust level: untrusted**

An attacker who can intercept, modify, inject, or replay traffic on the network path.

**Capabilities:**
- everything the passive observer can do
- can inject, modify, or replay packets
- can attempt to impersonate one side of the transfer
- can attempt to guess the rendezvous code while the room is still live

---

## Assets

### File content

**Sensitivity: high**
**Protection: end-to-end encryption after the Noise handshake**

The primary asset. File content should be readable only by the sender and receiver.

### File metadata inside the transfer protocol

**Sensitivity: medium-high**
**Protection: encrypted application messages**

File names and other transfer metadata live inside the encrypted channel. The relay does not need to read them.

### Transfer integrity

**Sensitivity: high**
**Protection: AEAD + final SHA-256 verification**

The transport must detect tampering or corruption in-flight and the receiver must verify the delivered artifact.

### Rendezvous code

**Sensitivity: high during session setup**
**Protection: short lifetime, user-controlled secrecy, handshake binding**

The code is the shared secret that authorizes a transfer. Anyone who learns it before the legitimate receiver can attempt to claim the session.

### Metadata outside the encrypted channel

**Sensitivity: medium**
**Protection: limited**

The relay and network can still observe connection timing, rough transfer size, and the IP addresses involved in the relay session.

---

## Non-goals

bore does **not** currently aim to provide:

- anonymity
- censorship resistance
- multi-party transfer
- long-term identity or accounts
- malware scanning or file-content validation
- protection against compromised endpoints

---

## Attack scenarios and current mitigations

### 1. Wrong-code or guessed-code connection attempts

**Attack:** An attacker tries to join a live room by guessing the rendezvous code before the intended receiver connects.

**Current mitigations:**
- the code contributes real entropy to the session secret
- the relay expires waiting rooms after a bounded lifetime
- each room is intended for a single sender/receiver pairing
- users can increase the number of code words for more entropy

**Residual risk:** The relay enforces per-IP rate limiting, which provides some online guessing resistance. However, a distributed attacker could still attempt guesses across many IPs.

### 2. Man-in-the-middle or handshake tampering

**Attack:** An attacker intercepts traffic and tries to establish different sessions with each side or alter the handshake in flight.

**Current mitigations:**
- Noise `XXpsk0` authenticates the session setup
- the rendezvous code is bound into the handshake as a PSK
- handshake tampering should fail rather than silently downgrade the session

**Residual risk:** If an attacker learns the rendezvous code before the intended receiver uses it, they can race the legitimate receiver. That is a limitation of code-based session authorization.

### 3. Malicious relay behavior

**Attack:** The relay operator logs, delays, drops, or reorders traffic, or attempts to inspect transfer contents.

**Current mitigations:**
- application payloads stay end-to-end encrypted
- authenticated encryption detects modified protected frames
- the relay does not participate in key derivation

**Residual risk:** The relay can still deny service, learn metadata, and make transfers fail. bore is designed to keep the relay payload-blind, not to make it harmless.

### 4. Replay or frame injection

**Attack:** An attacker captures encrypted traffic and replays it later or injects altered frames.

**Current mitigations:**
- the secure channel uses session-bound keys
- frame protection uses authenticated encryption with nonce progression
- modified ciphertext should fail decryption or integrity checks

**Residual risk:** Replay protection depends on the current ordered session model. Resume semantics and multi-session artifact handling are not implemented yet.

### 5. Rendezvous code interception

**Attack:** Someone sees the code during out-of-band sharing and tries to join first.

**Current mitigations:**
- the code is meant to be short-lived
- the handshake fails for peers without the exact code
- the session is designed around one receiver claiming one room

**Residual risk:** Secure code exchange is still the user's responsibility. If the code leaks early, bore cannot distinguish the attacker from the intended receiver.

### 6. Relay abuse or resource exhaustion

**Attack:** A client floods the relay with rooms, connections, or oversized traffic to degrade service.

**Current mitigations:**
- room count is bounded by registry configuration
- waiting rooms expire
- WebSocket message size is capped
- per-IP rate limiting on `/ws` and `/signal` endpoints
- explicit HTTP server timeouts (read, write, idle, header)
- operational metrics tracking for abuse detection

**Residual risk:** These are deliberate hardening controls but do not constitute full DDoS protection. External load balancing and CDN-level protection may be needed for public deployments.

### 7. Direct P2P connection attacks

**Attack:** An attacker on the direct path between peers attempts to intercept, modify, or disrupt the UDP connection.

**Current mitigations:**
- the Noise handshake establishes session keys before any file data is sent
- authenticated encryption (ChaCha20-Poly1305) protects every frame
- modified packets fail AEAD verification and are rejected
- packet injection does not affect the authenticated session

**Residual risk:** An active attacker can disrupt the direct UDP connection (e.g., by flooding the punched-through port), forcing bore to fall back to relay. This is a denial of direct service, not a confidentiality break. The transfer still succeeds via relay with the same encryption.

### 8. Malicious files or compromised endpoints

**Attack:** The transport succeeds, but one endpoint is already hostile or the file itself is harmful.

**Current mitigations:**
- none at the transport layer beyond preserving file integrity in transit

**Residual risk:** This is outside bore's security boundary. bore can protect the channel without making the payload safe.

---

## Trust boundaries

```text
┌─────────────────────────────────────────────────────┐
│                 Sender's machine                     │
│                                                     │
│  ┌─────────────┐    ┌──────────────────────────┐   │
│  │  Filesystem  │───►│  bore client             │   │
│  │  (plaintext) │    │  (plaintext → encrypted) │   │
│  └─────────────┘    └──────────┬───────────────┘   │
│                                │                    │
└────────────────────────────────┼────────────────────┘
                                 │ encrypted bytes
                                 ▼
                    ┌────────────────────────┐
                    │  Network / Relay        │
                    │  (encrypted bytes only) │
                    └────────────┬───────────┘
                                 │ encrypted bytes
                                 ▼
┌────────────────────────────────┼────────────────────┐
│                                │                    │
│  ┌──────────────────────────┐  │                    │
│  │  bore client             │◄┘                    │
│  │  (encrypted → plaintext) │                      │
│  └──────────┬───────────────┘                      │
│             │                                       │
│  ┌──────────▼──┐                                   │
│  │  Filesystem  │                                   │
│  │  (plaintext) │                                   │
│  └─────────────┘                                   │
│                                                     │
│                 Receiver's machine                   │
└─────────────────────────────────────────────────────┘
```

**Boundary 1: sender's machine ↔ network.** Plaintext exists only on the sender's machine before encryption.

**Boundary 2: network ↔ receiver's machine.** Encrypted data is decrypted only on the receiver's machine.

**Boundary 3: relay.** The relay sits inside the encrypted zone and handles only encrypted bytes plus connection metadata.

---

## Metadata exposure summary

| What | Sender sees | Receiver sees | Relay sees | Network sees |
|------|:-----------:|:------------:|:----------:|:------------:|
| File content | Yes | Yes | No | No |
| File metadata inside the encrypted channel | Yes | Yes | No | No |
| Peer IP address (direct) | Peer's public IP | Peer's public IP | Both (during signaling) | Peer-to-peer endpoints |
| Peer IP address (relay fallback) | Relay address | Relay address | Both relay clients | Client-to-relay endpoints |
| Transfer size (bytes) | Yes | Yes | Encrypted total (relay only) | Encrypted total |
| Transfer timing | Yes | Yes | Yes | Yes |
| Transport method used | Yes | Yes | Indirectly (relay usage = fallback) | Connection patterns |
| Rendezvous code | Yes | Yes | No | No |
| Room ID | Yes | Yes | Yes | Possibly, if it can inspect relay requests |

---

## Recommendations for users

1. **Share codes securely.** The code is the session secret.
2. **Use more words for sensitive transfers.** More words increase entropy.
3. **Self-host relays when metadata exposure matters.** The relay still sees connection-level metadata.
4. **Pay attention to transfer completion and integrity output.** Successful delivery should include verification.
5. **Do not treat bore as an anonymity tool.** It is not designed for that.
