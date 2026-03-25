// Package webui provides the web handler for the relay server.
//
// The relay previously embedded a React SPA. The browser surface is now
// served by a separate Python frontend (see frontend/). This handler
// returns a lightweight redirect or status message so the relay's HTTP
// mux still has something mounted at /.
package webui

import (
	"net/http"
)

// NewHandler returns an HTTP handler that redirects browsers to the
// Python frontend or shows a minimal status message.
func NewHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`<!DOCTYPE html>
<html lang="en">
<head><meta charset="utf-8"><title>bore relay</title></head>
<body style="font-family:system-ui;background:#0f172a;color:#e2e8f0;display:grid;place-items:center;min-height:100vh;margin:0;">
<div style="text-align:center;max-width:32rem;padding:2rem;">
<h1 style="font-size:1.5rem;margin-bottom:0.5rem;">bore relay</h1>
<p style="color:#94a3b8;">This is the bore relay server. The operator dashboard is served by the Python frontend on a separate port.</p>
<p style="margin-top:1rem;"><a href="/status" style="color:#c2712e;">/status</a> · <a href="/healthz" style="color:#c2712e;">/healthz</a> · <a href="/metrics" style="color:#c2712e;">/metrics</a></p>
</div>
</body>
</html>`))
	})
}
