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
)

const defaultRelayURL = "http://localhost:8080"

type relayStatus struct {
	Service       string `json:"service"`
	Status        string `json:"status"`
	UptimeSeconds int64  `json:"uptimeSeconds"`
	Rooms         struct {
		Total   int `json:"total"`
		Waiting int `json:"waiting"`
		Active  int `json:"active"`
	} `json:"rooms"`
	Limits struct {
		MaxRooms            int   `json:"maxRooms"`
		RoomTTLSeconds      int64 `json:"roomTTLSeconds"`
		ReapIntervalSeconds int64 `json:"reapIntervalSeconds"`
		MaxMessageSizeBytes int64 `json:"maxMessageSizeBytes"`
	} `json:"limits"`
}

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

func fetchStatus(relayURL string, timeout time.Duration) (*relayStatus, error) {
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

	var status relayStatus
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return nil, fmt.Errorf("decode %s: %w", endpoint, err)
	}
	if status.Service == "" || status.Status == "" {
		return nil, errors.New("relay status response missing service/status fields")
	}

	return &status, nil
}

func printStatus(relayURL string, status *relayStatus) {
	fmt.Println("bore-admin")
	fmt.Println("==========")
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
	fmt.Println("limits:")
	fmt.Printf("  max rooms:       %d\n", status.Limits.MaxRooms)
	fmt.Printf("  room ttl:        %s\n", formatDurationSeconds(status.Limits.RoomTTLSeconds))
	fmt.Printf("  reap interval:   %s\n", formatDurationSeconds(status.Limits.ReapIntervalSeconds))
	fmt.Printf("  max ws message:  %d bytes\n", status.Limits.MaxMessageSizeBytes)
}

func formatDurationSeconds(seconds int64) string {
	if seconds <= 0 {
		return "0s"
	}
	return (time.Duration(seconds) * time.Second).String()
}

func printHelp() {
	fmt.Println("bore-admin -- minimal relay operator CLI")
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
