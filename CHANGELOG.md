# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- Landed the Bore v2 Phase 1 workspace skeleton at repo root with `apps/api`, `apps/web`, `packages/contracts`, `infra/caddy`, `.env.example`, `docker-compose.yml`, and shared Bun verification scripts.
- Added the first typed v2 health and readiness contracts plus a Caddy-routed local Compose stack for the new web and API lane.

### Documentation

- Updated README and architecture docs so repo truth now distinguishes shipped Go-first v1 from the active v2 workspace scaffold.
- Expanded `BUILD.md` with rewrite progress, explicit feature decisions, and transitional deprecation stance for legacy surfaces.

## [1.0.1] - 2026-03-29

Patch release focused on relay input validation, browser-surface hardening, and docs cleanup after v1.0.0.

### Security

- Added shared room ID validation across client and relay paths so malformed room IDs are rejected before WebSocket or signaling setup.
- Limited `/signal` exchanges to live relay rooms created by a sender.
- Added restrictive browser headers on relay and frontend responses (`Content-Security-Policy`, `X-Content-Type-Options`, `X-Frame-Options`, `Referrer-Policy`) and disabled caching for live operator surfaces.
- Validated `BORE_RELAY_URL` as a bare relay origin in the frontend so relay endpoint composition stays explicit.

### Documentation

- Removed stale `BUILD.md` references and aligned README, architecture, and security docs with the separate frontend process.

## [1.0.0] - 2026-03-25

Production release. Bore ships P2P encrypted file transfer with QUIC direct transport, automatic relay fallback, Noise XXpsk0 end-to-end encryption, resumable transfers, an operator dashboard, and a full test suite with race detection clean.

### What ships in v1.0.0

#### Core transfer engine
- `bore send` and `bore receive` with direct P2P transport by default
- Automatic relay fallback when direct connection fails
- `--relay-only` flag to force relay transport
- Transport method reporting (direct/relay + fallback reason)
- Rendezvous code generation and parsing (2-5 words, 26-50 bits entropy)
- Noise `XXpsk0` handshake bound to the rendezvous code
- HKDF-SHA256 PSK derivation from rendezvous code
- ChaCha20-Poly1305 encrypted transfer channel with counter nonces
- SHA-256 file integrity verification
- Resumable single-file transfers with on-disk checkpoint state
- ResumeOffer protocol: receiver tells sender which chunk to resume from
- Deterministic transfer ID from (filename, size, SHA-256, chunk_size)

#### Direct transport
- STUN/NAT discovery for public address resolution
- Relay-coordinated signaling for peer candidate exchange
- UDP hole-punching with QUIC transport (default)
- QUIC-based direct transport via quic-go with production-quality congestion control (~340 MB/s loopback)
- ICE-like multi-candidate gathering (host interfaces, STUN server-reflexive)
- Candidate prioritization (host > srflx > relay)
- Connection quality metrics tracking (throughput, byte counters, timing)
- Graceful QUIC -> ReliableConn -> relay degradation chain
- ReliableConn UDP framing layer retained as legacy fallback
- Observable transport decisions with SelectionResult, Method, FallbackReason, DirectErr
- Expanded FallbackReason set: STUNFailed, NATUnfavorable, PunchFailed, SignalingFailed

#### Relay server
- Self-hostable WebSocket relay with room brokering and TTL reaper
- `/signal` WebSocket endpoint for relay-coordinated candidate exchange
- Bidirectional encrypted frame relay with byte/frame counting (fallback transport)
- `/healthz`, `/status`, and `/metrics` endpoints
- Per-IP token bucket rate limiting on `/ws` and `/signal` endpoints (default: 30 req/min)
- Explicit HTTP server timeouts (read 30s, write 30s, idle 120s, header 10s)
- Max header size limited to 1 MB
- Graceful shutdown handling
- Deployment packaging: Dockerfile with multi-stage build and systemd service unit

#### Operator dashboard
- FastAPI + Jinja2 + htmx frontend (no JavaScript build step)
- Product homepage at `/`
- Live relay operator page at `/ops/relay` with htmx 2-second polling
- Direct (inferred) success rate display
- Transport method breakdown (direct vs relay) in operator view
- Signaling health metrics
- Log-based alerting for rate limit spikes, WebSocket errors, room utilization
- HTTPX client fetching data from Go relay's `/status`, `/healthz`, `/metrics`
- Pydantic settings for configuration
- Static CSS via Tailwind CDN

#### Tooling
- `bore-admin status` relay polling CLI
- Standalone `punchthrough` CLI for NAT probing with JSON and human-readable output
- `bore status`, `bore components`, `bore version`, `bore help` commands

#### Verification
- Full test suite across client, relay, transport, crypto, metrics, rate limiting, room, and punchthrough packages
- Race detection clean (`go test -race ./...`)
- Integration test for full direct QUIC transfer (loopback)
- Integration test for direct-fails-relay-succeeds fallback path
- Integration test for /status transport stats after relay transfer
- Fuzz targets for rendezvous code and transfer frame parsing
- CI workflow: Go build/vet/test, golangci-lint, govulncheck, frontend ruff/pyright/pytest

### Architecture

The default transfer path is direct P2P. The client attempts STUN discovery, exchanges candidates through the relay's signaling channel, evaluates NAT feasibility, and attempts hole-punching. If direct fails at any step, the client falls back to the relay transport automatically. End-to-end encryption (Noise XXpsk0) protects all file data regardless of which transport path is used -- the relay is always payload-blind.

### Not yet implemented

- TURN-style relay candidate in multi-candidate gathering
- Directory transfer
- Connection migration for mobile/roaming scenarios
- Authenticated operator workflows
- External security audit

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

[1.0.1]: https://github.com/dunamismax/bore/compare/v1.0.0...v1.0.1
[1.0.0]: https://github.com/dunamismax/bore/compare/v0.2.0...v1.0.0
[0.2.0]: https://github.com/dunamismax/bore/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/dunamismax/bore/releases/tag/v0.1.0
