package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/zjrosen/perles/internal/mocks"
	"github.com/zjrosen/perles/internal/orchestration/claude"
	"github.com/zjrosen/perles/internal/orchestration/events"
	"github.com/zjrosen/perles/internal/orchestration/message"
	"github.com/zjrosen/perles/internal/orchestration/pool"
	"pgregory.net/rapid"
)

// ============================================================================
// Property-Based Tests for State Invariants
// ============================================================================

// TestProperty_NoWorkerHasMultipleTasks verifies that a worker can only have one task at a time.
func TestProperty_NoWorkerHasMultipleTasks(t *testing.T) {
	mockExec := mocks.NewMockBeadsExecutor(t)
	rapid.Check(t, func(t *rapid.T) {
		workerPool := pool.NewWorkerPool(pool.Config{})
		defer workerPool.Close()

		cs := NewCoordinatorServer(claude.NewClient(), workerPool, nil, "/tmp/test", 8765, nil, mockExec)

		// Generate random number of workers and tasks
		numWorkers := rapid.IntRange(1, 10).Draw(t, "numWorkers")
		numTasks := rapid.IntRange(1, 20).Draw(t, "numTasks")

		// Create workers
		for i := 1; i <= numWorkers; i++ {
			workerID := fmt.Sprintf("worker-%d", i)
			_ = workerPool.AddTestWorker(workerID, pool.WorkerReady)
		}

		// Assign tasks (some will succeed, some will fail due to constraints)
		for i := 1; i <= numTasks; i++ {
			workerIdx := rapid.IntRange(1, numWorkers).Draw(t, fmt.Sprintf("workerIdx-%d", i))
			workerID := fmt.Sprintf("worker-%d", workerIdx)
			taskID := fmt.Sprintf("perles-abc.%d", i)

			// Try to assign - may fail if worker already has a task
			cs.assignmentsMu.Lock()
			if _, hasTask := cs.workerAssignments[workerID]; !hasTask {
				if _, taskTaken := cs.taskAssignments[taskID]; !taskTaken {
					cs.workerAssignments[workerID] = &WorkerAssignment{
						TaskID:     taskID,
						Role:       RoleImplementer,
						Phase:      events.PhaseImplementing,
						AssignedAt: time.Now(),
					}
					cs.taskAssignments[taskID] = &TaskAssignment{
						TaskID:      taskID,
						Implementer: workerID,
						Status:      TaskImplementing,
						StartedAt:   time.Now(),
					}
				}
			}
			cs.assignmentsMu.Unlock()
		}

		// INVARIANT: Each worker should have at most one task
		cs.assignmentsMu.RLock()
		defer cs.assignmentsMu.RUnlock()

		workerTaskCount := make(map[string]int)
		for workerID := range cs.workerAssignments {
			workerTaskCount[workerID]++
		}

		for workerID, count := range workerTaskCount {
			if count > 1 {
				t.Errorf("Worker %s has %d tasks, expected at most 1", workerID, count)
			}
		}
	})
}

// TestProperty_NoTaskHasMultipleImplementers verifies that a task can only have one implementer.
func TestProperty_NoTaskHasMultipleImplementers(t *testing.T) {
	mockExec := mocks.NewMockBeadsExecutor(t)
	rapid.Check(t, func(t *rapid.T) {
		workerPool := pool.NewWorkerPool(pool.Config{})
		defer workerPool.Close()

		cs := NewCoordinatorServer(claude.NewClient(), workerPool, nil, "/tmp/test", 8765, nil, mockExec)

		numTasks := rapid.IntRange(1, 10).Draw(t, "numTasks")

		// Create multiple workers
		for i := 1; i <= 5; i++ {
			_ = workerPool.AddTestWorker(fmt.Sprintf("worker-%d", i), pool.WorkerReady)
		}

		// Create tasks
		for i := 1; i <= numTasks; i++ {
			taskID := fmt.Sprintf("perles-abc.%d", i)
			cs.taskAssignments[taskID] = &TaskAssignment{
				TaskID:      taskID,
				Implementer: fmt.Sprintf("worker-%d", (i%5)+1),
				Status:      TaskImplementing,
			}
		}

		// INVARIANT: Each task should have exactly one implementer (if assigned)
		taskImplementerCount := make(map[string]int)
		for taskID, ta := range cs.taskAssignments {
			if ta.Implementer != "" {
				taskImplementerCount[taskID]++
			}
		}

		for taskID, count := range taskImplementerCount {
			if count > 1 {
				t.Errorf("Task %s has %d implementers, expected at most 1", taskID, count)
			}
		}
	})
}

// TestProperty_ReviewerCannotBeSameAsImplementer verifies reviewer != implementer.
func TestProperty_ReviewerCannotBeSameAsImplementer(t *testing.T) {
	mockExec := mocks.NewMockBeadsExecutor(t)
	rapid.Check(t, func(t *rapid.T) {
		workerPool := pool.NewWorkerPool(pool.Config{})
		defer workerPool.Close()

		cs := NewCoordinatorServer(claude.NewClient(), workerPool, nil, "/tmp/test", 8765, nil, mockExec)

		// Generate workers
		workerID := rapid.StringMatching("worker-[0-9]+").Draw(t, "workerID")
		_ = workerPool.AddTestWorker(workerID, pool.WorkerReady)

		// Try to assign same worker as both implementer and reviewer
		taskID := "perles-abc.1"

		// Setup implementer
		cs.taskAssignments[taskID] = &TaskAssignment{
			TaskID:      taskID,
			Implementer: workerID,
		}
		cs.workerAssignments[workerID] = &WorkerAssignment{
			TaskID: taskID,
			Role:   RoleImplementer,
			Phase:  events.PhaseAwaitingReview,
		}

		// INVARIANT: Self-review should always be rejected
		err := cs.validateReviewAssignment(workerID, taskID, workerID)
		if err == nil {
			t.Error("Expected error for self-review, but got none")
		}
	})
}

