package modal

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"
	"github.com/stretchr/testify/require"
)

func TestNew_InputMode(t *testing.T) {
	cfg := Config{
		Title: "Test Modal",
		Inputs: []InputConfig{
			{Key: "name", Label: "Name", Placeholder: "Enter something..."},
		},
	}

	m := New(cfg)

	require.True(t, m.hasInputs, "expected hasInputs to be true when Inputs is set")
	require.Len(t, m.inputs, 1)
	require.Equal(t, cfg.Inputs[0].Placeholder, m.inputs[0].Placeholder)
}

func TestNew_ConfirmMode(t *testing.T) {
	cfg := Config{
		Title:   "Confirm Delete",
		Message: "Are you sure?",
		// No Inputs = confirmation mode
	}

	m := New(cfg)

	require.False(t, m.hasInputs, "expected hasInputs to be false when Inputs is empty")
	require.Equal(t, -1, m.focusedInput, "expected focusedInput -1 for confirm mode")
	require.Equal(t, FieldSave, m.focusedField, "expected focusedField FieldSave for confirm mode")
}

func TestNew_WithInitialValue(t *testing.T) {
	cfg := Config{
		Title: "Edit Name",
		Inputs: []InputConfig{
			{Key: "name", Label: "Name", Placeholder: "Enter name...", Value: "initial value"},
		},
	}

	m := New(cfg)

	require.Equal(t, cfg.Inputs[0].Value, m.inputs[0].Value())
}

func TestNew_WithMaxLength(t *testing.T) {
	cfg := Config{
		Title: "Short Input",
		Inputs: []InputConfig{
			{Key: "name", Label: "Name", Placeholder: "Enter...", MaxLength: 10},
		},
	}

	m := New(cfg)

	require.Equal(t, cfg.Inputs[0].MaxLength, m.inputs[0].CharLimit)
}

func TestNew_MultipleInputs(t *testing.T) {
	cfg := Config{
		Title: "Multiple Inputs",
		Inputs: []InputConfig{
			{Key: "first", Label: "First", Placeholder: "First..."},
			{Key: "second", Label: "Second", Placeholder: "Second..."},
			{Key: "third", Label: "Third", Placeholder: "Third..."},
		},
	}

	m := New(cfg)

	require.Len(t, m.inputs, 3)
	require.Equal(t, "first", m.inputKeys[0])
	require.Equal(t, "second", m.inputKeys[1])
	require.Equal(t, "third", m.inputKeys[2])
}

func TestInit_InputMode(t *testing.T) {
	m := New(Config{
		Title: "Test",
		Inputs: []InputConfig{
			{Key: "name", Label: "Name", Placeholder: "Enter..."},
		},
	})

	cmd := m.Init()
	require.NotNil(t, cmd, "expected Init() to return textinput.Blink command for input mode")
}

func TestInit_ConfirmMode(t *testing.T) {
	m := New(Config{
		Title: "Confirm",
	})

	cmd := m.Init()
	require.Nil(t, cmd, "expected Init() to return nil for confirmation mode")
}

func TestUpdate_Submit(t *testing.T) {
	m := New(Config{
		Title: "Test",
		Inputs: []InputConfig{
			{Key: "name", Label: "Name", Placeholder: "Enter...", Value: "my value"},
		},
	})

	// Navigate to Save button
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	require.Equal(t, -1, m.focusedInput, "expected focus on Save button")
	require.Equal(t, FieldSave, m.focusedField, "expected focus on Save button")

	// Press enter on Save
	enterMsg := tea.KeyMsg{Type: tea.KeyEnter}
	m, cmd := m.Update(enterMsg)

	require.NotNil(t, cmd, "expected command from Enter key on Save")

	msg := cmd()
	submitMsg, ok := msg.(SubmitMsg)
	require.True(t, ok, "expected SubmitMsg, got %T", msg)
	require.Equal(t, "my value", submitMsg.Values["name"])
}

func TestUpdate_Cancel(t *testing.T) {
	m := New(Config{
		Title: "Test",
		Inputs: []InputConfig{
			{Key: "name", Label: "Name", Placeholder: "Enter..."},
		},
	})

	escMsg := tea.KeyMsg{Type: tea.KeyEscape}
	_, cmd := m.Update(escMsg)

	require.NotNil(t, cmd, "expected command from Esc key")

	msg := cmd()
	_, ok := msg.(CancelMsg)
	require.True(t, ok, "expected CancelMsg, got %T", msg)
}

