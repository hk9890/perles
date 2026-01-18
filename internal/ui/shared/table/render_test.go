package table

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"github.com/stretchr/testify/require"
	"github.com/zjrosen/perles/internal/ui/styles"
)

// Test row data structure
type testRow struct {
	ID     int
	Name   string
	Status string
}

// Helper to create basic test columns
func createTestColumns() []ColumnConfig {
	return []ColumnConfig{
		{
			Key:    "id",
			Header: "ID",
			Width:  4,
			Render: func(row any, _ string, w int, _ bool) string {
				r := row.(*testRow)
				return styles.TruncateString(lipgloss.NewStyle().Render(string(rune('0'+r.ID))), w)
			},
		},
		{
			Key:    "name",
			Header: "Name",
			Width:  10,
			Render: func(row any, _ string, w int, _ bool) string {
				r := row.(*testRow)
				return styles.TruncateString(r.Name, w)
			},
		},
		{
			Key:    "status",
			Header: "Status",
			Width:  8,
			Align:  lipgloss.Right,
			Render: func(row any, _ string, w int, _ bool) string {
				r := row.(*testRow)
				return styles.TruncateString(r.Status, w)
			},
		},
	}
}

func TestRenderHeader_MultipleColumns(t *testing.T) {
	cols := createTestColumns()
	widths := []int{4, 10, 8}

	header := renderHeader(cols, widths)

	// Header should contain column headers
	require.Contains(t, header, "ID")
	require.Contains(t, header, "Name")
	require.Contains(t, header, "Status")

	// Should be joined with space separators
	require.Contains(t, header, " ")
}

func TestRenderHeader_EmptyColumns(t *testing.T) {
	header := renderHeader([]ColumnConfig{}, []int{})
	require.Empty(t, header, "empty columns should produce empty header")
}

func TestRenderHeader_Truncation(t *testing.T) {
	cols := []ColumnConfig{
		{Key: "col", Header: "VeryLongHeaderName", Width: 5},
	}
	widths := []int{5}

	header := renderHeader(cols, widths)

	// Header should be truncated (with "..." as truncation indicator)
	// Width 5 means "Ve..." (2 chars + "...")
	require.LessOrEqual(t, lipgloss.Width(header), 5, "header should be truncated to fit width")
}

func TestRenderCellWithBackground_Truncation(t *testing.T) {
	row := &testRow{ID: 1, Name: "VeryLongNameThatNeedsTruncation", Status: "OK"}
	col := ColumnConfig{
		Key:   "name",
		Width: 10,
		Render: func(r any, _ string, w int, _ bool) string {
			return r.(*testRow).Name
		},
	}

	cell := renderCellWithBackground(row, col, 10, false, styles.SelectionBackgroundColor)

	// Cell should be truncated
	require.LessOrEqual(t, lipgloss.Width(cell), 10, "cell should be truncated to width")
}

func TestRenderCellWithBackground_SelectionStyling(t *testing.T) {
	// Track whether the Render callback received the correct selected flag
	var receivedSelected bool
	row := &testRow{ID: 1, Name: "Test", Status: "OK"}
	col := ColumnConfig{
		Key:   "name",
		Width: 10,
		Render: func(r any, _ string, w int, selected bool) string {
			receivedSelected = selected
			return r.(*testRow).Name
		},
	}

	// Without selection
	receivedSelected = false
	_ = renderCellWithBackground(row, col, 10, false, styles.SelectionBackgroundColor)
	require.False(t, receivedSelected, "Render callback should receive selected=false")

	// With selection
	receivedSelected = false
	_ = renderCellWithBackground(row, col, 10, true, styles.SelectionBackgroundColor)
	require.True(t, receivedSelected, "Render callback should receive selected=true")
}

func TestPanicRecovery_CatchesBadTypeAssertion(t *testing.T) {
	// Create a column that will panic on type assertion
	col := ColumnConfig{
		Key:   "bad",
		Width: 10,
		Render: func(row any, _ string, w int, _ bool) string {
			// This will panic if row is not *testRow
			r := row.(*testRow)
			return r.Name
		},
	}

	// Pass a different type
	wrongType := "not a testRow"

	// Should not panic, should return error indicator
	result := safeRenderCallback(wrongType, col, 10, false)

	// Result should contain error indication
	require.Contains(t, result, "!ERR", "panic recovery should return error indicator")
}

