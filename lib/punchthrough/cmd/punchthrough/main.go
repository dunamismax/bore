// Command punchthrough is a CLI for NAT traversal and UDP hole-punching.
//
// Usage:
//
//	punchthrough version    Print version and build info
//	punchthrough probe      Discover NAT type via STUN probing
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/dunamismax/bore/lib/punchthrough/pkg/stun"
)

const version = "0.1.0"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(2)
	}

	switch os.Args[1] {
	case "version":
		cmdVersion()
	case "probe":
		if err := cmdProbe(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", os.Args[1])
		printUsage()
		os.Exit(2)
	}
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `punchthrough — NAT traversal and UDP hole-punching

Usage:
  punchthrough <command> [flags]

Commands:
  version     Print version and build info
  probe       Discover NAT type via STUN probing

Probe flags:
  --stun <server>    STUN server to probe (can be repeated; default: Google, Cloudflare)
  --timeout <dur>    Per-probe timeout (default: 5s)
  --json             Output results as JSON
  --verbose          Enable debug logging
`)
}

func cmdVersion() {
	fmt.Printf("punchthrough %s\n", version)
	fmt.Printf("Phase 1: STUN client and NAT discovery\n")
}

func cmdProbe(args []string) error {
	var (
		stunServers []string
		timeout     = 5 * time.Second
		jsonOutput  bool
		verbose     bool
	)

	// Simple flag parsing — no external deps.
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--stun":
			i++
			if i >= len(args) {
				return fmt.Errorf("--stun requires a server address")
			}
			stunServers = append(stunServers, args[i])
		case "--timeout":
			i++
			if i >= len(args) {
				return fmt.Errorf("--timeout requires a duration")
			}
			d, err := time.ParseDuration(args[i])
			if err != nil {
				return fmt.Errorf("invalid timeout %q: %w", args[i], err)
			}
			timeout = d
		case "--json":
			jsonOutput = true
		case "--verbose":
			verbose = true
		default:
			// Support --stun=server syntax.
			if strings.HasPrefix(args[i], "--stun=") {
				stunServers = append(stunServers, strings.TrimPrefix(args[i], "--stun="))
			} else if strings.HasPrefix(args[i], "--timeout=") {
				d, err := time.ParseDuration(strings.TrimPrefix(args[i], "--timeout="))
				if err != nil {
					return fmt.Errorf("invalid timeout: %w", err)
				}
				timeout = d
			} else {
				return fmt.Errorf("unknown flag: %s", args[i])
			}
		}
	}

	if verbose {
		slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug})))
	}

	cfg := &stun.Config{
		Servers: stunServers,
		Timeout: timeout,
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout*time.Duration(len(cfg.Servers)+3))
	defer cancel()

	result, err := stun.Probe(ctx, cfg)
	if err != nil && result == nil {
		return err
	}

	if jsonOutput {
		return printJSON(result, err)
	}

	return printHuman(result, err)
}

type jsonProbeResult struct {
	NATType    string          `json:"nat_type"`
	PublicAddr string          `json:"public_addr,omitempty"`
	Punchable  bool            `json:"punchable"`
	Duration   string          `json:"duration"`
	Probes     []jsonProbeItem `json:"probes"`
	Error      string          `json:"error,omitempty"`
}

type jsonProbeItem struct {
	Server     string `json:"server"`
	MappedAddr string `json:"mapped_addr,omitempty"`
	Duration   string `json:"duration"`
	Error      string `json:"error,omitempty"`
}

func printJSON(result *stun.ProbeResult, probeErr error) error {
	out := jsonProbeResult{
		NATType:   result.NATType.String(),
		Punchable: result.NATType.Punchable(),
		Duration:  result.Duration.Round(time.Millisecond).String(),
		Probes:    make([]jsonProbeItem, len(result.Probes)),
	}
	if result.PublicAddr != nil {
		out.PublicAddr = result.PublicAddr.String()
	}
	if probeErr != nil {
		out.Error = probeErr.Error()
	}
	for i, p := range result.Probes {
		item := jsonProbeItem{
			Server:   p.Server,
			Duration: p.Duration.Round(time.Millisecond).String(),
		}
		if p.MappedAddr != nil {
			item.MappedAddr = p.MappedAddr.String()
		}
		if p.Err != nil {
			item.Error = p.Err.Error()
		}
		out.Probes[i] = item
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

func printHuman(result *stun.ProbeResult, probeErr error) error {
	fmt.Printf("NAT type:       %s\n", result.NATType)
	if result.PublicAddr != nil {
		fmt.Printf("Public address: %s\n", result.PublicAddr)
	}
	fmt.Printf("Punchable:      %v\n", result.NATType.Punchable())
	fmt.Printf("Probe time:     %s\n", result.Duration.Round(time.Millisecond))
	fmt.Println()

	for _, p := range result.Probes {
		if p.Err != nil {
			fmt.Printf("  ✗ %s — %v (%s)\n", p.Server, p.Err, p.Duration.Round(time.Millisecond))
		} else {
			fmt.Printf("  ✓ %s — %s (%s)\n", p.Server, p.MappedAddr, p.Duration.Round(time.Millisecond))
		}
	}

	if probeErr != nil {
		fmt.Printf("\nWarning: %v\n", probeErr)
	}

	return nil
}
