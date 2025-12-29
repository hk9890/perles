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

// ============================================================================
// Integration Tests
//
// These tests verify the complete integration of state tracking, messaging,
// and worker pool management without external dependencies (BD, Claude).
// ============================================================================

// TestStateMachine_CompleteTaskWorkflow tests the entire task lifecycle using state tracking.
func TestStateMachine_CompleteTaskWorkflow(t *testing.T) {
	workerPool := pool.NewWorkerPool(pool.Config{})
	defer workerPool.Close()

	msgIssue := message.New()
	cs := NewCoordinatorServer(claude.NewClient(), workerPool, msgIssue, "/tmp/test", 8765, nil, mocks.NewMockBeadsExecutor(t))

	// Create workers
	implementer := workerPool.AddTestWorker("implementer", pool.WorkerReady)
	reviewer := workerPool.AddTestWorker("reviewer", pool.WorkerReady)

	ctx := context.Background()

	// Step 1: Validate task assignment
	t.Run("step1_validate_assignment", func(t *testing.T) {
		err := cs.validateTaskAssignment("implementer", "perles-abc.1")
		require.NoError(t, err, "Failed to validate task assignment")
	})

	// Step 2: Simulate assign_task (without actual bd call)
	t.Run("step2_assign_task", func(t *testing.T) {
		now := time.Now()

		// Update state as assign_task would
		cs.SetWorkerAssignment("implementer", &WorkerAssignment{
			TaskID:     "perles-abc.1",
			Role:       RoleImplementer,
			Phase:      events.PhaseImplementing,
			AssignedAt: now,
		})
		cs.SetTaskAssignment("perles-abc.1", &TaskAssignment{
			TaskID:      "perles-abc.1",
			Implementer: "implementer",
			Status:      TaskImplementing,
			StartedAt:   now,
		})

		// Update pool worker state
		err := implementer.AssignTask("perles-abc.1")
		require.NoError(t, err, "Failed to assign task to worker")

		// Verify via query_worker_state
		handler := cs.handlers["query_worker_state"]
		result, err := handler(ctx, json.RawMessage(`{}`))
		require.NoError(t, err, "query_worker_state failed")

		var response workerStateResponse
		err = json.Unmarshal([]byte(result.Content[0].Text), &response)
		require.NoError(t, err, "Failed to parse response")

		// Check implementer is in implementing phase
		var foundImplementer bool
		for _, w := range response.Workers {
			if w.WorkerID == "implementer" {
				foundImplementer = true
				if w.Phase != "implementing" {
					t.Errorf("Implementer phase = %q, want %q", w.Phase, "implementing")
				}
				if w.Role != "implementer" {
					t.Errorf("Implementer role = %q, want %q", w.Role, "implementer")
				}
			}
		}
		if !foundImplementer {
			t.Error("Implementer not found in workers list")
		}

		// Check task assignment
		ta, ok := response.TaskAssignments["perles-abc.1"]
		if !ok {
			t.Error("Task assignment not found")
		} else if ta.Implementer != "implementer" {
			t.Errorf("Task implementer = %q, want %q", ta.Implementer, "implementer")
		}
	})

	// Step 3: Implementer reports implementation complete
	t.Run("step3_implementation_complete", func(t *testing.T) {
		cs.assignmentsMu.Lock()
		cs.workerAssignments["implementer"].Phase = events.PhaseAwaitingReview
		cs.assignmentsMu.Unlock()

		// Verify phase change
		handler := cs.handlers["query_worker_state"]
		result, _ := handler(ctx, json.RawMessage(`{"worker_id": "implementer"}`))

		var response workerStateResponse
		_ = json.Unmarshal([]byte(result.Content[0].Text), &response)

		if len(response.Workers) > 0 && response.Workers[0].Phase != "awaiting_review" {
			t.Errorf("Phase = %q, want %q", response.Workers[0].Phase, "awaiting_review")
		}
	})

	// Step 4: Validate and assign reviewer
	t.Run("step4_assign_reviewer", func(t *testing.T) {
		err := cs.validateReviewAssignment("reviewer", "perles-abc.1", "implementer")
		require.NoError(t, err, "Failed to validate review assignment")

		// Update state
		cs.assignmentsMu.Lock()
		cs.workerAssignments["reviewer"] = &WorkerAssignment{
			TaskID:        "perles-abc.1",
			Role:          RoleReviewer,
			Phase:         events.PhaseReviewing,
			ImplementerID: "implementer",
			AssignedAt:    time.Now(),
		}
		cs.workerAssignments["implementer"].ReviewerID = "reviewer"
		cs.taskAssignments["perles-abc.1"].Reviewer = "reviewer"
		cs.taskAssignments["perles-abc.1"].Status = TaskInReview
		cs.taskAssignments["perles-abc.1"].ReviewStartedAt = time.Now()
		cs.assignmentsMu.Unlock()

		// Verify
		handler := cs.handlers["query_worker_state"]
		result, _ := handler(ctx, json.RawMessage(`{}`))

		var response workerStateResponse
		_ = json.Unmarshal([]byte(result.Content[0].Text), &response)

		// Check reviewer assignment
		var foundReviewer bool
		for _, w := range response.Workers {
			if w.WorkerID == "reviewer" {
				foundReviewer = true
				if w.Phase != "reviewing" {
					t.Errorf("Reviewer phase = %q, want %q", w.Phase, "reviewing")
				}
				if w.Role != "reviewer" {
					t.Errorf("Reviewer role = %q, want %q", w.Role, "reviewer")
				}
			}
		}
		if !foundReviewer {
			t.Error("Reviewer not found")
		}

		// Check task has reviewer
		ta := response.TaskAssignments["perles-abc.1"]
		if ta.Reviewer != "reviewer" {
			t.Errorf("Task reviewer = %q, want %q", ta.Reviewer, "reviewer")
		}
		if ta.Status != "in_review" {
			t.Errorf("Task status = %q, want %q", ta.Status, "in_review")
		}
	})

	// Step 5: Reviewer approves
	t.Run("step5_review_approved", func(t *testing.T) {
		cs.assignmentsMu.Lock()
		cs.taskAssignments["perles-abc.1"].Status = TaskApproved
		cs.workerAssignments["reviewer"].Phase = events.PhaseIdle
		cs.workerAssignments["reviewer"].TaskID = ""
		cs.assignmentsMu.Unlock()

		// Verify reviewer returns to idle
		handler := cs.handlers["query_worker_state"]
		result, _ := handler(ctx, json.RawMessage(`{"worker_id": "reviewer"}`))

		var response workerStateResponse
		_ = json.Unmarshal([]byte(result.Content[0].Text), &response)

		if len(response.Workers) > 0 && response.Workers[0].Phase != "idle" {
			t.Errorf("Reviewer phase = %q, want %q", response.Workers[0].Phase, "idle")
		}

		// Verify task status
		result, _ = handler(ctx, json.RawMessage(`{}`))
		_ = json.Unmarshal([]byte(result.Content[0].Text), &response)
		if response.TaskAssignments["perles-abc.1"].Status != "approved" {
			t.Errorf("Task status = %q, want %q", response.TaskAssignments["perles-abc.1"].Status, "approved")
		}
	})

	// Step 6: Approve commit
	t.Run("step6_approve_commit", func(t *testing.T) {
		cs.assignmentsMu.Lock()
		cs.taskAssignments["perles-abc.1"].Status = TaskCommitting
		cs.workerAssignments["implementer"].Phase = events.PhaseCommitting
		cs.assignmentsMu.Unlock()

		handler := cs.handlers["query_worker_state"]
		result, _ := handler(ctx, json.RawMessage(`{"worker_id": "implementer"}`))

		var response workerStateResponse
		_ = json.Unmarshal([]byte(result.Content[0].Text), &response)

		if len(response.Workers) > 0 && response.Workers[0].Phase != "committing" {
			t.Errorf("Implementer phase = %q, want %q", response.Workers[0].Phase, "committing")
		}
	})

	// Step 7: Task complete
	t.Run("step7_task_complete", func(t *testing.T) {
		cs.assignmentsMu.Lock()
		cs.taskAssignments["perles-abc.1"].Status = TaskCompleted
		cs.workerAssignments["implementer"].Phase = events.PhaseIdle
		cs.workerAssignments["implementer"].TaskID = ""
		cs.assignmentsMu.Unlock()

		implementer.CompleteTask()
		_ = reviewer // Already idle

		handler := cs.handlers["query_worker_state"]
		result, _ := handler(ctx, json.RawMessage(`{}`))

		var response workerStateResponse
		_ = json.Unmarshal([]byte(result.Content[0].Text), &response)

		// Both workers should be idle and available
		if len(response.ReadyWorkers) < 2 {
			t.Errorf("Expected 2 ready workers, got %d", len(response.ReadyWorkers))
		}

		// Task should be completed
		if response.TaskAssignments["perles-abc.1"].Status != "completed" {
			t.Errorf("Task status = %q, want %q", response.TaskAssignments["perles-abc.1"].Status, "completed")
		}
	})
}

