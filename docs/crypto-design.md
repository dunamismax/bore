# Cryptographic Design

## Purpose

This document describes bore's planned cryptographic approach. It is a design document, not a claim of implementation. The actual crypto layer will be implemented in Phase 2. No security properties described here are available until implementation, testing, and review are complete.

---

## Overview

bore's cryptographic protocol has two layers:

1. **Key exchange:** Noise Protocol Framework (XX pattern) with PAKE binding to the rendezvous code
2. **Data channel:** ChaCha20-Poly1305 AEAD for all post-handshake communication

The rendezvous code serves as a weak shared secret for PAKE — it's a cryptographic input, not just a routing hint. This means an attacker who doesn't know the code cannot complete the handshake, even if they can intercept all network traffic.

---

## Key exchange: Noise Protocol XX

### Why Noise?

The [Noise Protocol Framework](http://noiseprotocol.org/) is a well-studied, composable framework for building crypto protocols. It provides:

- Formal security proofs for each handshake pattern
- Defense against replay, reflection, and key-compromise attacks
- Simple, auditable implementations (Rust: `snow` crate)
- No dependency on PKI, certificates, or trusted third parties

### Why the XX pattern?

```text
XX handshake:
  → e
  ← e, ee, s, es
  → s, se
```

The XX pattern provides **mutual authentication without pre-shared keys**. Both peers generate ephemeral key pairs and exchange static keys, encrypted, during the handshake. This is the right choice for bore because:

- Neither peer knows the other's identity in advance
- Both peers need to authenticate (not just one)
- No certificate infrastructure exists or is desired
- The handshake is compact (3 messages)

### PAKE binding to the rendezvous code

The rendezvous code (e.g., `7-apple-beach-crown`) is bound to the Noise handshake as a pre-shared key (PSK) mixed into the handshake state. This means:

1. The code contributes entropy to the session key derivation
2. A peer with the wrong code derives different session keys
3. The handshake fails cleanly (authentication error) if codes don't match
4. The code's entropy directly affects brute-force resistance

**Approach:** Use the Noise `XXpsk0` or `XXpsk3` pattern variant, where the PSK is derived from the rendezvous code via a key derivation function. Alternatively, use the standard XX pattern and mix the code-derived key into the handshake prologue, ensuring it's authenticated by the transcript hash.

**Decision pending:** The exact PSK integration approach needs to be finalized during Phase 2 implementation, after evaluating `snow`'s PSK support and the security properties of each approach.

### Key derivation from the rendezvous code

The rendezvous code is short and low-entropy (~34 bits default). It cannot be used directly as a cryptographic key. Instead:

```text
code_string = "7-apple-beach-crown"
psk = HKDF-SHA256(
    salt = "bore-pake-v0",
    ikm  = code_string.as_bytes(),
    info = "bore handshake psk"
)
```

This produces a 256-bit PSK suitable for mixing into the Noise handshake state. The HKDF step doesn't add entropy — it's a formatting step. The actual entropy is still ~34 bits from the code.

### Entropy implications

The code's entropy budget is documented in detail in the code module (`bore-core/src/code.rs`). Summary:

| Words | Entropy (approx) | Brute-force at 1/sec | Brute-force at 100/sec |
|-------|-------------------|----------------------|------------------------|
| 2 | ~26 bits | ~2.1 years | ~7.8 days |
| 3 | ~34 bits | ~544 years | ~5.4 years |
| 4 | ~42 bits | ~139,000 years | ~1,390 years |
| 5 | ~50 bits | ~35.7 million years | ~357,000 years |

**Online brute-force is the relevant attack.** The PSK is mixed into the handshake — an attacker must complete the full handshake to test each guess. This is inherently online (requires interaction with the peer or relay). Combined with:

- Single-use codes (one attempt per code)
- Short expiry (5 minutes default)
- Rate limiting on the relay

The effective attack window is very narrow even for 2-word codes. 3 words is the default because it provides a comfortable margin for sensitive use cases.

**Offline brute-force is not possible** unless the attacker records a full handshake transcript. Even then, the Noise XX pattern provides forward secrecy through ephemeral keys — past session keys cannot be recovered from the transcript alone. However, if the attacker records the transcript and later obtains the code, they could derive the session key. This is an accepted risk, mitigated by code expiry and single-use semantics.

---

## Data channel: ChaCha20-Poly1305

### Why ChaCha20-Poly1305?

- **Performance:** Fast in software without hardware AES support (relevant for ARM devices, older hardware)
- **Safety:** No padding oracle attacks, no IV reuse concerns with counter-based nonces
- **Standard:** IETF RFC 8439, widely deployed in TLS 1.3 and WireGuard
- **Rust ecosystem:** `chacha20poly1305` crate is well-audited and maintained

### Encryption scheme

After the Noise XX handshake completes, both peers derive symmetric keys:

```text
handshake_output = Noise handshake result
send_key = HKDF-SHA256(handshake_output, info="bore send")
recv_key = HKDF-SHA256(handshake_output, info="bore recv")
```

Each direction uses its own key. The sender's `send_key` is the receiver's `recv_key` and vice versa.

### Nonce handling

- **Counter-based nonces:** Each frame uses a monotonically increasing 96-bit nonce
- **No nonce reuse:** Counter starts at 0 and increments for each frame. Maximum ~2^32 frames per session (counter is 32-bit, padded to 96 bits)
- **Replay detection:** Receiver tracks the highest nonce seen. Frames with nonces ≤ the highest seen are rejected
- **Out-of-order tolerance:** None in the initial implementation. Frames must arrive in order. This is acceptable for TCP/QUIC transports which provide ordered delivery

### Frame encryption

Each protocol frame (after the handshake) is encrypted:

```text
plaintext_frame = [type_tag (1 byte)] [payload (variable)]
nonce = counter.to_le_bytes() padded to 96 bits
aad = [frame_counter (8 bytes)] [session_id (16 bytes)]
ciphertext = ChaCha20-Poly1305.encrypt(key, nonce, aad, plaintext_frame)
wire_frame = [length (4 bytes)] [nonce (12 bytes)] [ciphertext] [tag (16 bytes)]
```

The Associated Authenticated Data (AAD) includes the frame counter and session ID, binding each frame to its position in the stream and its session.

### Key rotation

For long transfers (>2^32 frames or >1 TB), key rotation is necessary to avoid nonce exhaustion:

- After 2^31 frames (safety margin before 2^32 limit), both sides derive new keys:

```text
new_key = HKDF-SHA256(current_key, info="bore rekey", salt=frame_counter)
```

- Key rotation is coordinated via a Rekey protocol message (added in Phase 2)
- Both sides must agree on the rotation point

**Decision pending:** The exact key rotation trigger and coordination mechanism will be finalized during Phase 2.

---

## Key material lifecycle

### Ephemeral keys

- Generated per session using the system's CSPRNG
- Used during the Noise XX handshake only
- **Zeroized** immediately after the handshake completes
- Never written to disk

### Session keys

- Derived from the Noise handshake output
- Used for the duration of the transfer
- **Zeroized** when the session ends (complete, failed, or cancelled)
- Never written to disk in plaintext

### PSK (code-derived)

- Derived from the rendezvous code via HKDF
- Mixed into the Noise handshake state
- **Zeroized** after the handshake completes
- Never stored — re-derived from the code if needed

### Zeroization

All sensitive key material is zeroized on drop using the `zeroize` crate:

- `Zeroize` derive on all key-holding types
- `ZeroizeOnDrop` for automatic cleanup
- Explicit zeroize calls at session boundaries as defense-in-depth

---

## Crate dependencies (planned)

| Crate | Purpose | Notes |
|-------|---------|-------|
| `snow` | Noise Protocol implementation | Well-audited, maintained, supports PSK patterns |
| `chacha20poly1305` | AEAD encryption | RustCrypto, audited, IETF-standard |
| `hkdf` | Key derivation | RustCrypto, standard HKDF-SHA256 |
| `zeroize` | Key material cleanup | RustCrypto, prevents key leakage |
| `rand` | CSPRNG for key generation | Standard Rust randomness |

All planned crypto dependencies come from the RustCrypto ecosystem or are established, audited projects. No custom cryptographic primitives.

---

## What this design does NOT cover

- **Implementation details** — those come in Phase 2
- **Performance characteristics** — benchmarking comes after implementation
- **Formal verification** — out of scope for this project
- **Quantum resistance** — not a current goal; the Noise framework can be upgraded to post-quantum patterns when mature implementations exist
- **Side-channel resistance** — we rely on the audited implementations in `snow` and `chacha20poly1305` for this

---

## Open questions for Phase 2

1. **PSK pattern choice:** `XXpsk0`, `XXpsk3`, or prologue binding? Needs evaluation against `snow`'s API.
2. **Key rotation trigger:** Frame count vs. byte count vs. time-based? Frame count is simplest.
3. **Nonce format:** Counter-only or counter + random component? Counter-only is sufficient for ordered transports.
4. **AAD contents:** Session ID + frame counter is the minimum. Should we include protocol version?
5. **Error messages after handshake:** Should error messages be encrypted too? Yes — everything after handshake should be encrypted, including control messages.

---

*This document will be updated as implementation decisions are made in Phase 2. It represents the current design intent, not a security guarantee.*
