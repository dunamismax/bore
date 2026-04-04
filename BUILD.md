# BUILD.md

> **Required operating rule:** any agent touching Bore's build plan must update this file in the same change set when scope, sequencing, architecture decisions, or verified repo truth changes. Keep the checkboxes honest. Do not mark work complete unless the repo already proves it.

Bore is a peer-to-peer encrypted file transfer tool. No accounts, no cloud, no trust required. A sender shares a short human-readable code; a receiver enters it; an encrypted channel opens and the file moves. The default path is direct peer-to-peer over QUIC with automatic relay fallback when NAT traversal fails. All data is end-to-end encrypted regardless of transport path. The relay is payload-blind. Bore is being rewritten from Go (v1) and TypeScript (v2 prototype) into a fully native Swift application targeting macOS and iOS, with a small self-hostable Vapor relay server for signaling and NAT traversal fallback.

## Current repo truth

- [ ] The repo contains a buildable Swift Package with a shared core library.
- [ ] The repo contains a macOS app target with SwiftUI chrome.
- [ ] The repo contains an iOS app target with SwiftUI chrome.
- [ ] Network.framework QUIC transport is implemented and tested.
- [ ] CryptoKit-based encryption replaces the Go Noise implementation.
- [ ] The Vapor relay server builds and runs.
- [ ] Bonjour/mDNS local peer discovery works.
- [ ] Multipeer Connectivity is integrated for local-network transfers.
- [ ] Universal Links or a custom URL scheme handles transfer link sharing.
- [ ] File transfer completes end-to-end through the Swift stack.
- [ ] The legacy Go and TypeScript codepaths have been removed.

## Product guardrails

- [x] End-to-end encryption is non-negotiable. The relay must never see plaintext.
- [x] Short human-readable rendezvous codes remain the primary sharing mechanism.
- [x] Direct peer-to-peer is the default transport. Relay is the fallback.
- [x] The relay is self-hostable with a single binary and zero mandatory external dependencies.
- [x] No accounts, no sign-up, no cloud dependency for core transfer functionality.
- [x] macOS and iOS are first-class targets. No cross-platform abstraction layers.
- [x] Native Apple frameworks are preferred over third-party dependencies wherever possible.
- [x] SHA-256 file integrity verification is required on every completed transfer.
- [x] Privacy by design: minimal metadata exposure, no analytics, no telemetry.
- [ ] The rewrite preserves the security properties documented in SECURITY.md.
- [ ] Permanent dual-stack (Go + Swift) operation is an acceptable steady state.

## Stack

| Layer | Technology | Notes |
|---|---|---|
| Language | Swift 6+ | Strict concurrency, structured concurrency with async/await |
| UI framework | SwiftUI | Native on macOS and iOS; drag-and-drop on macOS, share sheet on iOS |
| Networking | Network.framework | Native Apple QUIC support (NWProtocolQUIC), UDP, TCP fallback |
| Encryption | CryptoKit | Curve25519 key agreement, ChaChaPoly AEAD, HKDF, SHA256 |
| Package manager | Swift Package Manager | Monorepo with library + app targets |
| Relay server | Vapor | Lightweight Swift HTTP/WebSocket server for signaling and relay fallback |
| Local discovery | Bonjour / NWBrowser | mDNS-based local network peer discovery via Network.framework |
| Local transport | Multipeer Connectivity | Optional local-network direct transfer without relay |
| Link sharing | Universal Links / Custom URL scheme | `bore://` scheme + Associated Domains for deep linking |
| Testing | XCTest + Swift Testing | Unit, integration, and UI tests |
| CI | GitHub Actions | Xcode build, test, lint on macOS runners |

## Phase status summary

- [ ] Phase 1 -- Bootstrap the Swift package and project structure.
- [ ] Phase 2 -- QUIC transport layer with Network.framework.
- [ ] Phase 3 -- End-to-end encryption with CryptoKit.
- [ ] Phase 4 -- Vapor relay server for signaling and fallback.
- [ ] Phase 5 -- Core transfer engine (send/receive/resume).
- [ ] Phase 6 -- Native UI with SwiftUI (macOS + iOS).
- [ ] Phase 7 -- Local peer discovery and Multipeer Connectivity.
- [ ] Phase 8 -- File handling, share sheet, drag-and-drop, and link sharing.
- [ ] Phase 9 -- Polish, testing, and hardening.
- [ ] Phase 10 -- Deployment, distribution, and launch.

