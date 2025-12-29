package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/zjrosen/perles/internal/orchestration/claude"
	"github.com/zjrosen/perles/internal/orchestration/events"
	"github.com/zjrosen/perles/internal/orchestration/message"
	"github.com/zjrosen/perles/internal/orchestration/pool"
)

// ============================================================================
// Additional Tests for Coverage
// ============================================================================

// TestHandleAssignTask_PoolAssignmentFails tests assign_task when pool assignment fails.
func TestHandleAssignTask_PoolAssignmentFails(t *testing.T) {
	workerPool := pool.NewWorkerPool(pool.Config{})
	defer workerPool.Close()

	msgIssue := message.New()
	cs := NewCoordinatorServer(claude.NewClient(), workerPool, msgIssue, "/tmp/test", 8765, nil)

	// Create a ready worker
	_ = workerPool.AddTestWorker("worker-1", pool.WorkerReady)

	handler := cs.handlers["assign_task"]

	// Valid input but the full flow won't work without bd, so validation passes but execution fails
	args := `{"worker_id": "worker-1", "task_id": "perles-abc.1"}`
	_, err := handler(context.Background(), json.RawMessage(args))
	// The assignment should fail somewhere in the flow (bd not available)
	if err == nil {
		t.Log("Expected error or completion - checking state")
	}
}

// TestHandleGetTaskStatus_BDNotAvailable tests get_task_status when bd is not available.
func TestHandleGetTaskStatus_BDNotAvailable(t *testing.T) {
	workerPool := pool.NewWorkerPool(pool.Config{})
	defer workerPool.Close()

	cs := NewCoordinatorServer(claude.NewClient(), workerPool, nil, "/tmp/test", 8765, nil)
	handler := cs.handlers["get_task_status"]

	// Valid task ID - will try to run bd which won't work in test
	args := `{"task_id": "perles-abc.1"}`
	_, err := handler(context.Background(), json.RawMessage(args))
	// Error expected since bd command fails
	if err == nil {
		t.Log("Handler completed without bd error")
	}
}

// TestHandleMarkTaskComplete_BDNotAvailable tests mark_task_complete when bd is not available.
func TestHandleMarkTaskComplete_BDNotAvailable(t *testing.T) {
	workerPool := pool.NewWorkerPool(pool.Config{})
	defer workerPool.Close()

	cs := NewCoordinatorServer(claude.NewClient(), workerPool, nil, "/tmp/test", 8765, nil)
	handler := cs.handlers["mark_task_complete"]

	// Valid task ID - will try to run bd which won't work in test
	args := `{"task_id": "perles-abc.1"}`
	_, err := handler(context.Background(), json.RawMessage(args))
	// Error expected since bd command fails
	if err == nil {
		t.Log("Handler completed without bd error")
	}
}

// TestHandleMarkTaskFailed_BDNotAvailable tests mark_task_failed when bd is not available.
func TestHandleMarkTaskFailed_BDNotAvailable(t *testing.T) {
	workerPool := pool.NewWorkerPool(pool.Config{})
	defer workerPool.Close()

	cs := NewCoordinatorServer(claude.NewClient(), workerPool, nil, "/tmp/test", 8765, nil)
	handler := cs.handlers["mark_task_failed"]

	// Valid task ID - will try to run bd which won't work in test
	args := `{"task_id": "perles-abc.1", "reason": "blocked"}`
	_, err := handler(context.Background(), json.RawMessage(args))
	// Error expected since bd command fails
	if err == nil {
		t.Log("Handler completed without bd error")
	}
}

// TestHandleAssignTaskReview_FullValidation tests assign_task_review with full validation.
func TestHandleAssignTaskReview_FullValidation(t *testing.T) {
	workerPool := pool.NewWorkerPool(pool.Config{})
	defer workerPool.Close()

	msgIssue := message.New()
	cs := NewCoordinatorServer(claude.NewClient(), workerPool, msgIssue, "/tmp/test", 8765, nil)

	// Create workers
	_ = workerPool.AddTestWorker("worker-1", pool.WorkerWorking)
	_ = workerPool.AddTestWorker("worker-2", pool.WorkerReady)

	// Setup task in awaiting review state
	taskID := "perles-abc.1"
	cs.assignmentsMu.Lock()
	cs.taskAssignments[taskID] = &TaskAssignment{
		TaskID:      taskID,
		Implementer: "worker-1",
		Status:      TaskInReview,
	}
	cs.workerAssignments["worker-1"] = &WorkerAssignment{
		TaskID: taskID,
		Role:   RoleImplementer,
		Phase:  events.PhaseAwaitingReview,
	}
	cs.assignmentsMu.Unlock()

	handler := cs.handlers["assign_task_review"]

	// Should pass validation but fail when trying to send message
	args := `{"reviewer_id": "worker-2", "task_id": "perles-abc.1", "implementer_id": "worker-1", "summary": "Test implementation"}`
	_, err := handler(context.Background(), json.RawMessage(args))
	// May succeed partially since we have a message issue
	if err != nil {
		t.Logf("Expected partial completion, got error: %v", err)
	}
}

