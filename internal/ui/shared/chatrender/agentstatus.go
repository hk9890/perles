// Package chatrender provides shared chat message rendering for chat-based UIs.
// This file provides helper functions for agent pane rendering shared between
// coordinator_pane.go and worker_pane.go.

package chatrender

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"

	"github.com/zjrosen/perles/internal/orchestration/events"
	"github.com/zjrosen/perles/internal/orchestration/metrics"
	"github.com/zjrosen/perles/internal/ui/styles"
)

// Status indicator styles (shared across all agent types)
var (
	statusReadyStyle = lipgloss.NewStyle().
				Foreground(lipgloss.AdaptiveColor{Light: "#43BF6D", Dark: "#73F59F"}) // Green - ready/available

	statusWorkingStyle = lipgloss.NewStyle().
				Foreground(lipgloss.AdaptiveColor{Light: "#54A0FF", Dark: "#54A0FF"}) // Blue - actively working

	statusPausedStyle = lipgloss.NewStyle().
				Foreground(lipgloss.AdaptiveColor{Light: "#FECA57", Dark: "#FECA57"}). // Yellow/amber - paused
				Bold(true)

	statusStoppedStyle = lipgloss.NewStyle().
				Foreground(lipgloss.AdaptiveColor{Light: "#F0A500", Dark: "#FFD93D"}) // Yellow/amber - stopped (caution)

	statusRetiredStyle = lipgloss.NewStyle().
				Foreground(lipgloss.AdaptiveColor{Light: "#FF6B6B", Dark: "#FF8787"}) // Red - retired/failed

	statusPendingStyle = lipgloss.NewStyle().
				Foreground(styles.TextSecondaryColor) // Muted - pending/starting

	// QueueCountStyle is the style for queue count display.
	// Uses orange color to draw attention to pending queued messages.
	queueCountStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#FFA500", Dark: "#FFB347"})
)

// Border colors for different process statuses (exported for callers that need direct access)
var (
	// StatusWorkingBorderColor is blue for actively working processes.
	StatusWorkingBorderColor = lipgloss.AdaptiveColor{Light: "#54A0FF", Dark: "#54A0FF"}
	// StatusStoppedBorderColor is red for stopped/retired/failed processes.
	StatusStoppedBorderColor = lipgloss.AdaptiveColor{Light: "#FF6B6B", Dark: "#FF8787"}
)

// StatusIndicator returns the indicator character and style for a process status.
// Used to show visual status in pane titles (●/○/⏸/⚠/✗).
//
// NOTE: Coordinator should NOT use this function for title building due to the
// coordinatorWorking flag that toggles Ready/Working visuals independently of ProcessStatus.
// See docs/proposals/agent-pane-extraction-proposal.md for rationale.
func StatusIndicator(status events.ProcessStatus) (string, lipgloss.Style) {
	switch status {
	case events.ProcessStatusReady:
		return "○", statusReadyStyle // Green circle - ready/available
	case events.ProcessStatusWorking:
		return "●", statusWorkingStyle // Blue filled circle - actively working
	case events.ProcessStatusPaused:
		return "⏸", statusPausedStyle // Yellow pause - paused
	case events.ProcessStatusStopped:
		return "⚠", statusStoppedStyle // Yellow caution - stopped (can be resumed)
	case events.ProcessStatusRetired, events.ProcessStatusFailed:
		return "✗", statusRetiredStyle // Red X - retired/failed
	case events.ProcessStatusPending, events.ProcessStatusStarting:
		return "○", statusPendingStyle // Muted circle - pending/starting
	default:
		// Unknown status - use ready style as default (defensive)
		return "?", statusReadyStyle
	}
}

// StatusBorderColor returns the border color for a given process status.
// Blue for working, red for stopped/retired/failed, default otherwise.
func StatusBorderColor(status events.ProcessStatus) lipgloss.AdaptiveColor {
	switch status {
	case events.ProcessStatusWorking:
		return StatusWorkingBorderColor
	case events.ProcessStatusStopped, events.ProcessStatusRetired, events.ProcessStatusFailed:
		return StatusStoppedBorderColor
	default:
		return styles.BorderDefaultColor
	}
}

// FormatQueueCount formats a queue count for display in pane bottom-left.
// Returns empty string if count is 0 or negative.
func FormatQueueCount(count int) string {
	if count <= 0 {
		return ""
	}
	return queueCountStyle.Render(fmt.Sprintf("[%d queued]", count))
}

// FormatMetricsDisplay formats token metrics for display in pane title.
// Returns empty string if metrics is nil or has no tokens used.
func FormatMetricsDisplay(m *metrics.TokenMetrics) string {
	if m == nil || m.TokensUsed <= 0 {
		return ""
	}
	return m.FormatContextDisplay()
}
