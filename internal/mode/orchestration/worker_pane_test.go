package orchestration

import (
	"testing"

	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/exp/teatest"
	"github.com/stretchr/testify/require"

	"github.com/zjrosen/perles/internal/orchestration/events"
	"github.com/zjrosen/perles/internal/orchestration/metrics"
)

func TestPhaseShortName(t *testing.T) {
	tests := []struct {
		name     string
		phase    events.ProcessPhase
		expected string
	}{
		{"idle returns empty", events.ProcessPhaseIdle, "idle"},
		{"implementing returns impl", events.ProcessPhaseImplementing, "impl"},
		{"awaiting review returns await", events.ProcessPhaseAwaitingReview, "await"},
		{"reviewing returns review", events.ProcessPhaseReviewing, "review"},
		{"addressing feedback returns feedback", events.ProcessPhaseAddressingFeedback, "feedback"},
		{"committing returns commit", events.ProcessPhaseCommitting, "commit"},
		{"unknown phase returns empty", events.ProcessPhase("unknown"), ""},
		{"empty phase returns empty", events.ProcessPhase(""), ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := phaseShortName(tt.phase)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatWorkerTitle_WithTaskAndPhase(t *testing.T) {
	title := formatWorkerTitle("worker-1", events.ProcessStatusWorking, "perles-abc.1", events.ProcessPhaseImplementing)

	// Should contain worker ID in uppercase
	require.Contains(t, title, "WORKER-1")
	// Should contain task ID
	require.Contains(t, title, "perles-abc.1")
	// Should contain phase short name in parentheses
	require.Contains(t, title, "(impl)")
}

func TestFormatWorkerTitle_WithTaskNoPhase(t *testing.T) {
	// Task assigned but phase is idle
	title := formatWorkerTitle("worker-2", events.ProcessStatusWorking, "perles-xyz.5", events.ProcessPhaseIdle)

	require.Contains(t, title, "WORKER-2")
	require.Contains(t, title, "perles-xyz.5")
	// Should show (idle) when phase is idle
	require.Contains(t, title, "(idle)")
}

func TestFormatWorkerTitle_Idle(t *testing.T) {
	// Ready worker with no task
	title := formatWorkerTitle("worker-3", events.ProcessStatusReady, "", events.ProcessPhaseIdle)

	require.Contains(t, title, "WORKER-3")
	// Should NOT contain task ID or phase (no task = no phase display)
	require.NotContains(t, title, "perles")
	require.NotContains(t, title, "(")
}

func TestFormatWorkerTitle_Retired(t *testing.T) {
	// Retired worker
	title := formatWorkerTitle("worker-4", events.ProcessStatusRetired, "", events.ProcessPhaseIdle)

	require.Contains(t, title, "WORKER-4")
	// Should NOT contain task info
	require.NotContains(t, title, "(")
}

func TestFormatWorkerTitle_AllPhases(t *testing.T) {
	// Test all phases produce expected short names
	phases := []struct {
		phase    events.ProcessPhase
		expected string
	}{
		{events.ProcessPhaseImplementing, "(impl)"},
		{events.ProcessPhaseAwaitingReview, "(await)"},
		{events.ProcessPhaseReviewing, "(review)"},
		{events.ProcessPhaseAddressingFeedback, "(feedback)"},
		{events.ProcessPhaseCommitting, "(commit)"},
	}

	for _, tt := range phases {
		t.Run(string(tt.phase), func(t *testing.T) {
			title := formatWorkerTitle("worker-1", events.ProcessStatusWorking, "task-123", tt.phase)
			require.Contains(t, title, tt.expected)
		})
	}
}

func TestFormatWorkerTitle_UnknownPhase(t *testing.T) {
	// Unknown phase should be handled gracefully (no parentheses)
	title := formatWorkerTitle("worker-1", events.ProcessStatusWorking, "task-123", events.ProcessPhase("unknown_phase"))

	require.Contains(t, title, "WORKER-1")
	require.Contains(t, title, "task-123")
	// Unknown phase should not produce parentheses
	require.NotContains(t, title, "(")
}

func TestRenderSingleWorkerPane_DoesNotCrash(t *testing.T) {
	// Create model (pool has been removed)
	m := New(Config{})

	// Initialize worker pane state for the worker
	m.workerPane.workerStatus["worker-1"] = events.ProcessStatusWorking
	m.workerPane.viewports = make(map[string]viewport.Model)

	// Should not panic
	require.NotPanics(t, func() {
		_ = m.renderSingleWorkerPane("worker-1", 80, 20)
	})
}

func TestWorkerPane_QueueCountDisplay(t *testing.T) {
	// Create model with a worker
	m := New(Config{})

	// Initialize worker pane state
	m.workerPane.workerStatus["worker-1"] = events.ProcessStatusWorking
	m.workerPane.viewports = make(map[string]viewport.Model)
	m.workerPane.workerQueueCounts["worker-1"] = 3

	// Render the pane
	pane := m.renderSingleWorkerPane("worker-1", 80, 20)

	// Should contain the queue count indicator
	require.Contains(t, pane, "[3 queued]")
}

func TestWorkerPane_NoQueueDisplay(t *testing.T) {
	// Create model with a worker
	m := New(Config{})

	// Initialize worker pane state with zero queue count
	m.workerPane.workerStatus["worker-1"] = events.ProcessStatusReady
	m.workerPane.viewports = make(map[string]viewport.Model)
	m.workerPane.workerQueueCounts["worker-1"] = 0

	// Render the pane
	pane := m.renderSingleWorkerPane("worker-1", 80, 20)

	// Should NOT contain the queue indicator
	require.NotContains(t, pane, "queued")
}

func TestWorkerPane_SetQueueCount(t *testing.T) {
	// Create model
	m := New(Config{})

	// Initial state should have empty map
	require.Empty(t, m.workerPane.workerQueueCounts)

	// Set queue count for a worker
	m = m.SetQueueCount("worker-1", 5)

	// Verify the count is stored
	require.Equal(t, 5, m.workerPane.workerQueueCounts["worker-1"])

	// Update to a different count
	m = m.SetQueueCount("worker-1", 2)
	require.Equal(t, 2, m.workerPane.workerQueueCounts["worker-1"])

	// Set count to zero
	m = m.SetQueueCount("worker-1", 0)
	require.Equal(t, 0, m.workerPane.workerQueueCounts["worker-1"])
}

func TestWorkerPane_QueueCountMultipleWorkers(t *testing.T) {
	// Create model with multiple workers
	m := New(Config{})

	// Initialize worker pane state for two workers
	m.workerPane.workerStatus["worker-1"] = events.ProcessStatusWorking
	m.workerPane.workerStatus["worker-2"] = events.ProcessStatusWorking
	m.workerPane.viewports = make(map[string]viewport.Model)

	// Set different queue counts
	m = m.SetQueueCount("worker-1", 3)
	m = m.SetQueueCount("worker-2", 7)

	// Verify both counts are stored independently
	require.Equal(t, 3, m.workerPane.workerQueueCounts["worker-1"])
	require.Equal(t, 7, m.workerPane.workerQueueCounts["worker-2"])
}

func TestQueuedCountStyle_OrangeForeground(t *testing.T) {
	// Verify the QueuedCountStyle has orange foreground color
	fg := QueuedCountStyle.GetForeground()

	// The style uses AdaptiveColor, cast and check
	adaptiveColor, ok := fg.(lipgloss.AdaptiveColor)
	require.True(t, ok, "foreground should be AdaptiveColor")

	// Verify the colors are orange (as specified in task)
	// Light: "#FFA500" (standard orange)
	// Dark: "#FFB347" (lighter orange for dark themes)
	require.Equal(t, "#FFA500", adaptiveColor.Light, "light mode should be orange")
	require.Equal(t, "#FFB347", adaptiveColor.Dark, "dark mode should be light orange")
}

// ============================================================================
// Golden Tests for Worker Pane
// ============================================================================
//
// These tests capture the visual output of the worker pane in various states.
// They serve as baseline snapshots before refactoring to detect visual regressions.
//
// To update golden files: go test -update ./internal/mode/orchestration/...
//
// NOTE: StatusIndicator tests are in internal/ui/shared/panes/agentstatus_test.go
// since the function was extracted to the shared panes package.

// newTestWorkerModel creates a model configured for worker pane golden tests.
// It sets up the minimal state needed to render the worker pane in isolation.
func newTestWorkerModel() Model {
	m := New(Config{})
	m = m.SetSize(80, 24) // Standard terminal size for consistent output

	// Ensure maps are initialized
	m.workerPane.viewports = make(map[string]viewport.Model)
	m.workerPane.contentDirty = make(map[string]bool)
	m.workerPane.hasNewContent = make(map[string]bool)
	m.workerPane.workerMessages = make(map[string][]ChatMessage)
	m.workerPane.workerMetrics = make(map[string]*metrics.TokenMetrics)
	m.workerPane.workerStatus = make(map[string]events.ProcessStatus)
	m.workerPane.workerTaskIDs = make(map[string]string)
	m.workerPane.workerPhases = make(map[string]events.ProcessPhase)
	m.workerPane.workerQueueCounts = make(map[string]int)

	return m
}

// addTestWorker adds a worker to the model with the given configuration.
func addTestWorker(m Model, workerID string, status events.ProcessStatus, taskID string, phase events.ProcessPhase) Model {
	// Add to workerIDs slice (used by ActiveWorkerIDs)
	m.workerPane.workerIDs = append(m.workerPane.workerIDs, workerID)

	m.workerPane.workerStatus[workerID] = status
	m.workerPane.workerTaskIDs[workerID] = taskID
	m.workerPane.workerPhases[workerID] = phase
	m.workerPane.contentDirty[workerID] = true

	// Add some sample output for realistic rendering
	m.workerPane.workerMessages[workerID] = []ChatMessage{
		{Role: "assistant", Content: "Working on the implementation..."},
		{Role: "user", Content: "Task assigned: " + taskID},
	}

	return m
}

func TestWorkerPane_Golden_ReadyStatus(t *testing.T) {
	// Ready status shows green empty circle ○
	m := newTestWorkerModel()
	m = addTestWorker(m, "worker-1", events.ProcessStatusReady, "", events.ProcessPhaseIdle)

	pane := m.renderSingleWorkerPane("worker-1", 80, 20)
	teatest.RequireEqualOutput(t, []byte(pane))
}

func TestWorkerPane_Golden_WorkingStatus(t *testing.T) {
	// Working status shows blue filled circle ● and blue border
	m := newTestWorkerModel()
	m = addTestWorker(m, "worker-1", events.ProcessStatusWorking, "perles-abc.1", events.ProcessPhaseImplementing)

	pane := m.renderSingleWorkerPane("worker-1", 80, 20)
	teatest.RequireEqualOutput(t, []byte(pane))
}

func TestWorkerPane_Golden_StoppedStatus(t *testing.T) {
	// Stopped status shows ⚠ indicator and red border
	m := newTestWorkerModel()
	m = addTestWorker(m, "worker-1", events.ProcessStatusStopped, "perles-xyz.2", events.ProcessPhaseIdle)

	pane := m.renderSingleWorkerPane("worker-1", 80, 20)
	teatest.RequireEqualOutput(t, []byte(pane))
}

func TestWorkerPane_Golden_RetiredStatus(t *testing.T) {
	// Retired status shows ✗ indicator and red border
	m := newTestWorkerModel()
	m = addTestWorker(m, "worker-1", events.ProcessStatusRetired, "", events.ProcessPhaseIdle)

	pane := m.renderSingleWorkerPane("worker-1", 80, 20)
	teatest.RequireEqualOutput(t, []byte(pane))
}

func TestWorkerPane_Golden_FailedStatus(t *testing.T) {
	// Failed status shows ✗ indicator and red border (same as Retired)
	m := newTestWorkerModel()
	m = addTestWorker(m, "worker-1", events.ProcessStatusFailed, "perles-err.1", events.ProcessPhaseIdle)

	pane := m.renderSingleWorkerPane("worker-1", 80, 20)
	teatest.RequireEqualOutput(t, []byte(pane))
}

func TestWorkerPane_Golden_MultipleWorkersStacked(t *testing.T) {
	// Multiple workers stacked vertically
	m := newTestWorkerModel()
	m = addTestWorker(m, "worker-1", events.ProcessStatusWorking, "perles-abc.1", events.ProcessPhaseImplementing)
	m = addTestWorker(m, "worker-2", events.ProcessStatusReady, "", events.ProcessPhaseIdle)
	m = addTestWorker(m, "worker-3", events.ProcessStatusWorking, "perles-xyz.2", events.ProcessPhaseReviewing)

	pane := m.renderWorkerPanes(80, 60)
	teatest.RequireEqualOutput(t, []byte(pane))
}

func TestWorkerPane_Golden_WithMetrics(t *testing.T) {
	// Worker pane with token metrics displayed in title right area
	m := newTestWorkerModel()
	m = addTestWorker(m, "worker-1", events.ProcessStatusWorking, "perles-abc.1", events.ProcessPhaseImplementing)
	m.workerPane.workerMetrics["worker-1"] = &metrics.TokenMetrics{
		TokensUsed:   25000,
		TotalTokens:  200000,
		OutputTokens: 800,
		TotalCostUSD: 0.25,
	}

	pane := m.renderSingleWorkerPane("worker-1", 80, 20)
	teatest.RequireEqualOutput(t, []byte(pane))
}

func TestWorkerPane_Golden_WithQueueCount(t *testing.T) {
	// Worker pane with queue count displayed in bottom-left
	m := newTestWorkerModel()
	m = addTestWorker(m, "worker-1", events.ProcessStatusWorking, "perles-abc.1", events.ProcessPhaseImplementing)
	m.workerPane.workerQueueCounts["worker-1"] = 4

	pane := m.renderSingleWorkerPane("worker-1", 80, 20)
	teatest.RequireEqualOutput(t, []byte(pane))
}