// TestStateMachine_ReviewDenialAndRework tests the denial → feedback → rework flow.
func TestStateMachine_ReviewDenialAndRework(t *testing.T) {
	workerPool := pool.NewWorkerPool(pool.Config{})
	defer workerPool.Close()

	msgIssue := message.New()
	cs := NewCoordinatorServer(claude.NewClient(), workerPool, msgIssue, "/tmp/test", 8765, nil, mocks.NewMockBeadsExecutor(t))

	// Create workers
	_ = workerPool.AddTestWorker("implementer", pool.WorkerReady)
	_ = workerPool.AddTestWorker("reviewer", pool.WorkerReady)

	ctx := context.Background()

	// Setup: task in review
	now := time.Now()
	cs.SetTaskAssignment("perles-abc.1", &TaskAssignment{
		TaskID:          "perles-abc.1",
		Implementer:     "implementer",
		Reviewer:        "reviewer",
		Status:          TaskInReview,
		StartedAt:       now,
		ReviewStartedAt: now,
	})
	cs.SetWorkerAssignment("implementer", &WorkerAssignment{
		TaskID:     "perles-abc.1",
		Role:       RoleImplementer,
		Phase:      events.PhaseAwaitingReview,
		ReviewerID: "reviewer",
		AssignedAt: now,
	})
	cs.SetWorkerAssignment("reviewer", &WorkerAssignment{
		TaskID:        "perles-abc.1",
		Role:          RoleReviewer,
		Phase:         events.PhaseReviewing,
		ImplementerID: "implementer",
		AssignedAt:    now,
	})

	// Step 1: Reviewer denies
	t.Run("step1_review_denied", func(t *testing.T) {
		cs.assignmentsMu.Lock()
		cs.taskAssignments["perles-abc.1"].Status = TaskDenied
		cs.workerAssignments["reviewer"].Phase = events.PhaseIdle
		cs.workerAssignments["reviewer"].TaskID = ""
		cs.taskAssignments["perles-abc.1"].Reviewer = ""
		cs.assignmentsMu.Unlock()

		handler := cs.handlers["query_worker_state"]
		result, _ := handler(ctx, json.RawMessage(`{}`))

		var response workerStateResponse
		_ = json.Unmarshal([]byte(result.Content[0].Text), &response)

		if response.TaskAssignments["perles-abc.1"].Status != "denied" {
			t.Errorf("Task status = %q, want %q", response.TaskAssignments["perles-abc.1"].Status, "denied")
		}

		// Reviewer should be back to ready
		foundReviewerReady := false
		for _, wID := range response.ReadyWorkers {
			if wID == "reviewer" {
				foundReviewerReady = true
				break
			}
		}
		if !foundReviewerReady {
			t.Error("Reviewer should be in ready_workers list")
		}
	})

	// Step 2: Send feedback to implementer
	t.Run("step2_send_feedback", func(t *testing.T) {
		cs.assignmentsMu.Lock()
		cs.taskAssignments["perles-abc.1"].Status = TaskImplementing
		cs.workerAssignments["implementer"].Phase = events.PhaseAddressingFeedback
		cs.workerAssignments["implementer"].ReviewerID = ""
		cs.assignmentsMu.Unlock()

		handler := cs.handlers["query_worker_state"]
		result, _ := handler(ctx, json.RawMessage(`{"worker_id": "implementer"}`))

		var response workerStateResponse
		_ = json.Unmarshal([]byte(result.Content[0].Text), &response)

		if len(response.Workers) > 0 && response.Workers[0].Phase != "addressing_feedback" {
			t.Errorf("Phase = %q, want %q", response.Workers[0].Phase, "addressing_feedback")
		}
	})

	// Step 3: Implementer completes rework
	t.Run("step3_rework_complete", func(t *testing.T) {
		cs.assignmentsMu.Lock()
		cs.workerAssignments["implementer"].Phase = events.PhaseAwaitingReview
		cs.assignmentsMu.Unlock()

		// Can now assign a new reviewer
		err := cs.validateReviewAssignment("reviewer", "perles-abc.1", "implementer")
		if err != nil {
			t.Errorf("Should be able to assign reviewer again: %v", err)
		}
	})
}