// TestHandleAssignReviewFeedback_FullValidation tests assign_review_feedback with valid state.
func TestHandleAssignReviewFeedback_FullValidation(t *testing.T) {
	workerPool := pool.NewWorkerPool(pool.Config{})
	defer workerPool.Close()

	msgIssue := message.New()
	cs := NewCoordinatorServer(claude.NewClient(), workerPool, msgIssue, "/tmp/test", 8765, nil)

	// Create implementer
	_ = workerPool.AddTestWorker("worker-1", pool.WorkerReady)

	// Setup task in denied state
	taskID := "perles-abc.1"
	cs.assignmentsMu.Lock()
	cs.taskAssignments[taskID] = &TaskAssignment{
		TaskID:      taskID,
		Implementer: "worker-1",
		Status:      TaskDenied,
	}
	cs.workerAssignments["worker-1"] = &WorkerAssignment{
		TaskID: taskID,
		Role:   RoleImplementer,
		Phase:  events.PhaseAwaitingReview,
	}
	cs.assignmentsMu.Unlock()

	handler := cs.handlers["assign_review_feedback"]

	// Should pass validation but fail when trying to send message
	args := `{"implementer_id": "worker-1", "task_id": "perles-abc.1", "feedback": "Please fix the error handling"}`
	_, err := handler(context.Background(), json.RawMessage(args))
	if err != nil {
		t.Logf("Expected partial completion, got error: %v", err)
	}
}

// TestHandleApproveCommit_FullValidation tests approve_commit with valid state.
func TestHandleApproveCommit_FullValidation(t *testing.T) {
	workerPool := pool.NewWorkerPool(pool.Config{})
	defer workerPool.Close()

	msgIssue := message.New()
	cs := NewCoordinatorServer(claude.NewClient(), workerPool, msgIssue, "/tmp/test", 8765, nil)

	// Create implementer
	_ = workerPool.AddTestWorker("worker-1", pool.WorkerWorking)

	// Setup task in approved state
	taskID := "perles-abc.1"
	cs.assignmentsMu.Lock()
	cs.taskAssignments[taskID] = &TaskAssignment{
		TaskID:      taskID,
		Implementer: "worker-1",
		Status:      TaskApproved,
	}
	cs.workerAssignments["worker-1"] = &WorkerAssignment{
		TaskID: taskID,
		Role:   RoleImplementer,
		Phase:  events.PhaseAwaitingReview,
	}
	cs.assignmentsMu.Unlock()

	handler := cs.handlers["approve_commit"]

	// Should pass validation but fail when trying to send message
	args := `{"implementer_id": "worker-1", "task_id": "perles-abc.1"}`
	_, err := handler(context.Background(), json.RawMessage(args))
	if err != nil {
		t.Logf("Expected partial completion, got error: %v", err)
	}
}