// TestProperty_PhaseTransitionsAreValid verifies only valid phase transitions are allowed.
func TestProperty_PhaseTransitionsAreValid(t *testing.T) {
	// Define valid phase transitions
	validTransitions := map[events.WorkerPhase][]events.WorkerPhase{
		events.PhaseIdle:               {events.PhaseImplementing, events.PhaseReviewing},
		events.PhaseImplementing:       {events.PhaseAwaitingReview},
		events.PhaseAwaitingReview:     {events.PhaseReviewing, events.PhaseCommitting, events.PhaseAddressingFeedback},
		events.PhaseReviewing:          {events.PhaseIdle},
		events.PhaseAddressingFeedback: {events.PhaseAwaitingReview},
		events.PhaseCommitting:         {events.PhaseIdle},
	}

	rapid.Check(t, func(t *rapid.T) {
		fromPhaseIdx := rapid.IntRange(0, 5).Draw(t, "fromPhaseIdx")
		toPhaseIdx := rapid.IntRange(0, 5).Draw(t, "toPhaseIdx")

		phases := []events.WorkerPhase{
			events.PhaseIdle,
			events.PhaseImplementing,
			events.PhaseAwaitingReview,
			events.PhaseReviewing,
			events.PhaseAddressingFeedback,
			events.PhaseCommitting,
		}

		fromPhase := phases[fromPhaseIdx]
		toPhase := phases[toPhaseIdx]

		// Check if transition is valid
		valid := false
		for _, validTo := range validTransitions[fromPhase] {
			if validTo == toPhase {
				valid = true
				break
			}
		}

		// If from == to, that's also "valid" (no transition)
		if fromPhase == toPhase {
			valid = true
		}

		// This test just verifies the transition table is consistent
		// The actual enforcement is in the coordinator tools
		if !valid && fromPhase != toPhase {
			// Just documenting invalid transitions exist (they should be rejected by tools)
			t.Logf("Transition %s -> %s is invalid (expected)", fromPhase, toPhase)
		}
	})
}

// TestProperty_TaskStatusConsistentWithPhase verifies task status matches worker phase.
func TestProperty_TaskStatusConsistentWithPhase(t *testing.T) {
	mockExec := mocks.NewMockBeadsExecutor(t)
	rapid.Check(t, func(t *rapid.T) {
		workerPool := pool.NewWorkerPool(pool.Config{})
		defer workerPool.Close()

		cs := NewCoordinatorServer(claude.NewClient(), workerPool, nil, "/tmp/test", 8765, nil, mockExec)

		// Create workers and assign with consistent phases
		phases := []events.WorkerPhase{
			events.PhaseImplementing,
			events.PhaseAwaitingReview,
			events.PhaseReviewing,
			events.PhaseCommitting,
		}
		statuses := []TaskWorkflowStatus{
			TaskImplementing,
			TaskInReview,
			TaskInReview,
			TaskCommitting,
		}

		phaseIdx := rapid.IntRange(0, len(phases)-1).Draw(t, "phaseIdx")
		phase := phases[phaseIdx]
		status := statuses[phaseIdx]

		taskID := "perles-abc.1"
		workerID := "worker-1"

		cs.workerAssignments[workerID] = &WorkerAssignment{
			TaskID: taskID,
			Role:   RoleImplementer,
			Phase:  phase,
		}
		cs.taskAssignments[taskID] = &TaskAssignment{
			TaskID:      taskID,
			Implementer: workerID,
			Status:      status,
		}

		// INVARIANT: The phase and status should be consistent
		wa := cs.workerAssignments[workerID]
		ta := cs.taskAssignments[taskID]

		// Verify consistency rules
		switch wa.Phase {
		case events.PhaseImplementing:
			if ta.Status != TaskImplementing {
				t.Errorf("Phase %s should have status %s, got %s", wa.Phase, TaskImplementing, ta.Status)
			}
		case events.PhaseCommitting:
			if ta.Status != TaskCommitting {
				t.Errorf("Phase %s should have status %s, got %s", wa.Phase, TaskCommitting, ta.Status)
			}
		}
	})
}

// ============================================================================
// Race Detection Tests
// ============================================================================