---

## Phase 1 -- Bootstrap the Swift package and project structure

### Objectives

- Establish the Swift Package Manager project structure as a monorepo.
- Create shared library targets for core logic and separate app targets for macOS and iOS.
- Set up CI with GitHub Actions building on macOS.
- Remove or archive the legacy Go and TypeScript codepaths.

### Checklist

- [ ] Create `Package.swift` with library target `BoreCore` and executable targets `BoreMac`, `BoreIOS`, `BoreRelay`.
- [ ] Create directory structure: `Sources/BoreCore/`, `Sources/BoreMac/`, `Sources/BoreIOS/`, `Sources/BoreRelay/`, `Tests/BoreCoreTests/`.
- [ ] Add Xcode project or workspace file if needed for iOS/macOS app targets with proper entitlements.
- [ ] Add `.swiftlint.yml` or equivalent linting configuration.
- [ ] Add GitHub Actions workflow for `swift build` and `swift test` on macOS.
- [ ] Add a minimal `BoreCore` module that compiles with placeholder public API surface.
- [ ] Add a minimal SwiftUI app shell for macOS that launches and shows a window.
- [ ] Add a minimal SwiftUI app shell for iOS that launches and shows a screen.
- [ ] Archive or remove `cmd/`, `internal/`, `apps/`, `packages/`, `web/`, `tui/`, `infra/`, `db/`, `go.mod`, `go.sum`, `package.json`, `bun.lock`, `docker-compose.yml`, and related config.
- [ ] Update `README.md` with Swift build instructions and project description.

### Exit criteria

- [ ] `swift build` succeeds for all targets on macOS.
- [ ] `swift test` runs and passes (even if only placeholder tests).
- [ ] CI builds green on GitHub Actions.
- [ ] The repo clearly presents itself as a Swift project, not a Go or TypeScript project.

### Verification

```bash
swift build
swift test
# CI workflow passes on push
```

---

## Phase 2 -- QUIC transport layer with Network.framework

### Objectives

- Implement the QUIC transport abstraction using Network.framework's native QUIC support.
- Support both direct peer-to-peer connections and connections through the relay.
- Handle connection lifecycle, multiplexing, and graceful shutdown.

### Checklist

- [ ] Define a `Transport` protocol in `BoreCore` abstracting over any bidirectional byte stream.
- [ ] Implement `QUICTransport` using `NWProtocolQUIC.Options` and `NWConnection` / `NWListener`.
- [ ] Implement QUIC listener for receiving incoming connections.
- [ ] Implement QUIC dialer for initiating outgoing connections.
- [ ] Add TLS configuration for QUIC using self-signed certificates or pre-shared keys.
- [ ] Implement connection quality metrics (throughput, bytes transferred, RTT where available).
- [ ] Add STUN client for NAT discovery (determine public IP and port).
- [ ] Add UDP hole-punching logic for direct peer-to-peer path establishment.
- [ ] Implement `WebSocketTransport` as the relay fallback path.
- [ ] Implement `TransportSelector` that tries direct QUIC first, falls back to WebSocket relay.
- [ ] Add unit tests for transport abstraction, STUN client, and connection lifecycle.

### Exit criteria

- [ ] Two peers can establish a direct QUIC connection on a local network.
- [ ] STUN discovery correctly identifies public addresses.
- [ ] Hole-punching succeeds for common NAT types.
- [ ] WebSocket fallback connects through a relay when direct fails.
- [ ] `TransportSelector` correctly sequences direct-then-relay with diagnostic metadata.

### Verification

```bash
swift test --filter BoreCoreTests.TransportTests
swift test --filter BoreCoreTests.STUNTests
# Manual: two machines on same LAN establish direct QUIC connection
# Manual: two machines behind NAT attempt hole-punch, verify fallback
```

---

## Phase 3 -- End-to-end encryption with CryptoKit

### Objectives

