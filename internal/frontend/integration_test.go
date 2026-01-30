package frontend_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/zjrosen/perles/internal/frontend"
)

// TestIntegration_APIAndSPARoutesOnSameMux verifies that when API routes and
// the SPA catch-all are registered on the same mux (as done in supervisor.go),
// they coexist correctly:
// - API routes take precedence and return proper responses
// - Unmatched routes fall through to the SPA handler
// - The SPA handler returns index.html for client-side routing
func TestIntegration_APIAndSPARoutesOnSameMux(t *testing.T) {
	// Create a test filesystem mimicking the embedded frontend
	testFS := fstest.MapFS{
		"index.html":        &fstest.MapFile{Data: []byte("<!DOCTYPE html><html><body>SPA App</body></html>")},
		"assets/app.js":     &fstest.MapFile{Data: []byte("console.log('app')")},
		"assets/styles.css": &fstest.MapFile{Data: []byte("body { margin: 0; }")},
	}

	// Create the handler (same pattern used in production)
	h := frontend.NewHandler(t.TempDir(), testFS, nil)

	// Create a mux and register routes in the same order as supervisor.go:
	// 1. MCP routes (simulated here as /mcp)
	// 2. API routes
	// 3. SPA catch-all LAST
	mux := http.NewServeMux()

	// Simulate MCP route (would be mcpCoordServer.ServeHTTP() in production)
	mux.HandleFunc("/mcp", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"mcp": "server"}`))
	})

	// Simulate worker routes (would be workerServers.ServeHTTP in production)
	mux.HandleFunc("/worker/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"worker": "endpoint"}`))
	})

	// Register API routes first
	h.RegisterAPIRoutes(mux)

	// Register SPA catch-all LAST (critical order!)
	h.RegisterSPAHandler(mux)

	server := httptest.NewServer(mux)
	defer server.Close()

	tests := []struct {
		name           string
		path           string
		method         string
		expectedStatus int
		checkBody      func(t *testing.T, body string)
	}{
		{
			name:           "MCP route works",
			path:           "/mcp",
			method:         http.MethodGet,
			expectedStatus: http.StatusOK,
			checkBody: func(t *testing.T, body string) {
				assert.Contains(t, body, `"mcp"`)
			},
		},
		{
			name:           "Worker route works",
			path:           "/worker/worker-1",
			method:         http.MethodGet,
			expectedStatus: http.StatusOK,
			checkBody: func(t *testing.T, body string) {
				assert.Contains(t, body, `"worker"`)
			},
		},
		{
			name:           "API health endpoint works",
			path:           "/api/health",
			method:         http.MethodGet,
			expectedStatus: http.StatusOK,
			checkBody: func(t *testing.T, body string) {
				var resp frontend.HealthResponse
				err := json.Unmarshal([]byte(body), &resp)
				require.NoError(t, err)
				assert.Equal(t, "ok", resp.Status)
			},
		},
		{
			name:           "API sessions endpoint works",
			path:           "/api/sessions",
			method:         http.MethodGet,
			expectedStatus: http.StatusOK,
			checkBody: func(t *testing.T, body string) {
				var resp frontend.SessionListResponse
				err := json.Unmarshal([]byte(body), &resp)
				require.NoError(t, err)
				// Should return empty array for empty temp dir
				assert.NotNil(t, resp.Apps)
			},
		},
		{
			name:           "Nonexistent API path returns 404 (not index.html)",
			path:           "/api/nonexistent",
			method:         http.MethodGet,
			expectedStatus: http.StatusNotFound,
			checkBody: func(t *testing.T, body string) {
				// Should NOT contain SPA content
				assert.NotContains(t, body, "SPA App")
			},
		},
		{
			name:           "Root path serves SPA index.html",
			path:           "/",
			method:         http.MethodGet,
			expectedStatus: http.StatusOK,
			checkBody: func(t *testing.T, body string) {
				assert.Contains(t, body, "SPA App")
			},
		},
		{
			name:           "Static asset is served directly",
			path:           "/assets/app.js",
			method:         http.MethodGet,
			expectedStatus: http.StatusOK,
			checkBody: func(t *testing.T, body string) {
				assert.Contains(t, body, "console.log")
			},
		},
		{
			name:           "Unknown path serves SPA index.html (client-side routing)",
			path:           "/some/deep/route",
			method:         http.MethodGet,
			expectedStatus: http.StatusOK,
			checkBody: func(t *testing.T, body string) {
				assert.Contains(t, body, "SPA App")
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req, err := http.NewRequest(tc.method, server.URL+tc.path, nil)
			require.NoError(t, err)

			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			defer func() { _ = resp.Body.Close() }()

			assert.Equal(t, tc.expectedStatus, resp.StatusCode)

			if tc.checkBody != nil {
				var body []byte
				body, err = readAllWithLimit(resp.Body, 1024*10)
				require.NoError(t, err)
				tc.checkBody(t, string(body))
			}
		})
	}
}

// readAllWithLimit reads up to limit bytes from the reader
func readAllWithLimit(r interface{ Read([]byte) (int, error) }, limit int) ([]byte, error) {
	buf := make([]byte, limit)
	n, err := r.Read(buf)
	if err != nil && err.Error() != "EOF" {
		return nil, err
	}
	return buf[:n], nil
}

// TestIntegration_RouteOrderMatters demonstrates that the order of route
// registration is critical - the SPA catch-all must be registered LAST.
func TestIntegration_RouteOrderMatters(t *testing.T) {
	testFS := fstest.MapFS{
		"index.html": &fstest.MapFile{Data: []byte("SPA")},
	}

	h := frontend.NewHandler(t.TempDir(), testFS, nil)

	// WRONG ORDER: SPA first, then API (this would break API routes)
	wrongOrderMux := http.NewServeMux()
	h.RegisterSPAHandler(wrongOrderMux) // SPA first (WRONG!)
	h.RegisterAPIRoutes(wrongOrderMux)  // API second

	// CORRECT ORDER: API first, then SPA
	correctOrderMux := http.NewServeMux()
	h.RegisterAPIRoutes(correctOrderMux)  // API first (CORRECT!)
	h.RegisterSPAHandler(correctOrderMux) // SPA last

	// Test that /api/health works correctly in correct order
	t.Run("correct order - API works", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
		w := httptest.NewRecorder()
		correctOrderMux.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var resp frontend.HealthResponse
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, "ok", resp.Status)
	})

	// Note: In Go 1.22+ ServeMux, more specific patterns take precedence,
	// so the order actually doesn't break things. However, this test
	// documents the expected behavior and best practice.
	t.Run("wrong order - API still works in Go 1.22+", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
		w := httptest.NewRecorder()
		wrongOrderMux.ServeHTTP(w, req)

		// In Go 1.22+, more specific patterns win regardless of order
		assert.Equal(t, http.StatusOK, w.Code)
	})
}
