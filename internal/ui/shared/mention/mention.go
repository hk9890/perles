// Package mention provides @mention autocomplete functionality for chat inputs.
package mention

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/hk9890/perles/internal/ui/styles"
)

// Process represents a mentionable process (coordinator or worker).
type Process struct {
	ID   string // Process ID (e.g., "coordinator", "worker-1")
	Role string // Role (e.g., "Coordinator", "Worker")
}

// Model holds the mention autocomplete state.
type Model struct {
	// Available processes that can be mentioned
	processes []Process

	// Current state
	active       bool   // Whether autocomplete is showing
	query        string // Current partial mention query (after @)
	queryStart   int    // Start position of @ in the input
	filtered     []Process
	cursor       int // Selected item in filtered list
	maxVisible   int // Max items to show before scrolling
	scrollOffset int // First visible item index
}

// New creates a new mention autocomplete model.
func New() Model {
	return Model{
		processes:  make([]Process, 0),
		filtered:   make([]Process, 0),
		maxVisible: 5,
	}
}

// SetProcesses updates the list of available processes.
func (m Model) SetProcesses(processes []Process) Model {
	m.processes = processes
	if m.active {
		m = m.updateFilter()
	}
	return m
}

// IsActive returns whether autocomplete is currently showing.
func (m Model) IsActive() bool {
	return m.active
}

// Query returns the current autocomplete query.
func (m Model) Query() string {
	return m.query
}

// ProcessCount returns the number of available processes.
func (m Model) ProcessCount() int {
	return len(m.processes)
}

// ProcessIDs returns the IDs of all available processes.
func (m Model) ProcessIDs() []string {
	ids := make([]string, len(m.processes))
	for i, p := range m.processes {
		ids[i] = p.ID
	}
	return ids
}

// Selected returns the currently selected process, or nil if none.
func (m Model) Selected() *Process {
	if !m.active || len(m.filtered) == 0 || m.cursor >= len(m.filtered) {
		return nil
	}
	return &m.filtered[m.cursor]
}

// Activate starts autocomplete at the given position.
func (m Model) Activate(queryStart int) Model {
	m.active = true
	m.queryStart = queryStart
	m.query = ""
	m.cursor = 0
	m.scrollOffset = 0
	m = m.updateFilter()
	return m
}

// Deactivate closes autocomplete.
func (m Model) Deactivate() Model {
	m.active = false
	m.query = ""
	m.queryStart = 0
	m.cursor = 0
	m.scrollOffset = 0
	m.filtered = nil
	return m
}

// UpdateQuery updates the query and re-filters.
// Returns false if no matches (caller should deactivate).
func (m Model) UpdateQuery(query string) (Model, bool) {
	m.query = query
	m = m.updateFilter()

	// If no matches, signal to close
	if len(m.filtered) == 0 {
		return m, false
	}

	return m, true
}

// updateFilter filters processes based on current query.
func (m Model) updateFilter() Model {
	query := strings.ToLower(m.query)

	m.filtered = make([]Process, 0, len(m.processes))
	for _, p := range m.processes {
		idLower := strings.ToLower(p.ID)
		if query == "" || strings.Contains(idLower, query) {
			m.filtered = append(m.filtered, p)
		}
	}

	// Reset cursor if out of bounds
	if m.cursor >= len(m.filtered) {
		m.cursor = 0
		m.scrollOffset = 0
	}

	return m
}

// Next moves to the next item.
func (m Model) Next() Model {
	if len(m.filtered) == 0 {
		return m
	}
	m.cursor = (m.cursor + 1) % len(m.filtered)
	m = m.ensureVisible()
	return m
}

// Prev moves to the previous item.
func (m Model) Prev() Model {
	if len(m.filtered) == 0 {
		return m
	}
	m.cursor = (m.cursor - 1 + len(m.filtered)) % len(m.filtered)
	m = m.ensureVisible()
	return m
}

// ensureVisible ensures the cursor is visible within the scroll window.
func (m Model) ensureVisible() Model {
	if m.cursor < m.scrollOffset {
		m.scrollOffset = m.cursor
	} else if m.cursor >= m.scrollOffset+m.maxVisible {
		m.scrollOffset = m.cursor - m.maxVisible + 1
	}
	return m
}

// HandleKey processes key events during autocomplete.
// Returns (updated model, consumed bool, selected Process if enter pressed).
func (m Model) HandleKey(msg tea.KeyMsg) (Model, bool, *Process) {
	if !m.active {
		return m, false, nil
	}

	switch msg.String() {
	case "ctrl+n", "down":
		return m.Next(), true, nil
	case "ctrl+p", "up":
		return m.Prev(), true, nil
	case "enter":
		selected := m.Selected()
		if selected != nil {
			return m.Deactivate(), true, selected
		}
		return m, true, nil
	case "esc":
		return m.Deactivate(), true, nil
	case "tab":
		// Tab is handled by parent for channel cycling
		return m, false, nil
	}

	return m, false, nil
}

// View renders the autocomplete popup.
// The width parameter is ignored - the popup auto-sizes to fit content.
func (m Model) View(_ int) string {
	if !m.active || len(m.filtered) == 0 {
		return ""
	}

	// Calculate visible items
	visibleCount := min(m.maxVisible, len(m.filtered))
	endIdx := min(m.scrollOffset+visibleCount, len(m.filtered))

	// Calculate content width based on longest visible item
	// Format: " @process-id" + 1 char padding
	maxLabelWidth := 0
	for i := m.scrollOffset; i < endIdx; i++ {
		labelWidth := len(" @") + len(m.filtered[i].ID) + 1
		if labelWidth > maxLabelWidth {
			maxLabelWidth = labelWidth
		}
	}
	contentWidth := max(maxLabelWidth, 12) // Minimum 12 chars

	// Styles - apply background to entire row for proper highlighting
	normalStyle := lipgloss.NewStyle().
		Foreground(styles.TextPrimaryColor).
		Width(contentWidth)
	selectedStyle := lipgloss.NewStyle().
		Foreground(styles.TextPrimaryColor).
		Background(styles.SelectionBackgroundColor).
		Width(contentWidth)

	// Build items
	var lines []string
	for i := m.scrollOffset; i < endIdx; i++ {
		p := m.filtered[i]

		// Format: @process-id (no role text)
		label := " @" + p.ID

		// Apply selection styling to entire row
		if i == m.cursor {
			lines = append(lines, selectedStyle.Render(label))
		} else {
			lines = append(lines, normalStyle.Render(label))
		}
	}

	// Build popup box
	content := strings.Join(lines, "\n")

	borderStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(styles.BorderDefaultColor)

	return borderStyle.Render(content)
}
