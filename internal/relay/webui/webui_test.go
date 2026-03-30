package webui

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func TestHandler_ServesBuiltAssets(t *testing.T) {
	handler := newHandler(fstest.MapFS{
		"index.html":           {Data: []byte("<!doctype html><title>home</title><h1>bore</h1>")},
		"ops/relay/index.html": {Data: []byte("<!doctype html><title>ops</title><h1>relay status</h1>")},
		"404.html":             {Data: []byte("<!doctype html><title>404</title><h1>missing</h1>")},
		"_astro/app.js":        {Data: []byte("console.log('ok')")},
	})

	t.Run("root", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("GET / = %d, want %d", rec.Code, http.StatusOK)
		}
		if !strings.Contains(rec.Body.String(), "<h1>bore</h1>") {
			t.Fatalf("GET / body = %q, want bore homepage", rec.Body.String())
		}
		if got := rec.Header().Get("Content-Security-Policy"); !strings.Contains(got, "default-src 'self'") {
			t.Fatalf("GET / Content-Security-Policy = %q", got)
		}
	})

	t.Run("clean nested route", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/ops/relay", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("GET /ops/relay = %d, want %d", rec.Code, http.StatusOK)
		}
		if !strings.Contains(rec.Body.String(), "relay status") {
			t.Fatalf("GET /ops/relay body = %q, want relay page", rec.Body.String())
		}
	})

	t.Run("static asset", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/_astro/app.js", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("GET /_astro/app.js = %d, want %d", rec.Code, http.StatusOK)
		}
		if got := rec.Header().Get("Content-Type"); !strings.Contains(got, "javascript") {
			t.Fatalf("GET /_astro/app.js Content-Type = %q", got)
		}
	})

	t.Run("404 page", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/missing", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("GET /missing = %d, want %d", rec.Code, http.StatusNotFound)
		}
		if !strings.Contains(rec.Body.String(), "missing") {
			t.Fatalf("GET /missing body = %q, want 404 page", rec.Body.String())
		}
	})
}

func TestHandler_FallbackExplainsBuildRequirement(t *testing.T) {
	t.Setenv(distDirEnvVar, t.TempDir())

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	NewHandler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET / fallback = %d, want %d", rec.Code, http.StatusOK)
	}

	body, err := io.ReadAll(rec.Result().Body)
	if err != nil {
		t.Fatalf("read fallback body: %v", err)
	}
	if !strings.Contains(string(body), "bun run build") {
		t.Fatalf("fallback body = %q, want build instructions", string(body))
	}
}
