package frontend

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testFS creates a mock filesystem for testing.
func testFS() fs.FS {
	return fstest.MapFS{
		"index.html": &fstest.MapFile{
			Data: []byte("<!DOCTYPE html><html><body>SPA</body></html>"),
		},
		"assets/index-abc123.js": &fstest.MapFile{
			Data: []byte("console.log('app');"),
		},
		"assets/index-def456.css": &fstest.MapFile{
			Data: []byte("body { color: black; }"),
		},
	}
}

func TestSPAHandler_RootPath(t *testing.T) {
	handler := NewSPAHandler(testFS())

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "SPA")
	assert.Contains(t, w.Header().Get("Content-Type"), "text/html")
}

func TestSPAHandler_ExistingJSFile(t *testing.T) {
	handler := NewSPAHandler(testFS())

	req := httptest.NewRequest(http.MethodGet, "/assets/index-abc123.js", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "console.log")
	// http.FileServer sets Content-Type based on extension
	assert.Contains(t, w.Header().Get("Content-Type"), "javascript")
}

func TestSPAHandler_ExistingCSSFile(t *testing.T) {
	handler := NewSPAHandler(testFS())

	req := httptest.NewRequest(http.MethodGet, "/assets/index-def456.css", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "color: black")
	assert.Contains(t, w.Header().Get("Content-Type"), "css")
}

func TestSPAHandler_NonExistentPath_ReturnsIndexHTML(t *testing.T) {
	handler := NewSPAHandler(testFS())

	// Non-existent paths should return index.html for SPA routing
	testPaths := []string{
		"/sessions",
		"/sessions/123",
		"/about",
		"/nonexistent/deep/path",
		"/some-route",
	}

	for _, path := range testPaths {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			require.Equal(t, http.StatusOK, w.Code, "path %s should return 200", path)
			assert.Contains(t, w.Body.String(), "SPA", "path %s should return index.html", path)
			assert.Contains(t, w.Header().Get("Content-Type"), "text/html")
		})
	}
}

func TestSPAHandler_APIPath_Returns404(t *testing.T) {
	handler := NewSPAHandler(testFS())

	// API paths should return 404, NOT index.html
	testPaths := []string{
		"/api/sessions",
		"/api/load-session",
		"/api/health",
		"/api/anything",
		"/api/nested/path",
	}

	for _, path := range testPaths {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			require.Equal(t, http.StatusNotFound, w.Code, "path %s should return 404", path)
			// Should NOT contain index.html content
			assert.NotContains(t, w.Body.String(), "SPA", "path %s should not return index.html", path)
		})
	}
}

func TestSPAHandler_APIPath_PostMethod(t *testing.T) {
	handler := NewSPAHandler(testFS())

	req := httptest.NewRequest(http.MethodPost, "/api/load-session", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	require.Equal(t, http.StatusNotFound, w.Code)
}

func TestSPAHandler_ContentTypes(t *testing.T) {
	testCases := []struct {
		name        string
		files       fstest.MapFS
		path        string
		wantStatus  int
		wantContent string
		wantType    string
	}{
		{
			name: "html file",
			files: fstest.MapFS{
				"index.html": &fstest.MapFile{Data: []byte("<html></html>")},
			},
			path:        "/",
			wantStatus:  http.StatusOK,
			wantContent: "<html>",
			wantType:    "text/html",
		},
		{
			name: "js file",
			files: fstest.MapFS{
				"index.html": &fstest.MapFile{Data: []byte("<html></html>")},
				"app.js":     &fstest.MapFile{Data: []byte("function() {}")},
			},
			path:        "/app.js",
			wantStatus:  http.StatusOK,
			wantContent: "function",
			wantType:    "javascript",
		},
		{
			name: "css file",
			files: fstest.MapFS{
				"index.html": &fstest.MapFile{Data: []byte("<html></html>")},
				"style.css":  &fstest.MapFile{Data: []byte(".class { margin: 0; }")},
			},
			path:        "/style.css",
			wantStatus:  http.StatusOK,
			wantContent: "margin",
			wantType:    "css",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			handler := NewSPAHandler(tc.files)

			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			require.Equal(t, tc.wantStatus, w.Code)
			assert.Contains(t, w.Body.String(), tc.wantContent)
			assert.Contains(t, w.Header().Get("Content-Type"), tc.wantType)
		})
	}
}

func TestSPAHandler_EmptyPath(t *testing.T) {
	// Empty path should be treated the same as "/"
	handler := NewSPAHandler(testFS())

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.URL.Path = "" // Modify after creation
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "SPA")
}

func TestSPAHandler_IndexHTMLDirect(t *testing.T) {
	handler := NewSPAHandler(testFS())

	req := httptest.NewRequest(http.MethodGet, "/index.html", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// http.FileServer redirects /index.html to / (canonical path)
	// This is expected behavior for clean URLs
	require.Equal(t, http.StatusMovedPermanently, w.Code)
	assert.Equal(t, "./", w.Header().Get("Location"))
}

func TestSPAHandler_AssetSubdirectory(t *testing.T) {
	handler := NewSPAHandler(testFS())

	// Request for assets directory itself (not a file)
	req := httptest.NewRequest(http.MethodGet, "/assets/", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Should fall back to index.html since assets/ directory listing isn't a file
	// The exact behavior depends on http.FileServer, but SPA routing should handle it
	require.Equal(t, http.StatusOK, w.Code)
}

func TestSPAHandler_NonExistentAsset_ReturnsIndexHTML(t *testing.T) {
	handler := NewSPAHandler(testFS())

	// Non-existent file in assets dir should return index.html
	req := httptest.NewRequest(http.MethodGet, "/assets/nonexistent.js", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "SPA")
}
