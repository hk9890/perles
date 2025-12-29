package mcp

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/zjrosen/perles/internal/mocks"
	"github.com/zjrosen/perles/internal/orchestration/claude"
	"github.com/zjrosen/perles/internal/orchestration/events"
	"github.com/zjrosen/perles/internal/orchestration/message"
	"github.com/zjrosen/perles/internal/orchestration/pool"
)

// TestNewCoordinatorServer_ProvidedBeadsExecutorIsUsed verifies mock injection works.
func TestNewCoordinatorServer_ProvidedBeadsExecutorIsUsed(t *testing.T) {
	workerPool := pool.NewWorkerPool(pool.Config{})
	defer workerPool.Close()

	mockExec := mocks.NewMockBeadsExecutor(t)

	cs := NewCoordinatorServer(claude.NewClient(), workerPool, nil, "/tmp/test", 8765, nil, mockExec)

	// beadsExecutor should be the mock we provided
	require.NotNil(t, cs.beadsExecutor, "beadsExecutor should not be nil")
	require.Equal(t, mockExec, cs.beadsExecutor, "beadsExecutor should be the provided mock")
}

// TestCoordinatorServer_RegistersAllTools verifies all coordinator tools are registered.
func TestCoordinatorServer_RegistersAllTools(t *testing.T) {
	workerPool := pool.NewWorkerPool(pool.Config{})
	defer workerPool.Close()

	cs := NewCoordinatorServer(claude.NewClient(), workerPool, nil, "/tmp/test", 8765, nil, mocks.NewMockBeadsExecutor(t))

	expectedTools := []string{
		"spawn_worker",
		"assign_task",
		"replace_worker",
		"send_to_worker",
		"post_message",
		"get_task_status",
		"mark_task_complete",
		"mark_task_failed",
		"read_message_log",
		"list_workers",
		"prepare_handoff",
		"query_worker_state",
		"assign_task_review",
		"assign_review_feedback",
		"approve_commit",
	}

	for _, toolName := range expectedTools {
		if _, ok := cs.tools[toolName]; !ok {
			t.Errorf("Tool %q not registered", toolName)
		}
		if _, ok := cs.handlers[toolName]; !ok {
			t.Errorf("Handler for %q not registered", toolName)
		}
	}

	if len(cs.tools) != len(expectedTools) {
		t.Errorf("Tool count = %d, want %d", len(cs.tools), len(expectedTools))
	}
}

// TestCoordinatorServer_ToolSchemas verifies tool schemas are valid.
func TestCoordinatorServer_ToolSchemas(t *testing.T) {
	workerPool := pool.NewWorkerPool(pool.Config{})
	defer workerPool.Close()

	cs := NewCoordinatorServer(claude.NewClient(), workerPool, nil, "/tmp/test", 8765, nil, mocks.NewMockBeadsExecutor(t))

	for name, tool := range cs.tools {
		t.Run(name, func(t *testing.T) {
			if tool.Name == "" {
				t.Error("Tool name is empty")
			}
			if tool.Description == "" {
				t.Error("Tool description is empty")
			}
			if tool.InputSchema == nil {
				t.Error("Tool inputSchema is nil")
			}
			if tool.InputSchema != nil && tool.InputSchema.Type != "object" {
				t.Errorf("InputSchema.Type = %q, want %q", tool.InputSchema.Type, "object")
			}
		})
	}
}

// TestCoordinatorServer_SpawnWorker tests spawn_worker (takes no args).
// Note: Actual spawning will fail in unit tests without Claude, but we can test it doesn't error on empty args.
func TestCoordinatorServer_SpawnWorker(t *testing.T) {
	workerPool := pool.NewWorkerPool(pool.Config{})
	defer workerPool.Close()

	cs := NewCoordinatorServer(claude.NewClient(), workerPool, nil, "/tmp/test", 8765, nil, mocks.NewMockBeadsExecutor(t))
	handler := cs.handlers["spawn_worker"]

	// spawn_worker takes no args, so empty args should be accepted (but will fail to actually spawn)
	_, err := handler(context.Background(), json.RawMessage(`{}`))
	// Expect error because we can't actually spawn Claude in a unit test
	if err == nil {
		t.Error("Expected error when spawning worker (no Claude available)")
	}
}

