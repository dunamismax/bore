use anyhow::Result;
use bore_core::{ALL_COMPONENTS, project_snapshot};
use clap::{Parser, Subcommand};
use tracing::info;
use tracing_subscriber::EnvFilter;

#[derive(Debug, Parser)]
#[command(
    name = "bore",
    version,
    about = "Privacy-first file transfer. No accounts, no cloud, no trust required.",
    long_about = "bore transfers files between two machines using short, human-readable codes.\n\n\
        End-to-end encrypted. Direct when possible, relayed when necessary.\n\
        The relay learns nothing about your files.\n\n\
        Currently in Phase 1 — protocol design and type foundations.\n\
        The transfer engine is not yet implemented."
)]
struct Cli {
    #[command(subcommand)]
    command: Option<Command>,

    /// Enable verbose output (set BORE_LOG=debug for more detail).
    #[arg(short, long, global = true)]
    verbose: bool,
}

#[derive(Debug, Subcommand)]
enum Command {
    /// Print the current project status and roadmap position.
    Status,

    /// Print the planned component map for the workspace.
    Components,

    // ------------------------------------------------------------------
    // Future commands (not yet implemented, listed for design reference)
    // ------------------------------------------------------------------
    /// Send files or directories to a receiver.
    ///
    /// Generates a short code for the receiver to use.
    #[command(hide = true)]
    Send {
        /// Path to the file or directory to send.
        path: String,
        /// Relay server URL (default: built-in public relay).
        #[arg(long)]
        relay: Option<String>,
        /// Number of code words (2-5, default: 3).
        #[arg(long, default_value = "3")]
        words: u8,
    },

    /// Receive files using a code from the sender.
    #[command(hide = true)]
    Receive {
        /// The code provided by the sender.
        code: String,
        /// Output directory (default: current directory).
        #[arg(short, long)]
        output: Option<String>,
        /// Resume an interrupted transfer.
        #[arg(long)]
        resume: bool,
    },

    /// Show transfer history.
    #[command(hide = true)]
    History,

    /// Run a bore relay server.
    #[command(hide = true)]
    Relay {
        /// Port to listen on.
        #[arg(short, long, default_value = "8080")]
        port: u16,
    },
}

fn main() -> Result<()> {
    let cli = Cli::parse();

    // Initialize tracing subscriber.
    // Use BORE_LOG env var for filter, defaulting to "warn" (or "info" with --verbose).
    let default_filter = if cli.verbose { "info" } else { "warn" };
    let filter =
        EnvFilter::try_from_env("BORE_LOG").unwrap_or_else(|_| EnvFilter::new(default_filter));

    tracing_subscriber::fmt()
        .with_env_filter(filter)
        .with_target(false)
        .init();

    info!("bore starting (phase: {})", bore_core::CURRENT_PHASE);

    match cli.command.unwrap_or(Command::Status) {
        Command::Status => print_status(),
        Command::Components => print_components(),
        Command::Send { .. } => {
            eprintln!("bore send is not yet implemented (planned for Phase 3-4)");
            eprintln!("run `bore status` to see current project state");
            std::process::exit(1);
        }
        Command::Receive { .. } => {
            eprintln!("bore receive is not yet implemented (planned for Phase 3-4)");
            eprintln!("run `bore status` to see current project state");
            std::process::exit(1);
        }
        Command::History => {
            eprintln!("bore history is not yet implemented (planned for Phase 7)");
            std::process::exit(1);
        }
        Command::Relay { .. } => {
            eprintln!("bore relay is not yet implemented (planned for Phase 6)");
            std::process::exit(1);
        }
    }

    Ok(())
}

fn print_status() {
    let snap = project_snapshot();

    println!("bore");
    println!("====");
    println!();
    println!("  phase:   {}", snap.phase);
    println!("  status:  {}", snap.status);
    println!("  mission: {}", snap.mission);
    println!();

    println!("  implemented:");
    for item in snap.implemented_now {
        println!("    - {item}");
    }

    println!();
    println!("  not yet built:");
    for item in snap.explicitly_not_implemented {
        println!("    - {item}");
    }

    println!();
    println!("  next:");
    for item in snap.next_focus {
        println!("    - {item}");
    }
}

fn print_components() {
    println!("bore components");
    println!("===============");
    println!();

    for component in ALL_COMPONENTS {
        println!("  {} ({})", component.name(), component.current_state());
        println!("    {}", component.description());
        println!();
    }
}
