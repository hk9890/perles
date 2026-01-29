package frontend

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSessionListResponse_JSON(t *testing.T) {
	resp := SessionListResponse{
		BasePath: "/home/user/.perles/sessions",
		Apps: []AppSessions{
			{
				Name: "my-project",
				Dates: []DateGroup{
					{
						Date: "2026-01-29",
						Sessions: []SessionSummary{
							{
								ID:          "abc123",
								Path:        "/home/user/.perles/sessions/my-project/2026-01-29/abc123",
								StartTime:   stringPtr("2026-01-29T10:00:00Z"),
								Status:      "running",
								WorkerCount: 2,
								ClientType:  "claude",
							},
						},
					},
				},
			},
		},
	}

	data, err := json.Marshal(resp)
	require.NoError(t, err)

	// Verify JSON structure matches expected camelCase keys
	var m map[string]any
	require.NoError(t, json.Unmarshal(data, &m))

	require.Contains(t, m, "basePath")
	require.Contains(t, m, "apps")

	apps := m["apps"].([]any)
	require.Len(t, apps, 1)

	app := apps[0].(map[string]any)
	require.Contains(t, app, "name")
	require.Contains(t, app, "dates")

	dates := app["dates"].([]any)
	date := dates[0].(map[string]any)
	require.Contains(t, date, "date")
	require.Contains(t, date, "sessions")

	sessions := date["sessions"].([]any)
	session := sessions[0].(map[string]any)
	require.Equal(t, "abc123", session["id"])
	require.Equal(t, "2026-01-29T10:00:00Z", session["startTime"])
	require.Equal(t, "running", session["status"])
	require.Equal(t, float64(2), session["workerCount"])
	require.Equal(t, "claude", session["clientType"])
}

func TestSessionSummary_NilStartTime(t *testing.T) {
	summary := SessionSummary{
		ID:          "test-id",
		Path:        "/path/to/session",
		StartTime:   nil, // Not set
		Status:      "unknown",
		WorkerCount: 0,
		ClientType:  "amp",
	}

	data, err := json.Marshal(summary)
	require.NoError(t, err)

	// StartTime should be null, not omitted (no omitempty)
	var m map[string]any
	require.NoError(t, json.Unmarshal(data, &m))
	require.Contains(t, m, "startTime")
	require.Nil(t, m["startTime"])
}

func TestLoadSessionResponse_JSON(t *testing.T) {
	accountabilitySummary := "Task completed successfully"
	resp := LoadSessionResponse{
		Path: "/sessions/app/2026-01-29/session123",
		Metadata: &SessionMetadata{
			SessionID:             "session123",
			StartTime:             "2026-01-29T10:00:00Z",
			Status:                "completed",
			SessionDir:            "/sessions/app/2026-01-29/session123",
			CoordinatorSessionRef: "coord-ref-123",
			Resumable:             true,
			Workers: []WorkerMeta{
				{
					ID:                 "worker-1",
					SpawnedAt:          "2026-01-29T10:01:00Z",
					HeadlessSessionRef: "worker-ref-1",
					WorkDir:            "/project",
				},
			},
			ClientType: "claude",
			TokenUsage: TokenUsageSummary{
				TotalInputTokens:  10000,
				TotalOutputTokens: 5000,
				TotalCostUSD:      0.15,
			},
			ApplicationName: "my-app",
			WorkDir:         "/home/user/projects/my-app",
			DatePartition:   "2026-01-29",
		},
		Fabric:      []json.RawMessage{json.RawMessage(`{"event":"test"}`)},
		MCPRequests: []json.RawMessage{json.RawMessage(`{"method":"tools/call"}`)},
		Commands:    []json.RawMessage{json.RawMessage(`{"type":"spawn"}`)},
		Messages:    []json.RawMessage{json.RawMessage(`{"content":"hello"}`)},
		Coordinator: CoordinatorData{
			Messages: []json.RawMessage{json.RawMessage(`{"role":"assistant"}`)},
			Raw:      []json.RawMessage{json.RawMessage(`{"type":"message"}`)},
		},
		Workers: map[string]WorkerData{
			"worker-1": {
				Messages:              []json.RawMessage{json.RawMessage(`{"role":"user"}`)},
				Raw:                   []json.RawMessage{json.RawMessage(`{"type":"input"}`)},
				AccountabilitySummary: &accountabilitySummary,
			},
		},
	}

	data, err := json.Marshal(resp)
	require.NoError(t, err)

	var m map[string]any
	require.NoError(t, json.Unmarshal(data, &m))

	// Verify top-level keys
	require.Contains(t, m, "path")
	require.Contains(t, m, "metadata")
	require.Contains(t, m, "fabric")
	require.Contains(t, m, "mcpRequests")
	require.Contains(t, m, "commands")
	require.Contains(t, m, "messages")
	require.Contains(t, m, "coordinator")
	require.Contains(t, m, "workers")

	// Verify metadata snake_case keys (backend format)
	metadata := m["metadata"].(map[string]any)
	require.Contains(t, metadata, "session_id")
	require.Contains(t, metadata, "start_time")
	require.Contains(t, metadata, "coordinator_session_ref")
	require.Contains(t, metadata, "client_type")
	require.Contains(t, metadata, "token_usage")
	require.Contains(t, metadata, "application_name")
	require.Contains(t, metadata, "work_dir")
	require.Contains(t, metadata, "date_partition")

	// Verify token_usage structure
	tokenUsage := metadata["token_usage"].(map[string]any)
	require.Contains(t, tokenUsage, "total_input_tokens")
	require.Contains(t, tokenUsage, "total_output_tokens")
	require.Contains(t, tokenUsage, "total_cost_usd")

	// Verify workers structure
	workers := m["workers"].(map[string]any)
	worker1 := workers["worker-1"].(map[string]any)
	require.Contains(t, worker1, "messages")
	require.Contains(t, worker1, "raw")
	require.Contains(t, worker1, "accountabilitySummary")
	require.Equal(t, "Task completed successfully", worker1["accountabilitySummary"])
}

