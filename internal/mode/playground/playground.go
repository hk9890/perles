// Package playground provides a simple mode for testing vimtextarea interactively.
package playground

import (
	"strings"

	"github.com/zjrosen/perles/internal/bql"
	"github.com/zjrosen/perles/internal/ui/shared/panes"
	"github.com/zjrosen/perles/internal/ui/shared/vimtextarea"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Model holds the playground state.
type Model struct {
	textarea vimtextarea.Model
	vimMode  vimtextarea.Mode // Track current vim mode for display
	width    int
	height   int
	quitting bool
}

// QuitMsg signals that the playground should exit.
type QuitMsg struct{}

// New creates a new playground model with vim mode enabled.
func New() Model {
	defaultMode := vimtextarea.ModeNormal // Start in Normal mode like vim
	ta := vimtextarea.New(vimtextarea.Config{
		VimEnabled:  true,
		DefaultMode: defaultMode,
		Placeholder: "Start typing... (use vim commands)",
		CharLimit:   0,  // Unlimited
		MaxHeight:   20, // Allow plenty of lines
	})
	ta.Focus()

	// Enable BQL syntax highlighting
	ta.SetLexer(bql.NewSyntaxLexer())

	return Model{
		textarea: ta,
		vimMode:  defaultMode, // Initialize mode tracking
	}
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	return nil
}

// calculateInputHeight returns the height of the input pane based on content.
// Height starts at 4 (2 content lines + 2 borders) and can grow to 6 (4 content + 2 borders).
func (m Model) calculateInputHeight() int {
	lineCount := len(m.textarea.Lines())
	// Height = content lines + 2 for borders, clamped to [4, 6]
	return max(min(lineCount+2, 6), 4)
}

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// Set textarea size - width minus padding, height max 4 lines
		m.textarea.SetSize(msg.Width-4, 4)
		return m, nil

	case tea.KeyMsg:
		// Handle Ctrl+C to quit
		if msg.Type == tea.KeyCtrlC {
			m.quitting = true
			return m, tea.Quit
		}

		// Forward all other keys to the textarea
		var cmd tea.Cmd
		m.textarea, cmd = m.textarea.Update(msg)
		return m, cmd

	case vimtextarea.ModeChangeMsg:
		// Update our tracked mode when vim mode changes
		m.vimMode = msg.Mode
		return m, nil

	case vimtextarea.SubmitMsg:
		// On submit (Shift+Enter), just clear the textarea for demo purposes
		m.textarea.Reset()
		// Keep focus and stay in Insert mode for easy re-entry
		m.textarea.Focus()
		return m, nil
	}

	return m, nil
}

// View implements tea.Model.
func (m Model) View() string {
	if m.quitting {
		return ""
	}

	var sb strings.Builder

	// Header
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("86")).
		MarginBottom(1)

	sb.WriteString(headerStyle.Render("Vim Textarea Playground"))
	sb.WriteString("\n\n")

	// Calculate dimensions for the bordered pane
	paneWidth := max(m.width-2, 20)
	// Dynamic height based on content (min 4, max 6)
	paneHeight := m.calculateInputHeight()

	// Render textarea inside BorderedPane with mode in bottom-left
	textareaView := m.textarea.View()

	borderedPane := panes.BorderedPane(panes.BorderConfig{
		Content:    textareaView,
		Width:      paneWidth,
		Height:     paneHeight,
		BottomLeft: m.textarea.ModeIndicator(), // Styled by component
		Focused:    true,
	})

	sb.WriteString(borderedPane)
	sb.WriteString("\n\n")

	// Help text
	helpStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241"))

	var helpText string
	if m.vimMode == vimtextarea.ModeNormal {
		helpText = "NORMAL: i=insert  a=append  o=line below  hjkl=move  w/b=word  dd=delete line  u=undo  Ctrl+R=redo"
	} else {
		helpText = "INSERT: type normally  ESC=normal mode  Shift+Enter=submit  Enter=newline  Backspace=delete"
	}

	sb.WriteString(helpStyle.Render(helpText))
	sb.WriteString("\n")

	// Footer
	footerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).
		MarginTop(1)

	pos := m.textarea.CursorPosition()
	footer := lipgloss.JoinHorizontal(
		lipgloss.Left,
		footerStyle.Render("Ctrl+C to quit"),
		footerStyle.Render("  │  "),
		footerStyle.Render("Line: "+itoa(pos.Row+1)+", Col: "+itoa(pos.Col+1)),
	)
	sb.WriteString(footer)

	return sb.String()
}

// itoa converts an int to a string without importing strconv.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	if n < 0 {
		return "-" + itoa(-n)
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}
