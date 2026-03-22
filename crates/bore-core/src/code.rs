//! Rendezvous code generation and parsing for bore.
//!
//! Codes are human-friendly identifiers used to connect a sender and receiver.
//! They follow the format `{channel}-{word}-{word}-{word}`, for example:
//! `7-guitar-castle-moon`.
//!
//! # Entropy budget
//!
//! The code has two components:
//!
//! - **Channel number**: 1–999, providing ~10 bits of entropy. The channel is
//!   primarily a routing hint for the relay (allowing multiple concurrent
//!   sessions), but it also contributes to the code's overall entropy.
//!
//! - **Words**: drawn from a curated 256-word list. Each word provides 8 bits
//!   of entropy. With 3 words (default), that's 24 bits from words alone.
//!
//! **Total entropy (default, 3 words):** ~34 bits (10 channel + 24 words).
//!
//! **Brute-force resistance:**
//! - 2^34 ≈ 17 billion possible codes
//! - At 1 attempt/second (rate-limited): ~544 years to exhaust
//! - At 100 attempts/second (aggressive): ~5.4 years
//! - Combined with single-use semantics and 5-minute expiry, online brute-force
//!   is not practical
//!
//! **Word count options:**
//! - 2 words: ~26 bits — adequate for casual use with rate limiting
//! - 3 words: ~34 bits — default, good balance of usability and security
//! - 4 words: ~42 bits — high security for sensitive transfers
//! - 5 words: ~50 bits — maximum security
//!
//! # Code lifecycle
//!
//! - Codes are **single-use**: once a receiver connects, the code is consumed
//! - Codes **expire**: default 5 minutes, configurable
//! - Codes are **bound to sessions**: the code is a cryptographic input to PAKE,
//!   not just a routing hint
//!
//! # Design decisions
//!
//! The wordlist is curated for:
//! - **Pronounceability**: easy to read aloud over a phone call
//! - **Low ambiguity**: no homophones, no easily confused words
//! - **Memorability**: concrete nouns and common adjectives
//! - **Typing ease**: short words (3-7 chars), no special characters
//!
//! The 256-word list (8 bits per word) is intentionally smaller than BIP-39's
//! 2048 words. This keeps the list easy to audit, reduces confusion, and makes
//! codes more memorable. The entropy tradeoff is compensated by the channel
//! number and the ability to add more words for sensitive transfers.

use std::fmt;

use serde::{Deserialize, Serialize};

use crate::error::CodeError;

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

/// Minimum number of words in a code.
pub const MIN_WORDS: u8 = 2;

/// Maximum number of words in a code.
pub const MAX_WORDS: u8 = 5;

/// Default number of words in a code.
pub const DEFAULT_WORDS: u8 = 3;

/// Minimum channel number.
pub const MIN_CHANNEL: u16 = 1;

/// Maximum channel number.
pub const MAX_CHANNEL: u16 = 999;

/// Default code expiry in seconds.
pub const DEFAULT_EXPIRY_SECS: u64 = 300; // 5 minutes

/// Bits of entropy per word (log2 of wordlist size).
pub const BITS_PER_WORD: u8 = 8; // 256 words = 2^8

/// Approximate bits of entropy from the channel number.
pub const BITS_CHANNEL: u8 = 10; // ~log2(999)

// ---------------------------------------------------------------------------
// Rendezvous code
// ---------------------------------------------------------------------------

/// A parsed rendezvous code.
///
/// Codes are single-use and expire. The code is cryptographically bound to the
/// session via PAKE — it is not just a routing hint.
#[derive(Debug, Clone, PartialEq, Eq, Hash, Serialize, Deserialize)]
pub struct RendezvousCode {
    /// Channel number (1–999).
    channel: u16,
    /// Words from the wordlist.
    words: Vec<String>,
}

impl RendezvousCode {
    /// Create a new rendezvous code from a channel number and words.
    ///
    /// Validates that the channel is in range and all words are in the wordlist.
    pub fn new(channel: u16, words: Vec<String>) -> std::result::Result<Self, CodeError> {
        if !(MIN_CHANNEL..=MAX_CHANNEL).contains(&channel) {
            return Err(CodeError::InvalidChannel(channel));
        }

        let word_count = words.len();
        if word_count < MIN_WORDS as usize || word_count > MAX_WORDS as usize {
            return Err(CodeError::Malformed(format!(
                "expected {MIN_WORDS}-{MAX_WORDS} words, got {word_count}"
            )));
        }

        for word in &words {
            let lower = word.to_lowercase();
            if !WORDLIST.contains(&lower.as_str()) {
                return Err(CodeError::UnknownWord(word.clone()));
            }
        }

        Ok(Self {
            channel,
            words: words.into_iter().map(|w| w.to_lowercase()).collect(),
        })
    }