func TestLoadSessionResponse_NilMetadata(t *testing.T) {
	resp := LoadSessionResponse{
		Path:        "/path/to/session",
		Metadata:    nil,
		Fabric:      []json.RawMessage{},
		MCPRequests: []json.RawMessage{},
		Commands:    []json.RawMessage{},
		Messages:    []json.RawMessage{},
		Coordinator: CoordinatorData{
			Messages: []json.RawMessage{},
			Raw:      []json.RawMessage{},
		},
		Workers: map[string]WorkerData{},
	}

	data, err := json.Marshal(resp)
	require.NoError(t, err)

	var m map[string]any
	require.NoError(t, json.Unmarshal(data, &m))
	require.Contains(t, m, "metadata")
	require.Nil(t, m["metadata"])
}

func TestWorkerData_OmitsAccountabilitySummaryWhenNil(t *testing.T) {
	worker := WorkerData{
		Messages:              []json.RawMessage{json.RawMessage(`{}`)},
		Raw:                   []json.RawMessage{json.RawMessage(`{}`)},
		AccountabilitySummary: nil, // Not set
	}

	data, err := json.Marshal(worker)
	require.NoError(t, err)

	var m map[string]any
	require.NoError(t, json.Unmarshal(data, &m))

	// accountabilitySummary should be omitted due to omitempty
	require.NotContains(t, m, "accountabilitySummary")
}

func TestWorkerData_IncludesAccountabilitySummaryWhenSet(t *testing.T) {
	summary := "Completed all tasks"
	worker := WorkerData{
		Messages:              []json.RawMessage{},
		Raw:                   []json.RawMessage{},
		AccountabilitySummary: &summary,
	}

	data, err := json.Marshal(worker)
	require.NoError(t, err)

	var m map[string]any
	require.NoError(t, json.Unmarshal(data, &m))

	require.Contains(t, m, "accountabilitySummary")
	require.Equal(t, "Completed all tasks", m["accountabilitySummary"])
}

func TestAPIError_JSON(t *testing.T) {
	t.Run("full error", func(t *testing.T) {
		apiErr := APIError{
			Error:   "Session not found",
			Code:    "NOT_FOUND",
			Details: "No session exists at the specified path",
		}

		data, err := json.Marshal(apiErr)
		require.NoError(t, err)

		var m map[string]any
		require.NoError(t, json.Unmarshal(data, &m))

		require.Equal(t, "Session not found", m["error"])
		require.Equal(t, "NOT_FOUND", m["code"])
		require.Equal(t, "No session exists at the specified path", m["details"])
	})

	t.Run("minimal error", func(t *testing.T) {
		apiErr := APIError{
			Error: "Internal server error",
			// Code and Details omitted
		}

		data, err := json.Marshal(apiErr)
		require.NoError(t, err)

		var m map[string]any
		require.NoError(t, json.Unmarshal(data, &m))

		require.Contains(t, m, "error")
		require.NotContains(t, m, "code")    // omitempty
		require.NotContains(t, m, "details") // omitempty
	})
}

