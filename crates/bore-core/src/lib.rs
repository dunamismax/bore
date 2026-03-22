//! Core project metadata and planning primitives for `bore`.
//!
//! Phase 0 is intentionally small: the crate exposes truthful status and architectural
//! boundaries so the rest of the workspace can grow from a stable, testable base.
//! No transfer protocol, cryptography, relay service, or persistence layer is
//! implemented yet.

/// Public project name.
pub const PROJECT_NAME: &str = "bore";

/// Current development phase.
pub const CURRENT_PHASE: &str = "phase-0";

/// Human-readable posture for the repository today.
pub const CURRENT_STATUS: &str = "Scaffold only. Architecture and docs are in place; secure transfer logic is not implemented yet.";

/// Short statement of intent for the project.
pub const MISSION: &str =
    "Privacy-first file transfer with human-friendly rendezvous and operator-grade growth paths.";

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum PlannedComponent {
    Cli,
    Core,
    Relay,
}

impl PlannedComponent {
    pub const fn name(self) -> &'static str {
        match self {
            Self::Cli => "cli",
            Self::Core => "core",
            Self::Relay => "relay",
        }
    }

    pub const fn current_state(self) -> &'static str {
        match self {
            Self::Cli => "minimal binary scaffold present",
            Self::Core => "minimal library scaffold present",
            Self::Relay => "planned, not started",
        }
    }
}

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

pub fn project_snapshot() -> ProjectSnapshot {
    ProjectSnapshot {
        name: PROJECT_NAME,
        phase: CURRENT_PHASE,
        status: CURRENT_STATUS,
        mission: MISSION,
        implemented_now: &[
            "Rust workspace scaffold",
            "Core crate for shared types and project metadata",
            "CLI crate for operator-facing entry points",
            "Repository docs for direction, constraints, and phased execution",
        ],
        explicitly_not_implemented: &[
            "Cryptographic protocol",
            "Direct peer-to-peer transport",
            "Relay service",
            "Code generation or rendezvous exchange",
            "Persistent transfer state",
        ],
        next_focus: &[
            "Define the transfer model and trust boundaries",
            "Pick initial crypto and capability envelope",
            "Design the human-friendly code flow",
            "Add tests around parsing, state transitions, and protocol framing",
        ],
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn phase_zero_snapshot_is_truthful() {
        let snapshot = project_snapshot();

        assert_eq!(snapshot.name, "bore");
        assert_eq!(snapshot.phase, "phase-0");
        assert!(
            snapshot
                .explicitly_not_implemented
                .contains(&"Cryptographic protocol")
        );
    }
}