    /// Parse a rendezvous code from its string representation.
    ///
    /// Expected format: `{channel}-{word}-{word}-{word}`
    pub fn parse(s: &str) -> std::result::Result<Self, CodeError> {
        let parts: Vec<&str> = s.split('-').collect();

        if parts.len() < (1 + MIN_WORDS as usize) {
            return Err(CodeError::Malformed(format!(
                "code must have at least a channel and {MIN_WORDS} words"
            )));
        }

        let channel: u16 = parts[0]
            .parse()
            .map_err(|_| CodeError::Malformed(format!("invalid channel number: '{}'", parts[0])))?;

        let words: Vec<String> = parts[1..].iter().map(|w| (*w).to_string()).collect();

        Self::new(channel, words)
    }

    /// Returns the channel number.
    pub fn channel(&self) -> u16 {
        self.channel
    }

    /// Returns the words in the code.
    pub fn words(&self) -> &[String] {
        &self.words
    }

    /// Returns the number of words in the code.
    pub fn word_count(&self) -> usize {
        self.words.len()
    }

    /// Returns the total estimated entropy in bits.
    pub fn entropy_bits(&self) -> u32 {
        u32::from(BITS_CHANNEL) + u32::from(BITS_PER_WORD) * self.words.len() as u32
    }
}

impl fmt::Display for RendezvousCode {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "{}", self.channel)?;
        for word in &self.words {
            write!(f, "-{word}")?;
        }
        Ok(())
    }
}

// ---------------------------------------------------------------------------
// Code generation (deterministic, from provided randomness)
// ---------------------------------------------------------------------------

/// Generate a rendezvous code from raw random bytes.
///
/// This function is deterministic given the same random input. The caller
/// is responsible for providing cryptographically random bytes.
///
/// Required bytes: 2 (channel) + 1 per word.
pub fn code_from_random_bytes(
    random: &[u8],
    word_count: u8,
) -> std::result::Result<RendezvousCode, CodeError> {
    if !(MIN_WORDS..=MAX_WORDS).contains(&word_count) {
        return Err(CodeError::Malformed(format!(
            "word count must be {MIN_WORDS}-{MAX_WORDS}, got {word_count}"
        )));
    }

    let needed = 2 + word_count as usize;
    if random.len() < needed {
        return Err(CodeError::Malformed(format!(
            "need at least {needed} random bytes, got {}",
            random.len()
        )));
    }

    // Channel from first 2 bytes, mapped to 1–999
    let raw_channel = u16::from_be_bytes([random[0], random[1]]);
    let channel = (raw_channel % MAX_CHANNEL) + MIN_CHANNEL;

    // Words from subsequent bytes
    let words: Vec<String> = random[2..2 + word_count as usize]
        .iter()
        .map(|&byte| WORDLIST[byte as usize].to_string())
        .collect();

    // Skip validation since we're constructing from known-good data
    Ok(RendezvousCode { channel, words })
}

// ---------------------------------------------------------------------------
// Code lifetime types (design only — enforcement is in the relay/transport)
// ---------------------------------------------------------------------------

/// Describes the lifetime policy for a rendezvous code.
///
/// These are type-level constraints. Enforcement happens in the relay
/// and session management layers (Phase 4+).
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub struct CodeLifetime {
    /// Maximum time in seconds before the code expires.
    pub expiry_secs: u64,
    /// Whether the code is single-use (consumed after first successful connection).
    pub single_use: bool,
}

impl Default for CodeLifetime {
    fn default() -> Self {
        Self {
            expiry_secs: DEFAULT_EXPIRY_SECS,
            single_use: true,
        }
    }
}

// ---------------------------------------------------------------------------
// Wordlist — 256 curated words
// ---------------------------------------------------------------------------

