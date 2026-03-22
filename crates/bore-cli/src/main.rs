use anyhow::Result;
use bore_core::{PlannedComponent, project_snapshot};
use clap::{Parser, Subcommand};

#[derive(Debug, Parser)]
#[command(
    name = "bore",
    version,
    about = "Phase-0 CLI scaffold for a privacy-first file transfer tool"
)]
struct Cli {
    #[command(subcommand)]
    command: Option<Command>,
}

#[derive(Debug, Subcommand)]
enum Command {
    /// Print the current project status.
    Status,
    /// Print the planned component map for the workspace.
    Components,
}

fn main() -> Result<()> {
    let cli = Cli::parse();

    match cli.command.unwrap_or(Command::Status) {
        Command::Status => print_status(),
        Command::Components => print_components(),
    }

    Ok(())
}

fn print_status() {
    let snapshot = project_snapshot();

    println!("{}", snapshot.name);
    println!("phase: {}", snapshot.phase);
    println!("status: {}", snapshot.status);
    println!("mission: {}", snapshot.mission);
    println!();

    println!("implemented now:");
    for item in snapshot.implemented_now {
        println!("- {item}");
    }

    println!();
    println!("not implemented yet:");
    for item in snapshot.explicitly_not_implemented {
        println!("- {item}");
    }

    println!();
    println!("next focus:");
    for item in snapshot.next_focus {
        println!("- {item}");
    }
}

fn print_components() {
    let components = [
        PlannedComponent::Cli,
        PlannedComponent::Core,
        PlannedComponent::Relay,
    ];

    for component in components {
        println!("{}: {}", component.name(), component.current_state());
    }
}