- Implement the Noise-equivalent handshake and secure channel using CryptoKit primitives.
- Preserve the security properties from the Go implementation: forward secrecy, PSK from rendezvous code, authenticated encryption.
- Keep the encryption layer transport-agnostic.

### Checklist

- [ ] Implement rendezvous code generation: cryptographic random bytes encoded as human-readable words.
- [ ] Implement rendezvous code parsing and validation.
- [ ] Derive PSK from rendezvous code using HKDF-SHA256 (`CryptoKit.HKDF`).
- [ ] Implement Noise XX-like handshake using Curve25519 (`CryptoKit.Curve25519.KeyAgreement`).
- [ ] Mix PSK into handshake transcript (equivalent to Noise XXpsk0 pattern).
- [ ] Derive session encryption keys from handshake output using HKDF.
- [ ] Implement encrypted framing using ChaChaPoly (`CryptoKit.ChaChaPoly`) with counter-based nonces.
- [ ] Implement `SecureChannel` that wraps any `Transport` with encryption/decryption.
- [ ] Add SHA-256 integrity verification for transferred files.
- [ ] Port or rewrite the rendezvous code word list and entropy model.
- [ ] Add unit tests for handshake, key derivation, encryption/decryption round-trips.
- [ ] Add test vectors to verify compatibility or document the break from Go wire format.

### Exit criteria

- [ ] Handshake completes over any `Transport` and produces a working `SecureChannel`.
- [ ] Encrypted data is not readable without the correct rendezvous code.
- [ ] Forward secrecy holds: each session uses ephemeral keys.
- [ ] File integrity is verified by SHA-256 after decryption.
- [ ] The crypto module has no third-party dependencies beyond CryptoKit.

### Verification

```bash
swift test --filter BoreCoreTests.CryptoTests
swift test --filter BoreCoreTests.HandshakeTests
swift test --filter BoreCoreTests.SecureChannelTests
# Review: crypto implementation matches SECURITY.md threat model
```

---

## Phase 4 -- Vapor relay server for signaling and fallback

### Objectives

- Build a self-hostable relay server in Swift using Vapor.
- Provide signaling for peer candidate exchange and fallback byte forwarding.
- Preserve the payload-blind relay model from v1.

### Checklist

- [ ] Add Vapor as a dependency in `Package.swift` for the `BoreRelay` target only.
- [ ] Implement WebSocket `/signal` endpoint for candidate exchange between peers.
- [ ] Implement WebSocket `/ws` endpoint for fallback encrypted byte forwarding.
- [ ] Implement room registry: create, join, pair, expire with TTL.
- [ ] Add `/healthz` health check endpoint.
- [ ] Add `/status` operator endpoint with room counts and uptime.
- [ ] Add per-IP rate limiting on `/ws` and `/signal` endpoints.
- [ ] Add configurable HTTP timeouts (read, write, idle).
- [ ] Add WebSocket frame size limits.
- [ ] Add structured JSON logging.
- [ ] Add `Dockerfile` for the relay server.
- [ ] Add configuration via environment variables (bind address, port, rate limits, TTLs).
- [ ] Add unit tests for room lifecycle, rate limiting, and signaling flow.

### Exit criteria

- [ ] The relay server builds as a single Swift binary via `swift build`.
- [ ] Two clients can exchange candidates through `/signal`.
- [ ] Fallback byte forwarding works through `/ws` without payload inspection.
- [ ] Rooms expire after TTL. Concurrent room count is bounded.
- [ ] Rate limiting prevents abuse on public-facing endpoints.
- [ ] The relay runs in Docker with a single `docker run` command.

### Verification

```bash
swift build --target BoreRelay
swift test --filter BoreRelayTests
# Manual: start relay, connect two clients, verify signaling
# Manual: verify rate limiting rejects excessive connections
# Docker: docker build -t bore-relay . && docker run -p 8080:8080 bore-relay
```

---

## Phase 5 -- Core transfer engine (send/receive/resume)

### Objectives

- Implement the file transfer framing, streaming, and integrity verification.
- Support resumable transfers with on-disk checkpoint state.
- Keep the engine transport-agnostic: it operates over `SecureChannel`.

### Checklist

