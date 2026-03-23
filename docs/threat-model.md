# Threat Model

## Purpose

This document defines the threat model for bore: who the actors are, what we're protecting, what attacks we expect, and what we explicitly don't cover. It's a practical reference for design decisions, not an academic exercise.

bore is a two-party file transfer tool. The threat model applies to the core transfer flow: sender generates a code, receiver enters it, files move. Everything else — relay infrastructure, deployment, operational security — is scoped separately where relevant.

---

## Actors

### Sender

**Trust level: trusted (by definition)**

The party initiating the transfer. They select files, generate a rendezvous code, and wait for a receiver. The sender is trusted to act honestly — bore does not protect against a malicious sender sending harmful files (that's the receiver's problem to evaluate).

**Capabilities:**
- Chooses files to send
- Generates the rendezvous code
- Controls the session lifetime
- Can cancel at any time

### Receiver

**Trust level: trusted (possesses the code)**

The party accepting the transfer. They know the rendezvous code (obtained out-of-band from the sender) and use it to connect, authenticate, and receive files. Possession of the code implies authorization — there is no additional access control.

**Capabilities:**
- Enters the rendezvous code
- Accepts or rejects the offered transfer
- Can cancel at any time
- Decides where to store received files

### Relay operator

**Trust level: untrusted for content, semi-trusted for availability**

The operator of a relay server used when direct peer-to-peer connection fails. The relay forwards encrypted bytes between sender and receiver. It is not trusted with content, metadata, or peer identity beyond what TCP/IP necessarily reveals.

**Capabilities:**
- Sees connection metadata (IP addresses, timing, byte counts)
- Can observe session duration and transfer size
- Can throttle, delay, or drop traffic
- Can refuse to relay
- Cannot decrypt content or protocol messages

**Honest-but-curious assumption:** The default threat model assumes the relay operator follows the protocol but may log and analyze all metadata they can see. This is the realistic model for public relays.

### Network observer (passive)

**Trust level: untrusted**

Any party that can observe network traffic between the sender, receiver, and/or relay. This includes ISPs, Wi-Fi operators, VPN providers, and state-level observers.

