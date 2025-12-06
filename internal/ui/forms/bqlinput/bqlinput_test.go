package bqlinput

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/exp/teatest"
	"github.com/muesli/termenv"
	"github.com/stretchr/testify/require"
)

func init() {
	// Force ANSI color output in tests (lipgloss disables colors when no TTY)
	lipgloss.SetColorProfile(termenv.ANSI256)
}

func TestNew_DefaultValues(t *testing.T) {
	m := New()

	require.Empty(t, m.Value())
	require.Equal(t, 0, m.Cursor())
	require.False(t, m.Focused(), "expected not focused by default")
	require.Equal(t, 40, m.Width())
}

func TestSetValue(t *testing.T) {
	m := New()
	m.SetValue("test")

	require.Equal(t, "test", m.Value())
}

func TestSetValue_ClampsCursor(t *testing.T) {
	m := New()
	m.SetValue("hello")
	m.SetCursor(5) // cursor at end

	// Now set shorter value
	m.SetValue("hi")

	require.Equal(t, 2, m.Cursor(), "expected cursor clamped to 2")
}

func TestSetCursor_ClampsToRange(t *testing.T) {
	m := New()
	m.SetValue("test")

	// Test negative
	m.SetCursor(-5)
	require.Equal(t, 0, m.Cursor(), "expected 0 for negative")

	// Test past end
	m.SetCursor(100)
	require.Equal(t, 4, m.Cursor(), "expected 4 (length)")

	// Test valid
	m.SetCursor(2)
	require.Equal(t, 2, m.Cursor())
}

func TestFocusBlur(t *testing.T) {
	m := New()

	m.Focus()
	require.True(t, m.Focused(), "expected focused after Focus()")

	m.Blur()
	require.False(t, m.Focused(), "expected not focused after Blur()")
}

func TestSetWidth(t *testing.T) {
	m := New()

	m.SetWidth(100)
	require.Equal(t, 100, m.Width())

	// Minimum width is 1
	m.SetWidth(0)
	require.Equal(t, 1, m.Width(), "expected minimum width 1")
}

func TestUpdate_NotFocused_IgnoresKeys(t *testing.T) {
	m := New()
	m.SetValue("test")

	// Not focused, so key should be ignored
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})

	require.Equal(t, "test", m.Value(), "expected value unchanged when not focused")
}

func TestUpdate_InsertChars(t *testing.T) {
	m := New()
	m.Focus()

	// Type "hi"
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}})

	require.Equal(t, "hi", m.Value())
	require.Equal(t, 2, m.Cursor())
}

func TestUpdate_InsertInMiddle(t *testing.T) {
	m := New()
	m.SetValue("hllo")
	m.SetCursor(1) // after 'h'
	m.Focus()

	// Insert 'e'
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})

	require.Equal(t, "hello", m.Value())
	require.Equal(t, 2, m.Cursor())
}

func TestUpdate_Space(t *testing.T) {
	m := New()
	m.SetValue("ab")
	m.SetCursor(1)
	m.Focus()

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeySpace})

	require.Equal(t, "a b", m.Value())
}

func TestUpdate_Backspace(t *testing.T) {
	m := New()
	m.SetValue("hello")
	m.SetCursor(5) // at end
	m.Focus()

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyBackspace})

	require.Equal(t, "hell", m.Value())
	require.Equal(t, 4, m.Cursor())
}

func TestUpdate_BackspaceAtStart(t *testing.T) {
	m := New()
	m.SetValue("test")
	m.SetCursor(0)
	m.Focus()

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyBackspace})

	require.Equal(t, "test", m.Value(), "expected unchanged 'test'")
}

func TestUpdate_Delete(t *testing.T) {
	m := New()
	m.SetValue("hello")
	m.SetCursor(0)
	m.Focus()

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDelete})

	require.Equal(t, "ello", m.Value())
	require.Equal(t, 0, m.Cursor())
}

func TestUpdate_DeleteAtEnd(t *testing.T) {
	m := New()
	m.SetValue("test")
	m.SetCursor(4)
	m.Focus()

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDelete})

	require.Equal(t, "test", m.Value(), "expected unchanged 'test'")
}

