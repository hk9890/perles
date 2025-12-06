package editissue

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"
	"github.com/stretchr/testify/require"
)

func TestEditMenu_New(t *testing.T) {
	m := New()

	require.Equal(t, OptionLabels, m.selected, "expected default selection at OptionLabels")
}

func TestEditMenu_SetSize(t *testing.T) {
	m := New()

	m = m.SetSize(120, 40)
	require.Equal(t, 120, m.viewportWidth, "expected viewport width to be 120")
	require.Equal(t, 40, m.viewportHeight, "expected viewport height to be 40")

	// Verify immutability
	m2 := m.SetSize(80, 24)
	require.Equal(t, 80, m2.viewportWidth, "expected new model width to be 80")
	require.Equal(t, 24, m2.viewportHeight, "expected new model height to be 24")
	require.Equal(t, 120, m.viewportWidth, "expected original model width unchanged")
}

func TestEditMenu_Selected(t *testing.T) {
	m := New()
	require.Equal(t, OptionLabels, m.Selected(), "expected OptionLabels selected by default")
}

func TestEditMenu_Update_NavigateDown_J(t *testing.T) {
	m := New()

	// Navigate down with 'j'
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	require.Equal(t, OptionPriority, m.selected, "expected OptionPriority after 'j'")

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	require.Equal(t, OptionStatus, m.selected, "expected OptionStatus after second 'j'")

	// At bottom boundary - should not go past
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	require.Equal(t, OptionStatus, m.selected, "expected selection to stay at OptionStatus (boundary)")
}

func TestEditMenu_Update_NavigateDown_Arrow(t *testing.T) {
	m := New()

	// Navigate down with arrow key
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	require.Equal(t, OptionPriority, m.selected, "expected OptionPriority after down arrow")

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	require.Equal(t, OptionStatus, m.selected, "expected OptionStatus after second down arrow")
}

func TestEditMenu_Update_NavigateDown_CtrlN(t *testing.T) {
	m := New()

	// Navigate down with ctrl+n
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlN})
	require.Equal(t, OptionPriority, m.selected, "expected OptionPriority after ctrl+n")
}

func TestEditMenu_Update_NavigateUp_K(t *testing.T) {
	m := New()
	// Start at bottom
	m.selected = OptionStatus

	// Navigate up with 'k'
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	require.Equal(t, OptionPriority, m.selected, "expected OptionPriority after 'k'")

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	require.Equal(t, OptionLabels, m.selected, "expected OptionLabels after second 'k'")

	// At top boundary - should not go past
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	require.Equal(t, OptionLabels, m.selected, "expected selection to stay at OptionLabels (boundary)")
}

func TestEditMenu_Update_NavigateUp_Arrow(t *testing.T) {
	m := New()
	m.selected = OptionStatus

	// Navigate up with arrow key
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	require.Equal(t, OptionPriority, m.selected, "expected OptionPriority after up arrow")

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	require.Equal(t, OptionLabels, m.selected, "expected OptionLabels after second up arrow")
}

func TestEditMenu_Update_NavigateUp_CtrlP(t *testing.T) {
	m := New()
	m.selected = OptionPriority

	// Navigate up with ctrl+p
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlP})
	require.Equal(t, OptionLabels, m.selected, "expected OptionLabels after ctrl+p")
}

func TestEditMenu_Update_Enter_EmitsSelectMsg(t *testing.T) {
	tests := []struct {
		name     string
		selected Option
	}{
		{"labels", OptionLabels},
		{"priority", OptionPriority},
		{"status", OptionStatus},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := New()
			m.selected = tt.selected

			_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
			require.NotNil(t, cmd, "expected command from Enter")

			msg := cmd()
			selectMsg, ok := msg.(SelectMsg)
			require.True(t, ok, "expected SelectMsg")
			require.Equal(t, tt.selected, selectMsg.Option, "expected correct option in SelectMsg")
		})
	}
}

func TestEditMenu_Update_Esc_EmitsCancelMsg(t *testing.T) {
	m := New()

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	require.NotNil(t, cmd, "expected command from Esc")

	msg := cmd()
	_, ok := msg.(CancelMsg)
	require.True(t, ok, "expected CancelMsg from Esc")
}

func TestEditMenu_View(t *testing.T) {
	m := New().SetSize(80, 24)
	view := m.View()

	// Should contain title
	require.Contains(t, view, "Edit Issue", "expected view to contain title")

	// Should contain options
	require.Contains(t, view, "Edit labels", "expected view to contain Labels option")
	require.Contains(t, view, "Change priority", "expected view to contain Priority option")
	require.Contains(t, view, "Change status", "expected view to contain Status option")

	// Should have selection indicator
	require.Contains(t, view, ">", "expected view to contain selection indicator")
}

func TestEditMenu_View_Stability(t *testing.T) {
	m := New().SetSize(80, 24)

	view1 := m.View()
	view2 := m.View()

	// Same model should produce identical output
	require.Equal(t, view1, view2, "expected stable output from same model")
}

// TestEditMenu_View_Golden uses teatest golden file comparison
// Run with -update flag to update golden files: go test -update ./internal/ui/editmenu/...
func TestEditMenu_View_Golden(t *testing.T) {
	m := New().SetSize(80, 24)
	view := m.View()
	teatest.RequireEqualOutput(t, []byte(view))
}

// TestEditMenu_View_PrioritySelected_Golden tests menu with priority selected
func TestEditMenu_View_PrioritySelected_Golden(t *testing.T) {
	m := New().SetSize(80, 24)
	m.selected = OptionPriority
	view := m.View()
	teatest.RequireEqualOutput(t, []byte(view))
}

// TestEditMenu_View_StatusSelected_Golden tests menu with status selected
func TestEditMenu_View_StatusSelected_Golden(t *testing.T) {
	m := New().SetSize(80, 24)
	m.selected = OptionStatus
	view := m.View()
	teatest.RequireEqualOutput(t, []byte(view))
}
