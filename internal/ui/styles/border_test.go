package styles

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/charmbracelet/lipgloss"
)

// Test colors for border rendering tests
var (
	testColorRed    = lipgloss.Color("#FF0000")
	testColorGreen  = lipgloss.Color("#00FF00")
	testColorBlue   = lipgloss.Color("#0000FF")
	testColorPurple = lipgloss.Color("#800080")
)

func TestRenderWithTitleBorder_Basic(t *testing.T) {
	result := RenderWithTitleBorder("content", "Title", "", 20, 5, false, testColorGreen, testColorGreen)

	// Should contain border characters
	require.Contains(t, result, "╭", "missing top-left corner")
	require.Contains(t, result, "╮", "missing top-right corner")
	require.Contains(t, result, "╰", "missing bottom-left corner")
	require.Contains(t, result, "╯", "missing bottom-right corner")

	// Should contain title in first line
	lines := strings.Split(result, "\n")
	require.NotEmpty(t, lines, "no lines in result")
	require.Contains(t, lines[0], "Title", "title not found in first line")
}

func TestRenderWithTitleBorder_Focused(t *testing.T) {
	unfocused := RenderWithTitleBorder("content", "Title", "", 20, 5, false, testColorGreen, testColorGreen)
	focused := RenderWithTitleBorder("content", "Title", "", 20, 5, true, testColorGreen, testColorGreen)

	// Both should have same structure but different styling
	unfocusedLines := strings.Split(unfocused, "\n")
	focusedLines := strings.Split(focused, "\n")

	require.Equal(t, len(unfocusedLines), len(focusedLines), "different line counts")

	// Both should contain title
	require.Contains(t, unfocused, "Title", "unfocused missing title")
	require.Contains(t, focused, "Title", "focused missing title")
}

func TestRenderWithTitleBorder_LongTitle(t *testing.T) {
	longTitle := "This Is A Very Long Title That Should Be Truncated"
	result := RenderWithTitleBorder("content", longTitle, "", 20, 5, false, testColorRed, testColorRed)

	// Should still have valid border structure
	require.Contains(t, result, "╭", "missing top-left corner")

	lines := strings.Split(result, "\n")
	require.NotEmpty(t, lines, "no lines in result")

	// First line should not exceed width
	firstLineWidth := lipgloss.Width(lines[0])
	require.LessOrEqual(t, firstLineWidth, 20, "first line too wide: %d > 20", firstLineWidth)

	// Should have truncation indicator
	require.Contains(t, lines[0], "...", "long title should be truncated with ellipsis")
}

func TestRenderWithTitleBorder_EmptyContent(t *testing.T) {
	result := RenderWithTitleBorder("", "Title", "", 20, 5, false, testColorBlue, testColorBlue)

	// Should still render proper border
	require.Contains(t, result, "╭", "missing top-left corner")
	require.Contains(t, result, "Title", "missing title")

	// Should have correct number of lines
	lines := strings.Split(result, "\n")
	// 1 top border + 3 content lines (height 5 - 2 borders) + 1 bottom border = 5
	require.Len(t, lines, 5, "expected 5 lines")
}

func TestRenderWithTitleBorder_NarrowWidth(t *testing.T) {
	result := RenderWithTitleBorder("x", "T", "", 6, 3, false, testColorPurple, testColorPurple)

	// Should still render something valid
	require.Contains(t, result, "╭", "missing top-left corner")
	require.Contains(t, result, "╯", "missing bottom-right corner")

	// Check line widths
	lines := strings.Split(result, "\n")
	for i, line := range lines {
		w := lipgloss.Width(line)
		require.LessOrEqual(t, w, 6, "line %d too wide: %d > 6, content: %q", i, w, line)
	}
}

func TestRenderWithTitleBorder_MinimalWidth(t *testing.T) {
	result := RenderWithTitleBorder("", "", "", 3, 3, false, BorderDefaultColor, BorderDefaultColor)

	// Should handle minimal size gracefully
	require.Contains(t, result, "╭", "missing top-left corner")
	require.Contains(t, result, "╯", "missing bottom-right corner")
}

func TestRenderWithTitleBorder_EmptyTitle(t *testing.T) {
	result := RenderWithTitleBorder("content", "", "", 20, 5, false, testColorGreen, testColorGreen)

	// First line should just be a plain border
	lines := strings.Split(result, "\n")
	require.NotEmpty(t, lines, "no lines in result")

	// Should start with top-left and be all dashes (no title text)
	require.True(t, strings.HasPrefix(lines[0], "╭"), "should start with top-left corner")
}

