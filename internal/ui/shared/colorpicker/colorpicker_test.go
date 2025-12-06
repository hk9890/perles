package colorpicker

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	m := New()

	require.Len(t, m.columns, 4)
	require.Len(t, m.columns[0], 10, "presets in column 1")
	require.Equal(t, 0, m.column)
	require.Equal(t, 0, m.selected)
	require.True(t, m.customEnabled)
	require.False(t, m.inCustomMode)
}

func TestDefaultPresets(t *testing.T) {
	expected := []struct {
		name string
		hex  string
	}{
		{"Red", "#FF8787"},
		{"Green", "#73F59F"},
		{"Blue", "#54A0FF"},
		{"Purple", "#7D56F4"},
		{"Yellow", "#FECA57"},
		{"Orange", "#FF9F43"},
		{"Teal", "#89DCEB"},
		{"Gray", "#BBBBBB"},
		{"Pink", "#CBA6F7"},
		{"Coral", "#FF6B6B"},
	}

	require.Len(t, DefaultPresets, len(expected))

	for i, exp := range expected {
		require.Equal(t, exp.name, DefaultPresets[i].Name, "preset[%d] name", i)
		require.Equal(t, exp.hex, DefaultPresets[i].Hex, "preset[%d] hex", i)
	}
}

func TestNavigationDown(t *testing.T) {
	m := New()

	// Navigate down within column 1
	for i := 0; i < 9; i++ {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
		require.Equal(t, i+1, m.selected, "after %d j presses", i+1)
	}

	// Try to go beyond bounds
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	require.Equal(t, 9, m.selected, "should not exceed bounds")
}

func TestNavigationUp(t *testing.T) {
	m := New()
	m.selected = 5

	// Navigate up
	for i := 5; i > 0; i-- {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
		require.Equal(t, i-1, m.selected)
	}

	// Try to go below zero
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	require.Equal(t, 0, m.selected, "should not go below 0")
}

func TestNavigationArrowKeys(t *testing.T) {
	m := New()

	// Down arrow
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	require.Equal(t, 1, m.selected, "down arrow")

	// Up arrow
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	require.Equal(t, 0, m.selected, "up arrow")
}

func TestColumnNavigation(t *testing.T) {
	m := New()

	// Start in column 0
	require.Equal(t, 0, m.column, "expected to start in column 0")

	// Move right with 'l' - through all 4 columns
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	require.Equal(t, 1, m.column, "after 'l'")

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
	require.Equal(t, 2, m.column, "after right")

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	require.Equal(t, 3, m.column, "after 'l'")

	// Can't go beyond rightmost column (column 3)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	require.Equal(t, 3, m.column, "capped at column 3")

	// Move left with 'h'
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	require.Equal(t, 2, m.column, "after 'h'")

	// Move left with left arrow
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	require.Equal(t, 1, m.column, "after left")

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	require.Equal(t, 0, m.column, "after left")

	// Can't go beyond leftmost column
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	require.Equal(t, 0, m.column, "capped at column 0")
}

func TestColumnNavigationClampsSelection(t *testing.T) {
	m := New()

	// Set selection to row 5
	m.selected = 5

	// Move to column 1 (should keep row 5 since all columns have 10 items)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	require.Equal(t, 5, m.selected, "selected should be preserved")
}

func TestSelectionEnter(t *testing.T) {
	m := New()
	m.selected = 2 // Blue

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	require.NotNil(t, cmd, "expected a command")

	msg := cmd()
	selectMsg, ok := msg.(SelectMsg)
	require.True(t, ok, "expected SelectMsg, got %T", msg)

	require.Equal(t, "#54A0FF", selectMsg.Hex)
}

func TestCancellation(t *testing.T) {
	m := New()

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})

	require.NotNil(t, cmd, "expected a command")

	msg := cmd()
	_, ok := msg.(CancelMsg)
	require.True(t, ok, "expected CancelMsg, got %T", msg)
}

func TestCustomModeToggle(t *testing.T) {
	m := New()

	require.False(t, m.inCustomMode, "should start in normal mode")

	// Press 'c' to enter custom mode
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})

	require.True(t, m.inCustomMode, "pressing 'c' should enter custom mode")
}

func TestCustomModeEscape(t *testing.T) {
	m := New()

	// Enter custom mode
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	require.True(t, m.inCustomMode, "should be in custom mode")

	// Press Esc to exit custom mode
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})

	require.False(t, m.inCustomMode, "Esc should exit custom mode")
}

