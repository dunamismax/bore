# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.2.0] - 2026-03-25

Second release. Phases 5-9 complete: P2P-first default path, QUIC direct transport, operator surfaces, verification hardening, and frontend rewrite.

### Added

- Direct P2P transport as the default path (STUN discovery, signaling, hole-punching)
- QUIC-based direct transport with production-quality congestion control (~340 MB/s loopback)
- ICE-like multi-candidate gathering (host, server-reflexive candidates)
- Connection quality metrics tracking (throughput, byte counters, timing)
- `--relay-only` flag to force relay transport
- Transport method reporting (direct/relay + fallback reason)
- Graceful QUIC -> ReliableConn -> relay degradation chain
- Direct transport success/failure rate inference in operator view
- Signaling started counter for /signal connections
- Log-based alerting for relay health (rate limit spikes, WebSocket errors, room utilization)
- Integration test for full direct QUIC transfer (loopback)
- Integration test for direct-fails-relay-succeeds fallback path
- Integration test for /status transport stats after relay transfer
- FastAPI + Jinja2 + htmx operator dashboard (replaces React SPA)
- htmx live-updating relay status with 2-second polling
- Direct (inferred) success rate display in relay ops page

### Changed

- Direct transport is now the default; relay is the automatic fallback
- Frontend rewritten from React + Vite + TypeScript to Python + FastAPI + htmx
- Removed `--direct` flag (direct is now default)
- bore-admin status output now includes transport section
- Relay /status endpoint now includes signalingStarted counter
- Updated all docs to reflect P2P-first architecture

### Removed

- React, Vite, Bun, TypeScript, and JavaScript build step
- `web/` directory (replaced by `frontend/`)

## [0.1.0] - 2026-03-24

First tagged release. Phases 0-3 are complete: relay-based encrypted file transfer with resume support and relay hardening.

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

[0.2.0]: https://github.com/dunamismax/bore/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/dunamismax/bore/releases/tag/v0.1.0