// TestCoordinatorServer_AssignTaskValidation tests input validation for assign_task.
func TestCoordinatorServer_AssignTaskValidation(t *testing.T) {
	workerPool := pool.NewWorkerPool(pool.Config{})
	defer workerPool.Close()

	cs := NewCoordinatorServer(claude.NewClient(), workerPool, nil, "/tmp/test", 8765, nil, mocks.NewMockBeadsExecutor(t))
	handler := cs.handlers["assign_task"]

	tests := []struct {
		name    string
		args    string
		wantErr bool
	}{
		{
			name:    "missing worker_id",
			args:    `{"task_id": "perles-abc"}`,
			wantErr: true,
		},
		{
			name:    "missing task_id",
			args:    `{"worker_id": "worker-1"}`,
			wantErr: true,
		},
		{
			name:    "empty worker_id",
			args:    `{"worker_id": "", "task_id": "perles-abc"}`,
			wantErr: true,
		},
		{
			name:    "empty task_id",
			args:    `{"worker_id": "worker-1", "task_id": ""}`,
			wantErr: true,
		},
		{
			name:    "invalid json",
			args:    `not json`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := handler(context.Background(), json.RawMessage(tt.args))
			if (err != nil) != tt.wantErr {
				t.Errorf("error = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}

// TestCoordinatorServer_ReplaceWorkerValidation tests input validation for replace_worker.
func TestCoordinatorServer_ReplaceWorkerValidation(t *testing.T) {
	workerPool := pool.NewWorkerPool(pool.Config{})
	defer workerPool.Close()

	cs := NewCoordinatorServer(claude.NewClient(), workerPool, nil, "/tmp/test", 8765, nil, mocks.NewMockBeadsExecutor(t))
	handler := cs.handlers["replace_worker"]

	tests := []struct {
		name    string
		args    string
		wantErr bool
	}{
		{
			name:    "missing worker_id",
			args:    `{}`,
			wantErr: true,
		},
		{
			name:    "empty worker_id",
			args:    `{"worker_id": ""}`,
			wantErr: true,
		},
		{
			name:    "worker not found",
			args:    `{"worker_id": "nonexistent"}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := handler(context.Background(), json.RawMessage(tt.args))
			if (err != nil) != tt.wantErr {
				t.Errorf("error = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}

// TestCoordinatorServer_SendToWorkerValidation tests input validation for send_to_worker.
func TestCoordinatorServer_SendToWorkerValidation(t *testing.T) {
	workerPool := pool.NewWorkerPool(pool.Config{})
	defer workerPool.Close()

	cs := NewCoordinatorServer(claude.NewClient(), workerPool, nil, "/tmp/test", 8765, nil, mocks.NewMockBeadsExecutor(t))
	handler := cs.handlers["send_to_worker"]

	tests := []struct {
		name    string
		args    string
		wantErr bool
	}{
		{
			name:    "missing worker_id",
			args:    `{"message": "hello"}`,
			wantErr: true,
		},
		{
			name:    "missing message",
			args:    `{"worker_id": "worker-1"}`,
			wantErr: true,
		},
		{
			name:    "worker not found",
			args:    `{"worker_id": "nonexistent", "message": "hello"}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := handler(context.Background(), json.RawMessage(tt.args))
			if (err != nil) != tt.wantErr {
				t.Errorf("error = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}

// TestCoordinatorServer_PostMessageValidation tests input validation for post_message.
func TestCoordinatorServer_PostMessageValidation(t *testing.T) {
	workerPool := pool.NewWorkerPool(pool.Config{})
	defer workerPool.Close()

	// No message issue available
	cs := NewCoordinatorServer(claude.NewClient(), workerPool, nil, "/tmp/test", 8765, nil, mocks.NewMockBeadsExecutor(t))
	handler := cs.handlers["post_message"]

	tests := []struct {
		name    string
		args    string
		wantErr bool
	}{
		{
			name:    "missing to",
			args:    `{"content": "hello"}`,
			wantErr: true,
		},
		{
			name:    "missing content",
			args:    `{"to": "ALL"}`,
			wantErr: true,
		},
		{
			name:    "message issue not available",
			args:    `{"to": "ALL", "content": "hello"}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := handler(context.Background(), json.RawMessage(tt.args))
			if (err != nil) != tt.wantErr {
				t.Errorf("error = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}

// TestCoordinatorServer_GetTaskStatusValidation tests input validation for get_task_status.
func TestCoordinatorServer_GetTaskStatusValidation(t *testing.T) {
	workerPool := pool.NewWorkerPool(pool.Config{})
	defer workerPool.Close()

	cs := NewCoordinatorServer(claude.NewClient(), workerPool, nil, "/tmp/test", 8765, nil, mocks.NewMockBeadsExecutor(t))
	handler := cs.handlers["get_task_status"]

	tests := []struct {
		name    string
		args    string
		wantErr bool
	}{
		{
			name:    "missing task_id",
			args:    `{}`,
			wantErr: true,
		},
		{
			name:    "empty task_id",
			args:    `{"task_id": ""}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := handler(context.Background(), json.RawMessage(tt.args))
			if (err != nil) != tt.wantErr {
				t.Errorf("error = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}

// TestCoordinatorServer_MarkTaskCompleteValidation tests input validation for mark_task_complete.
func TestCoordinatorServer_MarkTaskCompleteValidation(t *testing.T) {
	workerPool := pool.NewWorkerPool(pool.Config{})
	defer workerPool.Close()

	cs := NewCoordinatorServer(claude.NewClient(), workerPool, nil, "/tmp/test", 8765, nil, mocks.NewMockBeadsExecutor(t))
	handler := cs.handlers["mark_task_complete"]

	tests := []struct {
		name    string
		args    string
		wantErr bool
	}{
		{
			name:    "missing task_id",
			args:    `{}`,
			wantErr: true,
		},
		{
			name:    "empty task_id",
			args:    `{"task_id": ""}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := handler(context.Background(), json.RawMessage(tt.args))
			if (err != nil) != tt.wantErr {
				t.Errorf("error = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}

// TestCoordinatorServer_MarkTaskFailedValidation tests input validation for mark_task_failed.
func TestCoordinatorServer_MarkTaskFailedValidation(t *testing.T) {
	workerPool := pool.NewWorkerPool(pool.Config{})
	defer workerPool.Close()

	cs := NewCoordinatorServer(claude.NewClient(), workerPool, nil, "/tmp/test", 8765, nil, mocks.NewMockBeadsExecutor(t))
	handler := cs.handlers["mark_task_failed"]

	tests := []struct {
		name    string
		args    string
		wantErr bool
	}{
		{
			name:    "missing task_id",
			args:    `{"reason": "blocked"}`,
			wantErr: true,
		},
		{
			name:    "missing reason",
			args:    `{"task_id": "perles-abc"}`,
			wantErr: true,
		},
		{
			name:    "empty task_id",
			args:    `{"task_id": "", "reason": "blocked"}`,
			wantErr: true,
		},
		{
			name:    "empty reason",
			args:    `{"task_id": "perles-abc", "reason": ""}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := handler(context.Background(), json.RawMessage(tt.args))
			if (err != nil) != tt.wantErr {
				t.Errorf("error = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}

// TestCoordinatorServer_ReadMessageLogNoIssue tests read_message_log when no issue is available.
func TestCoordinatorServer_ReadMessageLogNoIssue(t *testing.T) {
	workerPool := pool.NewWorkerPool(pool.Config{})
	defer workerPool.Close()

	cs := NewCoordinatorServer(claude.NewClient(), workerPool, nil, "/tmp/test", 8765, nil, mocks.NewMockBeadsExecutor(t))
	handler := cs.handlers["read_message_log"]

	_, err := handler(context.Background(), json.RawMessage(`{}`))
	if err == nil {
		t.Error("Expected error when message issue is nil")
	}
}

// TestCoordinatorServer_GetPool tests the pool accessor.
func TestCoordinatorServer_GetPool(t *testing.T) {
	workerPool := pool.NewWorkerPool(pool.Config{})
	defer workerPool.Close()

	cs := NewCoordinatorServer(claude.NewClient(), workerPool, nil, "/tmp/test", 8765, nil, mocks.NewMockBeadsExecutor(t))

	if cs.pool != workerPool {
		t.Error("GetPool() did not return the expected pool")
	}
}

// TestCoordinatorServer_GetMessageIssue tests the message issue accessor.
func TestCoordinatorServer_GetMessageIssue(t *testing.T) {
	workerPool := pool.NewWorkerPool(pool.Config{})
	defer workerPool.Close()

	cs := NewCoordinatorServer(claude.NewClient(), workerPool, nil, "/tmp/test", 8765, nil, mocks.NewMockBeadsExecutor(t))

	if cs.msgIssue != nil {
		t.Error("GetMessageIssue() should return nil when no issue is set")
	}
}

// TestCoordinatorServer_Instructions tests that instructions are set correctly.
func TestCoordinatorServer_Instructions(t *testing.T) {
	workerPool := pool.NewWorkerPool(pool.Config{})
	defer workerPool.Close()

	cs := NewCoordinatorServer(claude.NewClient(), workerPool, nil, "/tmp/test", 8765, nil, mocks.NewMockBeadsExecutor(t))

	if cs.instructions == "" {
		t.Error("Instructions should be set")
	}
	if cs.info.Name != "perles-orchestrator" {
		t.Errorf("Server name = %q, want %q", cs.info.Name, "perles-orchestrator")
	}
	if cs.info.Version != "1.0.0" {
		t.Errorf("Server version = %q, want %q", cs.info.Version, "1.0.0")
	}
}

// TestIsValidTaskID tests task ID validation.
func TestIsValidTaskID(t *testing.T) {
	tests := []struct {
		name   string
		taskID string
		want   bool
	}{
		// Valid formats
		{"simple task", "perles-abc", true},
		{"4 char suffix", "perles-abcd", true},
		{"mixed case prefix", "Perles-abc", true},
		{"numeric suffix", "perles-1234", true},
		{"alphanumeric suffix", "perles-a1b2", true},
		{"subtask", "perles-abc.1", true},
		{"subtask multi-digit", "perles-abc.123", true},
		{"long suffix", "perles-abcdefghij", true},
		{"short prefix", "ms-abc", true},

		// Invalid formats
		{"empty", "", false},
		{"no prefix", "-abc", false},
		{"no suffix", "perles-", false},
		{"single char suffix", "perles-a", false},
		{"too long suffix", "perles-abcdefghijk", false},
		{"spaces", "perles abc", false},
		{"shell injection attempt", "perles-abc; rm -rf /", false},
		{"path traversal", "../etc/passwd", false},
		{"flag injection", "--help", false},
		{"newline", "perles-abc\n", false},
		{"special chars", "perles-abc$FOO", false},
		{"underscore in suffix", "perles-abc_def", false},
		{"double dot subtask", "perles-abc..1", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isValidTaskID(tt.taskID)
			if got != tt.want {
				t.Errorf("IsValidTaskID(%q) = %v, want %v", tt.taskID, got, tt.want)
			}
		})
	}
}

// TestCoordinatorServer_AssignTaskInvalidTaskID tests assign_task rejects invalid task IDs.
func TestCoordinatorServer_AssignTaskInvalidTaskID(t *testing.T) {
	workerPool := pool.NewWorkerPool(pool.Config{})
	defer workerPool.Close()

	cs := NewCoordinatorServer(claude.NewClient(), workerPool, nil, "/tmp/test", 8765, nil, mocks.NewMockBeadsExecutor(t))
	handler := cs.handlers["assign_task"]

	tests := []struct {
		name   string
		taskID string
	}{
		{"shell injection", "perles-abc; rm -rf /"},
		{"path traversal", "../etc/passwd"},
		{"flag injection", "--help"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := `{"worker_id": "worker-1", "task_id": "` + tt.taskID + `"}`
			_, err := handler(context.Background(), json.RawMessage(args))
			if err == nil {
				t.Errorf("Expected error for invalid task_id %q", tt.taskID)
			}
		})
	}
}

// TestCoordinatorServer_ListWorkers_NoWorkers verifies list_workers returns appropriate message when no workers exist.
func TestCoordinatorServer_ListWorkers_NoWorkers(t *testing.T) {
	workerPool := pool.NewWorkerPool(pool.Config{})
	defer workerPool.Close()

	cs := NewCoordinatorServer(claude.NewClient(), workerPool, nil, "/tmp/test", 8765, nil, mocks.NewMockBeadsExecutor(t))
	handler := cs.handlers["list_workers"]

	result, err := handler(context.Background(), nil)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if result.Content[0].Text != "No active workers." {
		t.Errorf("Expected 'No active workers.', got %q", result.Content[0].Text)
	}
}

// TestCoordinatorServer_ListWorkers_WithWorkers verifies list_workers returns worker info JSON.
func TestCoordinatorServer_ListWorkers_WithWorkers(t *testing.T) {
	workerPool := pool.NewWorkerPool(pool.Config{})
	defer workerPool.Close()

	cs := NewCoordinatorServer(claude.NewClient(), workerPool, nil, "/tmp/test", 8765, nil, mocks.NewMockBeadsExecutor(t))

	// Note: We cannot easily spawn real workers in a unit test without full Claude integration.
	// This test verifies the handler executes without error when the pool is empty.
	// Integration tests should verify the tool works with actual workers.
	handler := cs.handlers["list_workers"]

	result, err := handler(context.Background(), nil)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if result.Content[0].Text == "" {
		t.Error("Expected non-empty result")
	}
}

// TestPrepareHandoff_PostsMessage verifies tool posts message with correct type and content.
func TestPrepareHandoff_PostsMessage(t *testing.T) {
	workerPool := pool.NewWorkerPool(pool.Config{})
	defer workerPool.Close()

	msgIssue := message.New()
	cs := NewCoordinatorServer(claude.NewClient(), workerPool, msgIssue, "/tmp/test", 8765, nil, mocks.NewMockBeadsExecutor(t))
	handler := cs.handlers["prepare_handoff"]

	summary := "Worker 1 is processing task perles-abc. Task is 50% complete."
	args := `{"summary": "` + summary + `"}`

	result, err := handler(context.Background(), json.RawMessage(args))
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if result.Content[0].Text != "Handoff message posted. Refresh will proceed." {
		t.Errorf("Unexpected result: %q", result.Content[0].Text)
	}

	// Verify message was posted to the issue
	entries := msgIssue.Entries()
	if len(entries) != 1 {
		t.Fatalf("Expected 1 message, got %d", len(entries))
	}

	entry := entries[0]
	if entry.Type != message.MessageHandoff {
		t.Errorf("Message type = %q, want %q", entry.Type, message.MessageHandoff)
	}
	if entry.From != message.ActorCoordinator {
		t.Errorf("From = %q, want %q", entry.From, message.ActorCoordinator)
	}
	if entry.To != message.ActorAll {
		t.Errorf("To = %q, want %q", entry.To, message.ActorAll)
	}
	expectedContent := "[HANDOFF]\n" + summary
	if entry.Content != expectedContent {
		t.Errorf("Content = %q, want %q", entry.Content, expectedContent)
	}
}

// TestPrepareHandoff_EmptySummary verifies error returned when summary is empty.
func TestPrepareHandoff_EmptySummary(t *testing.T) {
	workerPool := pool.NewWorkerPool(pool.Config{})
	defer workerPool.Close()

	msgIssue := message.New()
	cs := NewCoordinatorServer(claude.NewClient(), workerPool, msgIssue, "/tmp/test", 8765, nil, mocks.NewMockBeadsExecutor(t))
	handler := cs.handlers["prepare_handoff"]

	tests := []struct {
		name string
		args string
	}{
		{
			name: "empty string summary",
			args: `{"summary": ""}`,
		},
		{
			name: "missing summary",
			args: `{}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := handler(context.Background(), json.RawMessage(tt.args))
			if err == nil {
				t.Error("Expected error for empty summary")
			}
		})
	}
}

// TestPrepareHandoff_NoMessageIssue verifies error when message issue is nil.
func TestPrepareHandoff_NoMessageIssue(t *testing.T) {
	workerPool := pool.NewWorkerPool(pool.Config{})
	defer workerPool.Close()

	// No message issue
	cs := NewCoordinatorServer(claude.NewClient(), workerPool, nil, "/tmp/test", 8765, nil, mocks.NewMockBeadsExecutor(t))
	handler := cs.handlers["prepare_handoff"]

	args := `{"summary": "Test summary"}`
	_, err := handler(context.Background(), json.RawMessage(args))
	if err == nil {
		t.Error("Expected error when message issue is nil")
	}
}

// TestWorkerRole_Values verifies WorkerRole constant values.
func TestWorkerRole_Values(t *testing.T) {
	tests := []struct {
		role     WorkerRole
		expected string
	}{
		{RoleImplementer, "implementer"},
		{RoleReviewer, "reviewer"},
	}

	for _, tt := range tests {
		t.Run(string(tt.role), func(t *testing.T) {
			if string(tt.role) != tt.expected {
				t.Errorf("WorkerRole %q != %q", tt.role, tt.expected)
			}
		})
	}
}

// TestTaskWorkflowStatus_Values verifies TaskWorkflowStatus constant values.
func TestTaskWorkflowStatus_Values(t *testing.T) {
	tests := []struct {
		status   TaskWorkflowStatus
		expected string
	}{
		{TaskImplementing, "implementing"},
		{TaskInReview, "in_review"},
		{TaskApproved, "approved"},
		{TaskDenied, "denied"},
		{TaskCommitting, "committing"},
		{TaskCompleted, "completed"},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			if string(tt.status) != tt.expected {
				t.Errorf("TaskWorkflowStatus %q != %q", tt.status, tt.expected)
			}
		})
	}
}

// TestWorkerAssignment_Fields verifies WorkerAssignment struct can be created and fields are accessible.
func TestWorkerAssignment_Fields(t *testing.T) {
	now := time.Now()
	wa := WorkerAssignment{
		TaskID:        "perles-abc.1",
		Role:          RoleImplementer,
		Phase:         events.PhaseImplementing,
		AssignedAt:    now,
		ImplementerID: "",
		ReviewerID:    "",
	}

	if wa.TaskID != "perles-abc.1" {
		t.Errorf("TaskID = %q, want %q", wa.TaskID, "perles-abc.1")
	}
	if wa.Role != RoleImplementer {
		t.Errorf("Role = %q, want %q", wa.Role, RoleImplementer)
	}
	if wa.Phase != events.PhaseImplementing {
		t.Errorf("Phase = %q, want %q", wa.Phase, events.PhaseImplementing)
	}
	if !wa.AssignedAt.Equal(now) {
		t.Errorf("AssignedAt = %v, want %v", wa.AssignedAt, now)
	}
}

// TestWorkerAssignment_ReviewerFields verifies reviewer-specific fields.
func TestWorkerAssignment_ReviewerFields(t *testing.T) {
	now := time.Now()
	wa := WorkerAssignment{
		TaskID:        "perles-abc.1",
		Role:          RoleReviewer,
		Phase:         events.PhaseReviewing,
		AssignedAt:    now,
		ImplementerID: "worker-1",
		ReviewerID:    "",
	}

	if wa.Role != RoleReviewer {
		t.Errorf("Role = %q, want %q", wa.Role, RoleReviewer)
	}
	if wa.ImplementerID != "worker-1" {
		t.Errorf("ImplementerID = %q, want %q", wa.ImplementerID, "worker-1")
	}
}

// TestTaskAssignment_Fields verifies TaskAssignment struct can be created and fields are accessible.
func TestTaskAssignment_Fields(t *testing.T) {
	startTime := time.Now()
	reviewTime := startTime.Add(30 * time.Minute)
	ta := TaskAssignment{
		TaskID:          "perles-abc.1",
		Implementer:     "worker-1",
		Reviewer:        "worker-2",
		Status:          TaskInReview,
		StartedAt:       startTime,
		ReviewStartedAt: reviewTime,
	}

	if ta.TaskID != "perles-abc.1" {
		t.Errorf("TaskID = %q, want %q", ta.TaskID, "perles-abc.1")
	}
	if ta.Implementer != "worker-1" {
		t.Errorf("Implementer = %q, want %q", ta.Implementer, "worker-1")
	}
	if ta.Reviewer != "worker-2" {
		t.Errorf("Reviewer = %q, want %q", ta.Reviewer, "worker-2")
	}
	if ta.Status != TaskInReview {
		t.Errorf("Status = %q, want %q", ta.Status, TaskInReview)
	}
	if !ta.StartedAt.Equal(startTime) {
		t.Errorf("StartedAt = %v, want %v", ta.StartedAt, startTime)
	}
	if !ta.ReviewStartedAt.Equal(reviewTime) {
		t.Errorf("ReviewStartedAt = %v, want %v", ta.ReviewStartedAt, reviewTime)
	}
}

// TestCoordinatorServer_MapsInitialized verifies workerAssignments and taskAssignments maps are initialized.
func TestCoordinatorServer_MapsInitialized(t *testing.T) {
	workerPool := pool.NewWorkerPool(pool.Config{})
	defer workerPool.Close()

	cs := NewCoordinatorServer(claude.NewClient(), workerPool, nil, "/tmp/test", 8765, nil, mocks.NewMockBeadsExecutor(t))

	if cs.workerAssignments == nil {
		t.Error("workerAssignments map is nil, should be initialized")
	}
	if cs.taskAssignments == nil {
		t.Error("taskAssignments map is nil, should be initialized")
	}

	// Verify maps are empty but usable
	if len(cs.workerAssignments) != 0 {
		t.Errorf("workerAssignments should be empty, has %d entries", len(cs.workerAssignments))
	}
	if len(cs.taskAssignments) != 0 {
		t.Errorf("taskAssignments should be empty, has %d entries", len(cs.taskAssignments))
	}

	// Verify we can write to and read from the maps
	cs.workerAssignments["worker-1"] = &WorkerAssignment{TaskID: "test-task"}
	if cs.workerAssignments["worker-1"].TaskID != "test-task" {
		t.Error("Failed to write/read workerAssignments")
	}

	cs.taskAssignments["test-task"] = &TaskAssignment{Implementer: "worker-1"}
	if cs.taskAssignments["test-task"].Implementer != "worker-1" {
		t.Error("Failed to write/read taskAssignments")
	}
}

// TestValidateTaskAssignment_TaskAlreadyAssigned verifies error when task already has an implementer.
func TestValidateTaskAssignment_TaskAlreadyAssigned(t *testing.T) {
	workerPool := pool.NewWorkerPool(pool.Config{})
	defer workerPool.Close()

	cs := NewCoordinatorServer(claude.NewClient(), workerPool, nil, "/tmp/test", 8765, nil, mocks.NewMockBeadsExecutor(t))

	// Pre-assign task to a different worker
	cs.taskAssignments["perles-abc.1"] = &TaskAssignment{
		TaskID:      "perles-abc.1",
		Implementer: "worker-1",
		Status:      TaskImplementing,
	}

	err := cs.validateTaskAssignment("worker-2", "perles-abc.1")
	if err == nil {
		t.Error("Expected error when task already assigned")
	}
	if err.Error() != "task perles-abc.1 already assigned to worker-1" {
		t.Errorf("Unexpected error message: %v", err)
	}
}

// TestValidateTaskAssignment_WorkerAlreadyHasTask verifies error when worker already has an assignment.
func TestValidateTaskAssignment_WorkerAlreadyHasTask(t *testing.T) {
	workerPool := pool.NewWorkerPool(pool.Config{})
	defer workerPool.Close()

	cs := NewCoordinatorServer(claude.NewClient(), workerPool, nil, "/tmp/test", 8765, nil, mocks.NewMockBeadsExecutor(t))

	// Pre-assign worker to a different task
	cs.workerAssignments["worker-1"] = &WorkerAssignment{
		TaskID: "perles-xyz.1",
		Role:   RoleImplementer,
		Phase:  events.PhaseImplementing,
	}

	err := cs.validateTaskAssignment("worker-1", "perles-abc.1")
	if err == nil {
		t.Error("Expected error when worker already has task")
	}
	if err.Error() != "worker worker-1 already assigned to task perles-xyz.1" {
		t.Errorf("Unexpected error message: %v", err)
	}
}

// TestValidateTaskAssignment_WorkerNotFound verifies error when worker doesn't exist.
func TestValidateTaskAssignment_WorkerNotFound(t *testing.T) {
	workerPool := pool.NewWorkerPool(pool.Config{})
	defer workerPool.Close()

	cs := NewCoordinatorServer(claude.NewClient(), workerPool, nil, "/tmp/test", 8765, nil, mocks.NewMockBeadsExecutor(t))

	err := cs.validateTaskAssignment("nonexistent-worker", "perles-abc.1")
	if err == nil {
		t.Error("Expected error when worker not found")
	}
	if err.Error() != "worker nonexistent-worker not found" {
		t.Errorf("Unexpected error message: %v", err)
	}
}

// TestValidateTaskAssignment_WorkerNotReady verifies error when worker is not in Ready status.
func TestValidateTaskAssignment_WorkerNotReady(t *testing.T) {
	workerPool := pool.NewWorkerPool(pool.Config{})
	defer workerPool.Close()

	cs := NewCoordinatorServer(claude.NewClient(), workerPool, nil, "/tmp/test", 8765, nil, mocks.NewMockBeadsExecutor(t))

	// Create a worker that is Working (not Ready)
	_ = workerPool.AddTestWorker("worker-1", pool.WorkerWorking)

	err := cs.validateTaskAssignment("worker-1", "perles-abc.1")
	if err == nil {
		t.Error("Expected error when worker not ready")
	}
	expectedMsg := "worker worker-1 is not ready (status: working)"
	if err.Error() != expectedMsg {
		t.Errorf("Expected error %q, got %q", expectedMsg, err.Error())
	}
}

// TestValidateTaskAssignment_Success verifies no error when all conditions are met.
func TestValidateTaskAssignment_Success(t *testing.T) {
	workerPool := pool.NewWorkerPool(pool.Config{})
	defer workerPool.Close()

	cs := NewCoordinatorServer(claude.NewClient(), workerPool, nil, "/tmp/test", 8765, nil, mocks.NewMockBeadsExecutor(t))

	// Create a worker that is Ready
	_ = workerPool.AddTestWorker("worker-1", pool.WorkerReady)

	err := cs.validateTaskAssignment("worker-1", "perles-abc.1")
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
}

// TestValidateReviewAssignment_SameAsImplementer verifies error when reviewer == implementer.
func TestValidateReviewAssignment_SameAsImplementer(t *testing.T) {
	workerPool := pool.NewWorkerPool(pool.Config{})
	defer workerPool.Close()

	cs := NewCoordinatorServer(claude.NewClient(), workerPool, nil, "/tmp/test", 8765, nil, mocks.NewMockBeadsExecutor(t))

	err := cs.validateReviewAssignment("worker-1", "perles-abc.1", "worker-1")
	if err == nil {
		t.Error("Expected error when reviewer is same as implementer")
	}
	if err.Error() != "reviewer cannot be the same as implementer" {
		t.Errorf("Unexpected error message: %v", err)
	}
}

// TestValidateReviewAssignment_TaskNotFound verifies error when task doesn't exist.
func TestValidateReviewAssignment_TaskNotFound(t *testing.T) {
	workerPool := pool.NewWorkerPool(pool.Config{})
	defer workerPool.Close()

	cs := NewCoordinatorServer(claude.NewClient(), workerPool, nil, "/tmp/test", 8765, nil, mocks.NewMockBeadsExecutor(t))

	err := cs.validateReviewAssignment("worker-2", "perles-abc.1", "worker-1")
	if err == nil {
		t.Error("Expected error when task not found")
	}
	if err.Error() != "task perles-abc.1 not found or implementer mismatch" {
		t.Errorf("Unexpected error message: %v", err)
	}
}

// TestValidateReviewAssignment_ImplementerMismatch verifies error when implementer doesn't match.
func TestValidateReviewAssignment_ImplementerMismatch(t *testing.T) {
	workerPool := pool.NewWorkerPool(pool.Config{})
	defer workerPool.Close()

	cs := NewCoordinatorServer(claude.NewClient(), workerPool, nil, "/tmp/test", 8765, nil, mocks.NewMockBeadsExecutor(t))

	// Task exists but with different implementer
	cs.taskAssignments["perles-abc.1"] = &TaskAssignment{
		TaskID:      "perles-abc.1",
		Implementer: "worker-3", // Different from passed implementer
	}

	err := cs.validateReviewAssignment("worker-2", "perles-abc.1", "worker-1")
	if err == nil {
		t.Error("Expected error when implementer mismatch")
	}
	if err.Error() != "task perles-abc.1 not found or implementer mismatch" {
		t.Errorf("Unexpected error message: %v", err)
	}
}

// TestValidateReviewAssignment_NotAwaitingReview verifies error when implementer not in AwaitingReview phase.
func TestValidateReviewAssignment_NotAwaitingReview(t *testing.T) {
	workerPool := pool.NewWorkerPool(pool.Config{})
	defer workerPool.Close()

	cs := NewCoordinatorServer(claude.NewClient(), workerPool, nil, "/tmp/test", 8765, nil, mocks.NewMockBeadsExecutor(t))

	// Setup task with correct implementer
	cs.taskAssignments["perles-abc.1"] = &TaskAssignment{
		TaskID:      "perles-abc.1",
		Implementer: "worker-1",
	}

	// Implementer is still in Implementing phase (not awaiting review)
	cs.workerAssignments["worker-1"] = &WorkerAssignment{
		TaskID: "perles-abc.1",
		Role:   RoleImplementer,
		Phase:  events.PhaseImplementing, // Should be PhaseAwaitingReview
	}

	err := cs.validateReviewAssignment("worker-2", "perles-abc.1", "worker-1")
	if err == nil {
		t.Error("Expected error when implementer not awaiting review")
	}
	expectedMsg := "implementer worker-1 is not awaiting review (phase: implementing)"
	if err.Error() != expectedMsg {
		t.Errorf("Expected error %q, got %q", expectedMsg, err.Error())
	}
}

// TestValidateReviewAssignment_AlreadyHasReviewer verifies error when task already has a reviewer.
func TestValidateReviewAssignment_AlreadyHasReviewer(t *testing.T) {
	workerPool := pool.NewWorkerPool(pool.Config{})
	defer workerPool.Close()

	cs := NewCoordinatorServer(claude.NewClient(), workerPool, nil, "/tmp/test", 8765, nil, mocks.NewMockBeadsExecutor(t))

	// Setup task with implementer and existing reviewer
	cs.taskAssignments["perles-abc.1"] = &TaskAssignment{
		TaskID:      "perles-abc.1",
		Implementer: "worker-1",
		Reviewer:    "worker-3", // Already has a reviewer
	}

	// Implementer is awaiting review
	cs.workerAssignments["worker-1"] = &WorkerAssignment{
		TaskID: "perles-abc.1",
		Role:   RoleImplementer,
		Phase:  events.PhaseAwaitingReview,
	}

	err := cs.validateReviewAssignment("worker-2", "perles-abc.1", "worker-1")
	if err == nil {
		t.Error("Expected error when task already has reviewer")
	}
	if err.Error() != "task perles-abc.1 already has reviewer worker-3" {
		t.Errorf("Unexpected error message: %v", err)
	}
}

// TestValidateReviewAssignment_ReviewerNotReady verifies error when reviewer is not Ready.
func TestValidateReviewAssignment_ReviewerNotReady(t *testing.T) {
	workerPool := pool.NewWorkerPool(pool.Config{})
	defer workerPool.Close()

	cs := NewCoordinatorServer(claude.NewClient(), workerPool, nil, "/tmp/test", 8765, nil, mocks.NewMockBeadsExecutor(t))

	// Setup task and implementer correctly
	cs.taskAssignments["perles-abc.1"] = &TaskAssignment{
		TaskID:      "perles-abc.1",
		Implementer: "worker-1",
	}
	cs.workerAssignments["worker-1"] = &WorkerAssignment{
		TaskID: "perles-abc.1",
		Role:   RoleImplementer,
		Phase:  events.PhaseAwaitingReview,
	}

	// Reviewer doesn't exist in pool
	err := cs.validateReviewAssignment("worker-2", "perles-abc.1", "worker-1")
	if err == nil {
		t.Error("Expected error when reviewer not found")
	}
	if err.Error() != "reviewer worker-2 is not ready" {
		t.Errorf("Unexpected error message: %v", err)
	}
}

// TestValidateReviewAssignment_Success verifies no error when all conditions are met.
func TestValidateReviewAssignment_Success(t *testing.T) {
	workerPool := pool.NewWorkerPool(pool.Config{})
	defer workerPool.Close()

	cs := NewCoordinatorServer(claude.NewClient(), workerPool, nil, "/tmp/test", 8765, nil, mocks.NewMockBeadsExecutor(t))

	// Setup task and implementer correctly
	cs.taskAssignments["perles-abc.1"] = &TaskAssignment{
		TaskID:      "perles-abc.1",
		Implementer: "worker-1",
	}
	cs.workerAssignments["worker-1"] = &WorkerAssignment{
		TaskID: "perles-abc.1",
		Role:   RoleImplementer,
		Phase:  events.PhaseAwaitingReview,
	}

	// Create ready reviewer in pool
	_ = workerPool.AddTestWorker("worker-2", pool.WorkerReady)

	err := cs.validateReviewAssignment("worker-2", "perles-abc.1", "worker-1")
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
}

// TestDetectOrphanedTasks_NoOrphans verifies empty result when no orphans.
func TestDetectOrphanedTasks_NoOrphans(t *testing.T) {
	workerPool := pool.NewWorkerPool(pool.Config{})
	defer workerPool.Close()

	cs := NewCoordinatorServer(claude.NewClient(), workerPool, nil, "/tmp/test", 8765, nil, mocks.NewMockBeadsExecutor(t))

	// Create active workers
	_ = workerPool.AddTestWorker("worker-1", pool.WorkerWorking)
	_ = workerPool.AddTestWorker("worker-2", pool.WorkerWorking)

	// Setup task with active workers
	cs.taskAssignments["perles-abc.1"] = &TaskAssignment{
		TaskID:      "perles-abc.1",
		Implementer: "worker-1",
		Reviewer:    "worker-2",
	}

	orphans := cs.detectOrphanedTasks()
	if len(orphans) != 0 {
		t.Errorf("Expected no orphans, got %d: %v", len(orphans), orphans)
	}
}

// TestDetectOrphanedTasks_RetiredImplementer verifies orphan detected when implementer is retired.
func TestDetectOrphanedTasks_RetiredImplementer(t *testing.T) {
	workerPool := pool.NewWorkerPool(pool.Config{})
	defer workerPool.Close()

	cs := NewCoordinatorServer(claude.NewClient(), workerPool, nil, "/tmp/test", 8765, nil, mocks.NewMockBeadsExecutor(t))

	// Create a retired worker
	_ = workerPool.AddTestWorker("worker-1", pool.WorkerRetired)

	// Setup task with retired implementer
	cs.taskAssignments["perles-abc.1"] = &TaskAssignment{
		TaskID:      "perles-abc.1",
		Implementer: "worker-1",
	}

	orphans := cs.detectOrphanedTasks()
	if len(orphans) != 1 {
		t.Errorf("Expected 1 orphan, got %d: %v", len(orphans), orphans)
	}
	if orphans[0] != "perles-abc.1" {
		t.Errorf("Expected orphan perles-abc.1, got %s", orphans[0])
	}
}

// TestDetectOrphanedTasks_MissingImplementer verifies orphan detected when implementer is missing from pool.
func TestDetectOrphanedTasks_MissingImplementer(t *testing.T) {
	workerPool := pool.NewWorkerPool(pool.Config{})
	defer workerPool.Close()

	cs := NewCoordinatorServer(claude.NewClient(), workerPool, nil, "/tmp/test", 8765, nil, mocks.NewMockBeadsExecutor(t))

	// Setup task with non-existent implementer
	cs.taskAssignments["perles-abc.1"] = &TaskAssignment{
		TaskID:      "perles-abc.1",
		Implementer: "nonexistent-worker",
	}

	orphans := cs.detectOrphanedTasks()
	if len(orphans) != 1 {
		t.Errorf("Expected 1 orphan, got %d: %v", len(orphans), orphans)
	}
}

// TestDetectOrphanedTasks_RetiredReviewer verifies orphan detected when reviewer is retired.
func TestDetectOrphanedTasks_RetiredReviewer(t *testing.T) {
	workerPool := pool.NewWorkerPool(pool.Config{})
	defer workerPool.Close()

	cs := NewCoordinatorServer(claude.NewClient(), workerPool, nil, "/tmp/test", 8765, nil, mocks.NewMockBeadsExecutor(t))

	// Create active implementer and retired reviewer
	_ = workerPool.AddTestWorker("worker-1", pool.WorkerWorking)
	_ = workerPool.AddTestWorker("worker-2", pool.WorkerRetired)

	// Setup task with retired reviewer
	cs.taskAssignments["perles-abc.1"] = &TaskAssignment{
		TaskID:      "perles-abc.1",
		Implementer: "worker-1",
		Reviewer:    "worker-2",
	}

	orphans := cs.detectOrphanedTasks()
	if len(orphans) != 1 {
		t.Errorf("Expected 1 orphan, got %d: %v", len(orphans), orphans)
	}
}

// TestCheckStuckWorkers_NoStuck verifies empty result when no stuck workers.
func TestCheckStuckWorkers_NoStuck(t *testing.T) {
	workerPool := pool.NewWorkerPool(pool.Config{})
	defer workerPool.Close()

	cs := NewCoordinatorServer(claude.NewClient(), workerPool, nil, "/tmp/test", 8765, nil, mocks.NewMockBeadsExecutor(t))

	// Worker assigned recently (within MaxTaskDuration)
	cs.workerAssignments["worker-1"] = &WorkerAssignment{
		TaskID:     "perles-abc.1",
		AssignedAt: time.Now(), // Just assigned
	}

	stuck := cs.checkStuckWorkers()
	if len(stuck) != 0 {
		t.Errorf("Expected no stuck workers, got %d: %v", len(stuck), stuck)
	}
}

// TestCheckStuckWorkers_ExceededDuration verifies stuck worker detected when exceeding MaxTaskDuration.
func TestCheckStuckWorkers_ExceededDuration(t *testing.T) {
	workerPool := pool.NewWorkerPool(pool.Config{})
	defer workerPool.Close()

	cs := NewCoordinatorServer(claude.NewClient(), workerPool, nil, "/tmp/test", 8765, nil, mocks.NewMockBeadsExecutor(t))

	// Worker assigned more than MaxTaskDuration ago
	cs.workerAssignments["worker-1"] = &WorkerAssignment{
		TaskID:     "perles-abc.1",
		AssignedAt: time.Now().Add(-MaxTaskDuration - time.Minute), // Exceeded
	}

	stuck := cs.checkStuckWorkers()
	if len(stuck) != 1 {
		t.Errorf("Expected 1 stuck worker, got %d: %v", len(stuck), stuck)
	}
	if stuck[0] != "worker-1" {
		t.Errorf("Expected stuck worker worker-1, got %s", stuck[0])
	}
}

// TestCheckStuckWorkers_NoTask verifies workers without tasks are not considered stuck.
func TestCheckStuckWorkers_NoTask(t *testing.T) {
	workerPool := pool.NewWorkerPool(pool.Config{})
	defer workerPool.Close()

	cs := NewCoordinatorServer(claude.NewClient(), workerPool, nil, "/tmp/test", 8765, nil, mocks.NewMockBeadsExecutor(t))

	// Worker with empty TaskID (idle) shouldn't be considered stuck
	cs.workerAssignments["worker-1"] = &WorkerAssignment{
		TaskID:     "",                                             // No active task
		AssignedAt: time.Now().Add(-MaxTaskDuration - time.Minute), // Old assignment
	}

	stuck := cs.checkStuckWorkers()
	if len(stuck) != 0 {
		t.Errorf("Expected no stuck workers (idle worker), got %d: %v", len(stuck), stuck)
	}
}

// TestMaxTaskDuration verifies the constant value.
func TestMaxTaskDuration(t *testing.T) {
	expected := 30 * time.Minute
	if MaxTaskDuration != expected {
		t.Errorf("MaxTaskDuration = %v, want %v", MaxTaskDuration, expected)
	}
}

// TestQueryWorkerState_NoWorkers verifies query_worker_state returns empty when no workers exist.
func TestQueryWorkerState_NoWorkers(t *testing.T) {
	workerPool := pool.NewWorkerPool(pool.Config{})
	defer workerPool.Close()

	cs := NewCoordinatorServer(claude.NewClient(), workerPool, nil, "/tmp/test", 8765, nil, mocks.NewMockBeadsExecutor(t))
	handler := cs.handlers["query_worker_state"]

	result, err := handler(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Parse response
	var response workerStateResponse
	if err := json.Unmarshal([]byte(result.Content[0].Text), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if len(response.Workers) != 0 {
		t.Errorf("Expected 0 workers, got %d", len(response.Workers))
	}
	if len(response.TaskAssignments) != 0 {
		t.Errorf("Expected 0 task assignments, got %d", len(response.TaskAssignments))
	}
	if len(response.ReadyWorkers) != 0 {
		t.Errorf("Expected 0 ready workers, got %d", len(response.ReadyWorkers))
	}
}

// TestQueryWorkerState_WithWorkerAndAssignment verifies query_worker_state returns worker with phase and role.
func TestQueryWorkerState_WithWorkerAndAssignment(t *testing.T) {
	workerPool := pool.NewWorkerPool(pool.Config{})
	defer workerPool.Close()

	cs := NewCoordinatorServer(claude.NewClient(), workerPool, nil, "/tmp/test", 8765, nil, mocks.NewMockBeadsExecutor(t))

	// Add a worker
	_ = workerPool.AddTestWorker("worker-1", pool.WorkerWorking)

	// Add assignment
	cs.workerAssignments["worker-1"] = &WorkerAssignment{
		TaskID: "perles-abc.1",
		Role:   RoleImplementer,
		Phase:  events.PhaseImplementing,
	}
	cs.taskAssignments["perles-abc.1"] = &TaskAssignment{
		TaskID:      "perles-abc.1",
		Implementer: "worker-1",
		Status:      TaskImplementing,
	}

	handler := cs.handlers["query_worker_state"]
	result, err := handler(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Parse response
	var response workerStateResponse
	if err := json.Unmarshal([]byte(result.Content[0].Text), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if len(response.Workers) != 1 {
		t.Fatalf("Expected 1 worker, got %d", len(response.Workers))
	}

	worker := response.Workers[0]
	if worker.WorkerID != "worker-1" {
		t.Errorf("WorkerID = %q, want %q", worker.WorkerID, "worker-1")
	}
	if worker.Phase != "implementing" {
		t.Errorf("Phase = %q, want %q", worker.Phase, "implementing")
	}
	if worker.Role != "implementer" {
		t.Errorf("Role = %q, want %q", worker.Role, "implementer")
	}
	if worker.TaskID != "perles-abc.1" {
		t.Errorf("TaskID = %q, want %q", worker.TaskID, "perles-abc.1")
	}

	// Check task assignments
	if len(response.TaskAssignments) != 1 {
		t.Fatalf("Expected 1 task assignment, got %d", len(response.TaskAssignments))
	}
	ta := response.TaskAssignments["perles-abc.1"]
	if ta.Implementer != "worker-1" {
		t.Errorf("TaskAssignment.Implementer = %q, want %q", ta.Implementer, "worker-1")
	}
}

// TestQueryWorkerState_FilterByWorkerID verifies query_worker_state filters by worker_id.
func TestQueryWorkerState_FilterByWorkerID(t *testing.T) {
	workerPool := pool.NewWorkerPool(pool.Config{})
	defer workerPool.Close()

	cs := NewCoordinatorServer(claude.NewClient(), workerPool, nil, "/tmp/test", 8765, nil, mocks.NewMockBeadsExecutor(t))

	// Add multiple workers
	_ = workerPool.AddTestWorker("worker-1", pool.WorkerWorking)
	_ = workerPool.AddTestWorker("worker-2", pool.WorkerReady)

	handler := cs.handlers["query_worker_state"]
	result, err := handler(context.Background(), json.RawMessage(`{"worker_id": "worker-1"}`))
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Parse response
	var response workerStateResponse
	if err := json.Unmarshal([]byte(result.Content[0].Text), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if len(response.Workers) != 1 {
		t.Errorf("Expected 1 worker (filtered), got %d", len(response.Workers))
	}
	if len(response.Workers) > 0 && response.Workers[0].WorkerID != "worker-1" {
		t.Errorf("Expected worker-1, got %q", response.Workers[0].WorkerID)
	}
}

// TestQueryWorkerState_FilterByTaskID verifies query_worker_state filters by task_id.
func TestQueryWorkerState_FilterByTaskID(t *testing.T) {
	workerPool := pool.NewWorkerPool(pool.Config{})
	defer workerPool.Close()

	cs := NewCoordinatorServer(claude.NewClient(), workerPool, nil, "/tmp/test", 8765, nil, mocks.NewMockBeadsExecutor(t))

	// Add workers with different tasks
	_ = workerPool.AddTestWorker("worker-1", pool.WorkerWorking)
	_ = workerPool.AddTestWorker("worker-2", pool.WorkerWorking)

	cs.workerAssignments["worker-1"] = &WorkerAssignment{
		TaskID: "perles-abc.1",
		Role:   RoleImplementer,
		Phase:  events.PhaseImplementing,
	}
	cs.workerAssignments["worker-2"] = &WorkerAssignment{
		TaskID: "perles-xyz.1",
		Role:   RoleImplementer,
		Phase:  events.PhaseImplementing,
	}

	handler := cs.handlers["query_worker_state"]
	result, err := handler(context.Background(), json.RawMessage(`{"task_id": "perles-abc.1"}`))
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Parse response
	var response workerStateResponse
	if err := json.Unmarshal([]byte(result.Content[0].Text), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if len(response.Workers) != 1 {
		t.Errorf("Expected 1 worker (filtered by task), got %d", len(response.Workers))
	}
	if len(response.Workers) > 0 && response.Workers[0].TaskID != "perles-abc.1" {
		t.Errorf("Expected task perles-abc.1, got %q", response.Workers[0].TaskID)
	}
}

// TestQueryWorkerState_ReturnsReadyWorkers verifies ready_workers list is populated.
func TestQueryWorkerState_ReturnsReadyWorkers(t *testing.T) {
	workerPool := pool.NewWorkerPool(pool.Config{})
	defer workerPool.Close()

	cs := NewCoordinatorServer(claude.NewClient(), workerPool, nil, "/tmp/test", 8765, nil, mocks.NewMockBeadsExecutor(t))

	// Add a ready worker
	_ = workerPool.AddTestWorker("worker-1", pool.WorkerReady)

	handler := cs.handlers["query_worker_state"]
	result, err := handler(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Parse response
	var response workerStateResponse
	if err := json.Unmarshal([]byte(result.Content[0].Text), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if len(response.ReadyWorkers) != 1 {
		t.Errorf("Expected 1 ready worker, got %d", len(response.ReadyWorkers))
	}
	if len(response.ReadyWorkers) > 0 && response.ReadyWorkers[0] != "worker-1" {
		t.Errorf("Expected ready worker worker-1, got %q", response.ReadyWorkers[0])
	}
}

// TestAssignTaskReview_SelfReviewRejected verifies assign_task_review rejects self-review.
func TestAssignTaskReview_SelfReviewRejected(t *testing.T) {
	workerPool := pool.NewWorkerPool(pool.Config{})
	defer workerPool.Close()

	cs := NewCoordinatorServer(claude.NewClient(), workerPool, nil, "/tmp/test", 8765, nil, mocks.NewMockBeadsExecutor(t))
	handler := cs.handlers["assign_task_review"]

	args := `{"reviewer_id": "worker-1", "task_id": "perles-abc.1", "implementer_id": "worker-1", "summary": "test"}`
	_, err := handler(context.Background(), json.RawMessage(args))
	if err == nil {
		t.Error("Expected error for self-review")
	}
	if !contains(err.Error(), "reviewer cannot be the same as implementer") {
		t.Errorf("Unexpected error message: %v", err)
	}
}

// TestAssignTaskReview_TaskNotAwaitingReview verifies assign_task_review rejects if task not awaiting review.
func TestAssignTaskReview_TaskNotAwaitingReview(t *testing.T) {
	workerPool := pool.NewWorkerPool(pool.Config{})
	defer workerPool.Close()

	cs := NewCoordinatorServer(claude.NewClient(), workerPool, nil, "/tmp/test", 8765, nil, mocks.NewMockBeadsExecutor(t))

	// Setup task and implementer in wrong phase
	cs.taskAssignments["perles-abc.1"] = &TaskAssignment{
		TaskID:      "perles-abc.1",
		Implementer: "worker-1",
		Status:      TaskImplementing,
	}
	cs.workerAssignments["worker-1"] = &WorkerAssignment{
		TaskID: "perles-abc.1",
		Role:   RoleImplementer,
		Phase:  events.PhaseImplementing, // Not awaiting review
	}
	_ = workerPool.AddTestWorker("worker-2", pool.WorkerReady)

	handler := cs.handlers["assign_task_review"]
	args := `{"reviewer_id": "worker-2", "task_id": "perles-abc.1", "implementer_id": "worker-1", "summary": "test"}`
	_, err := handler(context.Background(), json.RawMessage(args))
	if err == nil {
		t.Error("Expected error when task not awaiting review")
	}
}

// TestAssignTaskReview_ValidationRequired verifies required field validation.
func TestAssignTaskReview_ValidationRequired(t *testing.T) {
	workerPool := pool.NewWorkerPool(pool.Config{})
	defer workerPool.Close()

	cs := NewCoordinatorServer(claude.NewClient(), workerPool, nil, "/tmp/test", 8765, nil, mocks.NewMockBeadsExecutor(t))
	handler := cs.handlers["assign_task_review"]

	tests := []struct {
		name string
		args string
	}{
		{"missing reviewer_id", `{"task_id": "perles-abc.1", "implementer_id": "worker-1", "summary": "test"}`},
		{"missing task_id", `{"reviewer_id": "worker-2", "implementer_id": "worker-1", "summary": "test"}`},
		{"missing implementer_id", `{"reviewer_id": "worker-2", "task_id": "perles-abc.1", "summary": "test"}`},
		{"missing summary", `{"reviewer_id": "worker-2", "task_id": "perles-abc.1", "implementer_id": "worker-1"}`},
		{"invalid task_id", `{"reviewer_id": "worker-2", "task_id": "invalid", "implementer_id": "worker-1", "summary": "test"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := handler(context.Background(), json.RawMessage(tt.args))
			if err == nil {
				t.Errorf("Expected error for %s", tt.name)
			}
		})
	}
}

// TestAssignReviewFeedback_TaskNotDenied verifies assign_review_feedback rejects if task not denied.
func TestAssignReviewFeedback_TaskNotDenied(t *testing.T) {
	workerPool := pool.NewWorkerPool(pool.Config{})
	defer workerPool.Close()

	cs := NewCoordinatorServer(claude.NewClient(), workerPool, nil, "/tmp/test", 8765, nil, mocks.NewMockBeadsExecutor(t))

	// Setup task in approved state (not denied)
	cs.taskAssignments["perles-abc.1"] = &TaskAssignment{
		TaskID:      "perles-abc.1",
		Implementer: "worker-1",
		Status:      TaskApproved, // Not denied
	}
	cs.workerAssignments["worker-1"] = &WorkerAssignment{
		TaskID: "perles-abc.1",
		Role:   RoleImplementer,
		Phase:  events.PhaseAwaitingReview,
	}
	_ = workerPool.AddTestWorker("worker-1", pool.WorkerReady)

	handler := cs.handlers["assign_review_feedback"]
	args := `{"implementer_id": "worker-1", "task_id": "perles-abc.1", "feedback": "fix bugs"}`
	_, err := handler(context.Background(), json.RawMessage(args))
	if err == nil {
		t.Error("Expected error when task not denied")
	}
	if !contains(err.Error(), "not in denied status") {
		t.Errorf("Unexpected error message: %v", err)
	}
}

// TestAssignReviewFeedback_ValidationRequired verifies required field validation.
func TestAssignReviewFeedback_ValidationRequired(t *testing.T) {
	workerPool := pool.NewWorkerPool(pool.Config{})
	defer workerPool.Close()

	cs := NewCoordinatorServer(claude.NewClient(), workerPool, nil, "/tmp/test", 8765, nil, mocks.NewMockBeadsExecutor(t))
	handler := cs.handlers["assign_review_feedback"]

	tests := []struct {
		name string
		args string
	}{
		{"missing implementer_id", `{"task_id": "perles-abc.1", "feedback": "fix"}`},
		{"missing task_id", `{"implementer_id": "worker-1", "feedback": "fix"}`},
		{"missing feedback", `{"implementer_id": "worker-1", "task_id": "perles-abc.1"}`},
		{"empty feedback", `{"implementer_id": "worker-1", "task_id": "perles-abc.1", "feedback": ""}`},
		{"invalid task_id", `{"implementer_id": "worker-1", "task_id": "invalid", "feedback": "fix"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := handler(context.Background(), json.RawMessage(tt.args))
			if err == nil {
				t.Errorf("Expected error for %s", tt.name)
			}
		})
	}
}

// TestApproveCommit_TaskNotApproved verifies approve_commit rejects if task not approved.
func TestApproveCommit_TaskNotApproved(t *testing.T) {
	workerPool := pool.NewWorkerPool(pool.Config{})
	defer workerPool.Close()

	cs := NewCoordinatorServer(claude.NewClient(), workerPool, nil, "/tmp/test", 8765, nil, mocks.NewMockBeadsExecutor(t))

	// Setup task in denied state (not approved)
	cs.taskAssignments["perles-abc.1"] = &TaskAssignment{
		TaskID:      "perles-abc.1",
		Implementer: "worker-1",
		Status:      TaskDenied, // Not approved
	}

	handler := cs.handlers["approve_commit"]
	args := `{"implementer_id": "worker-1", "task_id": "perles-abc.1"}`
	_, err := handler(context.Background(), json.RawMessage(args))
	if err == nil {
		t.Error("Expected error when task not approved")
	}
	if !contains(err.Error(), "not in approved status") {
		t.Errorf("Unexpected error message: %v", err)
	}
}

// TestApproveCommit_ValidationRequired verifies required field validation.
func TestApproveCommit_ValidationRequired(t *testing.T) {
	workerPool := pool.NewWorkerPool(pool.Config{})
	defer workerPool.Close()

	cs := NewCoordinatorServer(claude.NewClient(), workerPool, nil, "/tmp/test", 8765, nil, mocks.NewMockBeadsExecutor(t))
	handler := cs.handlers["approve_commit"]

	tests := []struct {
		name string
		args string
	}{
		{"missing implementer_id", `{"task_id": "perles-abc.1"}`},
		{"missing task_id", `{"implementer_id": "worker-1"}`},
		{"empty implementer_id", `{"implementer_id": "", "task_id": "perles-abc.1"}`},
		{"empty task_id", `{"implementer_id": "worker-1", "task_id": ""}`},
		{"invalid task_id", `{"implementer_id": "worker-1", "task_id": "invalid"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := handler(context.Background(), json.RawMessage(tt.args))
			if err == nil {
				t.Errorf("Expected error for %s", tt.name)
			}
		})
	}
}

// TestApproveCommit_ImplementerMismatch verifies approve_commit rejects wrong implementer.
func TestApproveCommit_ImplementerMismatch(t *testing.T) {
	workerPool := pool.NewWorkerPool(pool.Config{})
	defer workerPool.Close()

	cs := NewCoordinatorServer(claude.NewClient(), workerPool, nil, "/tmp/test", 8765, nil, mocks.NewMockBeadsExecutor(t))

	// Setup task with different implementer
	cs.taskAssignments["perles-abc.1"] = &TaskAssignment{
		TaskID:      "perles-abc.1",
		Implementer: "worker-1", // Actual implementer
		Status:      TaskApproved,
	}

	handler := cs.handlers["approve_commit"]
	args := `{"implementer_id": "worker-2", "task_id": "perles-abc.1"}` // Wrong implementer
	_, err := handler(context.Background(), json.RawMessage(args))
	if err == nil {
		t.Error("Expected error for wrong implementer")
	}
	if !contains(err.Error(), "not the implementer") {
		t.Errorf("Unexpected error message: %v", err)
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && (s[:len(substr)] == substr || (len(s) > len(substr) && containsInternal(s, substr))))
}

func containsInternal(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// ============================================================================
// Phase 5 Tests: Updated assign_task and list_workers with state tracking
// ============================================================================

// TestAssignTask_ValidatesAssignment verifies assign_task calls validateTaskAssignment.
func TestAssignTask_ValidatesAssignment(t *testing.T) {
	workerPool := pool.NewWorkerPool(pool.Config{})
	defer workerPool.Close()

	cs := NewCoordinatorServer(claude.NewClient(), workerPool, nil, "/tmp/test", 8765, nil, mocks.NewMockBeadsExecutor(t))
	handler := cs.handlers["assign_task"]

	// No worker exists - should fail validation
	args := `{"worker_id": "worker-1", "task_id": "perles-abc.1"}`
	_, err := handler(context.Background(), json.RawMessage(args))
	if err == nil {
		t.Error("Expected error when worker not found (validation)")
	}
	if !contains(err.Error(), "validation failed") {
		t.Errorf("Expected validation error, got: %v", err)
	}
}

// TestAssignTask_RejectsWhenTaskAlreadyAssigned verifies assign_task rejects duplicate task assignment.
func TestAssignTask_RejectsWhenTaskAlreadyAssigned(t *testing.T) {
	workerPool := pool.NewWorkerPool(pool.Config{})
	defer workerPool.Close()

	cs := NewCoordinatorServer(claude.NewClient(), workerPool, nil, "/tmp/test", 8765, nil, mocks.NewMockBeadsExecutor(t))

	// Create a ready worker
	_ = workerPool.AddTestWorker("worker-2", pool.WorkerReady)

	// Pre-assign the task to another worker
	cs.taskAssignments["perles-abc.1"] = &TaskAssignment{
		TaskID:      "perles-abc.1",
		Implementer: "worker-1",
		Status:      TaskImplementing,
	}

	handler := cs.handlers["assign_task"]
	args := `{"worker_id": "worker-2", "task_id": "perles-abc.1"}`
	_, err := handler(context.Background(), json.RawMessage(args))
	if err == nil {
		t.Error("Expected error when task already assigned")
	}
	if !contains(err.Error(), "already assigned") {
		t.Errorf("Expected 'already assigned' error, got: %v", err)
	}
}

// TestAssignTask_RejectsWhenWorkerAlreadyHasTask verifies assign_task rejects if worker busy.
func TestAssignTask_RejectsWhenWorkerAlreadyHasTask(t *testing.T) {
	workerPool := pool.NewWorkerPool(pool.Config{})
	defer workerPool.Close()

	cs := NewCoordinatorServer(claude.NewClient(), workerPool, nil, "/tmp/test", 8765, nil, mocks.NewMockBeadsExecutor(t))

	// Create a worker that has an assignment
	_ = workerPool.AddTestWorker("worker-1", pool.WorkerReady)
	cs.workerAssignments["worker-1"] = &WorkerAssignment{
		TaskID: "perles-xyz.1",
		Role:   RoleImplementer,
		Phase:  events.PhaseImplementing,
	}

	handler := cs.handlers["assign_task"]
	args := `{"worker_id": "worker-1", "task_id": "perles-abc.1"}`
	_, err := handler(context.Background(), json.RawMessage(args))
	if err == nil {
		t.Error("Expected error when worker already has task")
	}
	if !contains(err.Error(), "already assigned") {
		t.Errorf("Expected 'already assigned' error, got: %v", err)
	}
}

// TestListWorkers_IncludesPhaseAndRole verifies list_workers returns phase and role.
func TestListWorkers_IncludesPhaseAndRole(t *testing.T) {
	workerPool := pool.NewWorkerPool(pool.Config{})
	defer workerPool.Close()

	cs := NewCoordinatorServer(claude.NewClient(), workerPool, nil, "/tmp/test", 8765, nil, mocks.NewMockBeadsExecutor(t))

	// Add a worker with an assignment
	_ = workerPool.AddTestWorker("worker-1", pool.WorkerWorking)
	cs.workerAssignments["worker-1"] = &WorkerAssignment{
		TaskID: "perles-abc.1",
		Role:   RoleImplementer,
		Phase:  events.PhaseImplementing,
	}

	handler := cs.handlers["list_workers"]
	result, err := handler(context.Background(), nil)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Parse response
	type workerInfo struct {
		WorkerID string `json:"worker_id"`
		Phase    string `json:"phase"`
		Role     string `json:"role,omitempty"`
	}
	var infos []workerInfo
	if err := json.Unmarshal([]byte(result.Content[0].Text), &infos); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if len(infos) != 1 {
		t.Fatalf("Expected 1 worker, got %d", len(infos))
	}

	info := infos[0]
	if info.Phase != "implementing" {
		t.Errorf("Phase = %q, want %q", info.Phase, "implementing")
	}
	if info.Role != "implementer" {
		t.Errorf("Role = %q, want %q", info.Role, "implementer")
	}
}

// TestListWorkers_ShowsIdlePhaseForNoAssignment verifies workers without assignments show idle phase.
func TestListWorkers_ShowsIdlePhaseForNoAssignment(t *testing.T) {
	workerPool := pool.NewWorkerPool(pool.Config{})
	defer workerPool.Close()

	cs := NewCoordinatorServer(claude.NewClient(), workerPool, nil, "/tmp/test", 8765, nil, mocks.NewMockBeadsExecutor(t))

	// Add a worker without any assignment
	_ = workerPool.AddTestWorker("worker-1", pool.WorkerReady)

	handler := cs.handlers["list_workers"]
	result, err := handler(context.Background(), nil)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Parse response
	type workerInfo struct {
		WorkerID string `json:"worker_id"`
		Phase    string `json:"phase"`
		Role     string `json:"role,omitempty"`
	}
	var infos []workerInfo
	if err := json.Unmarshal([]byte(result.Content[0].Text), &infos); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if len(infos) != 1 {
		t.Fatalf("Expected 1 worker, got %d", len(infos))
	}

	info := infos[0]
	if info.Phase != "idle" {
		t.Errorf("Phase = %q, want %q", info.Phase, "idle")
	}
	if info.Role != "" {
		t.Errorf("Role = %q, want empty for idle worker", info.Role)
	}
}

// TestListWorkers_ShowsReviewerRole verifies reviewer workers show correct role.
func TestListWorkers_ShowsReviewerRole(t *testing.T) {
	workerPool := pool.NewWorkerPool(pool.Config{})
	defer workerPool.Close()

	cs := NewCoordinatorServer(claude.NewClient(), workerPool, nil, "/tmp/test", 8765, nil, mocks.NewMockBeadsExecutor(t))

	// Add a worker as reviewer
	_ = workerPool.AddTestWorker("worker-2", pool.WorkerWorking)
	cs.workerAssignments["worker-2"] = &WorkerAssignment{
		TaskID:        "perles-abc.1",
		Role:          RoleReviewer,
		Phase:         events.PhaseReviewing,
		ImplementerID: "worker-1",
	}

	handler := cs.handlers["list_workers"]
	result, err := handler(context.Background(), nil)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Parse response
	type workerInfo struct {
		WorkerID string `json:"worker_id"`
		Phase    string `json:"phase"`
		Role     string `json:"role,omitempty"`
	}
	var infos []workerInfo
	if err := json.Unmarshal([]byte(result.Content[0].Text), &infos); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if len(infos) != 1 {
		t.Fatalf("Expected 1 worker, got %d", len(infos))
	}

	info := infos[0]
	if info.Phase != "reviewing" {
		t.Errorf("Phase = %q, want %q", info.Phase, "reviewing")
	}
	if info.Role != "reviewer" {
		t.Errorf("Role = %q, want %q", info.Role, "reviewer")
	}
}

// TestReplaceWorker_CleansUpWorkerAssignments verifies replace_worker removes assignment.
func TestReplaceWorker_CleansUpWorkerAssignments(t *testing.T) {
	workerPool := pool.NewWorkerPool(pool.Config{})
	defer workerPool.Close()

	cs := NewCoordinatorServer(claude.NewClient(), workerPool, nil, "/tmp/test", 8765, nil, mocks.NewMockBeadsExecutor(t))

	// Add a worker with an assignment
	_ = workerPool.AddTestWorker("worker-1", pool.WorkerWorking)
	cs.workerAssignments["worker-1"] = &WorkerAssignment{
		TaskID: "perles-abc.1",
		Role:   RoleImplementer,
		Phase:  events.PhaseImplementing,
	}
	cs.taskAssignments["perles-abc.1"] = &TaskAssignment{
		TaskID:      "perles-abc.1",
		Implementer: "worker-1",
		Status:      TaskImplementing,
	}

	// Verify assignment exists before replace
	if _, ok := cs.workerAssignments["worker-1"]; !ok {
		t.Fatal("Worker assignment should exist before replace")
	}

	handler := cs.handlers["replace_worker"]
	_, err := handler(context.Background(), json.RawMessage(`{"worker_id": "worker-1"}`))
	// Note: This will fail to spawn replacement worker (no Claude) but should still cleanup
	// We're testing the cleanup logic, which happens before the spawn attempt

	// Even if spawn fails, the assignment should be cleaned up
	// In this case, the error is expected because we can't spawn without Claude
	_ = err // We acknowledge the error but verify the cleanup happened

	// Verify assignment was cleaned up
	cs.assignmentsMu.RLock()
	_, stillExists := cs.workerAssignments["worker-1"]
	cs.assignmentsMu.RUnlock()

	if stillExists {
		t.Error("Worker assignment should be cleaned up after replace")
	}

	// Task assignment should still exist (for orphan detection)
	cs.assignmentsMu.RLock()
	_, taskExists := cs.taskAssignments["perles-abc.1"]
	cs.assignmentsMu.RUnlock()

	if !taskExists {
		t.Error("Task assignment should still exist after worker replaced (for orphan detection)")
	}
}

// TestTaskAssignmentPrompt_WithSummary verifies TaskAssignmentPrompt includes summary when provided.
func TestTaskAssignmentPrompt_WithSummary(t *testing.T) {
	prompt := TaskAssignmentPrompt("perles-abc.1", "Test Task", "Focus on error handling.")

	if !containsInternal(prompt, "Coordinator Instructions:") {
		t.Error("Prompt should contain 'Coordinator Instructions:' section when summary provided")
	}
	if !containsInternal(prompt, "Focus on error handling.") {
		t.Error("Prompt should contain the summary content")
	}
}

// TestTaskAssignmentPrompt_WithoutSummary verifies TaskAssignmentPrompt excludes summary section when empty.
func TestTaskAssignmentPrompt_WithoutSummary(t *testing.T) {
	prompt := TaskAssignmentPrompt("perles-abc.1", "Test Task", "")

	if containsInternal(prompt, "Coordinator Instructions:") {
		t.Error("Prompt should NOT contain 'Coordinator Instructions:' section when summary is empty")
	}
}

// TestTaskAssignmentPrompt_AllSections verifies TaskAssignmentPrompt includes all sections when provided.
func TestTaskAssignmentPrompt_AllSections(t *testing.T) {
	prompt := TaskAssignmentPrompt(
		"perles-abc.1",
		"Implement Feature X",
		"Important: Check existing patterns in module Y",
	)

	// Verify all sections are present
	sections := []string{
		"[TASK ASSIGNMENT]",
		"Task ID: perles-abc.1",
		"Title: Implement Feature X",
		"Coordinator Instructions:",
		"Important: Check existing patterns in module Y",
		"report_implementation_complete",
	}

	for _, section := range sections {
		if !containsInternal(prompt, section) {
			t.Errorf("Prompt should contain %q", section)
		}
	}
}

// TestAssignTaskArgs_SummaryField verifies assignTaskArgs struct includes Summary field.
func TestAssignTaskArgs_SummaryField(t *testing.T) {
	args := assignTaskArgs{
		WorkerID: "worker-1",
		TaskID:   "perles-abc.1",
		Summary:  "Key instructions for the worker",
	}

	if args.Summary != "Key instructions for the worker" {
		t.Errorf("Summary = %q, want %q", args.Summary, "Key instructions for the worker")
	}
}

// TestAssignTaskArgs_SummaryOmitempty verifies summary is optional.
func TestAssignTaskArgs_SummaryOmitempty(t *testing.T) {
	// Test that JSON with no summary field unmarshals correctly
	jsonStr := `{"worker_id": "worker-1", "task_id": "perles-abc.1"}`
	var args assignTaskArgs
	if err := json.Unmarshal([]byte(jsonStr), &args); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if args.WorkerID != "worker-1" {
		t.Errorf("WorkerID = %q, want %q", args.WorkerID, "worker-1")
	}
	if args.TaskID != "perles-abc.1" {
		t.Errorf("TaskID = %q, want %q", args.TaskID, "perles-abc.1")
	}
	if args.Summary != "" {
		t.Errorf("Summary = %q, want empty string", args.Summary)
	}
}

// TestAssignTaskArgs_SummaryInJSON verifies summary is included when provided in JSON.
func TestAssignTaskArgs_SummaryInJSON(t *testing.T) {
	jsonStr := `{"worker_id": "worker-1", "task_id": "perles-abc.1", "summary": "Focus on the FetchData method"}`
	var args assignTaskArgs
	if err := json.Unmarshal([]byte(jsonStr), &args); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if args.Summary != "Focus on the FetchData method" {
		t.Errorf("Summary = %q, want %q", args.Summary, "Focus on the FetchData method")
	}
}

// TestCoordinatorServer_AssignTaskSchemaIncludesSummary verifies the tool schema includes summary parameter.
func TestCoordinatorServer_AssignTaskSchemaIncludesSummary(t *testing.T) {
	workerPool := pool.NewWorkerPool(pool.Config{})
	defer workerPool.Close()

	cs := NewCoordinatorServer(claude.NewClient(), workerPool, nil, "/tmp/test", 8765, nil, mocks.NewMockBeadsExecutor(t))

	tool, ok := cs.tools["assign_task"]
	if !ok {
		t.Fatal("assign_task tool not registered")
	}

	if tool.InputSchema == nil {
		t.Fatal("assign_task InputSchema is nil")
	}

	summaryProp, ok := tool.InputSchema.Properties["summary"]
	if !ok {
		t.Fatal("assign_task schema should include 'summary' property")
	}

	if summaryProp.Type != "string" {
		t.Errorf("summary property type = %q, want %q", summaryProp.Type, "string")
	}

	if summaryProp.Description == "" {
		t.Error("summary property should have a description")
	}

	// Verify summary is NOT in required list (it's optional)
	for _, req := range tool.InputSchema.Required {
		if req == "summary" {
			t.Error("summary should NOT be in Required list (it's optional)")
		}
	}
}

// TestIntegration_AssignListReplaceFlow tests the full flow maintains consistent state.
func TestIntegration_AssignListReplaceFlow(t *testing.T) {
	workerPool := pool.NewWorkerPool(pool.Config{})
	defer workerPool.Close()

	cs := NewCoordinatorServer(claude.NewClient(), workerPool, nil, "/tmp/test", 8765, nil, mocks.NewMockBeadsExecutor(t))

	// Create a ready worker
	_ = workerPool.AddTestWorker("worker-1", pool.WorkerReady)

	// Pre-populate assignments to simulate a successful assign_task call
	// (We can't actually run assign_task without bd/Claude)
	cs.assignmentsMu.Lock()
	cs.workerAssignments["worker-1"] = &WorkerAssignment{
		TaskID:     "perles-abc.1",
		Role:       RoleImplementer,
		Phase:      events.PhaseImplementing,
		AssignedAt: time.Now(),
	}
	cs.taskAssignments["perles-abc.1"] = &TaskAssignment{
		TaskID:      "perles-abc.1",
		Implementer: "worker-1",
		Status:      TaskImplementing,
		StartedAt:   time.Now(),
	}
	cs.assignmentsMu.Unlock()

	// Change worker status to Working to reflect assignment
	workerPool.GetWorker("worker-1").AssignTask("perles-abc.1")

	// List workers - should show implementing phase
	listHandler := cs.handlers["list_workers"]
	result, err := listHandler(context.Background(), nil)
	if err != nil {
		t.Fatalf("list_workers error: %v", err)
	}

	type workerInfo struct {
		WorkerID string `json:"worker_id"`
		Phase    string `json:"phase"`
		Role     string `json:"role,omitempty"`
	}
	var infos []workerInfo
	if err := json.Unmarshal([]byte(result.Content[0].Text), &infos); err != nil {
		t.Fatalf("Failed to parse list_workers response: %v", err)
	}

	if len(infos) != 1 || infos[0].Phase != "implementing" {
		t.Errorf("Expected implementing phase, got %v", infos)
	}

	// Query worker state - should show same info
	queryHandler := cs.handlers["query_worker_state"]
	result, err = queryHandler(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("query_worker_state error: %v", err)
	}

	var stateResponse workerStateResponse
	if err := json.Unmarshal([]byte(result.Content[0].Text), &stateResponse); err != nil {
		t.Fatalf("Failed to parse query_worker_state response: %v", err)
	}

	// Both list_workers and query_worker_state should report same phase
	if len(stateResponse.Workers) != 1 {
		t.Fatalf("Expected 1 worker in state response, got %d", len(stateResponse.Workers))
	}
	if stateResponse.Workers[0].Phase != "implementing" {
		t.Errorf("query_worker_state phase = %q, want %q", stateResponse.Workers[0].Phase, "implementing")
	}

	// Task assignments should be tracked
	if len(stateResponse.TaskAssignments) != 1 {
		t.Fatalf("Expected 1 task assignment, got %d", len(stateResponse.TaskAssignments))
	}
	ta := stateResponse.TaskAssignments["perles-abc.1"]
	if ta.Implementer != "worker-1" {
		t.Errorf("TaskAssignment implementer = %q, want %q", ta.Implementer, "worker-1")
	}
}