func TestUpdate_CancelButton(t *testing.T) {
	m := New(Config{
		Title: "Test",
		Inputs: []InputConfig{
			{Key: "name", Label: "Name", Placeholder: "Enter..."},
		},
	})

	// Navigate to Cancel button (tab to Save, then right to Cancel)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})

	require.Equal(t, FieldCancel, m.focusedField, "expected focus on Cancel")

	// Press enter on Cancel
	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	require.NotNil(t, cmd, "expected command from Enter on Cancel")

	msg := cmd()
	_, ok := msg.(CancelMsg)
	require.True(t, ok, "expected CancelMsg, got %T", msg)
}

func TestUpdate_EmptySubmit(t *testing.T) {
	// In input mode, Save with empty input should NOT submit
	m := New(Config{
		Title: "Test",
		Inputs: []InputConfig{
			{Key: "name", Label: "Name", Placeholder: "Enter..."},
		},
	})

	// Navigate to Save button
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})

	// Press enter - should not submit because input is empty
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if cmd != nil {
		msg := cmd()
		_, ok := msg.(SubmitMsg)
		require.False(t, ok, "expected no SubmitMsg when input is empty in input mode")
	}
}

func TestUpdate_ConfirmSubmit(t *testing.T) {
	// In confirmation mode, Enter submits immediately (no input required)
	m := New(Config{
		Title:   "Confirm Delete",
		Message: "Are you sure?",
	})

	// Should start on Save button
	require.Equal(t, -1, m.focusedInput, "expected focus on Save")
	require.Equal(t, FieldSave, m.focusedField, "expected focus on Save")

	enterMsg := tea.KeyMsg{Type: tea.KeyEnter}
	m, cmd := m.Update(enterMsg)

	require.NotNil(t, cmd, "expected command from Enter key in confirmation mode")

	msg := cmd()
	submitMsg, ok := msg.(SubmitMsg)
	require.True(t, ok, "expected SubmitMsg, got %T", msg)
	require.Empty(t, submitMsg.Values, "expected empty values for confirmation mode")
}

func TestUpdate_WindowSizeMsg(t *testing.T) {
	m := New(Config{
		Title: "Test",
	})

	sizeMsg := tea.WindowSizeMsg{Width: 100, Height: 50}
	m, _ = m.Update(sizeMsg)

	require.Equal(t, 100, m.width)
	require.Equal(t, 50, m.height)
}

func TestUpdate_Navigation(t *testing.T) {
	m := New(Config{
		Title: "Test",
		Inputs: []InputConfig{
			{Key: "first", Label: "First", Placeholder: "First..."},
			{Key: "second", Label: "Second", Placeholder: "Second..."},
		},
	})

	// Should start on first input
	require.Equal(t, 0, m.focusedInput)

	// Tab to second input
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	require.Equal(t, 1, m.focusedInput, "expected focusedInput 1 after tab")

	// Tab to Save button
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	require.Equal(t, -1, m.focusedInput, "expected Save button focus")
	require.Equal(t, FieldSave, m.focusedField, "expected Save button focus")

	// Tab to Cancel button
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	require.Equal(t, FieldCancel, m.focusedField, "expected Cancel button focus")

	// Tab wraps to first input
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	require.Equal(t, 0, m.focusedInput, "expected wrap to first input")
}

func TestUpdate_NavigationReverse(t *testing.T) {
	m := New(Config{
		Title: "Test",
		Inputs: []InputConfig{
			{Key: "name", Label: "Name", Placeholder: "Name..."},
		},
	})

	// Start on input, shift+tab should go to Cancel
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	require.Equal(t, FieldCancel, m.focusedField, "expected Cancel from shift+tab")

	// Shift+tab to Save
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	require.Equal(t, FieldSave, m.focusedField, "expected Save from shift+tab")

	// Shift+tab wraps to input
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	require.Equal(t, 0, m.focusedInput, "expected wrap to input")
}

