package orchestration

import (
	"testing"

	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/x/exp/teatest"
	"github.com/stretchr/testify/require"

	"github.com/zjrosen/perles/internal/orchestration/events"
	"github.com/zjrosen/perles/internal/orchestration/metrics"
)

func TestBuildCoordinatorTitle_WithPort(t *testing.T) {
	m := New(Config{})
	m.mcpPort = 8467

	title := m.buildCoordinatorTitle()

	// Should contain port in muted style
	require.Contains(t, title, "COORDINATOR")
	require.Contains(t, title, "(8467)")
}

func TestBuildCoordinatorTitle_NoPort(t *testing.T) {
	m := New(Config{})
	m.mcpPort = 0

	title := m.buildCoordinatorTitle()

	// Should NOT contain port when port is 0
	require.Contains(t, title, "COORDINATOR")
	require.NotContains(t, title, "(")
	require.NotContains(t, title, ")")
}

func TestBuildCoordinatorTitle_MutedStyleApplied(t *testing.T) {
	m := New(Config{})
	m.mcpPort = 12345

	title := m.buildCoordinatorTitle()

	// The port should be styled with TitleContextStyle (muted).
	// Since Lipgloss applies ANSI escape codes, we just verify the structure
	// contains both COORDINATOR and the port number.
	require.Contains(t, title, "COORDINATOR")
	require.Contains(t, title, "12345")
}

// ============================================================================
// Queue Count Display Tests
// ============================================================================

func TestCoordinatorPane_QueueCountDisplay(t *testing.T) {
	// Create model with queue count
	m := New(Config{})
	m.coordinatorPane.viewports = make(map[string]viewport.Model)
	m.coordinatorPane.queueCount = 3

	// Render the pane
	pane := m.renderCoordinatorPane(80, 20, false)

	// Should contain the queue count indicator
	require.Contains(t, pane, "[3 queued]")
}

func TestCoordinatorPane_NoQueueDisplay(t *testing.T) {
	// Create model with zero queue count
	m := New(Config{})
	m.coordinatorPane.viewports = make(map[string]viewport.Model)
	m.coordinatorPane.queueCount = 0

	// Render the pane
	pane := m.renderCoordinatorPane(80, 20, false)

	// Should NOT contain the queue indicator
	require.NotContains(t, pane, "queued")
}

func TestCoordinatorPane_QueueCountUpdates(t *testing.T) {
	// Create model
	m := New(Config{})

	// Initial state should have zero queue count
	require.Equal(t, 0, m.coordinatorPane.queueCount)

	// Simulate queue count update
	m.coordinatorPane.queueCount = 5
	require.Equal(t, 5, m.coordinatorPane.queueCount)

	// Update to different count
	m.coordinatorPane.queueCount = 2
	require.Equal(t, 2, m.coordinatorPane.queueCount)

	// Reset to zero
	m.coordinatorPane.queueCount = 0
	require.Equal(t, 0, m.coordinatorPane.queueCount)
}

func TestCoordinatorPane_FullscreenHidesQueueCount(t *testing.T) {
	// Create model with queue count
	m := New(Config{})
	m.coordinatorPane.viewports = make(map[string]viewport.Model)
	m.coordinatorPane.queueCount = 5

	// Render in fullscreen mode
	pane := m.renderCoordinatorPane(80, 20, true)

	// Fullscreen mode should still show queue count (it's bottom-left, not metrics)
	require.Contains(t, pane, "[5 queued]")
}

// ============================================================================
// Golden Tests for Coordinator Pane
// ============================================================================
//
// These tests capture the visual output of the coordinator pane in various states.
// They serve as baseline snapshots before refactoring to detect visual regressions.
//
// To update golden files: go test -update ./internal/mode/orchestration/...

// newTestCoordinatorModel creates a model configured for coordinator pane golden tests.
// It sets up the minimal state needed to render the coordinator pane in isolation.
func newTestCoordinatorModel(status events.ProcessStatus, working bool) Model {
	m := New(Config{})
	m = m.SetSize(80, 24) // Standard terminal size for consistent output
	m.coordinatorStatus = status
	m.coordinatorWorking = working
	m.mcpPort = 8467 // Standard MCP port for display

	// Ensure viewports are initialized
	m.coordinatorPane.viewports = make(map[string]viewport.Model)
	m.coordinatorPane.viewports[viewportKey] = viewport.New(0, 0)

	// Add some sample chat content for realistic rendering
	m.coordinatorPane.messages = []ChatMessage{
		{Role: "assistant", Content: "I'll help you implement this feature."},
		{Role: "user", Content: "Please start with the tests."},
	}
	m.coordinatorPane.contentDirty = true

	return m
}