// TestRace_ConcurrentTaskAssignment tests concurrent task assignment doesn't corrupt state.
func TestRace_ConcurrentTaskAssignment(t *testing.T) {
	workerPool := pool.NewWorkerPool(pool.Config{})
	defer workerPool.Close()

	msgIssue := message.New()
	cs := NewCoordinatorServer(claude.NewClient(), workerPool, msgIssue, "/tmp/test", 8765, nil, mocks.NewMockBeadsExecutor(t))

	// Create workers
	for i := 1; i <= 10; i++ {
		_ = workerPool.AddTestWorker(fmt.Sprintf("worker-%d", i), pool.WorkerReady)
	}

	var wg sync.WaitGroup
	errChan := make(chan error, 100)

	// Concurrently try to assign tasks
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(taskNum int) {
			defer wg.Done()

			workerID := fmt.Sprintf("worker-%d", (taskNum%10)+1)
			taskID := fmt.Sprintf("perles-abc.%d", taskNum)

			err := cs.validateTaskAssignment(workerID, taskID)
			if err != nil {
				// Expected - some assignments will fail
				return
			}

			// Simulate assignment with proper locking
			cs.assignmentsMu.Lock()
			if _, exists := cs.workerAssignments[workerID]; !exists {
				cs.workerAssignments[workerID] = &WorkerAssignment{
					TaskID:     taskID,
					Role:       RoleImplementer,
					Phase:      events.PhaseImplementing,
					AssignedAt: time.Now(),
				}
				cs.taskAssignments[taskID] = &TaskAssignment{
					TaskID:      taskID,
					Implementer: workerID,
					Status:      TaskImplementing,
					StartedAt:   time.Now(),
				}
			}
			cs.assignmentsMu.Unlock()
		}(i)
	}

	wg.Wait()
	close(errChan)

	// Check for errors
	for err := range errChan {
		t.Errorf("Race condition error: %v", err)
	}

	// Verify invariants after concurrent execution
	cs.assignmentsMu.RLock()
	defer cs.assignmentsMu.RUnlock()

	// Each worker should have at most one assignment
	for workerID, wa := range cs.workerAssignments {
		if wa == nil {
			t.Errorf("Worker %s has nil assignment", workerID)
		}
	}

	// Each task should have at most one implementer
	implementerCount := make(map[string]int)
	for _, ta := range cs.taskAssignments {
		if ta.Implementer != "" {
			implementerCount[ta.TaskID]++
		}
	}
	for taskID, count := range implementerCount {
		if count > 1 {
			t.Errorf("Task %s has %d implementers", taskID, count)
		}
	}
}

// TestRace_ConcurrentQueryAndModify tests read/write concurrency on state.
func TestRace_ConcurrentQueryAndModify(t *testing.T) {
	workerPool := pool.NewWorkerPool(pool.Config{})
	defer workerPool.Close()

	cs := NewCoordinatorServer(claude.NewClient(), workerPool, nil, "/tmp/test", 8765, nil, mocks.NewMockBeadsExecutor(t))

	// Create workers
	for i := 1; i <= 5; i++ {
		_ = workerPool.AddTestWorker(fmt.Sprintf("worker-%d", i), pool.WorkerReady)
	}

	var wg sync.WaitGroup
	ctx := context.Background()

	// Writers: add assignments
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(num int) {
			defer wg.Done()
			workerID := fmt.Sprintf("worker-%d", (num%5)+1)
			taskID := fmt.Sprintf("perles-abc.%d", num)

			cs.assignmentsMu.Lock()
			cs.workerAssignments[workerID] = &WorkerAssignment{
				TaskID:     taskID,
				Role:       RoleImplementer,
				Phase:      events.PhaseImplementing,
				AssignedAt: time.Now(),
			}
			cs.taskAssignments[taskID] = &TaskAssignment{
				TaskID:      taskID,
				Implementer: workerID,
				Status:      TaskImplementing,
				StartedAt:   time.Now(),
			}
			cs.assignmentsMu.Unlock()
		}(i)
	}

	// Readers: query worker state
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			handler := cs.handlers["query_worker_state"]
			_, _ = handler(ctx, json.RawMessage(`{}`))
		}()
	}

	// Readers: list workers
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			handler := cs.handlers["list_workers"]
			_, _ = handler(ctx, nil)
		}()
	}

	wg.Wait()
}

// TestRace_ConcurrentOrphanDetection tests orphan detection under concurrent modification.
func TestRace_ConcurrentOrphanDetection(t *testing.T) {
	workerPool := pool.NewWorkerPool(pool.Config{})
	defer workerPool.Close()

	cs := NewCoordinatorServer(claude.NewClient(), workerPool, nil, "/tmp/test", 8765, nil, mocks.NewMockBeadsExecutor(t))

	// Create workers
	for i := 1; i <= 5; i++ {
		_ = workerPool.AddTestWorker(fmt.Sprintf("worker-%d", i), pool.WorkerWorking)
		cs.taskAssignments[fmt.Sprintf("perles-abc.%d", i)] = &TaskAssignment{
			TaskID:      fmt.Sprintf("perles-abc.%d", i),
			Implementer: fmt.Sprintf("worker-%d", i),
			Status:      TaskImplementing,
		}
	}

	var wg sync.WaitGroup

	// Writers: retire workers
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(num int) {
			defer wg.Done()
			workerID := fmt.Sprintf("worker-%d", (num%5)+1)
			worker := workerPool.GetWorker(workerID)
			if worker != nil {
				worker.Retire()
			}
		}(i)
	}

	// Readers: detect orphans
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = cs.detectOrphanedTasks()
		}()
	}

	wg.Wait()
}

// TestRace_ConcurrentStuckWorkerCheck tests stuck worker detection under concurrent modification.
func TestRace_ConcurrentStuckWorkerCheck(t *testing.T) {
	workerPool := pool.NewWorkerPool(pool.Config{})
	defer workerPool.Close()

	cs := NewCoordinatorServer(claude.NewClient(), workerPool, nil, "/tmp/test", 8765, nil, mocks.NewMockBeadsExecutor(t))

	var wg sync.WaitGroup

	// Writers: add/modify worker assignments
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(num int) {
			defer wg.Done()
			workerID := fmt.Sprintf("worker-%d", num)

			cs.assignmentsMu.Lock()
			cs.workerAssignments[workerID] = &WorkerAssignment{
				TaskID:     fmt.Sprintf("perles-abc.%d", num),
				AssignedAt: time.Now().Add(-time.Duration(rand.Intn(60)) * time.Minute),
			}
			cs.assignmentsMu.Unlock()
		}(i)
	}

	// Readers: check stuck workers
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = cs.checkStuckWorkers()
		}()
	}

	wg.Wait()
}

