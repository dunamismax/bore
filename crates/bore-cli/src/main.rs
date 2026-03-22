use std::path::Path;

use anyhow::{Context, Result};
use bore_core::rendezvous;
use bore_core::{ALL_COMPONENTS, project_snapshot};
use clap::{Parser, Subcommand};
use tracing::info;
use tracing_subscriber::EnvFilter;
use url::Url;

#[derive(Debug, Parser)]
#[command(
    name = "bore",
    version,
    about = "Privacy-first file transfer. No accounts, no cloud, no trust required.",
    long_about = "bore transfers files between two machines using short, human-readable codes.\n\n\
        End-to-end encrypted. Direct when possible, relayed when necessary.\n\
        The relay learns nothing about your files."
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

    /// Send a file to a receiver.
    ///
    /// Generates a short code for the receiver to use. The transfer is
    /// end-to-end encrypted through the relay — the relay cannot read
    /// your files.
    Send {
        /// Path to the file to send.
        path: String,
        /// Relay server URL (default: localhost:8080).
        #[arg(long)]
        relay: Option<String>,
        /// Number of code words (2-5, default: 3).
        #[arg(long, default_value = "3")]
        words: u8,
    },

    /// Receive a file using a code from the sender.
    ///
    /// The transfer is end-to-end encrypted — the relay cannot read the
    /// file contents.
    Receive {
        /// The code provided by the sender.
        code: String,
        /// Output directory (default: current directory).
        #[arg(short, long)]
        output: Option<String>,
        /// Relay server URL (default: localhost:8080).
        #[arg(long)]
        relay: Option<String>,
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
        Command::Send { path, relay, words } => {
            let rt = tokio::runtime::Runtime::new().context("failed to create tokio runtime")?;
            rt.block_on(cmd_send(&path, relay.as_deref(), words))?;
        }
        Command::Receive {
            code,
            output,
            relay,
        } => {
            let rt = tokio::runtime::Runtime::new().context("failed to create tokio runtime")?;
            rt.block_on(cmd_receive(&code, output.as_deref(), relay.as_deref()))?;
        }
        Command::History => {
            eprintln!("bore history is not yet implemented (planned for Phase 7)");
            std::process::exit(1);
        }
        Command::Relay { .. } => {
            eprintln!("bore relay is not yet implemented — use the Go relay server");
            eprintln!("  cd services/relay && go run ./cmd/relay");
            std::process::exit(1);
        }
    }

    Ok(())
}

async fn cmd_send(path: &str, relay: Option<&str>, words: u8) -> Result<()> {
    let relay_url = parse_relay_url(relay)?;
    let file_path = Path::new(path);

    if !file_path.exists() {
        anyhow::bail!("file not found: {}", path);
    }

    if !file_path.is_file() {
        anyhow::bail!(
            "not a file (directory transfer not yet supported): {}",
            path
        );
    }

    let filename = file_path
        .file_name()
        .and_then(|n| n.to_str())
        .context("could not extract filename")?;

    let data =
        std::fs::read(file_path).with_context(|| format!("failed to read file: {}", path))?;

    eprintln!("bore send — {filename} ({} bytes)", data.len());
    eprintln!();

    let result = rendezvous::send_with_code_callback(&relay_url, filename, &data, words, |code| {
        eprintln!("Code: {}", code.code_string());
        if relay.is_some() {
            eprintln!("Relay: {}", code.relay_url);
        }
        eprintln!();
        eprintln!("Waiting for receiver...");
    })
    .await
    .context("transfer failed")?;

    eprintln!();
    eprintln!(
        "Sent: {} ({} bytes, {} chunks)",
        result.transfer.filename, result.transfer.size, result.transfer.chunks_sent
    );
    let hex: String = result
        .transfer
        .sha256
        .iter()
        .map(|b| format!("{b:02x}"))
        .collect();
    eprintln!("SHA-256: {hex}");

    Ok(())
}

async fn cmd_receive(code: &str, output: Option<&str>, relay: Option<&str>) -> Result<()> {
    let relay_url = parse_relay_url(relay)?;

    eprintln!("bore receive — connecting...");
    eprintln!();

    let result = rendezvous::receive(code, &relay_url)
        .await
        .context("transfer failed")?;

    let out_dir = output.unwrap_or(".");
    let out_path = Path::new(out_dir).join(&result.transfer.filename);

    std::fs::write(&out_path, &result.transfer.data)
        .with_context(|| format!("failed to write file: {}", out_path.display()))?;

    eprintln!(
        "Received: {} ({} bytes, {} chunks)",
        result.transfer.filename, result.transfer.size, result.transfer.chunks_received
    );
    let hex: String = result
        .transfer
        .sha256
        .iter()
        .map(|b| format!("{b:02x}"))
        .collect();
    eprintln!("SHA-256: {hex}");
    eprintln!("Saved to: {}", out_path.display());

    Ok(())
}

fn parse_relay_url(relay: Option<&str>) -> Result<Url> {
    let url_str = relay.unwrap_or(rendezvous::DEFAULT_RELAY_URL);
    Url::parse(url_str).with_context(|| format!("invalid relay URL: {url_str}"))
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