func TestUpdate_CursorMovement(t *testing.T) {
	m := New()
	m.SetValue("hello")
	m.SetCursor(2)
	m.Focus()

	// Left
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	require.Equal(t, 1, m.Cursor(), "expected cursor at 1 after left")

	// Right
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
	require.Equal(t, 2, m.Cursor(), "expected cursor at 2 after right")

	// Home (Ctrl+A)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlA})
	require.Equal(t, 0, m.Cursor(), "expected cursor at 0 after home")

	// End (Ctrl+E)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlE})
	require.Equal(t, 5, m.Cursor(), "expected cursor at 5 after end")
}

func TestUpdate_CursorBounds(t *testing.T) {
	m := New()
	m.SetValue("hi")
	m.Focus()

	// At start, left should stay at 0
	m.SetCursor(0)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	require.Equal(t, 0, m.Cursor(), "expected cursor to stay at 0")

	// At end, right should stay at end
	m.SetCursor(2)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
	require.Equal(t, 2, m.Cursor(), "expected cursor to stay at 2")
}

func TestView_EmptyFocused(t *testing.T) {
	m := New()
	m.Focus()

	view := m.View()
	// Empty input when focused shows cursor
	require.NotEmpty(t, view, "expected cursor for focused empty input")
}

func TestView_EmptyNotFocused(t *testing.T) {
	m := New()
	// No focus, no placeholder

	view := m.View()
	require.Empty(t, view)
}

func TestView_Placeholder(t *testing.T) {
	m := New()
	m.SetPlaceholder("Enter query")

	view := m.View()
	require.Contains(t, view, "Enter query", "expected placeholder in view")
}

func TestView_WithValue_HasHighlighting(t *testing.T) {
	m := New()
	m.SetValue("status = open")

	view := m.View()
	// Should contain ANSI codes for highlighting
	require.Contains(t, view, "\x1b[", "expected ANSI codes in view for highlighting")
	// Should contain the text content
	require.Contains(t, view, "status", "expected 'status' in view")
}

func TestView_Focused_ShowsHighlightedText(t *testing.T) {
	m := New()
	m.SetValue("status = open")
	m.SetCursor(0) // cursor at start
	m.Focus()

	view := m.View()
	// View should show syntax-highlighted text with cursor
	require.NotEmpty(t, view, "expected non-empty view")
	// Should contain ANSI codes (for highlighting and cursor)
	require.Contains(t, view, "\x1b[", "expected ANSI codes in view")
}

func TestNextWordEnd(t *testing.T) {
	tests := []struct {
		name     string
		s        string
		pos      int
		expected int
	}{
		{"from start", "hello world", 0, 5},
		{"from middle of word", "hello world", 2, 5},
		{"from space", "hello world", 5, 11},
		{"from second word", "hello world", 6, 11},
		{"at end", "hello", 5, 5},
		{"with punctuation", "status:open", 0, 6},
		{"skip colon", "status:open", 6, 11}, // from ':', skips non-word then 'open'
		{"after colon", "status:open", 7, 11},
		{"multiple spaces", "a   b", 0, 1},
		{"empty string", "", 0, 0},
		{"underscores", "my_var next", 0, 6},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := nextWordEnd(tt.s, tt.pos)
			require.Equal(t, tt.expected, result, "nextWordEnd(%q, %d)", tt.s, tt.pos)
		})
	}
}

func TestPrevWordStart(t *testing.T) {
	tests := []struct {
		name     string
		s        string
		pos      int
		expected int
	}{
		{"from end", "hello world", 11, 6},
		{"from middle of second word", "hello world", 8, 6},
		{"from space", "hello world", 6, 0},
		{"from start of second word", "hello world", 6, 0},
		{"at start", "hello", 0, 0},
		{"with punctuation", "status:open", 11, 7},
		{"before colon", "status:open", 7, 0},
		{"at colon", "status:open", 6, 0},
		{"multiple spaces", "a   b", 5, 4}, // from after 'b', goes to start of 'b'
		{"empty string", "", 0, 0},
		{"underscores", "my_var next", 11, 7},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := prevWordStart(tt.s, tt.pos)
			require.Equal(t, tt.expected, result, "prevWordStart(%q, %d)", tt.s, tt.pos)
		})
	}
}