- [ ] Define transfer protocol messages: header, resume offer, chunk, end, error.
- [ ] Implement sender-side file streaming with chunked reads.
- [ ] Implement receiver-side reassembly with sequential chunk writes.
- [ ] Implement resume state persistence: JSON checkpoint + partial file on disk.
- [ ] Implement resume negotiation: sender offers, receiver responds with offset.
- [ ] Add SHA-256 final integrity check on completed receive.
- [ ] Add transfer progress reporting via async stream or callback.
- [ ] Add transfer cancellation support.
- [ ] Clean up resume state and partial files on successful completion.
- [ ] Clean up resume state on integrity verification failure.
- [ ] Add unit tests for framing, chunking, resume logic, and integrity verification.
- [ ] Add integration test: full send-receive cycle over a loopback transport.

### Exit criteria

- [ ] A file transfers correctly end-to-end through `SecureChannel` over loopback.
- [ ] Transfer resumes correctly after interruption mid-stream.
- [ ] SHA-256 integrity check passes on completed transfers.
- [ ] Corrupted transfers are detected and rejected.
- [ ] Resume state is cleaned up after success or unrecoverable failure.

### Verification

```bash
swift test --filter BoreCoreTests.TransferEngineTests
swift test --filter BoreCoreTests.ResumeTests
# Integration: send a large file, interrupt mid-transfer, resume, verify hash
```

---

## Phase 6 -- Native UI with SwiftUI (macOS + iOS)

### Objectives

- Build the native macOS and iOS applications with SwiftUI.
- macOS: window-based UI with drag-and-drop file selection.
- iOS: compact UI with share sheet integration and document picker.
- Both platforms: transfer progress, peer status, and transfer history.

### Checklist

- [ ] macOS app: main window with send/receive mode toggle.
- [ ] macOS app: drag-and-drop zone for file selection in send mode.
- [ ] macOS app: file picker fallback for send mode.
- [ ] macOS app: rendezvous code display with copy-to-clipboard.
- [ ] macOS app: code entry field for receive mode.
- [ ] macOS app: transfer progress bar with speed and ETA.
- [ ] macOS app: menu bar status item for background transfers (optional).
- [ ] iOS app: tab or navigation-based send/receive flow.
- [ ] iOS app: document picker for file selection.
- [ ] iOS app: share sheet extension for sending files from other apps.
- [ ] iOS app: code entry with large tap targets and paste support.
- [ ] iOS app: transfer progress with background task support.
- [ ] Shared: transfer history view (recent transfers, stored locally).
- [ ] Shared: settings view (relay URL, default save location, appearance).
- [ ] Shared: error states and retry UI for failed transfers.
- [ ] Shared: accessibility labels and VoiceOver support.
- [ ] Add SwiftUI preview providers for all major views.
- [ ] Add UI tests for core send and receive flows.

### Exit criteria

- [ ] macOS app sends a file via drag-and-drop and displays the rendezvous code.
- [ ] macOS app receives a file by entering a code and saves to the chosen location.
- [ ] iOS app sends a file from the document picker and displays the rendezvous code.
- [ ] iOS app receives a file by entering a code.
- [ ] Transfer progress is visible and accurate on both platforms.
- [ ] The UI handles errors, cancellation, and retry gracefully.

### Verification

```bash
xcodebuild test -scheme BoreMac -destination 'platform=macOS'
xcodebuild test -scheme BoreIOS -destination 'platform=iOS Simulator,name=iPhone 16'
# Manual: drag a file onto the macOS window, send, receive on another machine
# Manual: use share sheet on iOS to send a photo, receive on macOS
```

---

## Phase 7 -- Local peer discovery and Multipeer Connectivity

### Objectives

- Enable automatic peer discovery on local networks using Bonjour/mDNS.
- Integrate Multipeer Connectivity as an optional high-speed local transport.
- Allow peers on the same network to transfer files without a relay or rendezvous code.

### Checklist

- [ ] Implement Bonjour service advertisement using `NWListener` with a Bore service type (e.g., `_bore._udp`).
- [ ] Implement Bonjour service browsing using `NWBrowser` to discover local peers.
- [ ] Display discovered local peers in the UI with device name and availability.
- [ ] Implement direct local transfer to a discovered peer without rendezvous code entry.
- [ ] Integrate Multipeer Connectivity framework as an alternative local transport.
- [ ] Add UI for accepting or declining incoming local transfer requests.
- [ ] Add permission prompts for local network access (iOS requires `NSLocalNetworkUsageDescription`).
- [ ] Handle peer disappearance and connection failures gracefully.
- [ ] Add unit tests for discovery service lifecycle.
- [ ] Add integration test: two simulators discover each other and transfer a file.

