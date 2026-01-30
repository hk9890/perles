package session

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestMetadata_WorkflowFields_Serialization(t *testing.T) {
	now := time.Now().Truncate(time.Second) // Truncate for JSON round-trip

	meta := &Metadata{
		SessionID:                "test-session-123",
		StartTime:                now,
		Status:                   StatusCompleted,
		SessionDir:               "/test/dir",
		WorkflowCompletionStatus: "success",
		WorkflowCompletedAt:      now.Add(time.Hour),
		WorkflowSummary:          "All tasks completed successfully",
	}

	// Serialize to JSON
	data, err := json.Marshal(meta)
	require.NoError(t, err)

	// Verify fields are present in JSON
	var jsonMap map[string]any
	err = json.Unmarshal(data, &jsonMap)
	require.NoError(t, err)

	require.Equal(t, "success", jsonMap["workflow_completion_status"])
	require.NotEmpty(t, jsonMap["workflow_completed_at"])
	require.Equal(t, "All tasks completed successfully", jsonMap["workflow_summary"])
}

func TestMetadata_WorkflowFields_Deserialization(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	nowStr := now.Format(time.RFC3339)

	jsonStr := `{
		"session_id": "test-session-456",
		"start_time": "` + nowStr + `",
		"status": "completed",
		"session_dir": "/test/dir",
		"workflow_completion_status": "partial",
		"workflow_completed_at": "` + nowStr + `",
		"workflow_summary": "Completed 3 of 5 tasks",
		"workers": []
	}`

	var meta Metadata
	err := json.Unmarshal([]byte(jsonStr), &meta)
	require.NoError(t, err)

	require.Equal(t, "partial", meta.WorkflowCompletionStatus)
	require.True(t, now.Equal(meta.WorkflowCompletedAt), "WorkflowCompletedAt mismatch")
	require.Equal(t, "Completed 3 of 5 tasks", meta.WorkflowSummary)
}

func TestMetadata_WorkflowFields_Omitempty(t *testing.T) {
	meta := &Metadata{
		SessionID:  "test-session-789",
		StartTime:  time.Now().Truncate(time.Second),
		Status:     StatusRunning,
		SessionDir: "/test/dir",
		// Leave workflow fields empty/zero
		WorkflowCompletionStatus: "",
		WorkflowSummary:          "",
		// WorkflowCompletedAt is zero time
	}

	data, err := json.Marshal(meta)
	require.NoError(t, err)

	var jsonMap map[string]any
	err = json.Unmarshal(data, &jsonMap)
	require.NoError(t, err)

	// Empty string fields should be omitted
	_, hasStatus := jsonMap["workflow_completion_status"]
	require.False(t, hasStatus, "empty workflow_completion_status should be omitted")

	_, hasSummary := jsonMap["workflow_summary"]
	require.False(t, hasSummary, "empty workflow_summary should be omitted")
}

func TestMetadata_WorkflowCompletedAt_Omitzero(t *testing.T) {
	meta := &Metadata{
		SessionID:  "test-session-omitzero",
		StartTime:  time.Now().Truncate(time.Second),
		Status:     StatusRunning,
		SessionDir: "/test/dir",
		// WorkflowCompletedAt is zero time (default)
	}

	data, err := json.Marshal(meta)
	require.NoError(t, err)

	var jsonMap map[string]any
	err = json.Unmarshal(data, &jsonMap)
	require.NoError(t, err)

	// Zero time should be omitted via omitzero
	_, hasCompletedAt := jsonMap["workflow_completed_at"]
	require.False(t, hasCompletedAt, "zero workflow_completed_at should be omitted")
}

func TestMetadata_WorkflowFields_RoundTrip(t *testing.T) {
	now := time.Now().Truncate(time.Second)

	original := &Metadata{
		SessionID:                "test-roundtrip",
		StartTime:                now,
		Status:                   StatusCompleted,
		SessionDir:               "/test/dir",
		WorkflowCompletionStatus: "aborted",
		WorkflowCompletedAt:      now.Add(30 * time.Minute),
		WorkflowSummary:          "User cancelled workflow midway",
		ClientType:               "claude",
	}

	// Marshal
	data, err := json.Marshal(original)
	require.NoError(t, err)

	// Unmarshal
	var loaded Metadata
	err = json.Unmarshal(data, &loaded)
	require.NoError(t, err)

	// Verify workflow fields survived round-trip
	require.Equal(t, original.WorkflowCompletionStatus, loaded.WorkflowCompletionStatus)
	require.True(t, original.WorkflowCompletedAt.Equal(loaded.WorkflowCompletedAt),
		"WorkflowCompletedAt mismatch: expected %v, got %v",
		original.WorkflowCompletedAt, loaded.WorkflowCompletedAt)
	require.Equal(t, original.WorkflowSummary, loaded.WorkflowSummary)
}