func TestCustomModeValidHex(t *testing.T) {
	m := New()

	// Enter custom mode
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})

	// Type valid hex
	for _, r := range "#AABBCC" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}

	// Press Enter to move to Save button
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	// Press Enter again to save
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	require.NotNil(t, cmd, "expected a command")

	msg := cmd()
	selectMsg, ok := msg.(SelectMsg)
	require.True(t, ok, "expected SelectMsg, got %T", msg)

	require.Equal(t, "#AABBCC", selectMsg.Hex)
}

func TestCustomModeInvalidHex(t *testing.T) {
	m := New()

	// Enter custom mode
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})

	// Type invalid hex
	for _, r := range "notahex" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}

	// Press Enter - should stay in custom mode
	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	require.Nil(t, cmd, "invalid hex should not produce a command")
	require.True(t, m.inCustomMode, "should still be in custom mode after invalid hex")
}

func TestSetSelected(t *testing.T) {
	m := New()

	// Set to a known color in column 0
	m = m.SetSelected("#7D56F4") // Purple at index 3 in column 0
	require.Equal(t, 0, m.column, "Purple column")
	require.Equal(t, 3, m.selected, "Purple selected")

	// Case insensitive
	m = m.SetSelected("#ff8787") // Red at index 0 in column 0
	require.Equal(t, 0, m.column, "Red (case insensitive) column")
	require.Equal(t, 0, m.selected, "Red (case insensitive) selected")

	// Set to a color in column 1
	m = m.SetSelected("#A3E635") // Lime at index 0 in column 1
	require.Equal(t, 1, m.column, "Lime column")
	require.Equal(t, 0, m.selected, "Lime selected")

	// Set to a color in column 2
	m = m.SetSelected("#DC143C") // Crimson at index 0 in column 2
	require.Equal(t, 2, m.column, "Crimson column")
	require.Equal(t, 0, m.selected, "Crimson selected")

	// Set to a color in column 3 (grayscale)
	m = m.SetSelected("#000000") // Black at index 9 in column 3
	require.Equal(t, 3, m.column, "Black column")
	require.Equal(t, 9, m.selected, "Black selected")

	// Unknown color - should default to first selection
	m.column = 2
	m.selected = 5
	m = m.SetSelected("#123456")
	require.Equal(t, 0, m.column, "unknown color should default to column 0")
	require.Equal(t, 0, m.selected, "unknown color should default to selected 0")
}

func TestSetSize(t *testing.T) {
	m := New()
	m = m.SetSize(80, 24)

	require.Equal(t, 80, m.viewportWidth)
	require.Equal(t, 24, m.viewportHeight)
}

func TestSetBoxWidth(t *testing.T) {
	m := New()
	m = m.SetBoxWidth(40)

	require.Equal(t, 40, m.boxWidth)
}

func TestSelected(t *testing.T) {
	m := New()
	m.selected = 1 // Green

	preset := m.Selected()
	require.Equal(t, "Green", preset.Name)
	require.Equal(t, "#73F59F", preset.Hex)
}

func TestSelectedOutOfBounds(t *testing.T) {
	m := New()
	m.selected = -1

	preset := m.Selected()
	require.Empty(t, preset.Name, "out of bounds should return empty PresetColor")
	require.Empty(t, preset.Hex, "out of bounds should return empty PresetColor")

	m.selected = 100
	preset = m.Selected()
	require.Empty(t, preset.Name, "out of bounds should return empty PresetColor")
	require.Empty(t, preset.Hex, "out of bounds should return empty PresetColor")
}

func TestInCustomMode(t *testing.T) {
	m := New()

	require.False(t, m.InCustomMode(), "should not be in custom mode initially")

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})

	require.True(t, m.InCustomMode(), "should be in custom mode after pressing 'c'")
}

func TestCustomModeDisabled(t *testing.T) {
	m := New()
	m.customEnabled = false

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})

	require.False(t, m.inCustomMode, "pressing 'c' should not enter custom mode when disabled")
}

func TestCustomModeFocusCycling(t *testing.T) {
	m := New()

	// Enter custom mode
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	require.Equal(t, customFocusInput, m.customFocus, "should start on input field")

	// ctrl+n cycles: Input -> Save -> Cancel -> Input
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlN})
	require.Equal(t, customFocusSave, m.customFocus, "ctrl+n from Input")

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlN})
	require.Equal(t, customFocusCancel, m.customFocus, "ctrl+n from Save")

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlN})
	require.Equal(t, customFocusInput, m.customFocus, "ctrl+n from Cancel (cycle)")

	// ctrl+p cycles: Input -> Cancel -> Save -> Input
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlP})
	require.Equal(t, customFocusCancel, m.customFocus, "ctrl+p from Input (cycle)")

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlP})
	require.Equal(t, customFocusSave, m.customFocus, "ctrl+p from Cancel")

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlP})
	require.Equal(t, customFocusInput, m.customFocus, "ctrl+p from Save")
}