func TestCoordinatorPane_Golden_ReadyStatus(t *testing.T) {
	// Ready status with coordinatorWorking=false shows green empty circle ○
	m := newTestCoordinatorModel(events.ProcessStatusReady, false)

	pane := m.renderCoordinatorPane(80, 20, false)
	teatest.RequireEqualOutput(t, []byte(pane))
}

func TestCoordinatorPane_Golden_ReadyStatusWorking(t *testing.T) {
	// Ready status with coordinatorWorking=true shows blue filled circle ●
	// This captures the nuanced behavior where coordinatorWorking flag
	// toggles Ready/Working visuals independently of ProcessStatus.
	m := newTestCoordinatorModel(events.ProcessStatusReady, true)

	pane := m.renderCoordinatorPane(80, 20, false)
	teatest.RequireEqualOutput(t, []byte(pane))
}

func TestCoordinatorPane_Golden_WorkingStatus(t *testing.T) {
	// Working status shows blue filled circle ● and blue border
	m := newTestCoordinatorModel(events.ProcessStatusWorking, true)

	pane := m.renderCoordinatorPane(80, 20, false)
	teatest.RequireEqualOutput(t, []byte(pane))
}

func TestCoordinatorPane_Golden_PausedStatus(t *testing.T) {
	// Paused status shows ⏸ indicator
	m := newTestCoordinatorModel(events.ProcessStatusPaused, false)

	pane := m.renderCoordinatorPane(80, 20, false)
	teatest.RequireEqualOutput(t, []byte(pane))
}

func TestCoordinatorPane_Golden_StoppedStatus(t *testing.T) {
	// Stopped status shows ⚠ indicator and red border
	m := newTestCoordinatorModel(events.ProcessStatusStopped, false)

	pane := m.renderCoordinatorPane(80, 20, false)
	teatest.RequireEqualOutput(t, []byte(pane))
}

func TestCoordinatorPane_Golden_FailedStatus(t *testing.T) {
	// Failed status shows ✗ indicator and red border
	m := newTestCoordinatorModel(events.ProcessStatusFailed, false)

	pane := m.renderCoordinatorPane(80, 20, false)
	teatest.RequireEqualOutput(t, []byte(pane))
}

func TestCoordinatorPane_Golden_RetiredStatus(t *testing.T) {
	// Retired status shows ✗ indicator and red border (same as Failed)
	m := newTestCoordinatorModel(events.ProcessStatusRetired, false)

	pane := m.renderCoordinatorPane(80, 20, false)
	teatest.RequireEqualOutput(t, []byte(pane))
}

func TestCoordinatorPane_Golden_Fullscreen(t *testing.T) {
	// Fullscreen mode shows simplified title "● COORDINATOR" with no metrics,
	// and hardcoded CoordinatorColor for border (not status-based)
	m := newTestCoordinatorModel(events.ProcessStatusWorking, true)
	m.coordinatorPane.queueCount = 3 // Queue count should still show in fullscreen

	pane := m.renderCoordinatorPane(100, 30, true) // Larger size for fullscreen
	teatest.RequireEqualOutput(t, []byte(pane))
}

func TestCoordinatorPane_Golden_WithMetrics(t *testing.T) {
	// Coordinator pane with token metrics displayed in title right area
	m := newTestCoordinatorModel(events.ProcessStatusReady, false)
	m.coordinatorMetrics = &metrics.TokenMetrics{
		TokensUsed:   15000,
		TotalTokens:  200000,
		OutputTokens: 500,
		TotalCostUSD: 0.15,
	}

	pane := m.renderCoordinatorPane(80, 20, false)
	teatest.RequireEqualOutput(t, []byte(pane))
}

func TestCoordinatorPane_Golden_WithQueueCount(t *testing.T) {
	// Coordinator pane with queue count displayed in bottom-left
	m := newTestCoordinatorModel(events.ProcessStatusReady, false)
	m.coordinatorPane.queueCount = 5

	pane := m.renderCoordinatorPane(80, 20, false)
	teatest.RequireEqualOutput(t, []byte(pane))
}