**Capabilities:**
- Can observe encrypted traffic (timing, size, endpoints)
- Can perform traffic analysis (correlation of sender/receiver activity)
- Cannot decrypt content
- Cannot inject or modify traffic (that's the active attacker)

### Active attacker (MITM)

**Trust level: untrusted**

An attacker who can intercept, modify, inject, or replay traffic on the network path. This is the strongest network-level adversary.

**Capabilities:**
- Everything the passive observer can do
- Can inject, modify, or replay packets
- Can attempt to impersonate the sender or receiver
- Can attempt to brute-force the rendezvous code
- Can attempt to race the legitimate receiver to claim the code first

### Malicious relay

**Trust level: actively hostile**

A relay operator who deliberately attempts to compromise transfers. This is the relay operator threat model upgraded from honest-but-curious to actively malicious.

**Capabilities:**
- Everything the relay operator can do
- Can selectively modify, drop, or replay encrypted frames
- Can attempt to downgrade the protocol
- Can present different views to sender and receiver
- Can attempt to correlate sessions across time
- Cannot decrypt content (same limitation as honest relay)

---

## Assets

### File content

**Sensitivity: high**
**Protection: end-to-end encryption (ChaCha20-Poly1305 after Noise XX handshake)**

The primary asset. File content must never be readable by anyone other than the sender and receiver. This includes the relay, the network, and any party that doesn't know the rendezvous code.

### File metadata

**Sensitivity: medium-high**
**Protection: encrypted within protocol messages**

File names, sizes, directory structure, modification times, and permissions. This metadata is sent as part of the Offer message, which is encrypted end-to-end. The relay cannot see file metadata.

**Note:** Total transfer size (in encrypted bytes) is visible to the relay and network observers. Individual file sizes are not, since they're bundled into the encrypted stream.

### Sender and receiver identity

**Sensitivity: low (explicitly a non-goal)**
**Protection: none beyond standard IP-level protections**

bore does not hide who is communicating. IP addresses are visible to the relay and to each other (in direct mode). bore is not an anonymity tool — use Tor for that.

### Transfer timing and patterns

**Sensitivity: low**
**Protection: none**

When transfers happen, how long they take, and their rough size are visible to network observers and the relay. bore does not attempt to hide traffic patterns.

### Rendezvous code

**Sensitivity: high (during its lifetime)**
**Protection: short-lived, single-use, rate-limited, cryptographically bound**

The code is the shared secret that authorizes and authenticates the transfer. It must remain confidential during its (short) lifetime. After the session completes, the code has no value.

---

## Non-goals

bore explicitly does **not** attempt to provide:

### Anonymity

bore does not hide who is communicating. Both peers' IP addresses are visible to the relay, and to each other in direct mode. Traffic timing is observable. If you need anonymity, use Tor or a similar system and layer bore on top (which bore does not prevent, but also does not assist).

### Plausible deniability

Transfers are mutually authenticated. Both sides can prove a transfer occurred (they both hold the session key material, transcript, etc.). bore does not provide deniable encryption or off-the-record semantics.

### Multi-party transfer

bore is strictly two-party: one sender, one receiver. Multi-party protocols (e.g., one sender to many receivers) are out of scope and would require fundamentally different key exchange semantics.

### Long-term identity

bore does not maintain persistent identity. There are no user accounts, no contact lists, no trust-on-first-use, no key continuity. Each session is independent. This is a feature, not a limitation — it eliminates key management complexity and long-term credential compromise risks.

### Censorship resistance

bore does not attempt to evade network blocking. The relay can be blocked by IP, domain, or protocol fingerprint. bore does not disguise its traffic as other protocols.

### Protection against malicious file content

bore transfers bytes. It does not inspect, scan, or validate file content. A malicious sender can send malware, and bore will faithfully deliver it. This is the receiver's responsibility to evaluate, same as any file transfer mechanism.

### Protection against endpoint compromise

If the sender or receiver's machine is compromised, bore cannot help. An attacker with access to the filesystem can read files before encryption or after decryption. This is a fundamental limitation of any end-to-end encryption system.

---

## Attack scenarios and mitigations

### 1. Brute-force the rendezvous code

**Attack:** An attacker observes a session being set up (e.g., sees the sender waiting) and tries to guess the code before the legitimate receiver connects.

**Mitigations:**
- **Entropy:** Default 3-word code from a 256-word list + channel number = ~34 bits. At 1 attempt/second (relay rate limit), exhaustion takes ~544 years.
- **Single-use:** Code is consumed on first successful connection. An attacker gets one shot per code.
- **Expiry:** Codes expire after 5 minutes (default). The window for brute-force is narrow.
- **Rate limiting:** Relay enforces per-IP rate limits on code attempts.
- **Configurable:** Users can increase to 4-5 words for sensitive transfers (~42-50 bits).

**Residual risk:** If the attacker can attempt thousands of connections per second and the relay doesn't rate-limit effectively, the window is wider. But single-use semantics mean only one attempt succeeds.

### 2. Man-in-the-middle (MITM) attack

**Attack:** An attacker intercepts the connection between sender and receiver, attempting to negotiate separate sessions with each.

**Mitigations:**
- **Noise XX handshake:** Provides mutual authentication with transcript binding. Any modification to handshake messages is detected.
- **PAKE binding:** The rendezvous code is cryptographically bound to the handshake. An attacker who doesn't know the code cannot complete the handshake with either party.
- **Transcript hash:** Both parties can verify the handshake transcript. Any divergence means different codes or MITM interference.

**Residual risk:** If the attacker knows the code (e.g., overheard it), they can attempt to race the legitimate receiver. Single-use semantics and short expiry limit this window. The legitimate receiver would fail to connect, which is detectable.

### 3. Relay as MITM

**Attack:** A malicious relay attempts to intercept, modify, or analyze traffic.

**Mitigations:**
- **End-to-end encryption:** All protocol messages and file data are encrypted between sender and receiver. The relay sees only encrypted bytes.
- **Integrity protection:** AEAD (ChaCha20-Poly1305) detects any modification of encrypted frames. The relay cannot selectively alter content.
- **No trust required:** The relay does not participate in the key exchange. It cannot inject itself into the Noise handshake.

**Residual risk:** The relay can perform traffic analysis (timing, size), denial of service (dropping frames), or selective disruption (dropping specific frame indices). bore detects corrupted or missing frames but cannot prevent relay-level DoS.

### 4. Replay attack

**Attack:** An attacker records encrypted frames and replays them later.

**Mitigations:**
- **Counter-based nonces:** Each frame has a monotonically increasing nonce. Replayed frames have duplicate or out-of-order nonces and are rejected.
- **Session binding:** Frames are encrypted with session-specific keys. Frames from one session cannot be injected into another.

### 5. Code interception

**Attack:** An attacker intercepts the rendezvous code during out-of-band exchange (e.g., reads a text message, overhears a phone call).

**Mitigations:**
- **Short lifetime:** Codes expire quickly (5 minutes default).
- **Single-use:** Once used, the code cannot be reused.
- **Awareness:** Users are told the code is sensitive and should be shared securely.

**Residual risk:** If the code is intercepted before the legitimate receiver uses it, the attacker can impersonate the receiver. This is a fundamental limitation of code-based authentication — the security of the code exchange is the user's responsibility.

### 6. Denial of service

**Attack:** An attacker floods the relay with connections, exhausts resources, or disrupts ongoing transfers.

**Mitigations:**
- **Rate limiting:** Per-IP and per-room rate limits on the relay.
- **Room limits:** Maximum concurrent rooms, maximum transfer size, maximum connection duration.
- **Connection timeouts:** Idle connections are cleaned up.
- **Self-hosting:** Users can run their own relay, immune to attacks on the public relay.

**Residual risk:** A sufficiently resourced attacker can DoS any relay. Self-hosting and direct P2P connections are the escape hatch.

### 7. Resume bait-and-switch

**Attack:** An attacker (or compromised sender) changes the files between initial send and resume, hoping the receiver doesn't notice.

**Mitigations:**
- **Manifest hash:** The transfer manifest is hashed. On resume, the receiver verifies the manifest hash matches. Any change to file names, sizes, or order is detected.
- **Per-chunk integrity:** Each chunk is independently verified. Even if the manifest matches, individual chunk corruption is detected.

### 8. Protocol downgrade

**Attack:** An attacker or malicious relay attempts to force the peers to use a weaker protocol version.

**Mitigations:**
- **Version negotiation in handshake:** Protocol version is part of the Noise handshake, which is authenticated. Any tampering with the version is detected.
- **Strict compatibility:** Currently, versions must match exactly. No fallback to weaker versions.

---

## Trust boundaries

```text
┌─────────────────────────────────────────────────────┐
│                 Sender's machine                     │
│                                                     │
│  ┌─────────────┐    ┌──────────────────────────┐   │
│  │  Filesystem  │───►│  bore Go client          │   │
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
│  │  bore Go client          │◄┘                    │
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

**Boundary 1: Sender's machine ↔ Network.** Plaintext exists only on the sender's machine. Once data crosses this boundary, it is encrypted.

**Boundary 2: Network ↔ Receiver's machine.** Encrypted data is decrypted only on the receiver's machine.

**Boundary 3: Relay.** The relay sits entirely within the encrypted zone. It handles only encrypted bytes and connection metadata.

---

## Metadata exposure summary

| What | Sender sees | Receiver sees | Relay sees | Network sees |
|------|:-----------:|:------------:|:----------:|:------------:|
| File content | Yes | Yes | No | No |
| File names/sizes | Yes | Yes | No | No |
| Peer IP address | Receiver's IP (direct) | Sender's IP (direct) | Both IPs | Both IPs |
| Transfer size (bytes) | Yes | Yes | Encrypted total | Encrypted total |
| Transfer timing | Yes | Yes | Yes | Yes |
| Rendezvous code | Yes | Yes | Routing only | No |
| Session ID | Yes | Yes | Room ID | No |

---

## Recommendations for users

1. **Share codes securely.** The code is the session's security. Don't post it publicly. Voice, encrypted chat, or in-person are best.
2. **Use more words for sensitive transfers.** `--words 4` or `--words 5` for confidential files.
3. **Prefer direct connections.** Direct mode reveals fewer metadata to third parties (no relay involvement).
4. **Self-host relays for organizational use.** If relay metadata exposure matters, run your own.
5. **Verify transfer completions.** bore reports integrity verification results. Pay attention to them.
6. **Don't rely on bore for anonymity.** It's not designed for that. Use Tor if you need it.