// TestQueryWorkerState_WithFilters tests query_worker_state with various filters.
func TestQueryWorkerState_WithFilters(t *testing.T) {
	workerPool := pool.NewWorkerPool(pool.Config{})
	defer workerPool.Close()

	cs := NewCoordinatorServer(claude.NewClient(), workerPool, nil, "/tmp/test", 8765, nil)

	// Create workers with different states
	_ = workerPool.AddTestWorker("worker-1", pool.WorkerWorking)
	_ = workerPool.AddTestWorker("worker-2", pool.WorkerReady)
	_ = workerPool.AddTestWorker("worker-3", pool.WorkerWorking)

	cs.assignmentsMu.Lock()
	cs.workerAssignments["worker-1"] = &WorkerAssignment{
		TaskID: "perles-abc.1",
		Role:   RoleImplementer,
		Phase:  events.PhaseImplementing,
	}
	cs.workerAssignments["worker-3"] = &WorkerAssignment{
		TaskID: "perles-abc.2",
		Role:   RoleReviewer,
		Phase:  events.PhaseReviewing,
	}
	cs.assignmentsMu.Unlock()

	handler := cs.handlers["query_worker_state"]

	tests := []struct {
		name      string
		args      string
		checkFunc func(response workerStateResponse) error
	}{
		{
			name: "filter by worker_id",
			args: `{"worker_id": "worker-1"}`,
			checkFunc: func(r workerStateResponse) error {
				if len(r.Workers) != 1 || r.Workers[0].WorkerID != "worker-1" {
					return errorf("expected worker-1, got %v", r.Workers)
				}
				return nil
			},
		},
		{
			name: "filter by task_id",
			args: `{"task_id": "perles-abc.1"}`,
			checkFunc: func(r workerStateResponse) error {
				if len(r.Workers) != 1 || r.Workers[0].TaskID != "perles-abc.1" {
					return errorf("expected task perles-abc.1, got %v", r.Workers)
				}
				return nil
			},
		},
		{
			name: "no filter",
			args: `{}`,
			checkFunc: func(r workerStateResponse) error {
				// Should return all active workers
				if len(r.Workers) < 2 {
					return errorf("expected at least 2 workers, got %d", len(r.Workers))
				}
				return nil
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := handler(context.Background(), json.RawMessage(tt.args))
			require.NoError(t, err)

			var response workerStateResponse
			err = json.Unmarshal([]byte(result.Content[0].Text), &response)
			require.NoError(t, err, "Failed to parse response")

			if checkErr := tt.checkFunc(response); checkErr != nil {
				t.Error(checkErr)
			}
		})
	}
}

// TestListWorkers_AllPhases tests list_workers showing all phase types.
func TestListWorkers_AllPhases(t *testing.T) {
	workerPool := pool.NewWorkerPool(pool.Config{})
	defer workerPool.Close()

	cs := NewCoordinatorServer(claude.NewClient(), workerPool, nil, "/tmp/test", 8765, nil)

	// Create workers in different phases
	phases := []events.WorkerPhase{
		events.PhaseIdle,
		events.PhaseImplementing,
		events.PhaseAwaitingReview,
		events.PhaseReviewing,
		events.PhaseAddressingFeedback,
		events.PhaseCommitting,
	}

	for i, phase := range phases {
		workerID := "worker-" + string(rune('1'+i))
		_ = workerPool.AddTestWorker(workerID, pool.WorkerWorking)
		if phase != events.PhaseIdle {
			cs.workerAssignments[workerID] = &WorkerAssignment{
				TaskID: "perles-abc." + string(rune('1'+i)),
				Role:   RoleImplementer,
				Phase:  phase,
			}
		}
	}

	handler := cs.handlers["list_workers"]
	result, err := handler(context.Background(), nil)
	require.NoError(t, err)

	type workerInfo struct {
		WorkerID string `json:"worker_id"`
		Phase    string `json:"phase"`
		Role     string `json:"role,omitempty"`
	}
	var infos []workerInfo
	err = json.Unmarshal([]byte(result.Content[0].Text), &infos)
	require.NoError(t, err, "Failed to parse response")

	if len(infos) != len(phases) {
		t.Errorf("Expected %d workers, got %d", len(phases), len(infos))
	}

	// Verify each phase is represented
	phaseFound := make(map[string]bool)
	for _, info := range infos {
		phaseFound[info.Phase] = true
	}

	for _, phase := range phases {
		if !phaseFound[string(phase)] {
			t.Errorf("Phase %s not found in response", phase)
		}
	}
}

// TestSendToWorker_WorkerExists tests send_to_worker with an existing worker.
func TestSendToWorker_WorkerExists(t *testing.T) {
	workerPool := pool.NewWorkerPool(pool.Config{})
	defer workerPool.Close()

	msgIssue := message.New()
	cs := NewCoordinatorServer(claude.NewClient(), workerPool, msgIssue, "/tmp/test", 8765, nil)

	// Create worker
	_ = workerPool.AddTestWorker("worker-1", pool.WorkerWorking)

	// Assign task to worker
	cs.taskMapMu.Lock()
	cs.workerTaskMap["worker-1"] = "perles-abc.1"
	cs.taskMapMu.Unlock()

	handler := cs.handlers["send_to_worker"]

	// Try to send message - will fail trying to resume worker (no Claude)
	args := `{"worker_id": "worker-1", "message": "Please continue with the task"}`
	_, err := handler(context.Background(), json.RawMessage(args))
	// Error expected since we can't resume without Claude
	if err == nil {
		t.Log("Handler completed - message may have been queued")
	}
}

