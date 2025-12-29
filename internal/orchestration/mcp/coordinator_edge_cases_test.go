package mcp

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/zjrosen/perles/internal/mocks"
	"github.com/zjrosen/perles/internal/orchestration/claude"
	"github.com/zjrosen/perles/internal/orchestration/events"
	"github.com/zjrosen/perles/internal/orchestration/message"
	"github.com/zjrosen/perles/internal/orchestration/pool"
)

// ============================================================================
// Edge Case Tests
//
// These tests verify correct behavior at system boundaries and unusual states.
// ============================================================================

// TestEdge_EmptyPool tests operations when no workers exist.
func TestEdge_EmptyPool(t *testing.T) {
	workerPool := pool.NewWorkerPool(pool.Config{})
	defer workerPool.Close()

	cs := NewCoordinatorServer(claude.NewClient(), workerPool, nil, "/tmp/test", 8765, nil, mocks.NewMockBeadsExecutor(t))

	t.Run("list_workers_empty", func(t *testing.T) {
		handler := cs.handlers["list_workers"]
		result, err := handler(context.Background(), nil)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if result.Content[0].Text != "No active workers." {
			t.Errorf("Expected 'No active workers.', got %q", result.Content[0].Text)
		}
	})

	t.Run("query_worker_state_empty", func(t *testing.T) {
		handler := cs.handlers["query_worker_state"]
		result, err := handler(context.Background(), json.RawMessage(`{}`))
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		var response workerStateResponse
		if err := json.Unmarshal([]byte(result.Content[0].Text), &response); err != nil {
			t.Fatalf("Failed to parse: %v", err)
		}

		if len(response.Workers) != 0 {
			t.Errorf("Expected 0 workers, got %d", len(response.Workers))
		}
		if len(response.ReadyWorkers) != 0 {
			t.Errorf("Expected 0 ready workers, got %d", len(response.ReadyWorkers))
		}
	})

	t.Run("detect_orphaned_tasks_empty", func(t *testing.T) {
		orphans := cs.detectOrphanedTasks()
		if len(orphans) != 0 {
			t.Errorf("Expected 0 orphans, got %d", len(orphans))
		}
	})

	t.Run("check_stuck_workers_empty", func(t *testing.T) {
		stuck := cs.checkStuckWorkers()
		if len(stuck) != 0 {
			t.Errorf("Expected 0 stuck workers, got %d", len(stuck))
		}
	})
}

// TestEdge_AllWorkersRetired tests operations when all workers are retired.
func TestEdge_AllWorkersRetired(t *testing.T) {
	workerPool := pool.NewWorkerPool(pool.Config{})
	defer workerPool.Close()

	cs := NewCoordinatorServer(claude.NewClient(), workerPool, nil, "/tmp/test", 8765, nil, mocks.NewMockBeadsExecutor(t))

	// Create workers and retire them
	worker1 := workerPool.AddTestWorker("worker-1", pool.WorkerReady)
	worker2 := workerPool.AddTestWorker("worker-2", pool.WorkerReady)
	worker1.Retire()
	worker2.Retire()

	t.Run("no_ready_workers", func(t *testing.T) {
		handler := cs.handlers["query_worker_state"]
		result, _ := handler(context.Background(), json.RawMessage(`{}`))

		var response workerStateResponse
		_ = json.Unmarshal([]byte(result.Content[0].Text), &response)

		if len(response.ReadyWorkers) != 0 {
			t.Errorf("Expected 0 ready workers, got %d", len(response.ReadyWorkers))
		}
	})

	t.Run("validate_assignment_fails_retired", func(t *testing.T) {
		err := cs.validateTaskAssignment("worker-1", "perles-abc.1")
		if err == nil {
			t.Error("Expected error assigning to retired worker")
		}
	})
}

