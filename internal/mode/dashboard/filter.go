package dashboard

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/zjrosen/perles/internal/orchestration/controlplane"
	"github.com/zjrosen/perles/internal/ui/styles"
)

// FilterState manages the filter/search state for the dashboard.
type FilterState struct {
	textInput   textinput.Model
	active      bool                       // Whether filter input is active (focused)
	filterText  string                     // Current filter text
	stateFilter controlplane.WorkflowState // Filter by state (empty = all)
}

// NewFilterState creates a new filter state.
func NewFilterState() FilterState {
	ti := textinput.New()
	ti.Placeholder = "Filter workflows..."
	ti.Prompt = " "
	ti.CharLimit = 50
	ti.Width = 30

	return FilterState{
		textInput: ti,
		active:    false,
	}
}

// Activate activates the filter input.
func (f FilterState) Activate() FilterState {
	f.active = true
	f.textInput.Focus()
	return f
}

// Deactivate deactivates the filter input.
func (f FilterState) Deactivate() FilterState {
	f.active = false
	f.textInput.Blur()
	return f
}

// Clear clears the filter and deactivates it.
func (f FilterState) Clear() FilterState {
	f.active = false
	f.filterText = ""
	f.stateFilter = ""
	f.textInput.SetValue("")
	f.textInput.Blur()
	return f
}

// IsActive returns true if the filter input is active.
func (f FilterState) IsActive() bool {
	return f.active
}

// HasFilter returns true if there is an active filter.
func (f FilterState) HasFilter() bool {
	return f.filterText != "" || f.stateFilter != ""
}

// FilterText returns the current filter text.
func (f FilterState) FilterText() string {
	return f.filterText
}

// StateFilter returns the current state filter.
func (f FilterState) StateFilter() controlplane.WorkflowState {
	return f.stateFilter
}

// SetStateFilter sets the state filter.
func (f FilterState) SetStateFilter(state controlplane.WorkflowState) FilterState {
	f.stateFilter = state
	return f
}

// Update handles key messages when filter is active.
func (f FilterState) Update(msg tea.Msg) (FilterState, tea.Cmd) {
	if !f.active {
		return f, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEsc:
			// Escape clears filter and deactivates
			return f.Clear(), nil
		case tea.KeyEnter:
			// Enter confirms filter and deactivates input but keeps filter
			f.filterText = f.textInput.Value()
			f.active = false
			f.textInput.Blur()
			return f, nil
		}
	}

	// Forward to text input
	var cmd tea.Cmd
	f.textInput, cmd = f.textInput.Update(msg)
	f.filterText = f.textInput.Value()
	return f, cmd
}

// FilterWorkflows filters the workflow list based on current filter criteria.
func (f FilterState) FilterWorkflows(workflows []*controlplane.WorkflowInstance) []*controlplane.WorkflowInstance {
	if !f.HasFilter() {
		return workflows
	}

	var result []*controlplane.WorkflowInstance
	filterText := strings.ToLower(f.filterText)

	for _, wf := range workflows {
		// Check state filter
		if f.stateFilter != "" && wf.State != f.stateFilter {
			continue
		}

		// Check text filter (matches name)
		if filterText != "" {
			nameLower := strings.ToLower(wf.Name)
			if !strings.Contains(nameLower, filterText) {
				continue
			}
		}

		result = append(result, wf)
	}

	return result
}

// View renders the filter input bar.
func (f FilterState) View() string {
	if !f.active && !f.HasFilter() {
		return ""
	}

	var content strings.Builder

	filterIcon := lipgloss.NewStyle().Foreground(colorDimmed).Render(" ")

	if f.active {
		// Show active input
		content.WriteString(filterIcon)
		content.WriteString(f.textInput.View())
	} else if f.HasFilter() {
		// Show filter indicator
		filterStyle := lipgloss.NewStyle().
			Foreground(styles.BorderHighlightFocusColor).
			Italic(true)

		filterDesc := ""
		if f.filterText != "" {
			filterDesc = "\"" + f.filterText + "\""
		}
		if f.stateFilter != "" {
			if filterDesc != "" {
				filterDesc += " "
			}
			filterDesc += "[" + string(f.stateFilter) + "]"
		}

		content.WriteString(filterIcon)
		content.WriteString(filterStyle.Render("Filter: " + filterDesc))
		content.WriteString(lipgloss.NewStyle().Foreground(colorDimmed).Render(" (Esc to clear)"))
	}

	return content.String()
}

// Init returns the initial command for the filter (cursor blink).
func (f FilterState) Init() tea.Cmd {
	if f.active {
		return textinput.Blink
	}
	return nil
}
