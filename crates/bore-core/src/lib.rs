//! Core library for `bore` — a privacy-first file transfer tool.
//!
//! This crate owns the transfer model, session state, protocol types, codec,
//! rendezvous code system, and domain logic. It is designed to be embedded by
//! any frontend (CLI, GUI, FFI) and contains no IO or platform-specific code
//! in its public API.
//!
//! # Current state
//!
//! Phase 3: transfer engine. Builds on Phase 2's Noise XXpsk0 handshake with
//! PAKE binding to rendezvous codes, SecureChannel with ChaCha20-Poly1305 AEAD
//! encryption, counter-based nonces, and zeroized key material. The transfer
//! engine (chunking, streaming, SHA-256 integrity verification) is implemented
//! and tested. Transport abstraction is not yet implemented.

pub mod code;
pub mod codec;
pub mod crypto;
pub mod engine;
pub mod error;
pub mod protocol;
pub mod session;
pub mod transfer;

// ---------------------------------------------------------------------------
// Project metadata — truthful snapshot of the repo's current state.
// ---------------------------------------------------------------------------

/// Public project name.
pub const PROJECT_NAME: &str = "bore";

/// Current development phase.
pub const CURRENT_PHASE: &str = "phase-3";

/// Human-readable status for the repository today.
pub const CURRENT_STATUS: &str = "Transfer engine implemented. File chunking, SHA-256 integrity verification, encrypted streaming over SecureChannel. Built on Noise XXpsk0 handshake with ChaCha20-Poly1305 AEAD and HKDF-derived PSK.";

/// Short statement of intent.
pub const MISSION: &str = "Privacy-first file transfer with human-friendly rendezvous, end-to-end encryption, and zero-knowledge relay.";

/// Planned workspace component.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum PlannedComponent {
    Cli,
    Core,
    Relay,
}

impl PlannedComponent {
    pub const fn name(self) -> &'static str {
        match self {
            Self::Cli => "bore-cli",
            Self::Core => "bore-core",
            Self::Relay => "bore-relay",
        }
    }

    pub const fn current_state(self) -> &'static str {
        match self {
            Self::Cli => "scaffold — prints project status, tracing subscriber setup",
            Self::Core => "phase-3 — transfer engine, chunking, SHA-256 integrity, crypto layer",
            Self::Relay => "go relay exists — Rust client integration not started",
        }
    }

    pub const fn description(self) -> &'static str {
        match self {
            Self::Cli => "Operator-facing CLI: send, receive, history, relay management",
            Self::Core => "Shared library: transfer model, session state, crypto, protocol codec",
            Self::Relay => "Optional relay server: encrypted traffic forwarding, zero-knowledge",
        }
    }
}

/// All planned components.
pub const ALL_COMPONENTS: [PlannedComponent; 3] = [
    PlannedComponent::Core,
    PlannedComponent::Cli,
    PlannedComponent::Relay,
];

/// Runtime snapshot of the project's current state.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct ProjectSnapshot {
    pub name: &'static str,
    pub phase: &'static str,
    pub status: &'static str,
    pub mission: &'static str,
    pub implemented_now: &'static [&'static str],
    pub explicitly_not_implemented: &'static [&'static str],
    pub next_focus: &'static [&'static str],
}

/// Returns a truthful snapshot of the project's current state.
pub fn project_snapshot() -> ProjectSnapshot {
    ProjectSnapshot {
        name: PROJECT_NAME,
        phase: CURRENT_PHASE,
        status: CURRENT_STATUS,
        mission: MISSION,
        implemented_now: &[
            "Rust workspace (bore-core, bore-cli)",
            "Domain types with serde serialization (session, transfer, protocol, error)",
            "Session state machine with exhaustive transition tests",
            "Protocol message types (Hello, Offer, Accept, Reject, Data, Ack, Done, Error, Close)",
            "Frame codec for wire-format encoding/decoding",
            "Rendezvous code system (256-word list, ~34-bit entropy default)",
            "Noise XXpsk0 handshake with PAKE binding to rendezvous code",
            "SecureChannel with ChaCha20-Poly1305 AEAD encryption",
            "HKDF-SHA256 PSK derivation from rendezvous codes",
            "Counter-based nonces with replay detection (via snow)",
            "Multi-segment framing for payloads larger than 64KB",
            "Key material zeroization (zeroize crate + snow internals)",
            "Rekey support for long-running transfers",
            "Transfer engine with chunking, streaming, SHA-256 integrity verification",
            "Binary wire format for header/chunk/end messages over SecureChannel",
            "Filename validation (path traversal, null bytes, relative components)",
            "Typed error hierarchy using thiserror",
            "Structured tracing subscriber in CLI",
            "Threat model and crypto design documents",
            "CLI with planned command structure",
        ],
        explicitly_not_implemented: &[
            "Direct peer-to-peer transport (TCP, QUIC, hole-punching)",
            "Relay protocol integration from Rust client",
            "Rendezvous code exchange over network",
            "Resumable session state persistence",
            "NAT traversal integration (STUN/TURN, ICE-lite)",
        ],
        next_focus: &[
            "Phase 4: rendezvous and code exchange over network",
            "Phase 5: direct peer-to-peer transport",
            "Phase 6: relay service integration from Rust client",
        ],
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn snapshot_is_truthful() {
        let snap = project_snapshot();
        assert_eq!(snap.name, "bore");
        assert_eq!(snap.phase, "phase-3");
        assert!(!snap.explicitly_not_implemented.is_empty());
        assert!(!snap.next_focus.is_empty());
    }

    #[test]
    fn all_components_listed() {
        assert_eq!(ALL_COMPONENTS.len(), 3);
        assert_eq!(ALL_COMPONENTS[0], PlannedComponent::Core);
        assert_eq!(ALL_COMPONENTS[1], PlannedComponent::Cli);
        assert_eq!(ALL_COMPONENTS[2], PlannedComponent::Relay);
    }

    #[test]
    fn component_metadata_is_non_empty() {
        for component in ALL_COMPONENTS {
            assert!(!component.name().is_empty());
            assert!(!component.current_state().is_empty());
            assert!(!component.description().is_empty());
        }
    }
}