// TestStateMachine_MessagingDuringWorkflow tests message posting during task execution.
func TestStateMachine_MessagingDuringWorkflow(t *testing.T) {
	workerPool := pool.NewWorkerPool(pool.Config{})
	defer workerPool.Close()

	msgIssue := message.New()
	cs := NewCoordinatorServer(claude.NewClient(), workerPool, msgIssue, "/tmp/test", 8765, nil, mocks.NewMockBeadsExecutor(t))

	ctx := context.Background()

	// Post various messages
	postHandler := cs.handlers["post_message"]

	messages := []struct {
		to      string
		content string
	}{
		{"ALL", "Coordinator starting orchestration"},
		{"worker-1", "Direct message to worker-1"},
		{"ALL", "Task perles-abc.1 assigned to worker-1"},
		{"worker-2", "Please review when ready"},
	}

	for _, msg := range messages {
		args, _ := json.Marshal(map[string]string{
			"to":      msg.to,
			"content": msg.content,
		})
		_, err := postHandler(ctx, json.RawMessage(args))
		if err != nil {
			t.Errorf("Failed to post message to %q: %v", msg.to, err)
		}
	}

	// Read messages back
	readHandler := cs.handlers["read_message_log"]
	result, err := readHandler(ctx, json.RawMessage(`{"limit": 10}`))
	require.NoError(t, err, "Failed to read message log")

	var response messageLogResponse
	err = json.Unmarshal([]byte(result.Content[0].Text), &response)
	require.NoError(t, err, "Failed to parse response")

	if response.TotalCount != 4 {
		t.Errorf("Expected 4 messages, got %d", response.TotalCount)
	}
}

