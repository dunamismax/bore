# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.1.0] - 2026-03-24

First tagged release. Phases 0–3 are complete: relay-based encrypted file transfer with resume support and relay hardening.

### Added

- `bore send` and `bore receive` for relay-based encrypted file transfer
- Rendezvous code generation and parsing
- Noise `XXpsk0` handshake bound to the rendezvous code
- ChaCha20-Poly1305 encrypted transfer channel
- SHA-256 file integrity verification
- Resumable single-file transfers with on-disk checkpoint state
- ResumeOffer protocol: receiver tells sender which chunk to resume from
- Deterministic transfer ID from (filename, size, SHA-256, chunk size)
- Restart-vs-resume rules with metadata and partial file validation
- Self-hostable WebSocket relay with room brokering and TTL reaper
- Relay `/healthz`, `/status`, and `/metrics` endpoints
- Per-IP token bucket rate limiting on relay `/ws` and `/signal` endpoints
- Explicit HTTP server timeouts (read, write, idle, header)
- Relay-served embedded web UI at `/` and `/ops/relay`
- `bore-admin status` command for relay health polling
- Transport abstraction layer with relay adapter and direct stub
- STUN/NAT discovery, relay-coordinated signaling, and UDP hole-punching in the client transport selector
- UDP reliability/framing layer with sequence numbers, ACK, retransmit, and FIN
- `--direct` CLI flag on `bore send` and `bore receive` for opt-in direct-path attempts
- Observable transport selection with `SelectionResult`, `FallbackReason` tracking
- Standalone `punchthrough` CLI for NAT probing
- React + Vite + TypeScript browser surface (product page and relay ops page)
- Deployment packaging: Dockerfile with multi-stage build and systemd service unit
- CI workflow for Go and web verification

### Notes

- The verified transfer path is **relay-based**. Direct transport is implemented but pending real-world NAT validation; relay remains the default.
- Directory transfer, authenticated operator workflows, and external security review are planned for future releases.

[0.1.0]: https://github.com/dunamismax/bore/releases/tag/v0.1.0