// TestEdge_TaskIDFormats tests various task ID formats.
func TestEdge_TaskIDFormats(t *testing.T) {
	tests := []struct {
		name    string
		taskID  string
		isValid bool
	}{
		// Valid formats
		{"standard", "perles-abcd", true},
		{"with_subtask", "perles-abcd.1", true},
		{"multi_digit_subtask", "perles-abcd.123", true},
		{"short_prefix", "ms-abc", true},
		{"uppercase_prefix", "PERLES-abc", true},
		{"mixed_case", "PerLes-AbCd", true},
		{"numeric_suffix", "perles-1234", true},
		{"alphanumeric_suffix", "perles-a1b2", true},
		{"max_suffix_length", "perles-abcdefghij", true}, // 10 chars

		// Invalid formats
		{"empty", "", false},
		{"too_short_suffix", "perles-a", false},          // Only 1 char
		{"too_long_suffix", "perles-abcdefghijk", false}, // 11 chars
		{"no_dash", "perlesabcd", false},
		{"double_dash", "perles--abc", false},
		{"leading_dash", "-perles-abc", false},
		{"trailing_dash", "perles-abc-", false},
		{"special_chars", "perles-abc$", false},
		{"space", "perles abc", false},
		{"newline", "perles-abc\n", false},
		{"tab", "perles-abc\t", false},
		{"semicolon", "perles-abc;ls", false},
		{"path_traversal", "../perles-abc", false},
		{"command_injection", "perles-abc;rm -rf /", false},
		{"flag_injection", "--task-id=perles-abc", false},
		{"unicode", "perles-abc\u200b", false}, // Zero-width space
		{"subtask_double_dot", "perles-abc..1", false},
		{"subtask_no_number", "perles-abc.", false},
		{"subtask_alpha", "perles-abc.a", false},
		{"multiple_subtasks", "perles-abc.1.2", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isValidTaskID(tt.taskID)
			if result != tt.isValid {
				t.Errorf("IsValidTaskID(%q) = %v, want %v", tt.taskID, result, tt.isValid)
			}
		})
	}
}