func TestIsValidHex(t *testing.T) {
	tests := []struct {
		input string
		valid bool
	}{
		{"#AABBCC", true},
		{"#aabbcc", true},
		{"#123456", true},
		{"#FF8787", true},
		{"AABBCC", false},    // Missing #
		{"#ABC", false},      // Too short
		{"#AABBCCDD", false}, // Too long
		{"#GGGGGG", false},   // Invalid chars
		{"", false},
		{"hello", false},
	}

	for _, tt := range tests {
		result := isValidHex(tt.input)
		require.Equal(t, tt.valid, result, "isValidHex(%q)", tt.input)
	}
}

func TestViewRendersWithoutPanic(t *testing.T) {
	m := New()
	m = m.SetSize(80, 24)

	// Normal mode
	view := m.View()
	require.NotEmpty(t, view, "View() returned empty string in normal mode")

	// Custom mode
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	view = m.View()
	require.NotEmpty(t, view, "View() returned empty string in custom mode")
}

func TestOverlayRendersWithoutPanic(t *testing.T) {
	m := New()
	m = m.SetSize(80, 24)

	// With empty background
	result := m.Overlay("")
	require.NotEmpty(t, result, "Overlay() returned empty string with empty background")

	// With background
	background := "Some background content"
	result = m.Overlay(background)
	require.NotEmpty(t, result, "Overlay() returned empty string with background")
}

// TestCustomModeJKInputPassthrough verifies j/k keys are passed to text input when focused.
func TestCustomModeJKInputPassthrough(t *testing.T) {
	m := New()

	// Enter custom mode - focus starts on input
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	require.Equal(t, customFocusInput, m.customFocus, "expected focus on input")

	// Type 'j' - should be added to input, not navigate
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	require.Equal(t, "j", m.customInput.Value())
	require.Equal(t, customFocusInput, m.customFocus, "focus should stay on input")

	// Type 'k' - should be added to input, not navigate
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	require.Equal(t, "jk", m.customInput.Value())
	require.Equal(t, customFocusInput, m.customFocus, "focus should stay on input")
}

// TestSetSelectedResetsCustomMode verifies SetSelected exits custom mode.
func TestSetSelectedResetsCustomMode(t *testing.T) {
	m := New()

	// Enter custom mode
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	require.True(t, m.inCustomMode, "should be in custom mode")

	// SetSelected should reset to preset mode
	m = m.SetSelected("#FF8787")
	require.False(t, m.inCustomMode, "SetSelected should exit custom mode")
	require.Equal(t, customFocusInput, m.customFocus, "SetSelected should reset customFocus to input")
	require.False(t, m.showCustomError, "SetSelected should clear custom error")
}

// TestCustomModeJKNavigationWhenNotOnInput verifies j/k navigate when not on input.
func TestCustomModeJKNavigationWhenNotOnInput(t *testing.T) {
	m := New()

	// Enter custom mode and move to Save button
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab}) // Move to Save
	require.Equal(t, customFocusSave, m.customFocus, "expected focus on Save")

	// 'j' should navigate to Cancel
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	require.Equal(t, customFocusCancel, m.customFocus, "expected focus on Cancel after 'j'")

	// 'k' should navigate back to Save
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	require.Equal(t, customFocusSave, m.customFocus, "expected focus on Save after 'k'")
}

// TestColorPicker_View_Golden uses teatest golden file comparison.
// Run with -update flag to update golden files: go test -update ./internal/ui/colorpicker/...
func TestColorPicker_View_Golden(t *testing.T) {
	m := New().SetSize(80, 24)
	view := m.View()
	teatest.RequireEqualOutput(t, []byte(view))
}

// TestColorPicker_View_CustomMode_Golden tests the custom hex entry view.
func TestColorPicker_View_CustomMode_Golden(t *testing.T) {
	m := New().SetSize(80, 24)
	// Enter custom mode
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	// Type a valid hex color
	for _, r := range "#FF0000" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	view := m.View()
	teatest.RequireEqualOutput(t, []byte(view))
}