/// Curated wordlist for rendezvous codes.
///
/// 256 words = 8 bits of entropy per word.
///
/// Selection criteria:
/// - Concrete nouns and common adjectives
/// - 3-7 characters for easy typing
/// - No homophones or easily confused words
/// - Easy to pronounce and spell across English accents
/// - No offensive or sensitive terms
pub const WORDLIST: &[&str; 256] = &[
    "acorn", "adrift", "agent", "album", "alert", "amber", "anchor", "angel", "apple", "arena",
    "arrow", "atlas", "badge", "baker", "banjo", "basin", "beach", "beast", "berry", "blade",
    "blank", "blaze", "bloom", "board", "bonus", "booth", "brain", "brave", "brick", "bride",
    "brief", "brook", "brush", "cabin", "camel", "candy", "cargo", "cedar", "chalk", "charm",
    "chase", "chess", "chief", "cider", "civic", "claim", "cliff", "climb", "clock", "cloud",
    "coast", "cobra", "coral", "couch", "crane", "crash", "crown", "crush", "curve", "cycle",
    "dance", "delta", "demon", "denim", "depot", "diary", "disco", "diver", "dodge", "donor",
    "draft", "drain", "dream", "dress", "drift", "drink", "drive", "drone", "drum", "eagle",
    "earth", "elite", "ember", "envoy", "epoch", "event", "extra", "fable", "falcon", "feast",
    "fence", "fiber", "field", "flame", "flask", "fleet", "flint", "flood", "flora", "flute",
    "focal", "forge", "forum", "frame", "fresh", "frost", "fruit", "funds", "gamma", "gauge",
    "ghost", "giant", "glade", "glass", "gleam", "globe", "glyph", "goat", "grace", "grain",
    "graph", "grasp", "green", "grove", "guard", "guide", "guild", "haven", "hawk", "heart",
    "helix", "honey", "horse", "hotel", "human", "humor", "husky", "igloo", "index", "ivory",
    "jazzy", "jewel", "joint", "judge", "juice", "karma", "kayak", "knack", "kneel", "knife",
    "latch", "lemon", "lever", "light", "lilac", "linen", "llama", "lodge", "logic", "lotus",
    "lucky", "lunar", "magic", "major", "mango", "maple", "marsh", "melon", "mercy", "merit",
    "metal", "minor", "model", "moose", "motor", "mouse", "music", "noble", "north", "novel",
    "ocean", "olive", "onion", "opera", "orbit", "organ", "otter", "outer", "oxide", "ozone",
    "panda", "panel", "patch", "pearl", "phase", "piano", "pilot", "pixel", "plain", "plaza",
    "plumb", "plume", "polar", "pond", "prism", "prize", "proxy", "pulse", "quake", "quest",
    "quiet", "quota", "radar", "raven", "rebel", "reign", "ridge", "river", "robin", "robot",
    "royal", "rural", "salon", "satin", "scale", "scout", "shade", "shark", "shelf", "shell",
    "shift", "shine", "sigma", "silk", "siren", "slate", "sleet", "slope", "solar", "spark",
    "spice", "spoke", "squad", "stamp", "steel", "stone", "storm", "story", "sugar", "swift",
    "table", "tango", "tiger", "toast", "token", "tower",
];

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn wordlist_has_256_entries() {
        assert_eq!(WORDLIST.len(), 256);
    }

    #[test]
    fn wordlist_entries_are_lowercase() {
        for word in WORDLIST.iter() {
            assert_eq!(*word, word.to_lowercase(), "word '{word}' is not lowercase");
        }
    }

    #[test]
    fn wordlist_entries_are_unique() {
        let mut seen = std::collections::HashSet::new();
        for word in WORDLIST.iter() {
            assert!(seen.insert(*word), "duplicate word: '{word}'");
        }
    }

    #[test]
    fn wordlist_entries_are_reasonable_length() {
        for word in WORDLIST.iter() {
            assert!(
                word.len() >= 3 && word.len() <= 7,
                "word '{word}' has length {} (expected 3-7)",
                word.len()
            );
        }
    }

    #[test]
    fn wordlist_entries_are_alphabetic() {
        for word in WORDLIST.iter() {
            assert!(
                word.chars().all(|c| c.is_ascii_lowercase()),
                "word '{word}' contains non-lowercase-ascii characters"
            );
        }
    }

    // -----------------------------------------------------------------------
    // Code parsing
    // -----------------------------------------------------------------------

    #[test]
    fn parse_valid_three_word_code() {
        let code = RendezvousCode::parse("7-apple-beach-crown");
        assert!(code.is_ok());
        let code = code.unwrap();
        assert_eq!(code.channel(), 7);
        assert_eq!(code.word_count(), 3);
        assert_eq!(code.words(), &["apple", "beach", "crown"]);
        assert_eq!(code.to_string(), "7-apple-beach-crown");
    }

    #[test]
    fn parse_valid_two_word_code() {
        let code = RendezvousCode::parse("42-delta-storm").unwrap();
        assert_eq!(code.channel(), 42);
        assert_eq!(code.word_count(), 2);
    }

    #[test]
    fn parse_valid_four_word_code() {
        let code = RendezvousCode::parse("100-apple-beach-crown-delta").unwrap();
        assert_eq!(code.channel(), 100);
        assert_eq!(code.word_count(), 4);
    }

    #[test]
    fn parse_valid_five_word_code() {
        let code = RendezvousCode::parse("999-apple-beach-crown-delta-ember").unwrap();
        assert_eq!(code.channel(), 999);
        assert_eq!(code.word_count(), 5);
    }

    #[test]
    fn parse_rejects_channel_zero() {
        let result = RendezvousCode::parse("0-apple-beach-crown");
        assert!(result.is_err());
    }

    #[test]
    fn parse_rejects_channel_too_high() {
        let result = RendezvousCode::parse("1000-apple-beach-crown");
        assert!(result.is_err());
    }

    #[test]
    fn parse_rejects_unknown_word() {
        let result = RendezvousCode::parse("7-apple-xyzzy-crown");
        assert!(matches!(result, Err(CodeError::UnknownWord(_))));
    }

    #[test]
    fn parse_rejects_one_word() {
        let result = RendezvousCode::parse("7-apple");
        assert!(result.is_err());
    }

    #[test]
    fn parse_rejects_empty_string() {
        let result = RendezvousCode::parse("");
        assert!(result.is_err());
    }

    #[test]
    fn parse_rejects_no_channel() {
        let result = RendezvousCode::parse("apple-beach-crown");
        assert!(result.is_err());
    }

    #[test]
    fn parse_case_insensitive() {
        let code = RendezvousCode::parse("7-Apple-BEACH-Crown").unwrap();
        assert_eq!(code.words(), &["apple", "beach", "crown"]);
    }

    // -----------------------------------------------------------------------
    // Code generation
    // -----------------------------------------------------------------------

    #[test]
    fn generate_from_random_bytes() {
        let random = [0x00, 0x07, 0x08, 0x10, 0x20]; // channel bytes + 3 word bytes
        let code = code_from_random_bytes(&random, 3).unwrap();
        assert!(code.channel() >= MIN_CHANNEL);
        assert!(code.channel() <= MAX_CHANNEL);
        assert_eq!(code.word_count(), 3);
        // All words should be from the wordlist
        for word in code.words() {
            assert!(WORDLIST.contains(&word.as_str()));
        }
    }

    #[test]
    fn generate_deterministic() {
        let random = [0xAB, 0xCD, 0x42, 0x99, 0x01];
        let code1 = code_from_random_bytes(&random, 3).unwrap();
        let code2 = code_from_random_bytes(&random, 3).unwrap();
        assert_eq!(code1, code2);
    }

    #[test]
    fn generate_rejects_insufficient_bytes() {
        let random = [0x00, 0x01]; // only 2 bytes, need 2 + word_count
        let result = code_from_random_bytes(&random, 3);
        assert!(result.is_err());
    }

    #[test]
    fn generate_different_word_counts() {
        let random = [0x00, 0x07, 0x08, 0x10, 0x20, 0x30, 0x40];
        for count in MIN_WORDS..=MAX_WORDS {
            let code = code_from_random_bytes(&random, count).unwrap();
            assert_eq!(code.word_count(), count as usize);
        }
    }

    // -----------------------------------------------------------------------
    // Entropy calculation
    // -----------------------------------------------------------------------

    #[test]
    fn entropy_calculation() {
        let code = RendezvousCode::parse("7-apple-beach-crown").unwrap();
        // 10 bits channel + 3 * 8 bits = 34 bits
        assert_eq!(code.entropy_bits(), 34);
    }

    #[test]
    fn entropy_scales_with_words() {
        let random = [0x00, 0x07, 0x08, 0x10, 0x20, 0x30, 0x40];
        let code2 = code_from_random_bytes(&random, 2).unwrap();
        let code3 = code_from_random_bytes(&random, 3).unwrap();
        let code5 = code_from_random_bytes(&random, 5).unwrap();
        assert_eq!(code2.entropy_bits(), 26); // 10 + 2*8
        assert_eq!(code3.entropy_bits(), 34); // 10 + 3*8
        assert_eq!(code5.entropy_bits(), 50); // 10 + 5*8
    }

    // -----------------------------------------------------------------------
    // Serde round-trip
    // -----------------------------------------------------------------------

    #[test]
    fn code_serde_round_trip() {
        let code = RendezvousCode::parse("42-delta-storm-noble").unwrap();
        let json = serde_json::to_string(&code).unwrap();
        let back: RendezvousCode = serde_json::from_str(&json).unwrap();
        assert_eq!(code, back);
    }

    #[test]
    fn code_display_round_trip() {
        let code = RendezvousCode::parse("7-apple-beach-crown").unwrap();
        let displayed = code.to_string();
        let reparsed = RendezvousCode::parse(&displayed).unwrap();
        assert_eq!(code, reparsed);
    }

    // -----------------------------------------------------------------------
    // Code lifetime
    // -----------------------------------------------------------------------

    #[test]
    fn default_code_lifetime() {
        let lifetime = CodeLifetime::default();
        assert_eq!(lifetime.expiry_secs, 300);
        assert!(lifetime.single_use);
    }

    #[test]
    fn code_lifetime_serde_round_trip() {
        let lifetime = CodeLifetime {
            expiry_secs: 600,
            single_use: false,
        };
        let json = serde_json::to_string(&lifetime).unwrap();
        let back: CodeLifetime = serde_json::from_str(&json).unwrap();
        assert_eq!(lifetime, back);
    }
}
