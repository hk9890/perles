//nolint:tagliatelle // JSON tags use camelCase to match frontend TypeScript types
package frontend

import "encoding/json"

// SessionListResponse is the response for GET /api/sessions.
// It returns sessions organized hierarchically by application and date.
type SessionListResponse struct {
	BasePath string        `json:"basePath"`
	Apps     []AppSessions `json:"apps"`
}

// AppSessions groups sessions by application name.
type AppSessions struct {
	Name  string      `json:"name"`
	Dates []DateGroup `json:"dates"`
}

// DateGroup groups sessions by date partition.
type DateGroup struct {
	Date     string           `json:"date"`
	Sessions []SessionSummary `json:"sessions"`
}

// SessionSummary provides a lightweight summary of a session for listing.
type SessionSummary struct {
	ID          string  `json:"id"`
	Path        string  `json:"path"`
	StartTime   *string `json:"startTime"`
	Status      string  `json:"status"`
	WorkerCount int     `json:"workerCount"`
	ClientType  string  `json:"clientType"`
}

// LoadSessionRequest is the request body for POST /api/load-session.
type LoadSessionRequest struct {
	Path string `json:"path"`
}

// LoadSessionResponse contains all session data for the viewer.
// Fields use json.RawMessage to preserve the original JSONL structure
// without needing to parse every event type.
type LoadSessionResponse struct {
	Path        string                `json:"path"`
	Metadata    *SessionMetadata      `json:"metadata"`
	Fabric      []json.RawMessage     `json:"fabric"`
	MCPRequests []json.RawMessage     `json:"mcpRequests"`
	Commands    []json.RawMessage     `json:"commands"`
	Messages    []json.RawMessage     `json:"messages"`
	Coordinator CoordinatorData       `json:"coordinator"`
	Workers     map[string]WorkerData `json:"workers"`
}

// SessionMetadata contains parsed session metadata.
// Field names use snake_case JSON tags to match the existing metadata.json format
// that the backend writes.
type SessionMetadata struct {
	SessionID             string            `json:"session_id"`
	StartTime             string            `json:"start_time"`
	Status                string            `json:"status"`
	SessionDir            string            `json:"session_dir"`
	CoordinatorSessionRef string            `json:"coordinator_session_ref"`
	Resumable             bool              `json:"resumable"`
	Workers               []WorkerMeta      `json:"workers"`
	ClientType            string            `json:"client_type"`
	TokenUsage            TokenUsageSummary `json:"token_usage"`
	ApplicationName       string            `json:"application_name"`
	WorkDir               string            `json:"work_dir"`
	DatePartition         string            `json:"date_partition"`
}

// WorkerMeta contains worker metadata.
type WorkerMeta struct {
	ID                 string             `json:"id"`
	SpawnedAt          string             `json:"spawned_at"`
	HeadlessSessionRef string             `json:"headless_session_ref"`
	WorkDir            string             `json:"work_dir"`
	TokenUsage         *TokenUsageSummary `json:"token_usage,omitempty"`
}

// TokenUsageSummary aggregates token usage for display.
// Note: The frontend uses total_input_tokens while the backend uses context_tokens.
// The handler should map context_tokens to total_input_tokens for frontend compatibility.
type TokenUsageSummary struct {
	TotalInputTokens  int     `json:"total_input_tokens"`
	TotalOutputTokens int     `json:"total_output_tokens"`
	TotalCostUSD      float64 `json:"total_cost_usd"`
}

// CoordinatorData contains coordinator process data.
type CoordinatorData struct {
	Messages []json.RawMessage `json:"messages"`
	Raw      []json.RawMessage `json:"raw"`
}

// WorkerData contains worker process data.
type WorkerData struct {
	Messages              []json.RawMessage `json:"messages"`
	Raw                   []json.RawMessage `json:"raw"`
	AccountabilitySummary *string           `json:"accountabilitySummary,omitempty"`
}

// APIError provides consistent error response format.
type APIError struct {
	Error   string `json:"error"`
	Code    string `json:"code,omitempty"`
	Details string `json:"details,omitempty"`
}

// HealthResponse is the response for GET /api/health.
type HealthResponse struct {
	Status string `json:"status"`
}