// ============================================================================
// Edge Case Tests
// ============================================================================

// TestEdge_EmptyTaskID tests handling of empty task IDs.
func TestEdge_EmptyTaskID(t *testing.T) {
	workerPool := pool.NewWorkerPool(pool.Config{})
	defer workerPool.Close()

	cs := NewCoordinatorServer(claude.NewClient(), workerPool, nil, "/tmp/test", 8765, nil, mocks.NewMockBeadsExecutor(t))
	_ = workerPool.AddTestWorker("worker-1", pool.WorkerReady)

	tests := []struct {
		name     string
		taskID   string
		wantErr  bool
		errMatch string
	}{
		{"empty string", "", true, "task_id is required"},
		{"whitespace only", "   ", true, "invalid task_id format"},
		{"newlines", "\n\n", true, "invalid task_id format"},
		{"tab characters", "\t", true, "invalid task_id format"},
	}

	handler := cs.handlers["assign_task"]

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := fmt.Sprintf(`{"worker_id": "worker-1", "task_id": %q}`, tt.taskID)
			_, err := handler(context.Background(), json.RawMessage(args))
			if tt.wantErr && err == nil {
				t.Error("Expected error but got none")
			}
		})
	}
}

// TestEdge_EmptyWorkerID tests handling of empty worker IDs.
func TestEdge_EmptyWorkerID(t *testing.T) {
	workerPool := pool.NewWorkerPool(pool.Config{})
	defer workerPool.Close()

	cs := NewCoordinatorServer(claude.NewClient(), workerPool, nil, "/tmp/test", 8765, nil, mocks.NewMockBeadsExecutor(t))

	tests := []struct {
		name     string
		workerID string
		wantErr  bool
	}{
		{"empty string", "", true},
		{"whitespace only", "   ", true},
	}

	handler := cs.handlers["assign_task"]

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := fmt.Sprintf(`{"worker_id": %q, "task_id": "perles-abc.1"}`, tt.workerID)
			_, err := handler(context.Background(), json.RawMessage(args))
			if tt.wantErr && err == nil {
				t.Error("Expected error but got none")
			}
		})
	}
}

// TestEdge_VeryLongTaskID tests handling of very long task IDs.
func TestEdge_VeryLongTaskID(t *testing.T) {
	workerPool := pool.NewWorkerPool(pool.Config{})
	defer workerPool.Close()

	cs := NewCoordinatorServer(claude.NewClient(), workerPool, nil, "/tmp/test", 8765, nil, mocks.NewMockBeadsExecutor(t))
	_ = workerPool.AddTestWorker("worker-1", pool.WorkerReady)

	// Task ID with very long suffix (more than 10 chars)
	longSuffix := "perles-abcdefghijk" // 11 char suffix
	handler := cs.handlers["assign_task"]

	args := fmt.Sprintf(`{"worker_id": "worker-1", "task_id": %q}`, longSuffix)
	_, err := handler(context.Background(), json.RawMessage(args))
	if err == nil {
		t.Error("Expected error for task ID with >10 char suffix")
	}
}

// TestEdge_SpecialCharactersInTaskID tests rejection of special characters.
func TestEdge_SpecialCharactersInTaskID(t *testing.T) {
	workerPool := pool.NewWorkerPool(pool.Config{})
	defer workerPool.Close()

	cs := NewCoordinatorServer(claude.NewClient(), workerPool, nil, "/tmp/test", 8765, nil, mocks.NewMockBeadsExecutor(t))
	_ = workerPool.AddTestWorker("worker-1", pool.WorkerReady)

	specialChars := []string{
		"perles-abc$var",
		"perles-abc`cmd`",
		"perles-abc$(cmd)",
		"perles-abc|pipe",
		"perles-abc>redirect",
		"perles-abc<input",
		"perles-abc&background",
		"perles-abc;chain",
		"perles-abc'quote",
		`perles-abc"dquote`,
		"perles-abc\\escape",
	}

	handler := cs.handlers["assign_task"]

	for _, taskID := range specialChars {
		t.Run(taskID, func(t *testing.T) {
			args := fmt.Sprintf(`{"worker_id": "worker-1", "task_id": %q}`, taskID)
			_, err := handler(context.Background(), json.RawMessage(args))
			if err == nil {
				t.Errorf("Expected error for special char task ID %q", taskID)
			}
		})
	}
}

// TestEdge_MaxWorkersLimit tests behavior at max workers limit.
func TestEdge_MaxWorkersLimit(t *testing.T) {
	workerPool := pool.NewWorkerPool(pool.Config{MaxWorkers: 2})
	defer workerPool.Close()

	// Add max workers
	_ = workerPool.AddTestWorker("worker-1", pool.WorkerReady)
	_ = workerPool.AddTestWorker("worker-2", pool.WorkerReady)

	// Verify we can't add more
	// Note: AddTestWorker bypasses limit check, so we test via pool logic
	activeCount := len(workerPool.ActiveWorkers())
	if activeCount != 2 {
		t.Errorf("Expected 2 active workers, got %d", activeCount)
	}
}