func TestRenderWithTitleBorder_MultilineContent(t *testing.T) {
	content := "Line 1\nLine 2\nLine 3"
	result := RenderWithTitleBorder(content, "Title", "", 20, 7, false, testColorBlue, testColorBlue)

	// Should contain all content lines
	require.Contains(t, result, "Line 1", "missing Line 1")
	require.Contains(t, result, "Line 2", "missing Line 2")
	require.Contains(t, result, "Line 3", "missing Line 3")
}

func TestRenderWithTitleBorder_ContentPadding(t *testing.T) {
	result := RenderWithTitleBorder("Hi", "Title", "", 20, 5, false, testColorRed, testColorRed)

	lines := strings.Split(result, "\n")

	// Content lines (middle ones) should have consistent width
	// They should all be padded to the same width
	for i := 1; i < len(lines)-1; i++ {
		w := lipgloss.Width(lines[i])
		require.Equal(t, 20, w, "line %d width %d, expected 20: %q", i, w, lines[i])
	}
}

func TestRenderWithTitleBorder_DifferentColors(t *testing.T) {
	// Test that different colors all render correctly
	colors := []struct {
		name  string
		color lipgloss.TerminalColor
	}{
		{"red", testColorRed},
		{"green", testColorGreen},
		{"blue", testColorBlue},
		{"purple", testColorPurple},
	}

	for _, tc := range colors {
		t.Run(tc.name, func(t *testing.T) {
			result := RenderWithTitleBorder("content", "Title", "", 20, 5, false, tc.color, tc.color)
			require.Contains(t, result, "Title", "%s: missing title", tc.name)
			require.Contains(t, result, "╭", "%s: missing border", tc.name)
		})
	}
}

func TestRenderWithTitleBorder_RightTitleOnly(t *testing.T) {
	result := RenderWithTitleBorder("content", "", "Right", 20, 5, false, testColorGreen, testColorGreen)
	lines := strings.Split(result, "\n")
	require.NotEmpty(t, lines, "no lines in result")
	require.Contains(t, lines[0], "Right", "right title not found in first line")
}

func TestRenderWithTitleBorder_DualTitles(t *testing.T) {
	result := RenderWithTitleBorder("content", "Left", "Right", 30, 5, false, testColorGreen, testColorGreen)
	lines := strings.Split(result, "\n")
	require.NotEmpty(t, lines, "no lines in result")
	require.Contains(t, lines[0], "Left", "left title not found")
	require.Contains(t, lines[0], "Right", "right title not found")
}

func TestRenderWithTitleBorder_NoTitles(t *testing.T) {
	result := RenderWithTitleBorder("content", "", "", 20, 5, false, testColorGreen, testColorGreen)

	// Should still have valid border structure
	require.Contains(t, result, "╭", "missing top-left corner")
	require.Contains(t, result, "╮", "missing top-right corner")

	// First line should be a plain border (no title text)
	lines := strings.Split(result, "\n")
	require.NotEmpty(t, lines, "no lines in result")
	// Should only contain border characters and dashes
	require.True(t, strings.HasPrefix(lines[0], "╭"), "should start with top-left corner")
	require.True(t, strings.HasSuffix(lines[0], "╮"), "should end with top-right corner")
}

func TestTruncateString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		maxWidth int
		want     string
	}{
		{"fits", "Hello", 10, "Hello"},
		{"exact", "Hello", 5, "Hello"},
		{"truncate", "Hello World", 8, "Hello..."},
		{"very short", "Hello", 3, "..."},
		{"minimal", "Hello", 1, "."},
		{"zero", "Hello", 0, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateString(tt.input, tt.maxWidth)
			require.Equal(t, tt.want, got, "truncateString(%q, %d)", tt.input, tt.maxWidth)
		})
	}
}

func TestBuildTopBorder(t *testing.T) {
	borderStyle := lipgloss.NewStyle().Foreground(BorderDefaultColor)
	titleStyle := lipgloss.NewStyle().Foreground(testColorGreen)

	tests := []struct {
		name       string
		title      string
		innerWidth int
		wantTitle  bool
	}{
		{"normal", "Title", 20, true},
		{"empty title", "", 20, false},
		{"narrow", "Title", 3, false},
		{"just enough", "T", 6, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildTopBorder(tt.title, tt.innerWidth, borderStyle, titleStyle)

			require.True(t, strings.HasPrefix(got, "╭"), "should start with top-left corner")
			require.True(t, strings.HasSuffix(got, "╮"), "should end with top-right corner")

			hasTitle := strings.Contains(got, tt.title) && tt.title != ""
			if tt.wantTitle {
				require.True(t, hasTitle, "expected title %q in border: %s", tt.title, got)
			}
		})
	}
}
