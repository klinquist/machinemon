package server

import (
	"io/fs"
	"net/http"
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
		http.ServeFileFS(w, r, webFS, "index.html")
		return
	}
	f.Close()

	// Serve the actual file
	http.ServeFileFS(w, r, webFS, path)
}
