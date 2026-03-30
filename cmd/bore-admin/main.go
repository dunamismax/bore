package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	relaystatus "github.com/dunamismax/bore/internal/relay/status"
)

const defaultRelayURL = "http://localhost:8080"

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "bore-admin: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return runStatus(args)
	}

	switch args[0] {
	case "status":
		return runStatus(args[1:])
	case "help", "-h", "--help":
		printHelp()
		return nil
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func runStatus(args []string) error {
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	relayURL := fs.String("relay", defaultRelayURL, "relay base URL")
	timeout := fs.Duration("timeout", 5*time.Second, "HTTP timeout")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("unexpected arguments: %s", strings.Join(fs.Args(), " "))
	}

	status, err := fetchStatus(*relayURL, *timeout)
	if err != nil {
		return err
	}

	printStatus(*relayURL, status)
	return nil
}

func fetchStatus(relayURL string, timeout time.Duration) (*relaystatus.Response, error) {
	endpoint := strings.TrimRight(relayURL, "/") + "/status"

	client := &http.Client{Timeout: timeout}
	resp, err := client.Get(endpoint)
	if err != nil {
		return nil, fmt.Errorf("fetch %s: %w", endpoint, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch %s: unexpected status %s", endpoint, resp.Status)
	}

	var status relaystatus.Response
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return nil, fmt.Errorf("decode %s: %w", endpoint, err)
	}
	if status.Service == "" || status.Status == "" {
		return nil, errors.New("relay status response missing service/status fields")
	}

	return &status, nil
}

func printStatus(relayURL string, status *relaystatus.Response) {
	fmt.Println("bore-admin (compatibility shim)")
	fmt.Println("=============================")
	fmt.Println()
	fmt.Println("relay:   " + relayURL)
	fmt.Println("service: " + status.Service)
	fmt.Println("status:  " + status.Status)
	fmt.Printf("uptime:  %s\n", formatDurationSeconds(status.UptimeSeconds))
	fmt.Println()
	fmt.Println("rooms:")
	fmt.Printf("  total:   %d\n", status.Rooms.Total)
	fmt.Printf("  waiting: %d\n", status.Rooms.Waiting)
	fmt.Printf("  active:  %d\n", status.Rooms.Active)
	fmt.Println()
	fmt.Println("transport:")
	fmt.Printf("  signaling started: %d\n", status.Transport.SignalingStarted)
	fmt.Printf("  signal exchanges:  %d\n", status.Transport.SignalExchanges)
	fmt.Printf("  rooms relayed:     %d\n", status.Transport.RoomsRelayed)
	fmt.Printf("  bytes relayed:     %d\n", status.Transport.BytesRelayed)
	fmt.Printf("  frames relayed:    %d\n", status.Transport.FramesRelayed)
	// Inferred direct success: signaling exchanges that did not result in relay usage.
	directInferred := status.Transport.SignalExchanges - status.Transport.RoomsRelayed
	if directInferred < 0 {
		directInferred = 0
	}
	fmt.Printf("  direct (inferred): %d\n", directInferred)
	fmt.Println()
	fmt.Println("limits:")
	fmt.Printf("  max rooms:       %d\n", status.Limits.MaxRooms)
	fmt.Printf("  room ttl:        %s\n", formatDurationSeconds(status.Limits.RoomTTLSeconds))
	fmt.Printf("  reap interval:   %s\n", formatDurationSeconds(status.Limits.ReapIntervalSeconds))
	fmt.Printf("  max ws message:  %d bytes\n", status.Limits.MaxMessageSizeBytes)
	fmt.Println()
	fmt.Printf("hint: for live refresh, room gauges, and failure states, run: cd tui && bun run start --relay %s\n", relayURL)
}

func formatDurationSeconds(seconds int64) string {
	if seconds <= 0 {
		return "0s"
	}
	return (time.Duration(seconds) * time.Second).String()
}

func printHelp() {
	fmt.Println("bore-admin -- compatibility relay operator CLI")
	fmt.Println()
	fmt.Println("This remains for terse terminal checks alongside the primary OpenTUI operator surface in tui/.")
	fmt.Println("For live refresh, room gauges, and clearer failure handling, use: cd tui && bun run start")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  bore-admin status [--relay URL] [--timeout 5s]")
	fmt.Println("  bore-admin [--relay URL] [--timeout 5s]")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  status    query the relay /status endpoint and print a summary")
	fmt.Println()
	fmt.Println("Flags:")
	fmt.Println("  --relay URL      relay base URL (default: http://localhost:8080)")
	fmt.Println("  --timeout D      HTTP timeout (default: 5s)")
}
