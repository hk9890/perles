package orchestration

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"

	"github.com/zjrosen/perles/internal/orchestration/events"
	"github.com/zjrosen/perles/internal/ui/shared/chatrender"
	"github.com/zjrosen/perles/internal/ui/shared/panes"
	"github.com/zjrosen/perles/internal/ui/styles"
)

// Coordinator pane styles
var (
	coordinatorMessageStyle = lipgloss.NewStyle().
		Foreground(CoordinatorColor)
)

// renderCoordinatorPane renders the left pane showing coordinator chat history.
// When fullscreen=true, renders in fullscreen mode with simplified title and no metrics.
func (m Model) renderCoordinatorPane(width, height int, fullscreen bool) string {
	// Get viewport from map (will be modified by helper via pointer)
	vp := m.coordinatorPane.viewports[viewportKey]

	// Build title and metrics based on fullscreen mode
	var leftTitle, metricsDisplay, bottomLeft string
	var hasNewContent bool
	var borderColor lipgloss.AdaptiveColor

	if fullscreen {
		// Fullscreen: simplified title, no metrics or new content indicator
		// Uses hardcoded CoordinatorColor for border - intentionally bypasses StatusBorderColor()
		leftTitle = "● COORDINATOR"
		metricsDisplay = ""
		hasNewContent = false
		borderColor = CoordinatorColor
	} else {
		// Normal: dynamic status title with metrics
		leftTitle = m.buildCoordinatorTitle()
		metricsDisplay = chatrender.FormatMetricsDisplay(m.coordinatorMetrics)
		hasNewContent = m.coordinatorPane.hasNewContent
		// Use shared helper for border color based on status
		borderColor = chatrender.StatusBorderColor(m.coordinatorStatus)
	}

	// Add queue count if any messages are queued (using shared helper)
	bottomLeft = chatrender.FormatQueueCount(m.coordinatorPane.queueCount)

	// Use panes.ScrollablePane helper for viewport setup, padding, and auto-scroll
	result := panes.ScrollablePane(width, height, panes.ScrollableConfig{
		Viewport:       &vp,
		ContentDirty:   m.coordinatorPane.contentDirty,
		HasNewContent:  hasNewContent,
		MetricsDisplay: metricsDisplay,
		LeftTitle:      leftTitle,
		BottomLeft:     bottomLeft,
		TitleColor:     CoordinatorColor,
		BorderColor:    borderColor,
	}, m.renderCoordinatorContent)

	// Store updated viewport back to map (helper modified via pointer)
	m.coordinatorPane.viewports[viewportKey] = vp

	return result
}

// buildCoordinatorTitle builds the left title with status indicator for the coordinator pane.
// When port is available (> 0), it appends the port in muted style: "● COORDINATOR (8467)"
//
// NOTE: Coordinator does not use panes.StatusIndicator() for title building.
// The coordinatorWorking flag toggles Ready/Working visuals independently of ProcessStatus.
// See docs/proposals/agent-pane-extraction-proposal.md for rationale.
func (m Model) buildCoordinatorTitle() string {
	var indicator string
	var indicatorStyle lipgloss.Style

	// Use coordinatorStatus from v2 events instead of calling m.coord.Status()
	switch m.coordinatorStatus {
	case events.ProcessStatusReady, events.ProcessStatusWorking:
		// When ready or working, show indicator based on activity
		if m.coordinatorWorking {
			indicator = "●"
			indicatorStyle = workerWorkingStyle // Blue - actively working
		} else {
			indicator = "○"
			indicatorStyle = workerReadyStyle // Green - ready/waiting for input
		}
	case events.ProcessStatusPaused:
		indicator = "⏸"
		indicatorStyle = statusPausedStyle
	case events.ProcessStatusStopped:
		indicator = "⚠"
		indicatorStyle = workerStoppedStyle // Yellow/amber - stopped (can be resumed)
	case events.ProcessStatusFailed, events.ProcessStatusRetired:
		indicator = "✗"
		indicatorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF6B6B"))
	default:
		// StatusPending, StatusStarting, or no status yet - show empty circle
		indicator = "○"
		indicatorStyle = lipgloss.NewStyle().Foreground(styles.TextSecondaryColor)
	}

	title := fmt.Sprintf("%s COORDINATOR", indicatorStyle.Render(indicator))

	// Append port in muted style if available
	if m.mcpPort > 0 {
		portDisplay := TitleContextStyle.Render(fmt.Sprintf("(%d)", m.mcpPort))
		title = fmt.Sprintf("%s %s", title, portDisplay)
	}

	return title
}

// renderCoordinatorContent builds the pre-wrapped content string for the viewport.
func (m Model) renderCoordinatorContent(wrapWidth int) string {
	return renderChatContent(m.coordinatorPane.messages, wrapWidth, ChatRenderConfig{
		AgentLabel: "Coordinator",
		AgentColor: coordinatorMessageStyle.GetForeground().(lipgloss.AdaptiveColor),
	})
}