func TestMetadata_WorkflowFields_BackwardCompatibility(t *testing.T) {
	// JSON without workflow fields (simulating older session metadata)
	jsonStr := `{
		"session_id": "old-session",
		"start_time": "2026-01-14T10:00:00Z",
		"status": "completed",
		"session_dir": "/old/dir",
		"coordinator_id": "coord-1",
		"workers": [],
		"client_type": "claude"
	}`

	var meta Metadata
	err := json.Unmarshal([]byte(jsonStr), &meta)
	require.NoError(t, err)

	// Workflow fields should be zero/empty values
	require.Empty(t, meta.WorkflowCompletionStatus)
	require.True(t, meta.WorkflowCompletedAt.IsZero())
	require.Empty(t, meta.WorkflowSummary)

	// Other fields should load correctly
	require.Equal(t, "old-session", meta.SessionID)
	require.Equal(t, "coord-1", meta.CoordinatorID)
}

func TestMetadata_WorkflowCompletionStatus_AllValues(t *testing.T) {
	testCases := []struct {
		status string
	}{
		{"success"},
		{"partial"},
		{"aborted"},
		{""},
	}

	for _, tc := range testCases {
		t.Run("status_"+tc.status, func(t *testing.T) {
			meta := &Metadata{
				SessionID:                "test-status",
				StartTime:                time.Now().Truncate(time.Second),
				Status:                   StatusRunning,
				SessionDir:               "/test/dir",
				WorkflowCompletionStatus: tc.status,
			}

			data, err := json.Marshal(meta)
			require.NoError(t, err)

			var loaded Metadata
			err = json.Unmarshal(data, &loaded)
			require.NoError(t, err)

			require.Equal(t, tc.status, loaded.WorkflowCompletionStatus)
		})
	}
}

func TestMetadata_WorkflowID_Serialization(t *testing.T) {
	now := time.Now().Truncate(time.Second)

	meta := &Metadata{
		SessionID:  "test-session-wfid",
		StartTime:  now,
		Status:     StatusRunning,
		SessionDir: "/test/dir",
		WorkflowID: "workflow-abc-123",
	}

	// Serialize to JSON
	data, err := json.Marshal(meta)
	require.NoError(t, err)

	// Verify field is present in JSON with correct key
	var jsonMap map[string]any
	err = json.Unmarshal(data, &jsonMap)
	require.NoError(t, err)

	require.Equal(t, "workflow-abc-123", jsonMap["workflow_id"])
}

func TestMetadata_WorkflowID_Deserialization(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	nowStr := now.Format(time.RFC3339)

	jsonStr := `{
		"session_id": "test-session-wfid-deser",
		"start_time": "` + nowStr + `",
		"status": "running",
		"session_dir": "/test/dir",
		"workflow_id": "workflow-xyz-789",
		"workers": []
	}`

	var meta Metadata
	err := json.Unmarshal([]byte(jsonStr), &meta)
	require.NoError(t, err)

	require.Equal(t, "workflow-xyz-789", meta.WorkflowID)
}

func TestMetadata_WorkflowID_Omitempty(t *testing.T) {
	meta := &Metadata{
		SessionID:  "test-session-wfid-empty",
		StartTime:  time.Now().Truncate(time.Second),
		Status:     StatusRunning,
		SessionDir: "/test/dir",
		WorkflowID: "", // Empty - should be omitted
	}

	data, err := json.Marshal(meta)
	require.NoError(t, err)

	var jsonMap map[string]any
	err = json.Unmarshal(data, &jsonMap)
	require.NoError(t, err)

	// Empty workflow_id should be omitted
	_, hasWorkflowID := jsonMap["workflow_id"]
	require.False(t, hasWorkflowID, "empty workflow_id should be omitted")
}

func TestMetadata_WorkflowID_RoundTrip(t *testing.T) {
	now := time.Now().Truncate(time.Second)

	original := &Metadata{
		SessionID:  "test-roundtrip-wfid",
		StartTime:  now,
		Status:     StatusCompleted,
		SessionDir: "/test/dir",
		WorkflowID: "workflow-roundtrip-456",
		ClientType: "claude",
	}

	// Marshal
	data, err := json.Marshal(original)
	require.NoError(t, err)

	// Unmarshal
	var loaded Metadata
	err = json.Unmarshal(data, &loaded)
	require.NoError(t, err)

	// Verify workflow_id survived round-trip
	require.Equal(t, original.WorkflowID, loaded.WorkflowID)
}

func TestMetadata_WorkflowID_BackwardCompatibility(t *testing.T) {
	// JSON without workflow_id field (simulating older session metadata)
	jsonStr := `{
		"session_id": "old-session-no-wfid",
		"start_time": "2026-01-14T10:00:00Z",
		"status": "completed",
		"session_dir": "/old/dir",
		"coordinator_id": "coord-1",
		"workers": [],
		"client_type": "claude"
	}`

	var meta Metadata
	err := json.Unmarshal([]byte(jsonStr), &meta)
	require.NoError(t, err)

	// WorkflowID should be empty for old sessions
	require.Empty(t, meta.WorkflowID)

	// Other fields should load correctly
	require.Equal(t, "old-session-no-wfid", meta.SessionID)
	require.Equal(t, "coord-1", meta.CoordinatorID)
}
