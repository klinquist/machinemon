package server

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"strings"
)

// webFS holds the embedded filesystem for the React SPA.
// It's set by the server binary's main package via SetWebFS.
var webFS fs.FS

// SetWebFS configures the embedded filesystem used to serve the SPA.
func SetWebFS(fsys fs.FS) {
	webFS = fsys
}

func (s *Server) serveSPA(w http.ResponseWriter, r *http.Request) {
	if webFS == nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(`<!DOCTYPE html><html><body>
			<h1>MachineMon</h1>
			<p>Dashboard not built yet. Run <code>make web</code> to build the React SPA.</p>
		</body></html>`))
		return
	}

	path := r.URL.Path

	// Root or unknown paths → serve index.html (with base path injection)
	if path == "/" {
		s.serveIndex(w, r)
		return
	}

	// Try to serve the requested file (CSS, JS, images, etc.)
	filePath := path[1:] // strip leading slash
	f, err := webFS.Open(filePath)
	if err != nil {
		// File not found → SPA client-side routing fallback
		s.serveIndex(w, r)
		return
	}
	f.Close()

	http.ServeFileFS(w, r, webFS, filePath)
}

// serveIndex serves index.html, injecting <base> tag if base_path is configured.
func (s *Server) serveIndex(w http.ResponseWriter, r *http.Request) {
	// Read index.html and inject runtime path hints.
	f, err := webFS.Open("index.html")
	if err != nil {
		http.Error(w, "index.html not found", http.StatusInternalServerError)
		return
	}
	defer f.Close()

	data, err := io.ReadAll(f)
	if err != nil {
		http.Error(w, "failed to read index.html", http.StatusInternalServerError)
		return
	}

	effectiveBasePath := s.effectiveBasePath()
	faviconHref := "/favicon.svg"
	if effectiveBasePath != "" {
		faviconHref = effectiveBasePath + "/favicon.svg"
	}

	modified := data
	// Ensure favicon resolves correctly even with reverse-proxy subpath rewrites.
	modified = bytes.ReplaceAll(modified, []byte(`href="./favicon.svg"`), []byte(fmt.Sprintf(`href="%s"`, faviconHref)))
	modified = bytes.ReplaceAll(modified, []byte(`href="/favicon.svg"`), []byte(fmt.Sprintf(`href="%s"`, faviconHref)))

	if effectiveBasePath != "" {
		basePath := effectiveBasePath + "/"
		injection := fmt.Sprintf(`<base href="%s"><script>window.__BASE_PATH__=%q</script>`, basePath, effectiveBasePath)
		// Inject after <head>
		modified = bytes.Replace(modified, []byte("<head>"), []byte("<head>"+injection), 1)
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(modified)
}

func (s *Server) effectiveBasePath() string {
	basePath := strings.TrimSpace(s.cfg.BasePath)
	if basePath != "" {
		basePath = "/" + strings.Trim(basePath, "/")
		if basePath == "/" {
			return ""
		}
		return basePath
	}

	ext := strings.TrimSpace(s.cfg.ExternalURL)
	if ext == "" {
		return ""
	}
	u, err := url.Parse(ext)
	if err != nil {
		return ""
	}
	p := strings.TrimSpace(u.Path)
	if p == "" || p == "/" {
		return ""
	}
	return "/" + strings.Trim(p, "/")
}