### Exit criteria

- [ ] Two devices on the same network discover each other automatically.
- [ ] A local transfer completes without entering a rendezvous code.
- [ ] Encryption is maintained for local transfers (no plaintext local shortcuts).
- [ ] The UI correctly reflects discovered peers, availability, and transfer state.
- [ ] Local discovery does not interfere with relay-based transfers.

### Verification

```bash
swift test --filter BoreCoreTests.DiscoveryTests
# Manual: two Macs on same WiFi discover each other and transfer a file
# Manual: Mac and iPhone on same WiFi discover each other
# Manual: disable WiFi on one device, verify peer disappears from discovery
```

---

## Phase 8 -- File handling, share sheet, drag-and-drop, and link sharing

### Objectives

- Polish the file handling experience on both platforms.
- Implement Universal Links or custom URL scheme for one-tap receive.
- Integrate deeply with platform conventions: share sheet, drag-and-drop, Finder, Files app.

### Checklist

- [ ] Register `bore://` custom URL scheme for receive links (e.g., `bore://<relay>/<code>`).
- [ ] Implement Associated Domains for Universal Links (e.g., `https://bore.pub/r/<code>`).
- [ ] Generate shareable links from the send UI (copy, share sheet, AirDrop the link).
- [ ] Handle incoming links to auto-populate relay and code in receive flow.
- [ ] macOS: register as a drag-and-drop destination for files from Finder.
- [ ] macOS: support sending multiple files (or a directory) as a single transfer.
- [ ] iOS: share sheet extension that accepts files, images, and documents.
- [ ] iOS: receive into Files app or photo library as appropriate.
- [ ] Add file type detection and appropriate icons in transfer UI.
- [ ] Add support for large files with streaming (no full-file memory buffering).
- [ ] Handle sandboxed file access correctly on both platforms (security-scoped bookmarks on macOS).
- [ ] Add unit tests for URL scheme parsing and link generation.

### Exit criteria

- [ ] Tapping a `bore://` link on iOS opens the app and starts receiving.
- [ ] Clicking a Universal Link on macOS opens the app and starts receiving.
- [ ] Share sheet on iOS can send any file type through Bore.
- [ ] Drag-and-drop on macOS initiates a send for the dropped files.
- [ ] Large files (1GB+) transfer without excessive memory usage.

### Verification

```bash
swift test --filter BoreCoreTests.LinkHandlingTests
# Manual: generate a bore:// link, open on iOS, verify receive flow starts
# Manual: drag a 2GB file onto macOS app, send to iOS, verify integrity
# Manual: use iOS share sheet from Photos to send an image via Bore
```

---

## Phase 9 -- Polish, testing, and hardening

### Objectives

- Reach production quality across both platforms.
- Harden security, handle edge cases, and verify all security properties.
- Comprehensive test coverage for core, transport, crypto, and UI.

### Checklist

- [ ] Security audit of CryptoKit handshake implementation against Noise spec.
- [ ] Verify forward secrecy, PSK binding, and authenticated encryption properties.
- [ ] Fuzz transfer protocol message parsing.
- [ ] Test NAT traversal across common NAT types (full cone, restricted, symmetric).
- [ ] Test relay fallback under various network conditions.
- [ ] Test resume after app backgrounding (iOS) and sleep (macOS).
- [ ] Test with very large files (10GB+), very small files (0 bytes), and Unicode filenames.
- [ ] Test concurrent transfers.
- [ ] Add rate limiting and abuse prevention in the client for malformed peer messages.
- [ ] Review and harden all `Info.plist` privacy descriptions.
- [ ] Add App Transport Security exceptions only where required (relay HTTP for local dev).
- [ ] Performance profiling: memory, CPU, battery impact during transfer.
- [ ] Accessibility audit: VoiceOver, Dynamic Type, reduced motion.
- [ ] Localization infrastructure (strings files) even if only English ships initially.
- [ ] Update `SECURITY.md` to reflect the Swift implementation's security properties.
- [ ] Update `ARCHITECTURE.md` to describe the Swift architecture.
- [ ] Update `README.md` with final build, install, and usage instructions.

