package webui

import (
	"embed"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"strings"
)

//go:embed all:dist
var dist embed.FS

type handler struct {
	root       fs.FS
	fileServer http.Handler
}

// NewHandler returns the static web handler backed by the embedded Astro build.
func NewHandler() http.Handler {
	root, err := fs.Sub(dist, "dist")
	if err != nil {
		panic(err)
	}

	return &handler{
		root:       root,
		fileServer: http.FileServer(http.FS(root)),
	}
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	filePath, redirectPath, ok := resolvePath(h.root, r.URL.Path)
	if !ok {
		serveFile(w, r, h.root, "404.html", http.StatusNotFound)
		return
	}

	if redirectPath != "" {
		http.Redirect(w, r, redirectPath, http.StatusPermanentRedirect)
		return
	}

	applyHeaders(w, filePath)
	if filePath == "index.html" || strings.HasSuffix(filePath, "/index.html") {
		serveFile(w, r, h.root, filePath, http.StatusOK)
		return
	}

	cleanRequestPath := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
	if cleanRequestPath == "." {
		cleanRequestPath = ""
	}
	if filePath != cleanRequestPath {
		r = r.Clone(r.Context())
		r.URL.Path = "/" + filePath
	}

	h.fileServer.ServeHTTP(w, r)
}

func applyHeaders(w http.ResponseWriter, filePath string) {
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Frame-Options", "DENY")

	if strings.HasPrefix(filePath, "assets/") || strings.HasPrefix(filePath, "_astro/") {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		return
	}

	w.Header().Set("Cache-Control", "no-store")
}

func resolvePath(root fs.FS, requestPath string) (filePath string, redirectPath string, ok bool) {
	clean := strings.TrimPrefix(path.Clean("/"+requestPath), "/")
	if clean == "." {
		clean = ""
	}

	if clean == "" {
		if existsFile(root, "index.html") {
			return "index.html", "", true
		}
		return "", "", false
	}

	if existsFile(root, clean) {
		return clean, "", true
	}

	indexPath := path.Join(clean, "index.html")
	if existsFile(root, indexPath) {
		if !strings.HasSuffix(requestPath, "/") {
			return "", ensureTrailingSlash(requestPath), true
		}
		return indexPath, "", true
	}

	return "", "", false
}

func existsFile(root fs.FS, name string) bool {
	info, err := fs.Stat(root, name)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

func ensureTrailingSlash(requestPath string) string {
	if requestPath == "" {
		return "/"
	}
	if strings.HasSuffix(requestPath, "/") {
		return requestPath
	}
	return requestPath + "/"
}

func serveFile(w http.ResponseWriter, r *http.Request, root fs.FS, name string, status int) {
	applyHeaders(w, name)
	data, err := fs.ReadFile(root, name)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	if contentType := mime.TypeByExtension(path.Ext(name)); contentType != "" {
		w.Header().Set("Content-Type", contentType)
	}
	w.WriteHeader(status)
	_, _ = w.Write(data)
}