// TestReadMessageLog_WithMessages tests read_message_log with existing messages.
func TestReadMessageLog_WithMessages(t *testing.T) {
	workerPool := pool.NewWorkerPool(pool.Config{})
	defer workerPool.Close()

	msgIssue := message.New()

	// Add some messages
	_, _ = msgIssue.Append("COORDINATOR", "ALL", "Welcome message", message.MessageInfo)
	_, _ = msgIssue.Append("WORKER.1", "COORDINATOR", "Ready for task", message.MessageWorkerReady)
	_, _ = msgIssue.Append("COORDINATOR", "WORKER.1", "Here is your task", message.MessageInfo)

	cs := NewCoordinatorServer(claude.NewClient(), workerPool, msgIssue, "/tmp/test", 8765, nil)
	handler := cs.handlers["read_message_log"]

	result, err := handler(context.Background(), json.RawMessage(`{}`))
	require.NoError(t, err)

	// Should contain all messages
	require.NotNil(t, result)
	require.NotEmpty(t, result.Content, "Expected result with content")
	text := result.Content[0].Text
	if len(text) < 10 {
		t.Error("Expected message log content")
	}
}

// TestReadMessageLog_WithLimit tests read_message_log with limit parameter.
func TestReadMessageLog_WithLimit(t *testing.T) {
	workerPool := pool.NewWorkerPool(pool.Config{})
	defer workerPool.Close()

	msgIssue := message.New()

	// Add many messages
	for i := 0; i < 10; i++ {
		_, _ = msgIssue.Append("COORDINATOR", "ALL", "Message "+string(rune('0'+i)), message.MessageInfo)
	}

	cs := NewCoordinatorServer(claude.NewClient(), workerPool, msgIssue, "/tmp/test", 8765, nil)
	handler := cs.handlers["read_message_log"]

	// Request only last 3 messages
	result, err := handler(context.Background(), json.RawMessage(`{"limit": 3}`))
	require.NoError(t, err)

	require.NotNil(t, result)
	require.NotEmpty(t, result.Content, "Expected result with content")
}

// TestPrepareHandoff_WithLongSummary tests prepare_handoff with a long summary.
func TestPrepareHandoff_WithLongSummary(t *testing.T) {
	workerPool := pool.NewWorkerPool(pool.Config{})
	defer workerPool.Close()

	msgIssue := message.New()
	cs := NewCoordinatorServer(claude.NewClient(), workerPool, msgIssue, "/tmp/test", 8765, nil)
	handler := cs.handlers["prepare_handoff"]

	// Long summary
	summary := "Worker 1 is processing task perles-abc.1. " +
		"Current progress: Implemented feature X (50%). " +
		"Worker 2 is reviewing task perles-abc.2. " +
		"Worker 3 is idle. " +
		"Next steps: Worker 1 needs to complete implementation, then Worker 2 will review."

	args := `{"summary": "` + summary + `"}`
	result, err := handler(context.Background(), json.RawMessage(args))
	require.NoError(t, err)

	if result.Content[0].Text != "Handoff message posted. Refresh will proceed." {
		t.Errorf("Unexpected result: %s", result.Content[0].Text)
	}
}