func TestUpdate_CtrlF_WordForward(t *testing.T) {
	m := New()
	m.SetValue("hello world")
	m.SetCursor(0)
	m.Focus()

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlF})

	require.Equal(t, 5, m.Cursor(), "expected cursor at 5 after ctrl+f")
}

func TestUpdate_CtrlB_WordBackward(t *testing.T) {
	m := New()
	m.SetValue("hello world")
	m.SetCursor(11)
	m.Focus()

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlB})

	require.Equal(t, 6, m.Cursor(), "expected cursor at 6 after ctrl+b")
}

func TestUpdate_AltRight_WordForward(t *testing.T) {
	m := New()
	m.SetValue("hello world")
	m.SetCursor(0)
	m.Focus()

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight, Alt: true})

	require.Equal(t, 5, m.Cursor(), "expected cursor at 5 after alt+right")
}

func TestUpdate_AltLeft_WordBackward(t *testing.T) {
	m := New()
	m.SetValue("hello world")
	m.SetCursor(11)
	m.Focus()

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyLeft, Alt: true})

	require.Equal(t, 6, m.Cursor(), "expected cursor at 6 after alt+left")
}

func TestUpdate_AltF_WordForward(t *testing.T) {
	// macOS option+right sends Alt+f
	m := New()
	m.SetValue("hello world")
	m.SetCursor(0)
	m.Focus()

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}, Alt: true})

	require.Equal(t, 5, m.Cursor(), "expected cursor at 5 after alt+f")
}

func TestUpdate_AltB_WordBackward(t *testing.T) {
	// macOS option+left sends Alt+b
	m := New()
	m.SetValue("hello world")
	m.SetCursor(11)
	m.Focus()

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}, Alt: true})

	require.Equal(t, 6, m.Cursor(), "expected cursor at 6 after alt+b")
}

func TestHeight_Empty(t *testing.T) {
	m := New()
	m.SetWidth(40)

	// Empty value should return height 1
	require.Equal(t, 1, m.Height(), "expected height 1 for empty")
}

func TestHeight_SingleLine(t *testing.T) {
	m := New()
	m.SetWidth(40)
	m.SetValue("status = open")

	// Short text should fit on one line
	require.Equal(t, 1, m.Height(), "expected height 1 for short text")
}

func TestHeight_MultiLine(t *testing.T) {
	m := New()
	m.SetWidth(20) // narrow width to force wrapping
	m.SetValue("status = open and priority = p0 and type = bug")

	// Long text should wrap to multiple lines
	require.GreaterOrEqual(t, m.Height(), 2, "expected height >= 2 for long text")
}

func TestView_SingleLine(t *testing.T) {
	m := New()
	m.SetWidth(40)
	m.SetValue("status = open")

	view := m.View()

	// Should not contain newlines for short text
	require.NotContains(t, view, "\n", "expected no newlines in single-line text")

	// Should contain the content
	require.Contains(t, view, "status", "expected 'status' in view")
}

func TestView_MultiLine(t *testing.T) {
	m := New()
	m.SetWidth(20) // narrow width to force wrapping
	m.SetValue("status = open and priority = p0 and type = bug")

	view := m.View()

	// Should contain newlines for wrapped text
	require.Contains(t, view, "\n", "expected newlines in wrapped text")

	// Line count should match Height()
	lineCount := strings.Count(view, "\n") + 1
	require.Equal(t, m.Height(), lineCount, "expected lines to match Height()")
}

func TestView_PreservesHighlighting(t *testing.T) {
	m := New()
	m.SetWidth(20)
	m.SetValue("status = open and priority = p0")

	view := m.View()

	// Should contain ANSI codes for syntax highlighting
	require.Contains(t, view, "\x1b[", "expected ANSI codes in view for highlighting")
}

func TestView_FocusedShowsCursor(t *testing.T) {
	m := New()
	m.SetWidth(20)
	m.SetValue("status = open")
	m.Focus()

	view := m.View()

	// Should contain cursor code (reverse video)
	require.Contains(t, view, "\x1b[7m", "expected cursor ANSI code in focused view")
}