// TestStateMachine_OrphanDetection tests detection of orphaned tasks.
func TestStateMachine_OrphanDetection(t *testing.T) {
	workerPool := pool.NewWorkerPool(pool.Config{})
	defer workerPool.Close()

	cs := NewCoordinatorServer(claude.NewClient(), workerPool, nil, "/tmp/test", 8765, nil, mocks.NewMockBeadsExecutor(t))

	// Create a worker and assign a task
	worker := workerPool.AddTestWorker("worker-1", pool.WorkerWorking)
	cs.SetTaskAssignment("perles-abc.1", &TaskAssignment{
		TaskID:      "perles-abc.1",
		Implementer: "worker-1",
		Status:      TaskImplementing,
	})
	cs.SetWorkerAssignment("worker-1", &WorkerAssignment{
		TaskID:     "perles-abc.1",
		Role:       RoleImplementer,
		Phase:      events.PhaseImplementing,
		AssignedAt: time.Now(),
	})

	// No orphans initially
	t.Run("no_orphans_initially", func(t *testing.T) {
		orphans := cs.detectOrphanedTasks()
		if len(orphans) != 0 {
			t.Errorf("Expected 0 orphans, got %d: %v", len(orphans), orphans)
		}
	})

	// Retire the worker - task becomes orphaned
	t.Run("worker_retired_creates_orphan", func(t *testing.T) {
		worker.Retire()

		orphans := cs.detectOrphanedTasks()
		if len(orphans) != 1 {
			t.Errorf("Expected 1 orphan, got %d: %v", len(orphans), orphans)
		}
		if len(orphans) > 0 && orphans[0] != "perles-abc.1" {
			t.Errorf("Expected orphan perles-abc.1, got %s", orphans[0])
		}
	})

	// Add task with nonexistent implementer
	t.Run("nonexistent_implementer_is_orphan", func(t *testing.T) {
		cs.SetTaskAssignment("perles-xyz.1", &TaskAssignment{
			TaskID:      "perles-xyz.1",
			Implementer: "worker-nonexistent",
			Status:      TaskImplementing,
		})

		orphans := cs.detectOrphanedTasks()
		if len(orphans) != 2 {
			t.Errorf("Expected 2 orphans, got %d: %v", len(orphans), orphans)
		}
	})
}

