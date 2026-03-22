//! Transfer model types for bore.
//!
//! These types describe what is being transferred, how it is chunked, and how
//! integrity is verified. They are pure data — no IO, no filesystem access.
//!
//! Note: these are the domain types. The actual chunking engine and streaming
//! implementation will come in Phase 3.

use std::path::PathBuf;

use serde::{Deserialize, Serialize};

// ---------------------------------------------------------------------------
// Transfer intent
// ---------------------------------------------------------------------------

/// What the sender wants to transfer.
///
/// Created locally by the sender before any network activity.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct TransferIntent {
    /// Human-readable name for this transfer (e.g., file name or directory name).
    pub name: String,
    /// The entries to transfer.
    pub entries: Vec<FileEntry>,
    /// Total size in bytes across all entries.
    pub total_bytes: u64,
}

impl TransferIntent {
    /// Number of files in this transfer.
    pub fn file_count(&self) -> usize {
        self.entries.len()
    }

    /// Human-readable total size.
    pub fn human_size(&self) -> String {
        human_bytes(self.total_bytes)
    }
}

// ---------------------------------------------------------------------------
// File entry
// ---------------------------------------------------------------------------

/// A single file or directory entry in a transfer manifest.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct FileEntry {
    /// Path relative to the transfer root (never absolute, never starts with `..`).
    pub relative_path: PathBuf,
    /// Type of this entry.
    pub entry_type: EntryType,
    /// Size in bytes (0 for directories).
    pub size: u64,
    /// Whether this entry should be executable on the receiving side (Unix only).
    pub executable: bool,
}

/// The type of a transfer entry.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum EntryType {
    /// A regular file.
    File,
    /// A directory (will be created on the receiving side).
    Directory,
}

impl EntryType {
    pub const fn as_str(self) -> &'static str {
        match self {
            Self::File => "file",
            Self::Directory => "directory",
        }
    }
}

// ---------------------------------------------------------------------------
// Chunk model
// ---------------------------------------------------------------------------

/// Default chunk size: 256 KiB.
pub const DEFAULT_CHUNK_SIZE: u32 = 256 * 1024;

/// A chunk of a file being transferred.
///
/// Each chunk is independently verifiable. The receiver checks integrity
/// per-chunk and can report failures immediately.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct Chunk {
    /// Index of this chunk within the file (0-based).
    pub index: u64,
    /// Byte offset within the file.
    pub offset: u64,
    /// Length of this chunk in bytes.
    pub length: u32,
}

// ---------------------------------------------------------------------------
// Transfer progress
// ---------------------------------------------------------------------------

/// Snapshot of transfer progress at a point in time.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct TransferProgress {
    /// Total bytes expected across all files.
    pub total_bytes: u64,
    /// Bytes transferred so far.
    pub bytes_transferred: u64,
    /// Total number of files.
    pub total_files: usize,
    /// Files completed so far.
    pub files_completed: usize,
    /// Current file being transferred (if any).
    pub current_file: Option<PathBuf>,
}

impl TransferProgress {
    /// Progress as a fraction (0.0 to 1.0).
    pub fn fraction(&self) -> f64 {
        if self.total_bytes == 0 {
            return 1.0;
        }
        self.bytes_transferred as f64 / self.total_bytes as f64
    }

    /// Progress as a percentage (0 to 100).
    pub fn percent(&self) -> u8 {
        (self.fraction() * 100.0) as u8
    }
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

/// Format bytes as a human-readable string (e.g., "1.5 MB").
pub fn human_bytes(bytes: u64) -> String {
    const UNITS: &[&str] = &["B", "KB", "MB", "GB", "TB"];

    if bytes == 0 {
        return "0 B".to_string();
    }

    let mut size = bytes as f64;
    let mut unit_index = 0;

    while size >= 1024.0 && unit_index < UNITS.len() - 1 {
        size /= 1024.0;
        unit_index += 1;
    }

    if unit_index == 0 {
        format!("{bytes} B")
    } else {
        format!("{size:.1} {}", UNITS[unit_index])
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn human_bytes_formatting() {
        assert_eq!(human_bytes(0), "0 B");
        assert_eq!(human_bytes(512), "512 B");
        assert_eq!(human_bytes(1024), "1.0 KB");
        assert_eq!(human_bytes(1536), "1.5 KB");
        assert_eq!(human_bytes(1_048_576), "1.0 MB");
        assert_eq!(human_bytes(1_073_741_824), "1.0 GB");
    }

    #[test]
    fn transfer_progress_fraction() {
        let progress = TransferProgress {
            total_bytes: 1000,
            bytes_transferred: 500,
            total_files: 2,
            files_completed: 1,
            current_file: Some(PathBuf::from("test.txt")),
        };
        assert!((progress.fraction() - 0.5).abs() < f64::EPSILON);
        assert_eq!(progress.percent(), 50);
    }

    #[test]
    fn transfer_progress_zero_total() {
        let progress = TransferProgress {
            total_bytes: 0,
            bytes_transferred: 0,
            total_files: 0,
            files_completed: 0,
            current_file: None,
        };
        assert!((progress.fraction() - 1.0).abs() < f64::EPSILON);
    }

    #[test]
    fn transfer_intent_file_count() {
        let intent = TransferIntent {
            name: "photos".to_string(),
            entries: vec![
                FileEntry {
                    relative_path: PathBuf::from("a.jpg"),
                    entry_type: EntryType::File,
                    size: 1000,
                    executable: false,
                },
                FileEntry {
                    relative_path: PathBuf::from("b.jpg"),
                    entry_type: EntryType::File,
                    size: 2000,
                    executable: false,
                },
            ],
            total_bytes: 3000,
        };
        assert_eq!(intent.file_count(), 2);
        assert_eq!(intent.human_size(), "2.9 KB");
    }

    #[test]
    fn transfer_intent_serde_round_trip() {
        let intent = TransferIntent {
            name: "test-transfer".to_string(),
            entries: vec![
                FileEntry {
                    relative_path: PathBuf::from("file.txt"),
                    entry_type: EntryType::File,
                    size: 4096,
                    executable: false,
                },
                FileEntry {
                    relative_path: PathBuf::from("subdir"),
                    entry_type: EntryType::Directory,
                    size: 0,
                    executable: false,
                },
            ],
            total_bytes: 4096,
        };
        let json = serde_json::to_string(&intent).unwrap();
        let back: TransferIntent = serde_json::from_str(&json).unwrap();
        assert_eq!(intent, back);
    }

    #[test]
    fn chunk_serde_round_trip() {
        let chunk = Chunk {
            index: 7,
            offset: 1_835_008,
            length: 262_144,
        };
        let json = serde_json::to_string(&chunk).unwrap();
        let back: Chunk = serde_json::from_str(&json).unwrap();
        assert_eq!(chunk, back);
    }

    #[test]
    fn transfer_progress_serde_round_trip() {
        let progress = TransferProgress {
            total_bytes: 10_000,
            bytes_transferred: 5_000,
            total_files: 3,
            files_completed: 1,
            current_file: Some(PathBuf::from("photo.jpg")),
        };
        let json = serde_json::to_string(&progress).unwrap();
        let back: TransferProgress = serde_json::from_str(&json).unwrap();
        assert_eq!(progress, back);
    }
}