func TestPanicRecovery_NilRenderCallback(t *testing.T) {
	col := ColumnConfig{
		Key:    "nil",
		Width:  10,
		Render: nil,
	}

	result := safeRenderCallback(&testRow{}, col, 10, false)

	require.Empty(t, result, "nil Render callback should return empty string")
}

func TestRenderRow_SelectionBackground(t *testing.T) {
	// Enable ANSI colors for this test only
	oldProfile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.ANSI256)
	defer lipgloss.SetColorProfile(oldProfile)

	// Track whether the Render callbacks are called
	var callCount int
	row := &testRow{ID: 1, Name: "Test", Status: "OK"}
	cols := []ColumnConfig{
		{
			Key:    "name",
			Header: "Name",
			Width:  10,
			Render: func(row any, _ string, w int, selected bool) string {
				callCount++
				// Note: selected is always false because selection background
				// is applied at the row level, not passed to render callbacks
				return row.(*testRow).Name
			},
		},
		{
			Key:    "status",
			Header: "Status",
			Width:  8,
			Render: func(row any, _ string, w int, selected bool) string {
				callCount++
				return row.(*testRow).Status
			},
		},
	}
	widths := []int{10, 8}
	fullWidth := 10 + 8 + 1 // columns + separator

	// Render without selection
	callCount = 0
	result := renderRow(row, cols, widths, false, fullWidth)
	require.Equal(t, 2, callCount, "should call Render for each column")
	require.NotContains(t, result, "\x1b[48", "unselected row should not have background color")

	// Render with selection - should contain background ANSI codes
	callCount = 0
	result = renderRow(row, cols, widths, true, fullWidth)
	require.Equal(t, 2, callCount, "should call Render for each column")
	require.Contains(t, result, "\x1b[", "selected row should have ANSI styling")
}

func TestRenderRow_EmptyColumns(t *testing.T) {
	row := &testRow{ID: 1, Name: "Test", Status: "OK"}
	result := renderRow(row, []ColumnConfig{}, []int{}, false, 0)
	require.Empty(t, result, "empty columns should produce empty row")
}

func TestRenderEmptyState_Centered(t *testing.T) {
	msg := "No data available"
	width := 40
	height := 10

	result := renderEmptyState(msg, width, height)

	// Should contain the message
	require.Contains(t, result, msg)

	// Should be multiple lines (for vertical centering)
	lines := strings.Split(result, "\n")
	require.Equal(t, height, len(lines), "should have correct number of lines")
}

func TestRenderEmptyState_DefaultMessage(t *testing.T) {
	result := renderEmptyState("", 40, 10)
	require.Contains(t, result, "No data", "should use default message when empty")
}

func TestRenderEmptyState_ZeroDimensions(t *testing.T) {
	require.Empty(t, renderEmptyState("test", 0, 10), "zero width should return empty")
	require.Empty(t, renderEmptyState("test", 40, 0), "zero height should return empty")
	require.Empty(t, renderEmptyState("test", -1, 10), "negative width should return empty")
	require.Empty(t, renderEmptyState("test", 40, -1), "negative height should return empty")
}

func TestRenderEmptyState_NarrowWidth(t *testing.T) {
	msg := "VeryLongMessageThatNeedsTruncation"
	width := 10
	height := 5

	result := renderEmptyState(msg, width, height)

	// Each line should fit within width (accounting for possible truncation)
	lines := strings.Split(result, "\n")
	for _, line := range lines {
		require.LessOrEqual(t, lipgloss.Width(line), width+5, "line should fit within width (with some tolerance for styling)")
	}
}

func TestAlignText_Left(t *testing.T) {
	result := alignText("test", 10, lipgloss.Left)
	require.Equal(t, "test      ", result, "left-aligned text should have trailing spaces")
}

func TestAlignText_Right(t *testing.T) {
	result := alignText("test", 10, lipgloss.Right)
	require.Equal(t, "      test", result, "right-aligned text should have leading spaces")
}

func TestAlignText_Center(t *testing.T) {
	result := alignText("test", 10, lipgloss.Center)
	require.Equal(t, "   test   ", result, "centered text should have balanced spacing")
}

func TestAlignText_ExactWidth(t *testing.T) {
	result := alignText("test", 4, lipgloss.Left)
	require.Equal(t, "test", result, "text at exact width should remain unchanged")
}

func TestAlignText_Overflow(t *testing.T) {
	result := alignText("longtext", 4, lipgloss.Left)
	require.Equal(t, "longtext", result, "overflowing text should not be modified by alignment")
}