// TestEdge_WorkerIDFormats tests various worker ID edge cases in tool calls.
func TestEdge_WorkerIDFormats(t *testing.T) {
	workerPool := pool.NewWorkerPool(pool.Config{})
	defer workerPool.Close()

	cs := NewCoordinatorServer(claude.NewClient(), workerPool, nil, "/tmp/test", 8765, nil, mocks.NewMockBeadsExecutor(t))

	tests := []struct {
		name     string
		workerID string
		wantErr  bool
	}{
		{"valid_simple", "worker-1", true}, // Error because worker doesn't exist
		{"empty", "", true},
		{"whitespace", "   ", true},
		{"very_long", "worker-" + string(make([]byte, 1000)), true},
		{"special_chars", "worker;rm -rf /", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := cs.handlers["send_to_worker"]
			args := `{"worker_id": "` + tt.workerID + `", "message": "test"}`
			_, err := handler(context.Background(), json.RawMessage(args))
			if (err != nil) != tt.wantErr {
				t.Errorf("error = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}

// TestEdge_MaxTaskDurationBoundary tests behavior at exactly MaxTaskDuration.
func TestEdge_MaxTaskDurationBoundary(t *testing.T) {
	workerPool := pool.NewWorkerPool(pool.Config{})
	defer workerPool.Close()

	cs := NewCoordinatorServer(claude.NewClient(), workerPool, nil, "/tmp/test", 8765, nil, mocks.NewMockBeadsExecutor(t))

	tests := []struct {
		name        string
		duration    time.Duration
		expectStuck bool
	}{
		{"just_under", MaxTaskDuration - time.Second, false}, // Use second margin to avoid timing issues
		{"just_over", MaxTaskDuration + time.Second, true},   // Use second margin to ensure definitely over
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset state
			cs.assignmentsMu.Lock()
			cs.workerAssignments = make(map[string]*WorkerAssignment)
			cs.assignmentsMu.Unlock()

			cs.SetWorkerAssignment("worker-1", &WorkerAssignment{
				TaskID:     "perles-abc.1",
				AssignedAt: time.Now().Add(-tt.duration),
			})

			stuck := cs.checkStuckWorkers()
			isStuck := len(stuck) > 0

			if isStuck != tt.expectStuck {
				t.Errorf("For duration %v: stuck = %v, expectStuck = %v", tt.duration, isStuck, tt.expectStuck)
			}
		})
	}
}

// TestEdge_AssignmentMapsConsistency tests that workerAssignments and taskAssignments stay consistent.
func TestEdge_AssignmentMapsConsistency(t *testing.T) {
	workerPool := pool.NewWorkerPool(pool.Config{})
	defer workerPool.Close()

	cs := NewCoordinatorServer(claude.NewClient(), workerPool, nil, "/tmp/test", 8765, nil, mocks.NewMockBeadsExecutor(t))

	// Create worker
	_ = workerPool.AddTestWorker("worker-1", pool.WorkerReady)

	// Create valid assignment
	now := time.Now()
	cs.SetTaskAssignment("perles-abc.1", &TaskAssignment{
		TaskID:      "perles-abc.1",
		Implementer: "worker-1",
		Status:      TaskImplementing,
		StartedAt:   now,
	})
	cs.SetWorkerAssignment("worker-1", &WorkerAssignment{
		TaskID:     "perles-abc.1",
		Role:       RoleImplementer,
		Phase:      events.PhaseImplementing,
		AssignedAt: now,
	})

	// Check consistency
	t.Run("maps_are_consistent", func(t *testing.T) {
		cs.assignmentsMu.RLock()
		defer cs.assignmentsMu.RUnlock()

		ta := cs.taskAssignments["perles-abc.1"]
		wa := cs.workerAssignments["worker-1"]

		if ta.Implementer != "worker-1" {
			t.Errorf("Task implementer = %q, want %q", ta.Implementer, "worker-1")
		}
		if wa.TaskID != "perles-abc.1" {
			t.Errorf("Worker task = %q, want %q", wa.TaskID, "perles-abc.1")
		}
	})

	// Test inconsistent state detection
	t.Run("inconsistent_state_detected", func(t *testing.T) {
		// Simulate inconsistent state: task points to worker-2, but no such worker assignment
		cs.SetTaskAssignment("perles-xyz.1", &TaskAssignment{
			TaskID:      "perles-xyz.1",
			Implementer: "worker-nonexistent", // No such worker
			Status:      TaskImplementing,
		})

		// This should be detected as an orphan
		orphans := cs.detectOrphanedTasks()
		found := false
		for _, o := range orphans {
			if o == "perles-xyz.1" {
				found = true
				break
			}
		}
		if !found {
			t.Error("Expected perles-xyz.1 to be detected as orphan")
		}
	})
}

// TestEdge_NilMessageIssue tests behavior when message issue is nil.
func TestEdge_NilMessageIssue(t *testing.T) {
	workerPool := pool.NewWorkerPool(pool.Config{})
	defer workerPool.Close()

	// Create server with nil message issue
	cs := NewCoordinatorServer(claude.NewClient(), workerPool, nil, "/tmp/test", 8765, nil, mocks.NewMockBeadsExecutor(t))

	t.Run("post_message_fails", func(t *testing.T) {
		handler := cs.handlers["post_message"]
		args := `{"to": "ALL", "content": "test"}`
		_, err := handler(context.Background(), json.RawMessage(args))
		if err == nil {
			t.Error("Expected error when message issue is nil")
		}
	})

	t.Run("read_message_log_fails", func(t *testing.T) {
		handler := cs.handlers["read_message_log"]
		_, err := handler(context.Background(), nil)
		if err == nil {
			t.Error("Expected error when message issue is nil")
		}
	})

	t.Run("prepare_handoff_fails", func(t *testing.T) {
		handler := cs.handlers["prepare_handoff"]
		args := `{"summary": "test summary"}`
		_, err := handler(context.Background(), json.RawMessage(args))
		if err == nil {
			t.Error("Expected error when message issue is nil")
		}
	})
}

// TestEdge_LargeNumberOfWorkers tests performance with many workers.
func TestEdge_LargeNumberOfWorkers(t *testing.T) {
	workerPool := pool.NewWorkerPool(pool.Config{})
	defer workerPool.Close()

	cs := NewCoordinatorServer(claude.NewClient(), workerPool, nil, "/tmp/test", 8765, nil, mocks.NewMockBeadsExecutor(t))

	// Create 100 workers
	for i := 0; i < 100; i++ {
		id := "worker-" + string(rune('0'+i/10)) + string(rune('0'+i%10))
		_ = workerPool.AddTestWorker(id, pool.WorkerReady)
	}

	t.Run("list_workers_all", func(t *testing.T) {
		handler := cs.handlers["list_workers"]
		result, err := handler(context.Background(), nil)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		var infos []map[string]any
		if err := json.Unmarshal([]byte(result.Content[0].Text), &infos); err != nil {
			t.Fatalf("Failed to parse: %v", err)
		}

		if len(infos) != 100 {
			t.Errorf("Expected 100 workers, got %d", len(infos))
		}
	})

	t.Run("query_worker_state_all", func(t *testing.T) {
		handler := cs.handlers["query_worker_state"]
		result, err := handler(context.Background(), json.RawMessage(`{}`))
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		var response workerStateResponse
		if err := json.Unmarshal([]byte(result.Content[0].Text), &response); err != nil {
			t.Fatalf("Failed to parse: %v", err)
		}

		if len(response.Workers) != 100 {
			t.Errorf("Expected 100 workers, got %d", len(response.Workers))
		}
		if len(response.ReadyWorkers) != 100 {
			t.Errorf("Expected 100 ready workers, got %d", len(response.ReadyWorkers))
		}
	})
}

// TestEdge_LargeNumberOfTasks tests performance with many tasks.
func TestEdge_LargeNumberOfTasks(t *testing.T) {
	workerPool := pool.NewWorkerPool(pool.Config{})
	defer workerPool.Close()

	cs := NewCoordinatorServer(claude.NewClient(), workerPool, nil, "/tmp/test", 8765, nil, mocks.NewMockBeadsExecutor(t))

	// Create 100 tasks (more than workers)
	for i := 0; i < 100; i++ {
		taskID := "perles-" + string(rune('a'+i/26)) + string(rune('a'+i%26)) + "c.1"
		cs.SetTaskAssignment(taskID, &TaskAssignment{
			TaskID:      taskID,
			Implementer: "worker-" + string(rune('0'+i%10)),
			Status:      TaskImplementing,
			StartedAt:   time.Now(),
		})
	}

	t.Run("query_worker_state_with_many_tasks", func(t *testing.T) {
		handler := cs.handlers["query_worker_state"]
		result, err := handler(context.Background(), json.RawMessage(`{}`))
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		var response workerStateResponse
		if err := json.Unmarshal([]byte(result.Content[0].Text), &response); err != nil {
			t.Fatalf("Failed to parse: %v", err)
		}

		if len(response.TaskAssignments) != 100 {
			t.Errorf("Expected 100 task assignments, got %d", len(response.TaskAssignments))
		}
	})

	t.Run("detect_orphaned_tasks_many", func(t *testing.T) {
		// All tasks have nonexistent workers, so all should be orphans
		orphans := cs.detectOrphanedTasks()
		if len(orphans) != 100 {
			t.Errorf("Expected 100 orphans (workers don't exist), got %d", len(orphans))
		}
	})
}

// TestEdge_MalformedJSON tests handling of malformed JSON in tool arguments.
func TestEdge_MalformedJSON(t *testing.T) {
	workerPool := pool.NewWorkerPool(pool.Config{})
	defer workerPool.Close()

	cs := NewCoordinatorServer(claude.NewClient(), workerPool, nil, "/tmp/test", 8765, nil, mocks.NewMockBeadsExecutor(t))

	malformedCases := []string{
		`not json`,
		`{`,
		`}`,
		`[]`,
		`{"worker_id": }`,
		`{"worker_id": "test", }`,
		`null`,
		`123`,
		`"string"`,
	}

	handlers := []string{"assign_task", "replace_worker", "send_to_worker", "post_message"}

	for _, handler := range handlers {
		for _, malformed := range malformedCases {
			t.Run(handler+"_"+malformed, func(t *testing.T) {
				h := cs.handlers[handler]
				_, err := h(context.Background(), json.RawMessage(malformed))
				if err == nil {
					t.Error("Expected error for malformed JSON")
				}
			})
		}
	}
}

// TestEdge_UnicodeContent tests handling of unicode in message content.
func TestEdge_UnicodeContent(t *testing.T) {
	workerPool := pool.NewWorkerPool(pool.Config{})
	defer workerPool.Close()

	msgIssue := message.New()
	cs := NewCoordinatorServer(claude.NewClient(), workerPool, msgIssue, "/tmp/test", 8765, nil, mocks.NewMockBeadsExecutor(t))

	unicodeStrings := []string{
		"Hello 世界",
		"Привет мир",
		"مرحبا بالعالم",
		"שלום עולם",
		"🚀 Launch!",
		"Emoji: 👨‍👩‍👧‍👦",
		"Mixed: Hello 世界 🌍",
		"Zero-width: test\u200btest",
		"Newlines: line1\nline2",
		"Tabs: col1\tcol2",
	}

	handler := cs.handlers["post_message"]
	ctx := context.Background()

	for _, content := range unicodeStrings {
		t.Run(content[:min(10, len(content))], func(t *testing.T) {
			// Properly escape for JSON
			jsonContent, _ := json.Marshal(content)
			args := `{"to": "ALL", "content": ` + string(jsonContent) + `}`
			result, err := handler(ctx, json.RawMessage(args))
			if err != nil {
				t.Errorf("Failed for unicode content: %v", err)
			}
			if result == nil {
				t.Error("Expected non-nil result")
			}
		})
	}
}

// TestEdge_WorkerPhaseTransitions tests phase boundary conditions.
func TestEdge_WorkerPhaseTransitions(t *testing.T) {
	workerPool := pool.NewWorkerPool(pool.Config{})
	defer workerPool.Close()

	cs := NewCoordinatorServer(claude.NewClient(), workerPool, nil, "/tmp/test", 8765, nil, mocks.NewMockBeadsExecutor(t))

	// All valid phase values
	phases := []events.WorkerPhase{
		events.PhaseIdle,
		events.PhaseImplementing,
		events.PhaseAwaitingReview,
		events.PhaseReviewing,
		events.PhaseAddressingFeedback,
		events.PhaseCommitting,
	}

	for _, phase := range phases {
		t.Run("phase_"+string(phase), func(t *testing.T) {
			cs.SetWorkerAssignment("worker-1", &WorkerAssignment{
				TaskID:     "perles-abc.1",
				Role:       RoleImplementer,
				Phase:      phase,
				AssignedAt: time.Now(),
			})

			handler := cs.handlers["query_worker_state"]
			result, err := handler(context.Background(), json.RawMessage(`{"worker_id": "worker-1"}`))
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			var response workerStateResponse
			if err := json.Unmarshal([]byte(result.Content[0].Text), &response); err != nil {
				t.Fatalf("Failed to parse: %v", err)
			}

			// Phase should be preserved and queryable
			if len(response.Workers) > 0 && response.Workers[0].Phase != string(phase) {
				t.Errorf("Phase = %q, want %q", response.Workers[0].Phase, phase)
			}
		})
	}
}

// TestEdge_TaskStatusTransitions tests all task status values.
func TestEdge_TaskStatusTransitions(t *testing.T) {
	workerPool := pool.NewWorkerPool(pool.Config{})
	defer workerPool.Close()

	cs := NewCoordinatorServer(claude.NewClient(), workerPool, nil, "/tmp/test", 8765, nil, mocks.NewMockBeadsExecutor(t))

	statuses := []TaskWorkflowStatus{
		TaskImplementing,
		TaskInReview,
		TaskApproved,
		TaskDenied,
		TaskCommitting,
		TaskCompleted,
	}

	for _, status := range statuses {
		t.Run("status_"+string(status), func(t *testing.T) {
			cs.SetTaskAssignment("perles-abc.1", &TaskAssignment{
				TaskID:      "perles-abc.1",
				Implementer: "worker-1",
				Status:      status,
			})

			handler := cs.handlers["query_worker_state"]
			result, err := handler(context.Background(), json.RawMessage(`{}`))
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			var response workerStateResponse
			if err := json.Unmarshal([]byte(result.Content[0].Text), &response); err != nil {
				t.Fatalf("Failed to parse: %v", err)
			}

			// Status should be preserved
			if ta, ok := response.TaskAssignments["perles-abc.1"]; ok {
				if ta.Status != string(status) {
					t.Errorf("Status = %q, want %q", ta.Status, status)
				}
			}
		})
	}
}

// min helper (Go < 1.21 compatibility)
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
