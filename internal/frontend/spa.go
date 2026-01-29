// Package frontend provides HTTP handlers for serving the embedded SPA frontend.
package frontend

import (
	"io/fs"
	"net/http"
	"strings"
)

// SPAHandler serves a single-page application from an embedded filesystem.
// It implements SPA routing by serving index.html for requests that don't
// match actual files, EXCEPT for /api/* paths which return 404.
type SPAHandler struct {
	fileServer http.Handler
	fsys       fs.FS
}

// NewSPAHandler creates an SPA-aware file server from the given filesystem.
// The filesystem should be pre-processed with fs.Sub() to strip any prefix
// (e.g., "dist/") before being passed to this function.
func NewSPAHandler(fsys fs.FS) *SPAHandler {
	return &SPAHandler{
		fileServer: http.FileServer(http.FS(fsys)),
		fsys:       fsys,
	}
}

// ServeHTTP handles requests by:
// 1. Returning 404 for /api/* paths (API routes should not be served by SPA)
// 2. Serving the actual file if it exists
// 3. Falling back to index.html for SPA client-side routing
func (h *SPAHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// API paths should return 404, not index.html
	if strings.HasPrefix(r.URL.Path, "/api/") {
		http.NotFound(w, r)
		return
	}

	// Clean the path (remove leading slash for fs operations)
	path := strings.TrimPrefix(r.URL.Path, "/")

	// For root path, serve index.html directly
	if path == "" {
		path = "index.html"
	}

	// Check if the requested file exists
	if _, err := fs.Stat(h.fsys, path); err == nil {
		// File exists, serve it normally
		h.fileServer.ServeHTTP(w, r)
		return
	}

	// File doesn't exist - serve index.html for SPA routing
	r.URL.Path = "/"
	h.fileServer.ServeHTTP(w, r)
}