func TestHealthResponse_JSON(t *testing.T) {
	resp := HealthResponse{Status: "ok"}

	data, err := json.Marshal(resp)
	require.NoError(t, err)

	// Verify exact expected format
	require.JSONEq(t, `{"status":"ok"}`, string(data))
}

func TestLoadSessionRequest_JSON(t *testing.T) {
	req := LoadSessionRequest{
		Path: "/home/user/.perles/sessions/app/2026-01-29/session123",
	}

	data, err := json.Marshal(req)
	require.NoError(t, err)

	var m map[string]any
	require.NoError(t, json.Unmarshal(data, &m))

	require.Contains(t, m, "path")
	require.Equal(t, "/home/user/.perles/sessions/app/2026-01-29/session123", m["path"])
}

func TestLoadSessionRequest_Unmarshal(t *testing.T) {
	jsonData := `{"path":"/sessions/my-app/2026-01-29/abc123"}`

	var req LoadSessionRequest
	err := json.Unmarshal([]byte(jsonData), &req)
	require.NoError(t, err)

	require.Equal(t, "/sessions/my-app/2026-01-29/abc123", req.Path)
}

func TestSessionMetadata_FieldAlignment(t *testing.T) {
	// Test that SessionMetadata can be unmarshaled from actual metadata.json format
	// This validates alignment with internal/orchestration/session/metadata.go
	jsonData := `{
		"session_id": "test-session",
		"start_time": "2026-01-29T10:00:00Z",
		"status": "completed",
		"session_dir": "/sessions/app/2026-01-29/test-session",
		"coordinator_session_ref": "coord-ref",
		"resumable": true,
		"workers": [
			{
				"id": "worker-1",
				"spawned_at": "2026-01-29T10:01:00Z",
				"headless_session_ref": "worker-ref",
				"work_dir": "/project"
			}
		],
		"client_type": "claude",
		"token_usage": {
			"total_input_tokens": 5000,
			"total_output_tokens": 2000,
			"total_cost_usd": 0.10
		},
		"application_name": "test-app",
		"work_dir": "/home/user/projects/test-app",
		"date_partition": "2026-01-29"
	}`

	var meta SessionMetadata
	err := json.Unmarshal([]byte(jsonData), &meta)
	require.NoError(t, err)

	require.Equal(t, "test-session", meta.SessionID)
	require.Equal(t, "2026-01-29T10:00:00Z", meta.StartTime)
	require.Equal(t, "completed", meta.Status)
	require.Equal(t, "/sessions/app/2026-01-29/test-session", meta.SessionDir)
	require.Equal(t, "coord-ref", meta.CoordinatorSessionRef)
	require.True(t, meta.Resumable)
	require.Len(t, meta.Workers, 1)
	require.Equal(t, "worker-1", meta.Workers[0].ID)
	require.Equal(t, "2026-01-29T10:01:00Z", meta.Workers[0].SpawnedAt)
	require.Equal(t, "claude", meta.ClientType)
	require.Equal(t, 5000, meta.TokenUsage.TotalInputTokens)
	require.Equal(t, 2000, meta.TokenUsage.TotalOutputTokens)
	require.Equal(t, 0.10, meta.TokenUsage.TotalCostUSD)
	require.Equal(t, "test-app", meta.ApplicationName)
	require.Equal(t, "/home/user/projects/test-app", meta.WorkDir)
	require.Equal(t, "2026-01-29", meta.DatePartition)
}

func TestTokenUsageSummary_JSON(t *testing.T) {
	usage := TokenUsageSummary{
		TotalInputTokens:  15000,
		TotalOutputTokens: 8000,
		TotalCostUSD:      0.25,
	}

	data, err := json.Marshal(usage)
	require.NoError(t, err)

	var m map[string]any
	require.NoError(t, json.Unmarshal(data, &m))

	// Verify snake_case keys
	require.Contains(t, m, "total_input_tokens")
	require.Contains(t, m, "total_output_tokens")
	require.Contains(t, m, "total_cost_usd")

	require.Equal(t, float64(15000), m["total_input_tokens"])
	require.Equal(t, float64(8000), m["total_output_tokens"])
	require.Equal(t, 0.25, m["total_cost_usd"])
}

// stringPtr returns a pointer to the given string.
func stringPtr(s string) *string {
	return &s
}
