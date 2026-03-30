package webui

import (
	"fmt"
	"io/fs"
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
)

const frontendContentSecurityPolicy = "default-src 'self'; connect-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self' data:; object-src 'none'; base-uri 'none'; frame-ancestors 'none'; form-action 'none'"

const distDirEnvVar = "BORE_WEB_DIST_DIR"

// NewHandler serves the built Astro frontend when web/dist is present and
// otherwise returns a small fallback page explaining how to build it.
func NewHandler() http.Handler {
	if assets, ok := resolveAssetsFS(); ok {
		return newHandler(assets)
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		w.Header().Set("Content-Security-Policy", frontendContentSecurityPolicy)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`<!DOCTYPE html>
<html lang="en">
<head><meta charset="utf-8"><title>bore web assets not built</title></head>
<body style="font-family:'Avenir Next','Segoe UI',sans-serif;background:#0c1117;color:#edf2f7;display:grid;place-items:center;min-height:100vh;margin:0;padding:1.5rem;">
<div style="max-width:42rem;padding:2rem;border:1px solid rgba(255,255,255,0.1);border-radius:24px;background:rgba(14,23,33,0.88);box-shadow:0 28px 80px rgba(0,0,0,0.32);">
<p style="margin:0 0 0.75rem;text-transform:uppercase;letter-spacing:0.14em;font-size:0.78rem;color:#ffd4aa;">bore web surface</p>
<h1 style="margin:0 0 1rem;font-size:2rem;font-family:'Iowan Old Style','Palatino Linotype',Georgia,serif;">Web assets are not built yet.</h1>
<p style="margin:0 0 1rem;color:#a5b1bd;line-height:1.6;">This relay can serve the Astro frontend same-origin once the static build output exists. Build it from the repo root with <code>cd web && bun install && bun run build</code>, then restart the relay.</p>
<p style="margin:0 0 1rem;color:#a5b1bd;line-height:1.6;">If the build output lives elsewhere, point the relay at it with <code>` + distDirEnvVar + `</code>.</p>
<p style="margin:0;color:#a5b1bd;"><a href="/status" style="color:#ffd4aa;">/status</a> · <a href="/healthz" style="color:#ffd4aa;">/healthz</a> · <a href="/metrics" style="color:#ffd4aa;">/metrics</a></p>
</div>
</body>
</html>`))
	})
}

func newHandler(assets fs.FS) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		w.Header().Set("Content-Security-Policy", frontendContentSecurityPolicy)

		name, ok := resolveRequestPath(assets, r.URL.Path)
		if !ok {
			if serveFile(w, r, assets, "404.html", http.StatusNotFound) == nil {
				return
			}
			http.NotFound(w, r)
			return
		}

		if err := serveFile(w, r, assets, name, http.StatusOK); err != nil {
			http.Error(w, fmt.Sprintf("read %s: %v", name, err), http.StatusInternalServerError)
		}
	})
}

func resolveAssetsFS() (fs.FS, bool) {
	if value := strings.TrimSpace(os.Getenv(distDirEnvVar)); value != "" {
		return openDirFS(value)
	}

	candidates := []string{}
	exePath, err := os.Executable()
	if err == nil {
		exeDir := filepath.Dir(exePath)
		candidates = append(
			candidates,
			filepath.Join(exeDir, "web", "dist"),
			filepath.Join(exeDir, "..", "web", "dist"),
		)
	}

	candidates = append(candidates, filepath.Join("web", "dist"))

	for _, candidate := range candidates {
		if assets, ok := openDirFS(candidate); ok {
			return assets, true
		}
	}

	return nil, false
}

func openDirFS(dir string) (fs.FS, bool) {
	if dir == "" {
		return nil, false
	}
	info, err := os.Stat(filepath.Join(dir, "index.html"))
	if err != nil || info.IsDir() {
		return nil, false
	}
	return os.DirFS(dir), true
}

func resolveRequestPath(assets fs.FS, requestPath string) (string, bool) {
	cleaned := strings.TrimPrefix(path.Clean("/"+requestPath), "/")
	if cleaned == "." || cleaned == "" {
		return "index.html", true
	}

	if fileExists(assets, cleaned) {
		return cleaned, true
	}

	if path.Ext(cleaned) != "" {
		return "", false
	}

	indexPath := path.Join(cleaned, "index.html")
	if fileExists(assets, indexPath) {
		return indexPath, true
	}

	htmlPath := cleaned + ".html"
	if fileExists(assets, htmlPath) {
		return htmlPath, true
	}

	return "", false
}

func fileExists(assets fs.FS, name string) bool {
	info, err := fs.Stat(assets, name)
	return err == nil && !info.IsDir()
}

func serveFile(w http.ResponseWriter, r *http.Request, assets fs.FS, name string, status int) error {
	body, err := fs.ReadFile(assets, name)
	if err != nil {
		return err
	}

	contentType := mime.TypeByExtension(path.Ext(name))
	if contentType == "" {
		contentType = http.DetectContentType(body)
	}
	if strings.HasSuffix(name, ".html") && !strings.Contains(contentType, "charset=") {
		contentType += "; charset=utf-8"
	}

	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(status)
	if r.Method == http.MethodHead {
		return nil
	}

	_, err = w.Write(body)
	return err
}
