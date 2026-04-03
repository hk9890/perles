// Package nobeads provides the empty state view shown when no .beads directory exists.
package nobeads

import (
	"strings"

	"github.com/hk9890/perles/internal/keys"
	"github.com/hk9890/perles/internal/ui/shared/chainart"
	"github.com/hk9890/perles/internal/ui/styles"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Model holds the nobeads view state.
type Model struct {
	width      int
	height     int
	problem    string
	suggestion string
}

// New creates a new nobeads view.
func New(problem, suggestion string) Model {
	return Model{problem: problem, suggestion: suggestion}
}

// Init returns the initial command.
func (m Model) Init() tea.Cmd {
	return nil
}

// Update handles messages.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, keys.Common.Quit), key.Matches(msg, keys.Common.Escape):
			return m, tea.Quit
		}
	}
	return m, nil
}

// View renders the empty state.
func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}

	art := chainart.BuildChainArt()

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(styles.TextPrimaryColor).
		MarginTop(1)

	messageStyle := lipgloss.NewStyle().
		Foreground(styles.TextDescriptionColor)

	hintStyle := lipgloss.NewStyle().
		Foreground(styles.TextMutedColor).
		Italic(true).
		MarginTop(2)

	// Build content
	var content strings.Builder

	content.WriteString(art)
	content.WriteString("\n\n")
	content.WriteString(titleStyle.Render("Oh no! Looks like there's a break in the chain!"))
	content.WriteString("\n\n")
	content.WriteString(messageStyle.Render("Perles could not discover a supported beads runtime from this project."))
	content.WriteString("\n\n")
	if strings.TrimSpace(m.problem) != "" {
		content.WriteString(messageStyle.Render("Detected issue: " + m.problem))
		content.WriteString("\n\n")
	}
	content.WriteString(messageStyle.Render("Perles support policy: beads v1+ with backend=dolt and dolt_mode=server."))
	content.WriteString("\n\n")
	content.WriteString(messageStyle.Render("Try one of these options:"))
	content.WriteString("\n\n")
	content.WriteString(messageStyle.Render("  1. (Recommended) Run 'bd bootstrap' in this project"))
	content.WriteString("\n")
	content.WriteString(messageStyle.Render("  2. Ensure metadata.json has backend=dolt and dolt_mode=server"))
	content.WriteString("\n")
	content.WriteString(messageStyle.Render("  3. If the project is embedded/shared mode, switch it to server mode"))
	content.WriteString("\n")
	content.WriteString(messageStyle.Render("  4. Then retry Perles from this repository root"))
	if strings.TrimSpace(m.suggestion) != "" {
		content.WriteString("\n\n")
		content.WriteString(messageStyle.Render("Suggested next step: " + m.suggestion))
	}
	content.WriteString("\n\n")
	content.WriteString(hintStyle.Render("Press q to quit"))

	// Center the content
	containerStyle := lipgloss.NewStyle().
		Width(m.width).
		Height(m.height).
		Align(lipgloss.Center, lipgloss.Center)

	return containerStyle.Render(content.String())
}

// SetSize updates the view dimensions.
func (m Model) SetSize(width, height int) Model {
	m.width = width
	m.height = height
	return m
}
