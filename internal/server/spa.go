package server

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"net/http"
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

	// Try to serve the requested file
	path := r.URL.Path
	if path == "/" {
		path = "index.html"
	} else {
		path = path[1:] // strip leading slash
	}

	// Check if file exists in embedded FS
	f, err := webFS.Open(path)
	if err != nil {
		// File not found - serve index.html for SPA client-side routing
		s.serveIndex(w, r)
		return
	}
	f.Close()

	// Serve the actual file
	http.ServeFileFS(w, r, webFS, path)
}

// serveIndex serves index.html, injecting <base> tag if base_path is configured.
func (s *Server) serveIndex(w http.ResponseWriter, r *http.Request) {
	if s.cfg.BasePath == "" {
		// No base path — serve index.html directly
		http.ServeFileFS(w, r, webFS, "index.html")
		return
	}

	// Read index.html and inject base path
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

	basePath := strings.TrimRight(s.cfg.BasePath, "/") + "/"
	injection := fmt.Sprintf(`<base href="%s"><script>window.__BASE_PATH__=%q</script>`, basePath, strings.TrimRight(s.cfg.BasePath, "/"))

	// Inject after <head>
	modified := bytes.Replace(data, []byte("<head>"), []byte("<head>"+injection), 1)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(modified)
}