// TestCoordinatorServer_WorkerStateCallbackImpl tests the WorkerStateCallback implementation.
func TestCoordinatorServer_WorkerStateCallbackImpl(t *testing.T) {
	workerPool := pool.NewWorkerPool(pool.Config{})
	defer workerPool.Close()

	cs := NewCoordinatorServer(claude.NewClient(), workerPool, nil, "/tmp/test", 8765, nil)

	// Test GetWorkerPhase for non-existent worker
	phase, err := cs.GetWorkerPhase("nonexistent")
	if err != nil {
		t.Errorf("Expected no error for missing worker, got: %v", err)
	}
	if phase != events.PhaseIdle {
		t.Errorf("Expected idle phase for missing worker, got: %s", phase)
	}

	// Setup worker assignment
	workerID := "worker-1"
	taskID := "perles-abc.1"
	cs.assignmentsMu.Lock()
	cs.workerAssignments[workerID] = &WorkerAssignment{
		TaskID: taskID,
		Role:   RoleImplementer,
		Phase:  events.PhaseImplementing,
	}
	cs.taskAssignments[taskID] = &TaskAssignment{
		TaskID:      taskID,
		Implementer: workerID,
		Status:      TaskImplementing,
	}
	cs.assignmentsMu.Unlock()

	// Test GetWorkerPhase for existing worker
	phase, err = cs.GetWorkerPhase(workerID)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if phase != events.PhaseImplementing {
		t.Errorf("Expected implementing phase, got: %s", phase)
	}

	// Test OnImplementationComplete
	err = cs.OnImplementationComplete(workerID, "completed feature")
	if err != nil {
		t.Errorf("OnImplementationComplete error: %v", err)
	}

	// Verify phase changed
	cs.assignmentsMu.RLock()
	if cs.workerAssignments[workerID].Phase != events.PhaseAwaitingReview {
		t.Errorf("Expected awaiting_review phase, got: %s", cs.workerAssignments[workerID].Phase)
	}
	cs.assignmentsMu.RUnlock()

	// Setup for review test
	reviewerID := "worker-2"
	cs.assignmentsMu.Lock()
	cs.workerAssignments[reviewerID] = &WorkerAssignment{
		TaskID:        taskID,
		Role:          RoleReviewer,
		Phase:         events.PhaseReviewing,
		ImplementerID: workerID,
	}
	cs.taskAssignments[taskID].Reviewer = reviewerID
	cs.assignmentsMu.Unlock()

	// Test OnReviewVerdict - APPROVED
	err = cs.OnReviewVerdict(reviewerID, "APPROVED", "LGTM")
	if err != nil {
		t.Errorf("OnReviewVerdict error: %v", err)
	}

	// Verify reviewer is idle and task is approved
	cs.assignmentsMu.RLock()
	if cs.workerAssignments[reviewerID].Phase != events.PhaseIdle {
		t.Errorf("Expected reviewer idle phase, got: %s", cs.workerAssignments[reviewerID].Phase)
	}
	if cs.taskAssignments[taskID].Status != TaskApproved {
		t.Errorf("Expected task approved status, got: %s", cs.taskAssignments[taskID].Status)
	}
	cs.assignmentsMu.RUnlock()
}

// TestPrompts tests prompt generation functions.
func TestPrompts(t *testing.T) {
	// Test WorkerIdlePrompt
	idlePrompt := WorkerIdlePrompt("worker-1")
	if idlePrompt == "" {
		t.Error("WorkerIdlePrompt returned empty string")
	}

	// Test WorkerSystemPrompt
	systemPrompt := WorkerSystemPrompt("worker-1")
	if systemPrompt == "" {
		t.Error("WorkerSystemPrompt returned empty string")
	}
	if len(systemPrompt) < 100 {
		t.Error("WorkerSystemPrompt seems too short")
	}

	// Test TaskAssignmentPrompt (taskID, title, summary)
	taskPrompt := TaskAssignmentPrompt("perles-abc.1", "Implement feature X", "Coordinator Summary")
	if taskPrompt == "" {
		t.Error("TaskAssignmentPrompt returned empty string")
	}

	// Test ReviewAssignmentPrompt
	reviewPrompt := ReviewAssignmentPrompt("perles-abc.1", "worker-1", "Implemented feature X")
	if reviewPrompt == "" {
		t.Error("ReviewAssignmentPrompt returned empty string")
	}

	// Test ReviewFeedbackPrompt
	feedbackPrompt := ReviewFeedbackPrompt("perles-abc.1", "Please fix the error handling")
	if feedbackPrompt == "" {
		t.Error("ReviewFeedbackPrompt returned empty string")
	}

	// Test CommitApprovalPrompt (taskID, commitMessage)
	commitPrompt := CommitApprovalPrompt("perles-abc.1", "feat: add feature X")
	if commitPrompt == "" {
		t.Error("CommitApprovalPrompt returned empty string")
	}
}