func TestUpdate_HorizontalNavigation(t *testing.T) {
	m := New(Config{
		Title: "Test",
	})

	// Confirm mode starts on Save
	require.Equal(t, FieldSave, m.focusedField, "expected Save focus")

	// Right to Cancel
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
	require.Equal(t, FieldCancel, m.focusedField, "expected Cancel after right")

	// Left back to Save
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	require.Equal(t, FieldSave, m.focusedField, "expected Save after left")
}

func TestView_InputMode(t *testing.T) {
	m := New(Config{
		Title: "New View",
		Inputs: []InputConfig{
			{Key: "name", Label: "View Name", Placeholder: "Enter view name..."},
		},
	})

	view := m.View()

	// Should contain title
	require.True(t, containsString(view, "New View"), "expected view to contain title")

	// Should contain input label
	require.True(t, containsString(view, "View Name"), "expected view to contain input label")

	// Should contain Save button
	require.True(t, containsString(view, "Save"), "expected view to contain 'Save' button")

	// Should contain Cancel button
	require.True(t, containsString(view, "Cancel"), "expected view to contain 'Cancel' button")
}

func TestView_ConfirmMode(t *testing.T) {
	m := New(Config{
		Title:   "Delete View",
		Message: "Are you sure you want to delete?",
	})

	view := m.View()

	// Should contain title
	require.True(t, containsString(view, "Delete View"), "expected view to contain title")

	// Should contain message
	require.True(t, containsString(view, "Are you sure"), "expected view to contain message")

	// Should contain Confirm button
	require.True(t, containsString(view, "Confirm"), "expected view to contain 'Confirm' button")

	// Should contain Cancel button
	require.True(t, containsString(view, "Cancel"), "expected view to contain 'Cancel' button")
}

func TestSetSize(t *testing.T) {
	m := New(Config{Title: "Test"})

	m.SetSize(200, 100)

	require.Equal(t, 200, m.width)
	require.Equal(t, 100, m.height)
}

func TestOverlay(t *testing.T) {
	m := New(Config{
		Title: "Test Modal",
	})
	m.SetSize(80, 24)

	// Create a simple background
	bg := ""
	for i := 0; i < 24; i++ {
		for j := 0; j < 80; j++ {
			bg += "."
		}
		if i < 23 {
			bg += "\n"
		}
	}

	result := m.Overlay(bg)

	// Result should contain modal content
	require.True(t, containsString(result, "Test Modal"), "expected overlay to contain modal content")

	// Result should still have some background dots
	require.True(t, containsString(result, "..."), "expected overlay to preserve some background")
}

// containsString checks if s contains substr, ignoring ANSI escape sequences
func containsString(s, substr string) bool {
	// Simple check - could be improved to strip ANSI codes
	return len(s) > 0 && len(substr) > 0 && (s == substr || len(s) > len(substr)) && findSubstring(s, substr)
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	// Also check by iterating through runes (in case of ANSI sequences)
	return stringContains(s, substr)
}

func stringContains(s, substr string) bool {
	// Brute force search that handles ANSI codes better
	subRunes := []rune(substr)
	sRunes := []rune(s)

	for i := 0; i <= len(sRunes)-len(subRunes); i++ {
		match := true
		for j := 0; j < len(subRunes); j++ {
			if sRunes[i+j] != subRunes[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

// Golden tests for modal rendering

func TestView_InputMode_Golden(t *testing.T) {
	m := New(Config{
		Title: "New View",
		Inputs: []InputConfig{
			{Key: "name", Label: "View Name", Placeholder: "Enter view name..."},
		},
	})
	teatest.RequireEqualOutput(t, []byte(m.View()))
}

func TestView_ConfirmMode_Golden(t *testing.T) {
	m := New(Config{
		Title:   "Delete View",
		Message: "Are you sure you want to delete?",
	})
	teatest.RequireEqualOutput(t, []byte(m.View()))
}

func TestView_MultipleInputs_Golden(t *testing.T) {
	m := New(Config{
		Title: "Create Item",
		Inputs: []InputConfig{
			{Key: "name", Label: "Name", Placeholder: "Enter name..."},
			{Key: "color", Label: "Color", Placeholder: "#RRGGBB"},
		},
	})
	teatest.RequireEqualOutput(t, []byte(m.View()))
}
