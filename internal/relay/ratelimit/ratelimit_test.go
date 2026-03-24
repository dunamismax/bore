package ratelimit

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestLimiter_BasicAllow(t *testing.T) {
	l := NewLimiter(Config{
		Rate:   3,
		Window: time.Second,
	})
	defer l.Stop()

	ip := "10.0.0.1"
	for i := range 3 {
		if !l.Allow(ip) {
			t.Fatalf("request %d should be allowed", i+1)
		}
	}

	// 4th request should be denied.
	if l.Allow(ip) {
		t.Fatal("4th request should be denied")
	}
}

func TestLimiter_DifferentIPs(t *testing.T) {
	l := NewLimiter(Config{
		Rate:   1,
		Window: time.Second,
	})
	defer l.Stop()

	if !l.Allow("10.0.0.1") {
		t.Fatal("first IP should be allowed")
	}
	if !l.Allow("10.0.0.2") {
		t.Fatal("second IP should be allowed")
	}

	// Both should now be denied.
	if l.Allow("10.0.0.1") {
		t.Fatal("first IP second request should be denied")
	}
	if l.Allow("10.0.0.2") {
		t.Fatal("second IP second request should be denied")
	}
}

func TestLimiter_WindowReset(t *testing.T) {
	l := NewLimiter(Config{
		Rate:   1,
		Window: 50 * time.Millisecond,
	})
	defer l.Stop()

	ip := "10.0.0.1"
	if !l.Allow(ip) {
		t.Fatal("first request should be allowed")
	}
	if l.Allow(ip) {
		t.Fatal("second request should be denied")
	}

	time.Sleep(60 * time.Millisecond)

	if !l.Allow(ip) {
		t.Fatal("request after window reset should be allowed")
	}
}

func TestLimiter_Cleanup(t *testing.T) {
	l := NewLimiter(Config{
		Rate:            1,
		Window:          20 * time.Millisecond,
		CleanupInterval: 30 * time.Millisecond,
	})
	defer l.Stop()

	l.Allow("10.0.0.1")
	l.Allow("10.0.0.2")

	if l.Len() != 2 {
		t.Fatalf("expected 2 tracked IPs, got %d", l.Len())
	}

	// Wait for 2× window + cleanup interval.
	time.Sleep(100 * time.Millisecond)

	if l.Len() != 0 {
		t.Fatalf("expected 0 tracked IPs after cleanup, got %d", l.Len())
	}
}

func TestLimiter_Concurrent(t *testing.T) {
	l := NewLimiter(Config{
		Rate:   100,
		Window: time.Second,
	})
	defer l.Stop()

	var wg sync.WaitGroup
	const goroutines = 20
	const opsPerGoroutine = 50

	allowed := make([]int, goroutines)
	wg.Add(goroutines)
	for g := range goroutines {
		go func(idx int) {
			defer wg.Done()
			for range opsPerGoroutine {
				if l.Allow("10.0.0.1") {
					allowed[idx]++
				}
			}
		}(g)
	}
	wg.Wait()

	total := 0
	for _, n := range allowed {
		total += n
	}
	if total > 100 {
		t.Fatalf("total allowed = %d, should not exceed rate limit of 100", total)
	}
	if total < 1 {
		t.Fatal("at least some requests should have been allowed")
	}
}

func TestExtractIP_WithPort(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = "192.168.1.1:12345"

	ip := ExtractIP(r)
	if ip != "192.168.1.1" {
		t.Fatalf("ExtractIP = %q, want %q", ip, "192.168.1.1")
	}
}

func TestExtractIP_IPv6(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = "[::1]:8080"

	ip := ExtractIP(r)
	if ip != "::1" {
		t.Fatalf("ExtractIP = %q, want %q", ip, "::1")
	}
}

func TestExtractIP_NoPort(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = "10.0.0.1"

	ip := ExtractIP(r)
	if ip != "10.0.0.1" {
		t.Fatalf("ExtractIP = %q, want %q", ip, "10.0.0.1")
	}
}