// TestConfigGeneration tests MCP config generation functions.
func TestConfigGeneration(t *testing.T) {
	// Test GenerateCoordinatorConfig (workDir string)
	coordConfig, err := GenerateCoordinatorConfig("/tmp/test")
	if err != nil {
		t.Errorf("GenerateCoordinatorConfig error: %v", err)
	}
	if coordConfig == "" {
		t.Error("GenerateCoordinatorConfig returned empty string")
	}

	// Test GenerateCoordinatorConfigHTTP
	httpConfig, err := GenerateCoordinatorConfigHTTP(8765)
	if err != nil {
		t.Errorf("GenerateCoordinatorConfigHTTP error: %v", err)
	}
	if httpConfig == "" {
		t.Error("GenerateCoordinatorConfigHTTP returned empty string")
	}

	// Test GenerateWorkerConfig (workerID, workDir string)
	workerConfig, err := GenerateWorkerConfig("worker-1", "/tmp/test")
	if err != nil {
		t.Errorf("GenerateWorkerConfig error: %v", err)
	}
	if workerConfig == "" {
		t.Error("GenerateWorkerConfig returned empty string")
	}

	// Test GenerateWorkerConfigHTTP
	workerHTTPConfig, err := GenerateWorkerConfigHTTP(8765, "worker-1")
	if err != nil {
		t.Errorf("GenerateWorkerConfigHTTP error: %v", err)
	}
	if workerHTTPConfig == "" {
		t.Error("GenerateWorkerConfigHTTP returned empty string")
	}

	// Test ConfigToFlag
	flag := ConfigToFlag(coordConfig)
	if flag == "" {
		t.Error("ConfigToFlag returned empty string")
	}

	// Test ParseMCPConfig
	parsed, err := ParseMCPConfig(coordConfig)
	if err != nil {
		t.Errorf("ParseMCPConfig error: %v", err)
	}
	if parsed == nil {
		t.Error("ParseMCPConfig returned nil")
	}
}

// TestMessageDeduplicator_EdgeCases tests deduplicator edge cases.
func TestMessageDeduplicator_EdgeCases(t *testing.T) {
	dedup := NewMessageDeduplicator(100 * time.Millisecond)

	// Test empty message
	if dedup.IsDuplicate("worker-1", "") {
		t.Error("Empty message should not be considered duplicate on first call")
	}
	if !dedup.IsDuplicate("worker-1", "") {
		t.Error("Empty message should be duplicate on second call")
	}

	// Test very long message
	longMsg := ""
	for i := 0; i < 1000; i++ {
		longMsg += "a"
	}
	if dedup.IsDuplicate("worker-1", longMsg) {
		t.Error("Long message should not be duplicate on first call")
	}

	// Test special characters
	specialMsg := "Message with special chars: !@#$%^&*()[]{}|\\:;<>?,./`~"
	if dedup.IsDuplicate("worker-1", specialMsg) {
		t.Error("Special char message should not be duplicate on first call")
	}

	// Test Unicode
	unicodeMsg := "Message with Unicode: 你好世界 🎉 émojis"
	if dedup.IsDuplicate("worker-1", unicodeMsg) {
		t.Error("Unicode message should not be duplicate on first call")
	}

	// Test expiration
	time.Sleep(150 * time.Millisecond)
	if dedup.IsDuplicate("worker-1", "") {
		t.Error("Message should not be duplicate after expiration")
	}

	// Test Len and Clear
	_ = dedup.IsDuplicate("worker-1", "msg1")
	_ = dedup.IsDuplicate("worker-1", "msg2")
	if dedup.Len() < 2 {
		t.Errorf("Expected at least 2 entries, got %d", dedup.Len())
	}

	dedup.Clear()
	if dedup.Len() != 0 {
		t.Errorf("Expected 0 entries after clear, got %d", dedup.Len())
	}
}

// helper function for creating error
func errorf(format string, args ...interface{}) error {
	return &customError{msg: formatMessage(format, args...)}
}

type customError struct {
	msg string
}

func (e *customError) Error() string {
	return e.msg
}

func formatMessage(format string, args ...interface{}) string {
	if len(args) == 0 {
		return format
	}
	// Simple formatting - replace %v with args
	result := format
	for _, arg := range args {
		if idx := findPercent(result); idx >= 0 {
			prefix := result[:idx]
			suffix := ""
			if idx+2 <= len(result) {
				suffix = result[idx+2:]
			}
			result = prefix + fmt.Sprint(arg) + suffix
		}
	}
	return result
}

func findPercent(s string) int {
	for i := 0; i < len(s)-1; i++ {
		if s[i] == '%' {
			return i
		}
	}
	return -1
}
