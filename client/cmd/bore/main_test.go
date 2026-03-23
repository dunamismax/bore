package main

import (
	"flag"
	"testing"
)

func newParseFlagSet(name string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.String("relay", "", "")
	fs.Int("words", 3, "")
	fs.String("output", ".", "")
	return fs
}

func TestParsePrimaryArgLeadingPositional(t *testing.T) {
	fs := newParseFlagSet("send")
	got, err := parsePrimaryArg(fs, []string{"./payload.txt", "--relay", "http://127.0.0.1:8080", "--words", "4"}, "bore send <path>")
	if err != nil {
		t.Fatalf("parsePrimaryArg: %v", err)
	}
	if got != "./payload.txt" {
		t.Fatalf("primary = %q, want ./payload.txt", got)
	}
	if gotRelay := fs.Lookup("relay").Value.String(); gotRelay != "http://127.0.0.1:8080" {
		t.Fatalf("relay = %q, want http://127.0.0.1:8080", gotRelay)
	}
	if gotWords := fs.Lookup("words").Value.String(); gotWords != "4" {
		t.Fatalf("words = %q, want 4", gotWords)
	}
}

func TestParsePrimaryArgFlagsFirst(t *testing.T) {
	fs := newParseFlagSet("receive")
	got, err := parsePrimaryArg(fs, []string{"--relay", "http://127.0.0.1:8080", "--output", "/tmp/out", "relay_000001-7-apple-beach"}, "bore receive <code>")
	if err != nil {
		t.Fatalf("parsePrimaryArg: %v", err)
	}
	if got != "relay_000001-7-apple-beach" {
		t.Fatalf("primary = %q, want relay_000001-7-apple-beach", got)
	}
	if gotOutput := fs.Lookup("output").Value.String(); gotOutput != "/tmp/out" {
		t.Fatalf("output = %q, want /tmp/out", gotOutput)
	}
}

func TestParsePrimaryArgMissing(t *testing.T) {
	fs := newParseFlagSet("send")
	if _, err := parsePrimaryArg(fs, []string{"--relay", "http://127.0.0.1:8080"}, "bore send <path>"); err == nil {
		t.Fatal("expected missing positional argument error")
	}
}

func TestParsePrimaryArgRejectsExtraArgs(t *testing.T) {
	fs := newParseFlagSet("send")
	if _, err := parsePrimaryArg(fs, []string{"./payload.txt", "extra.txt"}, "bore send <path>"); err == nil {
		t.Fatal("expected extra argument error")
	}
}
