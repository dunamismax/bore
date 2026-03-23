// bore -- privacy-first file transfer. No accounts, no cloud, no trust required.
//
// Usage:
//
//	bore send <path> [--relay URL] [--words N]
//	bore receive <code> [--relay URL] [--output DIR]
//	bore status
//	bore components
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/dunamismax/bore/client/internal/code"
	"github.com/dunamismax/bore/client/internal/rendezvous"
)

const version = "0.1.0"

func main() {
	// Global verbose flag (must be parsed before subcommand).
	verbose := false
	for _, arg := range os.Args[1:] {
		if arg == "-v" || arg == "--verbose" || arg == "-verbose" {
			verbose = true
			break
		}
	}

	level := slog.LevelWarn
	if verbose {
		level = slog.LevelInfo
	}
	if os.Getenv("BORE_LOG") == "debug" {
		level = slog.LevelDebug
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})))

	if len(os.Args) < 2 {
		printStatus()
		os.Exit(0)
	}

	subcmd := os.Args[1]
	args := os.Args[2:]

	// Strip global -v/--verbose from subcommand args.
	filtered := args[:0]
	for _, a := range args {
		if a != "-v" && a != "--verbose" && a != "-verbose" {
			filtered = append(filtered, a)
		}
	}
	args = filtered

	switch subcmd {
	case "send":
		if err := cmdSend(args); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
	case "receive", "recv":
		if err := cmdReceive(args); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
	case "status", "":
		printStatus()
	case "components":
		printComponents()
	case "history":
		fmt.Fprintln(os.Stderr, "bore history is not yet implemented")
		os.Exit(1)
	case "relay":
		fmt.Fprintln(os.Stderr, "bore relay is not yet implemented -- use the Go relay server")
		fmt.Fprintln(os.Stderr, "  cd services/relay && go run ./cmd/relay")
		os.Exit(1)
	case "-v", "--verbose", "-verbose":
		// Global flag without subcommand: print status.
		printStatus()
	case "version", "--version", "-version":
		fmt.Println("bore", version)
	case "help", "--help", "-help", "-h":
		printHelp()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %q\n", subcmd)
		fmt.Fprintln(os.Stderr, "Run 'bore help' for usage.")
		os.Exit(1)
	}
}

// ---------------------------------------------------------------------------
// Commands
// ---------------------------------------------------------------------------

func parsePrimaryArg(fs *flag.FlagSet, args []string, usage string) (string, error) {
	primary := ""
	parseArgs := args
	if len(args) > 0 && args[0] != "" && args[0][0] != '-' {
		primary = args[0]
		parseArgs = args[1:]
	}

	if err := fs.Parse(parseArgs); err != nil {
		return "", err
	}
	if primary == "" {
		if fs.NArg() < 1 {
			return "", fmt.Errorf("usage: %s", usage)
		}
		primary = fs.Arg(0)
		if fs.NArg() > 1 {
			return "", fmt.Errorf("unexpected extra arguments: %v", fs.Args()[1:])
		}
	} else if fs.NArg() > 0 {
		return "", fmt.Errorf("unexpected extra arguments: %v", fs.Args())
	}

	return primary, nil
}

func cmdSend(args []string) error {
	fs := flag.NewFlagSet("send", flag.ContinueOnError)
	relayFlag := fs.String("relay", "", "relay server URL (default: "+rendezvous.DefaultRelayURL+")")
	wordsFlag := fs.Int("words", code.DefaultWords, "number of code words (2-5)")

	path, err := parsePrimaryArg(fs, args, "bore send <path> [--relay URL] [--words N]")
	if err != nil {
		return err
	}

	relayURL := *relayFlag
	if relayURL == "" {
		relayURL = rendezvous.DefaultRelayURL
	}
	wordCount := *wordsFlag

	if wordCount < code.MinWords || wordCount > code.MaxWords {
		return fmt.Errorf("--words must be %d-%d, got %d", code.MinWords, code.MaxWords, wordCount)
	}

	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("file not found: %s", path)
		}
		return fmt.Errorf("stat %s: %w", path, err)
	}
	if info.IsDir() {
		return fmt.Errorf("not a file (directory transfer not yet supported): %s", path)
	}

	filename := filepath.Base(path)
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}

	fmt.Fprintf(os.Stderr, "bore send -- %s (%d bytes)\n\n", filename, len(data))

	result, err := rendezvous.SendWithCodeCallback(
		context.Background(),
		relayURL,
		filename,
		data,
		wordCount,
		func(fc code.FullRendezvousCode) {
			fmt.Fprintf(os.Stderr, "Code: %s\n", fc.CodeString())
			if *relayFlag != "" {
				fmt.Fprintf(os.Stderr, "Relay: %s\n", fc.RelayURL)
			}
			fmt.Fprintln(os.Stderr)
			fmt.Fprintln(os.Stderr, "Waiting for receiver...")
		},
	)
	if err != nil {
		return fmt.Errorf("transfer failed: %w", err)
	}

	fmt.Fprintln(os.Stderr)
	fmt.Fprintf(os.Stderr, "Sent: %s (%d bytes, %d chunks)\n",
		result.Transfer.Filename, result.Transfer.Size, result.Transfer.ChunksSent)
	fmt.Fprintf(os.Stderr, "SHA-256: %x\n", result.Transfer.SHA256)
	return nil
}