func TestView_WordBoundaryWrapping(t *testing.T) {
	// Test that wrapping breaks at word boundaries
	m := New()
	m.SetWidth(15)
	m.SetValue("status = open and ready = true")

	view := m.View()
	lines := strings.Split(view, "\n")

	// Check we have multiple lines
	require.GreaterOrEqual(t, len(lines), 2, "expected multiple lines")

	// Each line should have reasonable content (not cut mid-word if possible)
	for _, line := range lines {
		visibleWidth := lipgloss.Width(line)
		require.LessOrEqual(t, visibleWidth, 15+5, "line too long: width=%d, line=%q", visibleWidth, line)
	}
}

func TestCursorAtWrapBoundary(t *testing.T) {
	// Test cursor navigation near wrap boundary
	m := New()
	m.SetWidth(20)
	m.SetValue("status = open and priority = p0")
	m.Focus()

	// Test cursor at various positions
	testCases := []struct {
		pos      int
		expected int // lines should always match Height()
	}{
		{0, 2},  // start
		{10, 2}, // middle of first word
		{14, 2}, // near wrap point
		{20, 2}, // past wrap
		{25, 2}, // middle of second line
	}

	for _, tc := range testCases {
		m.SetCursor(tc.pos)
		view := m.View()
		lines := strings.Split(view, "\n")

		require.Len(t, lines, tc.expected, "cursor at %d: expected %d lines", tc.pos, tc.expected)

		// Verify cursor marker is present exactly once
		cursorCount := strings.Count(view, "\x1b[7m")
		require.Equal(t, 1, cursorCount, "cursor at %d: expected exactly 1 cursor marker", tc.pos)
	}
}

func TestCursorMovementWithWrapping(t *testing.T) {
	// Test that left/right cursor movement works correctly with wrapped text
	m := New()
	m.SetWidth(20)
	m.SetValue("status = open and priority = p0")
	m.SetCursor(0)
	m.Focus()

	// Move right through the text
	initialHeight := m.Height()
	for i := 0; i < len(m.Value()); i++ {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})

		// Height should stay constant
		require.Equal(t, initialHeight, m.Height(), "height changed at cursor %d", m.Cursor())

		// Cursor should advance
		require.Equal(t, i+1, m.Cursor())
	}
}

// Golden tests - run with -update flag to update golden files:
// go test ./internal/ui/bqlinput/... -update

func TestBqlInput_View_Golden_Empty(t *testing.T) {
	m := New()
	m.SetWidth(40)
	view := m.View()
	teatest.RequireEqualOutput(t, []byte(view))
}

func TestBqlInput_View_Golden_EmptyFocused(t *testing.T) {
	m := New()
	m.SetWidth(40)
	m.Focus()
	view := m.View()
	teatest.RequireEqualOutput(t, []byte(view))
}

func TestBqlInput_View_Golden_Placeholder(t *testing.T) {
	m := New()
	m.SetWidth(40)
	m.SetPlaceholder("Enter a BQL query...")
	view := m.View()
	teatest.RequireEqualOutput(t, []byte(view))
}

func TestBqlInput_View_Golden_SingleLine(t *testing.T) {
	m := New()
	m.SetWidth(40)
	m.SetValue("status = open")
	view := m.View()
	teatest.RequireEqualOutput(t, []byte(view))
}

func TestBqlInput_View_Golden_SingleLineFocused(t *testing.T) {
	m := New()
	m.SetWidth(40)
	m.SetValue("status = open")
	m.Focus()
	m.SetCursor(7) // cursor on "="
	view := m.View()
	teatest.RequireEqualOutput(t, []byte(view))
}

func TestBqlInput_View_Golden_MultiLine(t *testing.T) {
	m := New()
	m.SetWidth(25)
	m.SetValue("status = open and priority = p0 and type = bug")
	view := m.View()
	teatest.RequireEqualOutput(t, []byte(view))
}

func TestBqlInput_View_Golden_MultiLineFocused(t *testing.T) {
	m := New()
	m.SetWidth(25)
	m.SetValue("status = open and priority = p0 and type = bug")
	m.Focus()
	m.SetCursor(20) // cursor in second line
	view := m.View()
	teatest.RequireEqualOutput(t, []byte(view))
}

func TestBqlInput_View_Golden_ComplexQuery(t *testing.T) {
	m := New()
	m.SetWidth(50)
	m.SetValue("status = open and (priority <= p1 or type in (bug, task)) order by created")
	view := m.View()
	teatest.RequireEqualOutput(t, []byte(view))
}