// TestStateMachine_StuckWorkerDetection tests detection of stuck workers.
func TestStateMachine_StuckWorkerDetection(t *testing.T) {
	workerPool := pool.NewWorkerPool(pool.Config{})
	defer workerPool.Close()

	cs := NewCoordinatorServer(claude.NewClient(), workerPool, nil, "/tmp/test", 8765, nil, mocks.NewMockBeadsExecutor(t))

	// Create workers with different assignment times
	_ = workerPool.AddTestWorker("worker-1", pool.WorkerWorking)
	_ = workerPool.AddTestWorker("worker-2", pool.WorkerWorking)

	// Worker 1: recent assignment (not stuck)
	cs.SetWorkerAssignment("worker-1", &WorkerAssignment{
		TaskID:     "perles-abc.1",
		AssignedAt: time.Now().Add(-5 * time.Minute),
	})

	// Worker 2: old assignment (stuck)
	cs.SetWorkerAssignment("worker-2", &WorkerAssignment{
		TaskID:     "perles-xyz.1",
		AssignedAt: time.Now().Add(-MaxTaskDuration - time.Minute),
	})

	stuck := cs.checkStuckWorkers()

	if len(stuck) != 1 {
		t.Errorf("Expected 1 stuck worker, got %d: %v", len(stuck), stuck)
	}
	if len(stuck) > 0 && stuck[0] != "worker-2" {
		t.Errorf("Expected worker-2 to be stuck, got %s", stuck[0])
	}
}

// TestStateMachine_PrepareHandoff tests handoff message posting.
func TestStateMachine_PrepareHandoff(t *testing.T) {
	workerPool := pool.NewWorkerPool(pool.Config{})
	defer workerPool.Close()

	msgIssue := message.New()
	cs := NewCoordinatorServer(claude.NewClient(), workerPool, msgIssue, "/tmp/test", 8765, nil, mocks.NewMockBeadsExecutor(t))

	// Setup some state
	_ = workerPool.AddTestWorker("worker-1", pool.WorkerWorking)
	cs.SetWorkerAssignment("worker-1", &WorkerAssignment{
		TaskID: "perles-abc.1",
		Role:   RoleImplementer,
		Phase:  events.PhaseImplementing,
	})
	cs.SetTaskAssignment("perles-abc.1", &TaskAssignment{
		TaskID:      "perles-abc.1",
		Implementer: "worker-1",
		Status:      TaskImplementing,
	})

	// Call prepare_handoff
	handler := cs.handlers["prepare_handoff"]
	summary := "Worker-1 is implementing perles-abc.1. Progress: 50%. Current focus: adding tests."
	args := `{"summary": "` + summary + `"}`

	result, err := handler(context.Background(), json.RawMessage(args))
	require.NoError(t, err, "prepare_handoff failed")

	if result.Content[0].Text != "Handoff message posted. Refresh will proceed." {
		t.Errorf("Unexpected result: %q", result.Content[0].Text)
	}

	// Verify message was posted
	entries := msgIssue.Entries()
	require.Len(t, entries, 1)

	entry := entries[0]
	if entry.Type != message.MessageHandoff {
		t.Errorf("Message type = %q, want %q", entry.Type, message.MessageHandoff)
	}
	if entry.From != message.ActorCoordinator {
		t.Errorf("From = %q, want %q", entry.From, message.ActorCoordinator)
	}
}