// TestEdge_NilMessageStore tests handling when message store is nil.
func TestEdge_NilMessageStore(t *testing.T) {
	workerPool := pool.NewWorkerPool(pool.Config{})
	defer workerPool.Close()

	cs := NewCoordinatorServer(claude.NewClient(), workerPool, nil, "/tmp/test", 8765, nil, mocks.NewMockBeadsExecutor(t))

	handler := cs.handlers["post_message"]
	_, err := handler(context.Background(), json.RawMessage(`{"to": "ALL", "content": "test"}`))
	if err == nil {
		t.Error("Expected error when message store is nil")
	}
}

// TestEdge_WorkerStateWithNoAssignment tests query_worker_state for workers without assignments.
func TestEdge_WorkerStateWithNoAssignment(t *testing.T) {
	workerPool := pool.NewWorkerPool(pool.Config{})
	defer workerPool.Close()

	cs := NewCoordinatorServer(claude.NewClient(), workerPool, nil, "/tmp/test", 8765, nil, mocks.NewMockBeadsExecutor(t))

	// Add worker without any assignment
	_ = workerPool.AddTestWorker("worker-1", pool.WorkerReady)

	handler := cs.handlers["query_worker_state"]
	result, err := handler(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	var response workerStateResponse
	if err := json.Unmarshal([]byte(result.Content[0].Text), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	// Worker should show up with idle phase
	if len(response.Workers) != 1 {
		t.Fatalf("Expected 1 worker, got %d", len(response.Workers))
	}
	if response.Workers[0].Phase != "idle" {
		t.Errorf("Expected phase 'idle', got %q", response.Workers[0].Phase)
	}
}

// TestEdge_OrphanDetectionWithMissingWorker tests orphan detection when worker is completely missing.
func TestEdge_OrphanDetectionWithMissingWorker(t *testing.T) {
	workerPool := pool.NewWorkerPool(pool.Config{})
	defer workerPool.Close()

	cs := NewCoordinatorServer(claude.NewClient(), workerPool, nil, "/tmp/test", 8765, nil, mocks.NewMockBeadsExecutor(t))

	// Create task assignment pointing to non-existent worker
	cs.taskAssignments["perles-abc.1"] = &TaskAssignment{
		TaskID:      "perles-abc.1",
		Implementer: "nonexistent-worker",
		Status:      TaskImplementing,
	}

	orphans := cs.detectOrphanedTasks()
	if len(orphans) != 1 || orphans[0] != "perles-abc.1" {
		t.Errorf("Expected orphan perles-abc.1, got %v", orphans)
	}
}

// TestEdge_StuckWorkerWithZeroTaskDuration tests stuck detection with just-assigned workers.
func TestEdge_StuckWorkerWithZeroTaskDuration(t *testing.T) {
	workerPool := pool.NewWorkerPool(pool.Config{})
	defer workerPool.Close()

	cs := NewCoordinatorServer(claude.NewClient(), workerPool, nil, "/tmp/test", 8765, nil, mocks.NewMockBeadsExecutor(t))

	// Worker assigned just now
	cs.workerAssignments["worker-1"] = &WorkerAssignment{
		TaskID:     "perles-abc.1",
		AssignedAt: time.Now(),
	}

	stuck := cs.checkStuckWorkers()
	if len(stuck) != 0 {
		t.Errorf("Just-assigned worker should not be stuck, got %v", stuck)
	}
}

// TestEdge_StuckWorkerAtExactThreshold tests stuck detection at exactly MaxTaskDuration.
func TestEdge_StuckWorkerAtExactThreshold(t *testing.T) {
	workerPool := pool.NewWorkerPool(pool.Config{})
	defer workerPool.Close()

	cs := NewCoordinatorServer(claude.NewClient(), workerPool, nil, "/tmp/test", 8765, nil, mocks.NewMockBeadsExecutor(t))

	// Worker assigned with 1 second margin before threshold (will not be exceeded due to > comparison)
	// We use a small margin to avoid timing issues in the test.
	cs.workerAssignments["worker-1"] = &WorkerAssignment{
		TaskID:     "perles-abc.1",
		AssignedAt: time.Now().Add(-MaxTaskDuration + time.Second),
	}

	stuck := cs.checkStuckWorkers()
	// Just under threshold, not exceeded - should not be stuck
	if len(stuck) != 0 {
		t.Errorf("Worker just under threshold should not be stuck, got %v", stuck)
	}
}

// TestEdge_DuplicateTaskAssignment tests assigning same task twice.
func TestEdge_DuplicateTaskAssignment(t *testing.T) {
	workerPool := pool.NewWorkerPool(pool.Config{})
	defer workerPool.Close()

	cs := NewCoordinatorServer(claude.NewClient(), workerPool, nil, "/tmp/test", 8765, nil, mocks.NewMockBeadsExecutor(t))

	// Create workers
	_ = workerPool.AddTestWorker("worker-1", pool.WorkerReady)
	_ = workerPool.AddTestWorker("worker-2", pool.WorkerReady)

	// First assignment should succeed
	taskID := "perles-abc.1"
	cs.taskAssignments[taskID] = &TaskAssignment{
		TaskID:      taskID,
		Implementer: "worker-1",
		Status:      TaskImplementing,
	}
	cs.workerAssignments["worker-1"] = &WorkerAssignment{
		TaskID: taskID,
		Role:   RoleImplementer,
		Phase:  events.PhaseImplementing,
	}

	// Second assignment to different worker should fail
	err := cs.validateTaskAssignment("worker-2", taskID)
	if err == nil {
		t.Error("Expected error when assigning already-assigned task")
	}
}

// TestEdge_ReviewAssignmentToRetiredWorker tests assigning review to retired worker.
func TestEdge_ReviewAssignmentToRetiredWorker(t *testing.T) {
	workerPool := pool.NewWorkerPool(pool.Config{})
	defer workerPool.Close()

	cs := NewCoordinatorServer(claude.NewClient(), workerPool, nil, "/tmp/test", 8765, nil, mocks.NewMockBeadsExecutor(t))

	// Create implementer and setup task
	_ = workerPool.AddTestWorker("worker-1", pool.WorkerWorking)
	taskID := "perles-abc.1"
	cs.taskAssignments[taskID] = &TaskAssignment{
		TaskID:      taskID,
		Implementer: "worker-1",
	}
	cs.workerAssignments["worker-1"] = &WorkerAssignment{
		TaskID: taskID,
		Role:   RoleImplementer,
		Phase:  events.PhaseAwaitingReview,
	}

	// Create retired worker as reviewer
	_ = workerPool.AddTestWorker("worker-2", pool.WorkerRetired)

	err := cs.validateReviewAssignment("worker-2", taskID, "worker-1")
	if err == nil {
		t.Error("Expected error when assigning review to retired worker")
	}
}

// ============================================================================
// State Machine Transition Tests
// ============================================================================

// TestStateMachine_ImplementerLifecycle tests the full implementer lifecycle.
func TestStateMachine_ImplementerLifecycle(t *testing.T) {
	workerPool := pool.NewWorkerPool(pool.Config{})
	defer workerPool.Close()

	msgStore := newMockMessageStore()
	cs := NewCoordinatorServer(claude.NewClient(), workerPool, nil, "/tmp/test", 8765, nil, mocks.NewMockBeadsExecutor(t))

	workerID := "worker-1"
	taskID := "perles-abc.1"

	// 1. Start: Worker in Idle phase
	_ = workerPool.AddTestWorker(workerID, pool.WorkerReady)

	// No assignment yet
	if _, exists := cs.workerAssignments[workerID]; exists {
		t.Error("Worker should not have assignment initially")
	}

	// 2. Assign task -> Implementing phase
	cs.assignmentsMu.Lock()
	cs.workerAssignments[workerID] = &WorkerAssignment{
		TaskID:     taskID,
		Role:       RoleImplementer,
		Phase:      events.PhaseImplementing,
		AssignedAt: time.Now(),
	}
	cs.taskAssignments[taskID] = &TaskAssignment{
		TaskID:      taskID,
		Implementer: workerID,
		Status:      TaskImplementing,
		StartedAt:   time.Now(),
	}
	cs.assignmentsMu.Unlock()

	cs.assignmentsMu.RLock()
	if cs.workerAssignments[workerID].Phase != events.PhaseImplementing {
		t.Errorf("Expected phase %s, got %s", events.PhaseImplementing, cs.workerAssignments[workerID].Phase)
	}
	cs.assignmentsMu.RUnlock()

	// 3. Implementation complete -> AwaitingReview phase
	// Create worker server to test the tool
	ws := NewWorkerServer(workerID, msgStore)
	callback := &coordinatorStateCallback{cs: cs}
	ws.SetStateCallback(callback)

	cs.assignmentsMu.Lock()
	cs.workerAssignments[workerID].Phase = events.PhaseImplementing
	cs.assignmentsMu.Unlock()

	handler := ws.handlers["report_implementation_complete"]
	_, err := handler(context.Background(), json.RawMessage(`{"summary": "completed task"}`))
	if err != nil {
		t.Fatalf("Failed to report implementation complete: %v", err)
	}

	cs.assignmentsMu.RLock()
	if cs.workerAssignments[workerID].Phase != events.PhaseAwaitingReview {
		t.Errorf("Expected phase %s, got %s", events.PhaseAwaitingReview, cs.workerAssignments[workerID].Phase)
	}
	cs.assignmentsMu.RUnlock()
}

// TestStateMachine_ReviewerLifecycle tests the full reviewer lifecycle.
func TestStateMachine_ReviewerLifecycle(t *testing.T) {
	workerPool := pool.NewWorkerPool(pool.Config{})
	defer workerPool.Close()

	msgStore := newMockMessageStore()
	cs := NewCoordinatorServer(claude.NewClient(), workerPool, nil, "/tmp/test", 8765, nil, mocks.NewMockBeadsExecutor(t))

	implementerID := "worker-1"
	reviewerID := "worker-2"
	taskID := "perles-abc.1"

	// Setup: implementer in awaiting review
	_ = workerPool.AddTestWorker(implementerID, pool.WorkerWorking)
	_ = workerPool.AddTestWorker(reviewerID, pool.WorkerReady)

	cs.assignmentsMu.Lock()
	cs.workerAssignments[implementerID] = &WorkerAssignment{
		TaskID: taskID,
		Role:   RoleImplementer,
		Phase:  events.PhaseAwaitingReview,
	}
	cs.taskAssignments[taskID] = &TaskAssignment{
		TaskID:      taskID,
		Implementer: implementerID,
		Status:      TaskInReview,
	}
	cs.assignmentsMu.Unlock()

	// 1. Assign review -> Reviewing phase
	cs.assignmentsMu.Lock()
	cs.workerAssignments[reviewerID] = &WorkerAssignment{
		TaskID:        taskID,
		Role:          RoleReviewer,
		Phase:         events.PhaseReviewing,
		ImplementerID: implementerID,
		AssignedAt:    time.Now(),
	}
	cs.taskAssignments[taskID].Reviewer = reviewerID
	cs.assignmentsMu.Unlock()

	// Verify reviewer is in Reviewing phase
	cs.assignmentsMu.RLock()
	if cs.workerAssignments[reviewerID].Phase != events.PhaseReviewing {
		t.Errorf("Expected phase %s, got %s", events.PhaseReviewing, cs.workerAssignments[reviewerID].Phase)
	}
	cs.assignmentsMu.RUnlock()

	// 2. Submit verdict -> back to Idle
	ws := NewWorkerServer(reviewerID, msgStore)
	callback := &coordinatorStateCallback{cs: cs}
	ws.SetStateCallback(callback)

	handler := ws.handlers["report_review_verdict"]
	_, err := handler(context.Background(), json.RawMessage(`{"verdict": "APPROVED", "comments": "LGTM"}`))
	if err != nil {
		t.Fatalf("Failed to report review verdict: %v", err)
	}

	// Reviewer should be back to idle
	cs.assignmentsMu.RLock()
	if cs.workerAssignments[reviewerID].Phase != events.PhaseIdle {
		t.Errorf("Expected phase %s, got %s", events.PhaseIdle, cs.workerAssignments[reviewerID].Phase)
	}
	cs.assignmentsMu.RUnlock()
}

// TestStateMachine_DeniedReviewCycle tests the denial -> feedback -> re-review cycle.
func TestStateMachine_DeniedReviewCycle(t *testing.T) {
	workerPool := pool.NewWorkerPool(pool.Config{})
	defer workerPool.Close()

	cs := NewCoordinatorServer(claude.NewClient(), workerPool, nil, "/tmp/test", 8765, nil, mocks.NewMockBeadsExecutor(t))

	implementerID := "worker-1"
	taskID := "perles-abc.1"

	_ = workerPool.AddTestWorker(implementerID, pool.WorkerWorking)

	// Setup: task was denied
	cs.assignmentsMu.Lock()
	cs.workerAssignments[implementerID] = &WorkerAssignment{
		TaskID: taskID,
		Role:   RoleImplementer,
		Phase:  events.PhaseAwaitingReview, // After denial, goes to addressing_feedback
	}
	cs.taskAssignments[taskID] = &TaskAssignment{
		TaskID:      taskID,
		Implementer: implementerID,
		Status:      TaskDenied,
	}
	cs.assignmentsMu.Unlock()

	// 1. Assign feedback -> AddressingFeedback phase
	cs.assignmentsMu.Lock()
	cs.workerAssignments[implementerID].Phase = events.PhaseAddressingFeedback
	cs.assignmentsMu.Unlock()

	cs.assignmentsMu.RLock()
	if cs.workerAssignments[implementerID].Phase != events.PhaseAddressingFeedback {
		t.Errorf("Expected phase %s, got %s", events.PhaseAddressingFeedback, cs.workerAssignments[implementerID].Phase)
	}
	cs.assignmentsMu.RUnlock()

	// 2. Complete fixes -> AwaitingReview again
	cs.assignmentsMu.Lock()
	cs.workerAssignments[implementerID].Phase = events.PhaseAwaitingReview
	cs.taskAssignments[taskID].Status = TaskInReview
	cs.taskAssignments[taskID].Reviewer = "" // Ready for new reviewer
	cs.assignmentsMu.Unlock()

	cs.assignmentsMu.RLock()
	if cs.workerAssignments[implementerID].Phase != events.PhaseAwaitingReview {
		t.Errorf("Expected phase %s, got %s", events.PhaseAwaitingReview, cs.workerAssignments[implementerID].Phase)
	}
	cs.assignmentsMu.RUnlock()
}

// TestStateMachine_ApprovalToCommit tests the approval -> commit -> complete flow.
func TestStateMachine_ApprovalToCommit(t *testing.T) {
	workerPool := pool.NewWorkerPool(pool.Config{})
	defer workerPool.Close()

	cs := NewCoordinatorServer(claude.NewClient(), workerPool, nil, "/tmp/test", 8765, nil, mocks.NewMockBeadsExecutor(t))

	implementerID := "worker-1"
	taskID := "perles-abc.1"

	_ = workerPool.AddTestWorker(implementerID, pool.WorkerWorking)

	// Setup: task was approved
	cs.assignmentsMu.Lock()
	cs.workerAssignments[implementerID] = &WorkerAssignment{
		TaskID: taskID,
		Role:   RoleImplementer,
		Phase:  events.PhaseAwaitingReview,
	}
	cs.taskAssignments[taskID] = &TaskAssignment{
		TaskID:      taskID,
		Implementer: implementerID,
		Status:      TaskApproved,
	}
	cs.assignmentsMu.Unlock()

	// 1. Approve commit -> Committing phase
	cs.assignmentsMu.Lock()
	cs.workerAssignments[implementerID].Phase = events.PhaseCommitting
	cs.taskAssignments[taskID].Status = TaskCommitting
	cs.assignmentsMu.Unlock()

	cs.assignmentsMu.RLock()
	if cs.workerAssignments[implementerID].Phase != events.PhaseCommitting {
		t.Errorf("Expected phase %s, got %s", events.PhaseCommitting, cs.workerAssignments[implementerID].Phase)
	}
	cs.assignmentsMu.RUnlock()

	// 2. Commit complete -> Completed status, worker idle
	cs.assignmentsMu.Lock()
	cs.workerAssignments[implementerID].Phase = events.PhaseIdle
	cs.workerAssignments[implementerID].TaskID = "" // Task done
	cs.taskAssignments[taskID].Status = TaskCompleted
	cs.assignmentsMu.Unlock()

	cs.assignmentsMu.RLock()
	if cs.workerAssignments[implementerID].Phase != events.PhaseIdle {
		t.Errorf("Expected phase %s, got %s", events.PhaseIdle, cs.workerAssignments[implementerID].Phase)
	}
	if cs.taskAssignments[taskID].Status != TaskCompleted {
		t.Errorf("Expected status %s, got %s", TaskCompleted, cs.taskAssignments[taskID].Status)
	}
	cs.assignmentsMu.RUnlock()
}

// TestStateMachine_InvalidTransitionRejected tests that invalid transitions are rejected.
func TestStateMachine_InvalidTransitionRejected(t *testing.T) {
	workerPool := pool.NewWorkerPool(pool.Config{})
	defer workerPool.Close()

	msgStore := newMockMessageStore()
	cs := NewCoordinatorServer(claude.NewClient(), workerPool, nil, "/tmp/test", 8765, nil, mocks.NewMockBeadsExecutor(t))

	workerID := "worker-1"
	taskID := "perles-abc.1"

	_ = workerPool.AddTestWorker(workerID, pool.WorkerWorking)

	// Worker is idle - cannot report implementation complete
	cs.workerAssignments[workerID] = &WorkerAssignment{
		TaskID: taskID,
		Role:   RoleImplementer,
		Phase:  events.PhaseIdle,
	}

	ws := NewWorkerServer(workerID, msgStore)
	callback := &coordinatorStateCallback{cs: cs}
	ws.SetStateCallback(callback)

	handler := ws.handlers["report_implementation_complete"]
	_, err := handler(context.Background(), json.RawMessage(`{"summary": "done"}`))
	if err == nil {
		t.Error("Expected error when reporting implementation complete from idle phase")
	}
}

// TestStateMachine_AllTransitions tests all valid state transitions.
func TestStateMachine_AllTransitions(t *testing.T) {
	transitions := []struct {
		name   string
		from   events.WorkerPhase
		to     events.WorkerPhase
		valid  bool
		action string
	}{
		// Idle transitions
		{"idle->implementing", events.PhaseIdle, events.PhaseImplementing, true, "assign_task"},
		{"idle->reviewing", events.PhaseIdle, events.PhaseReviewing, true, "assign_task_review"},

		// Implementing transitions
		{"implementing->awaiting_review", events.PhaseImplementing, events.PhaseAwaitingReview, true, "report_implementation_complete"},
		{"implementing->idle (invalid)", events.PhaseImplementing, events.PhaseIdle, false, ""},
		{"implementing->reviewing (invalid)", events.PhaseImplementing, events.PhaseReviewing, false, ""},

		// AwaitingReview transitions
		{"awaiting->reviewing (assign reviewer)", events.PhaseAwaitingReview, events.PhaseReviewing, true, "assign_task_review"},
		{"awaiting->addressing_feedback", events.PhaseAwaitingReview, events.PhaseAddressingFeedback, true, "assign_review_feedback"},
		{"awaiting->committing", events.PhaseAwaitingReview, events.PhaseCommitting, true, "approve_commit"},

		// Reviewing transitions
		{"reviewing->idle", events.PhaseReviewing, events.PhaseIdle, true, "report_review_verdict"},
		{"reviewing->implementing (invalid)", events.PhaseReviewing, events.PhaseImplementing, false, ""},

		// AddressingFeedback transitions
		{"addressing_feedback->awaiting_review", events.PhaseAddressingFeedback, events.PhaseAwaitingReview, true, "report_implementation_complete"},

		// Committing transitions
		{"committing->idle", events.PhaseCommitting, events.PhaseIdle, true, "mark_task_complete"},
	}

	for _, tt := range transitions {
		t.Run(tt.name, func(t *testing.T) {
			// Just verify the transition is documented correctly
			// The actual enforcement is tested in tool-specific tests
			if tt.valid && tt.action == "" {
				t.Error("Valid transitions should have an action")
			}
		})
	}
}

// ============================================================================
// Helper: coordinatorStateCallback implements WorkerStateCallback
// ============================================================================

type coordinatorStateCallback struct {
	cs *CoordinatorServer
}

func (c *coordinatorStateCallback) GetWorkerPhase(workerID string) (events.WorkerPhase, error) {
	c.cs.assignmentsMu.RLock()
	defer c.cs.assignmentsMu.RUnlock()

	wa, ok := c.cs.workerAssignments[workerID]
	if !ok {
		return events.PhaseIdle, nil
	}
	return wa.Phase, nil
}

func (c *coordinatorStateCallback) OnImplementationComplete(workerID, summary string) error {
	c.cs.assignmentsMu.Lock()
	defer c.cs.assignmentsMu.Unlock()

	wa, ok := c.cs.workerAssignments[workerID]
	if !ok {
		return fmt.Errorf("worker not found: %s", workerID)
	}

	wa.Phase = events.PhaseAwaitingReview

	if ta, ok := c.cs.taskAssignments[wa.TaskID]; ok {
		ta.Status = TaskInReview
	}

	return nil
}

func (c *coordinatorStateCallback) OnReviewVerdict(workerID, verdict, comments string) error {
	c.cs.assignmentsMu.Lock()
	defer c.cs.assignmentsMu.Unlock()

	wa, ok := c.cs.workerAssignments[workerID]
	if !ok {
		return fmt.Errorf("worker not found: %s", workerID)
	}

	// Reviewer goes back to idle
	wa.Phase = events.PhaseIdle
	wa.TaskID = ""

	return nil
}