func cmdReceive(args []string) error {
	fs := flag.NewFlagSet("receive", flag.ContinueOnError)
	relayFlag := fs.String("relay", "", "relay server URL (default: "+rendezvous.DefaultRelayURL+")")
	outputFlag := fs.String("output", ".", "output directory")

	codeStr, err := parsePrimaryArg(fs, args, "bore receive <code> [--relay URL] [--output DIR]")
	if err != nil {
		return err
	}

	relayURL := *relayFlag
	if relayURL == "" {
		relayURL = rendezvous.DefaultRelayURL
	}
	outDir := *outputFlag

	fmt.Fprintln(os.Stderr, "bore receive -- connecting...")
	fmt.Fprintln(os.Stderr)

	result, err := rendezvous.Receive(context.Background(), codeStr, relayURL)
	if err != nil {
		return fmt.Errorf("transfer failed: %w", err)
	}

	outPath := filepath.Join(outDir, result.Transfer.Filename)
	if err := os.WriteFile(outPath, result.Transfer.Data, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", outPath, err)
	}

	fmt.Fprintf(os.Stderr, "Received: %s (%d bytes, %d chunks)\n",
		result.Transfer.Filename, result.Transfer.Size, result.Transfer.ChunksReceived)
	fmt.Fprintf(os.Stderr, "SHA-256: %x\n", result.Transfer.SHA256)
	fmt.Fprintf(os.Stderr, "Saved to: %s\n", outPath)
	return nil
}

// ---------------------------------------------------------------------------
// Status / components
// ---------------------------------------------------------------------------

func printStatus() {
	fmt.Println("bore")
	fmt.Println("====")
	fmt.Println()
	fmt.Println("  version: " + version)
	fmt.Println("  status:  relay-based encrypted file transfer")
	fmt.Println("  mission: privacy-first file transfer. no accounts, no cloud, no trust required.")
	fmt.Println()
	fmt.Println("  implemented:")
	for _, item := range implementedItems {
		fmt.Println("    -", item)
	}
	fmt.Println()
	fmt.Println("  not yet built:")
	for _, item := range notYetBuilt {
		fmt.Println("    -", item)
	}
	fmt.Println()
	fmt.Println("  next:")
	for _, item := range nextFocus {
		fmt.Println("    -", item)
	}
}

func printComponents() {
	fmt.Println("bore components")
	fmt.Println("===============")
	fmt.Println()
	for _, c := range components {
		fmt.Printf("  %s (%s)\n", c.name, c.state)
		fmt.Printf("    %s\n", c.desc)
		fmt.Println()
	}
}

func printHelp() {
	fmt.Println("bore -- privacy-first file transfer")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  bore send <path> [--relay URL] [--words N]")
	fmt.Println("  bore receive <code> [--relay URL] [--output DIR]")
	fmt.Println("  bore status")
	fmt.Println("  bore components")
	fmt.Println()
	fmt.Println("Flags:")
	fmt.Println("  -v, --verbose    enable verbose output")
	fmt.Println()
	fmt.Println("Send flags:")
	fmt.Println("  --relay URL      relay server URL (default: http://localhost:8080)")
	fmt.Println("  --words N        number of code words, 2-5 (default: 3)")
	fmt.Println()
	fmt.Println("Receive flags:")
	fmt.Println("  --relay URL      relay server URL (default: http://localhost:8080)")
	fmt.Println("  --output DIR     output directory (default: current directory)")
	fmt.Println()
	fmt.Println("Environment:")
	fmt.Println("  BORE_LOG=debug   enable debug logging")
}

// ---------------------------------------------------------------------------
// Status data
// ---------------------------------------------------------------------------

var implementedItems = []string{
	"Noise_XXpsk0_25519_ChaChaPoly_SHA256 end-to-end encryption",
	"HKDF-SHA256 PSK derivation from rendezvous code",
	"ChaCha20-Poly1305 AEAD data channel with counter nonces",
	"SHA-256 per-file integrity verification",
	"file transfer with chunking (256 KiB chunks)",
	"human-readable rendezvous codes (2-5 words, 26-50 bits entropy)",
	"WebSocket relay transport (zero-knowledge relay)",
	"bore send / bore receive CLI commands",
}

var notYetBuilt = []string{
	"direct P2P transport (UDP hole-punching)",
	"resumable transfers",
	"directory transfer",
	"transfer history",
	"rate limiting and DoS protection on relay",
	"security audit",
}

var nextFocus = []string{
	"direct P2P path via punchthrough library",
	"relay rate limiting",
	"resumable transfers",
}

type component struct {
	name  string
	state string
	desc  string
}

var components = []component{
	{
		"bore-client (Go)",
		"active",
		"Go client library and CLI: crypto, transfer engine, rendezvous, WebSocket transport",
	},
	{
		"relay",
		"active",
		"Go relay server: zero-knowledge WebSocket stream broker, room registry",
	},
	{
		"punchthrough",
		"active",
		"Go NAT traversal library: STUN probing, UDP hole-punching",
	},
	{
		"bore-admin",
		"active",
		"Go operator CLI: relay status polling over the relay /status endpoint",
	},
}