### Exit criteria

- [ ] All unit and integration tests pass.
- [ ] No known security regressions from the Go implementation.
- [ ] App runs without crashes or memory leaks across a 24-hour soak test.
- [ ] Accessibility audit passes with no critical issues.
- [ ] Documentation reflects the shipped Swift product, not the legacy Go/TS codebase.

### Verification

```bash
swift test
xcodebuild test -scheme BoreMac -destination 'platform=macOS'
xcodebuild test -scheme BoreIOS -destination 'platform=iOS Simulator,name=iPhone 16'
# Instruments: memory leaks profile during large transfer
# Instruments: time profiler during transfer for CPU hot spots
# Manual: VoiceOver walkthrough of send and receive flows
```

---

## Phase 10 -- Deployment, distribution, and launch

### Objectives

- Ship Bore to the Mac App Store and iOS App Store.
- Ship the relay server as a Docker image and standalone binary.
- Publish documentation and a public relay for default use.

### Checklist

- [ ] Configure App Store Connect for Bore (macOS and iOS).
- [ ] Add app icons, screenshots, and App Store metadata.
- [ ] Configure code signing, provisioning profiles, and entitlements.
- [ ] Enable App Sandbox on macOS with appropriate entitlements (network, file access).
- [ ] Submit for App Store review.
- [ ] Publish relay Docker image to GitHub Container Registry.
- [ ] Publish relay binary via GitHub Releases.
- [ ] Set up a default public relay (e.g., `relay.bore.pub`).
- [ ] Configure Associated Domains for Universal Links on the public relay domain.
- [ ] Add Homebrew formula or Cask for macOS CLI/app distribution outside the App Store.
- [ ] Write launch blog post or announcement.
- [ ] Tag release with semantic version.
- [ ] Archive the legacy Go/TS code in a `legacy/` branch or separate archive repo.
- [ ] Update `CHANGELOG.md` with the Swift rewrite release notes.

### Exit criteria

- [ ] Bore is available on the Mac App Store and iOS App Store.
- [ ] The relay Docker image runs with `docker run ghcr.io/dunamismax/bore-relay`.
- [ ] The default public relay is reachable and passes health checks.
- [ ] Universal Links resolve correctly through the public relay domain.
- [ ] A new user can install, send, and receive a file within 2 minutes.

### Verification

```bash
# App Store: download on a fresh device, complete a transfer
# Docker: docker run ghcr.io/dunamismax/bore-relay, verify /healthz
# Universal Links: open https://bore.pub/r/<code> on iOS, verify app opens
# Homebrew: brew install bore, verify CLI or app launches
# Fresh clone: git clone, swift build, swift test -- all green
```

---

## Cross-phase verification gates

- [ ] Each completed phase updates code, docs, and this file together.
- [ ] Each completed phase leaves the repo with `swift build` and `swift test` passing.
- [ ] No phase is complete until its exit criteria and verification checkboxes are both satisfied.
- [ ] Security-relevant phases (3, 4, 9) require explicit review of `SECURITY.md` alignment.
- [ ] UI phases (6, 7, 8) require manual testing on both macOS and iOS before sign-off.
- [ ] The relay server (Phase 4) must be independently deployable before UI work begins.

## Definition of done for any completed phase

- [ ] Work items are reflected in repo reality, not just in chat or docs.
- [ ] Exit criteria are satisfied for the completed phase.
- [ ] Verification expectations are checked off honestly.
- [ ] `BUILD.md` reflects the current truth at the same time as the repo change.
- [ ] `swift build` and `swift test` pass at the repo root.

## When to retire this file

- [ ] Retire `BUILD.md` only after Bore is no longer in an active rewrite program.
- [ ] Move enduring current-state guidance into `README.md`, `ARCHITECTURE.md`, `SECURITY.md`, and `CONTRIBUTING.md`.
- [ ] Delete the file only when the repo documents one current architecture and no longer needs a phase tracker.