// TestStateMachine_MultipleTasksMultipleWorkers tests managing multiple concurrent tasks.
func TestStateMachine_MultipleTasksMultipleWorkers(t *testing.T) {
	workerPool := pool.NewWorkerPool(pool.Config{})
	defer workerPool.Close()

	cs := NewCoordinatorServer(claude.NewClient(), workerPool, nil, "/tmp/test", 8765, nil, mocks.NewMockBeadsExecutor(t))

	// Create 5 workers
	for i := 1; i <= 5; i++ {
		workerID := "worker-" + string(rune('0'+i))
		_ = workerPool.AddTestWorker(workerID, pool.WorkerReady)
	}

	// Assign 3 tasks to 3 different workers
	tasks := []struct {
		taskID   string
		workerID string
	}{
		{"perles-abc.1", "worker-1"},
		{"perles-abc.2", "worker-2"},
		{"perles-abc.3", "worker-3"},
	}

	for _, task := range tasks {
		err := cs.validateTaskAssignment(task.workerID, task.taskID)
		require.NoError(t, err, "Failed to validate assignment for %s", task.taskID)

		cs.SetWorkerAssignment(task.workerID, &WorkerAssignment{
			TaskID:     task.taskID,
			Role:       RoleImplementer,
			Phase:      events.PhaseImplementing,
			AssignedAt: time.Now(),
		})
		cs.SetTaskAssignment(task.taskID, &TaskAssignment{
			TaskID:      task.taskID,
			Implementer: task.workerID,
			Status:      TaskImplementing,
			StartedAt:   time.Now(),
		})
	}

	// Verify state
	handler := cs.handlers["query_worker_state"]
	result, _ := handler(context.Background(), json.RawMessage(`{}`))

	var response workerStateResponse
	_ = json.Unmarshal([]byte(result.Content[0].Text), &response)

	// 3 workers working, 2 ready
	workingCount := 0
	for _, w := range response.Workers {
		if w.Phase == "implementing" {
			workingCount++
		}
	}
	if workingCount != 3 {
		t.Errorf("Expected 3 working workers, got %d", workingCount)
	}

	if len(response.ReadyWorkers) != 2 {
		t.Errorf("Expected 2 ready workers, got %d", len(response.ReadyWorkers))
	}

	if len(response.TaskAssignments) != 3 {
		t.Errorf("Expected 3 task assignments, got %d", len(response.TaskAssignments))
	}
}

// TestStateMachine_WorkerReplacementFlow tests the worker replacement workflow.
func TestStateMachine_WorkerReplacementFlow(t *testing.T) {
	workerPool := pool.NewWorkerPool(pool.Config{})
	defer workerPool.Close()

	cs := NewCoordinatorServer(claude.NewClient(), workerPool, nil, "/tmp/test", 8765, nil, mocks.NewMockBeadsExecutor(t))

	// Setup: worker with task
	worker := workerPool.AddTestWorker("worker-1", pool.WorkerWorking)
	cs.SetWorkerAssignment("worker-1", &WorkerAssignment{
		TaskID:     "perles-abc.1",
		Role:       RoleImplementer,
		Phase:      events.PhaseImplementing,
		AssignedAt: time.Now(),
	})
	cs.SetTaskAssignment("perles-abc.1", &TaskAssignment{
		TaskID:      "perles-abc.1",
		Implementer: "worker-1",
		Status:      TaskImplementing,
	})

	// Simulate worker retirement (as replace_worker would do before spawning)
	worker.Retire()

	// Clean up assignments (as replace_worker does)
	cs.assignmentsMu.Lock()
	delete(cs.workerAssignments, "worker-1")
	cs.assignmentsMu.Unlock()

	// Task should now be orphaned
	orphans := cs.detectOrphanedTasks()
	if len(orphans) != 1 {
		t.Errorf("Expected 1 orphan after worker replacement started, got %d", len(orphans))
	}

	// Create a new worker to take over
	newWorker := workerPool.AddTestWorker("worker-2", pool.WorkerReady)
	_ = newWorker

	// Re-assign the task
	err := cs.validateTaskAssignment("worker-2", "perles-abc.1")
	if err == nil {
		// Update task assignment to point to new worker
		cs.assignmentsMu.Lock()
		cs.taskAssignments["perles-abc.1"].Implementer = "worker-2"
		cs.assignmentsMu.Unlock()

		cs.SetWorkerAssignment("worker-2", &WorkerAssignment{
			TaskID:     "perles-abc.1",
			Role:       RoleImplementer,
			Phase:      events.PhaseImplementing,
			AssignedAt: time.Now(),
		})

		// No longer orphaned
		orphans = cs.detectOrphanedTasks()
		if len(orphans) != 0 {
			t.Errorf("Expected 0 orphans after reassignment, got %d: %v", len(orphans), orphans)
		}
	}
}
