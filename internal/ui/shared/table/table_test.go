package table

import (
	"fmt"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/exp/teatest"
	"github.com/stretchr/testify/require"
	"github.com/zjrosen/perles/internal/ui/styles"
)

// testWorkflow is a test row data structure for golden tests
type testWorkflow struct {
	ID       int
	Name     string
	Status   string
	Workers  int
	Tokens   int64
	Priority string
}

// createGoldenTestConfig creates a standard table config for golden tests
func createGoldenTestConfig() TableConfig {
	return TableConfig{
		Columns: []ColumnConfig{
			{
				Key:    "id",
				Header: "#",
				Width:  3,
				Render: func(row any, _ string, w int, _ bool) string {
					wf := row.(*testWorkflow)
					return styles.TruncateString(fmt.Sprintf("%d", wf.ID), w)
				},
			},
			{
				Key:    "status",
				Header: "St",
				Width:  2,
				Render: func(row any, _ string, w int, _ bool) string {
					wf := row.(*testWorkflow)
					switch wf.Status {
					case "running":
						return "▶"
					case "paused":
						return "⏸"
					case "completed":
						return "✓"
					case "failed":
						return "✗"
					default:
						return "○"
					}
				},
			},
			{
				Key:      "name",
				Header:   "Name",
				MinWidth: 10,
				Render: func(row any, _ string, w int, _ bool) string {
					wf := row.(*testWorkflow)
					return styles.TruncateString(wf.Name, w)
				},
			},
			{
				Key:    "workers",
				Header: "Workers",
				Width:  8,
				Align:  lipgloss.Right,
				Render: func(row any, _ string, w int, _ bool) string {
					wf := row.(*testWorkflow)
					return fmt.Sprintf("%*d", w, wf.Workers)
				},
			},
			{
				Key:    "tokens",
				Header: "Tokens",
				Width:  10,
				Align:  lipgloss.Right,
				Render: func(row any, _ string, w int, _ bool) string {
					wf := row.(*testWorkflow)
					return fmt.Sprintf("%*d", w, wf.Tokens)
				},
			},
		},
		ShowHeader:   true,
		ShowBorder:   true,
		Title:        "Workflows",
		EmptyMessage: "No workflows yet",
	}
}

// createTestWorkflows creates sample workflow data for golden tests
func createTestWorkflows() []any {
	return []any{
		&testWorkflow{ID: 1, Name: "Build authentication system", Status: "running", Workers: 3, Tokens: 125000},
		&testWorkflow{ID: 2, Name: "Fix payment bug", Status: "paused", Workers: 1, Tokens: 45000},
		&testWorkflow{ID: 3, Name: "Deploy to production", Status: "completed", Workers: 0, Tokens: 87500},
		&testWorkflow{ID: 4, Name: "Integration tests", Status: "failed", Workers: 0, Tokens: 12300},
		&testWorkflow{ID: 5, Name: "Database refactoring", Status: "pending", Workers: 0, Tokens: 0},
	}
}

// Golden test: Empty table with EmptyMessage
func TestTable_View_Golden_Empty(t *testing.T) {
	cfg := createGoldenTestConfig()
	tbl := New(cfg).
		SetRows([]any{}).
		SetSize(80, 15)

	view := tbl.View()
	teatest.RequireEqualOutput(t, []byte(view))
}

// Golden test: Normal rendering with multiple rows
func TestTable_View_Golden_WithRows(t *testing.T) {
	cfg := createGoldenTestConfig()
	rows := createTestWorkflows()
	tbl := New(cfg).
		SetRows(rows).
		SetSize(80, 15)

	view := tbl.View()
	teatest.RequireEqualOutput(t, []byte(view))
}

// Golden test: Selection highlighting
func TestTable_View_Golden_WithSelection(t *testing.T) {
	cfg := createGoldenTestConfig()
	rows := createTestWorkflows()
	tbl := New(cfg).
		SetRows(rows).
		SetSize(80, 15)

	view := tbl.ViewWithSelection(2)
	teatest.RequireEqualOutput(t, []byte(view))
}

// Golden test: Content truncation with ellipsis
func TestTable_View_Golden_Truncation(t *testing.T) {
	cfg := createGoldenTestConfig()
	// Use extra-long names to force truncation
	rows := []any{
		&testWorkflow{ID: 1, Name: "This is an extremely long workflow name that needs truncation", Status: "running", Workers: 3, Tokens: 125000},
		&testWorkflow{ID: 2, Name: "Another very long name that will not fit in the column", Status: "paused", Workers: 1, Tokens: 45000},
	}
	tbl := New(cfg).
		SetRows(rows).
		SetSize(60, 10) // Narrow width to force truncation

	view := tbl.View()
	teatest.RequireEqualOutput(t, []byte(view))
}

// Golden test: Responsive sizing at 80 chars
func TestTable_View_Golden_NarrowWidth(t *testing.T) {
	cfg := createGoldenTestConfig()
	rows := createTestWorkflows()
	tbl := New(cfg).
		SetRows(rows).
		SetSize(80, 12)

	view := tbl.View()
	teatest.RequireEqualOutput(t, []byte(view))
}

// Golden test: Minimum width handling at 60 chars
func TestTable_View_Golden_VeryNarrow(t *testing.T) {
	cfg := createGoldenTestConfig()
	rows := createTestWorkflows()
	tbl := New(cfg).
		SetRows(rows).
		SetSize(60, 12)

	view := tbl.View()
	teatest.RequireEqualOutput(t, []byte(view))
}

// Golden test: Single row edge case
func TestTable_View_Golden_SingleRow(t *testing.T) {
	cfg := createGoldenTestConfig()
	rows := []any{
		&testWorkflow{ID: 1, Name: "Only workflow", Status: "running", Workers: 2, Tokens: 50000},
	}
	tbl := New(cfg).
		SetRows(rows).
		SetSize(80, 10)

	view := tbl.View()
	teatest.RequireEqualOutput(t, []byte(view))
}

// Golden test: ShowHeader: false
func TestTable_View_Golden_NoHeader(t *testing.T) {
	cfg := createGoldenTestConfig()
	cfg.ShowHeader = false
	rows := createTestWorkflows()
	tbl := New(cfg).
		SetRows(rows).
		SetSize(80, 12)

	view := tbl.View()
	teatest.RequireEqualOutput(t, []byte(view))
}

// Golden test: ShowBorder: false
func TestTable_View_Golden_NoBorder(t *testing.T) {
	cfg := createGoldenTestConfig()
	cfg.ShowBorder = false
	rows := createTestWorkflows()
	tbl := New(cfg).
		SetRows(rows).
		SetSize(80, 12)

	view := tbl.View()
	teatest.RequireEqualOutput(t, []byte(view))
}

// Golden test: Unicode/emoji content
func TestTable_View_Golden_Unicode(t *testing.T) {
	cfg := TableConfig{
		Columns: []ColumnConfig{
			{
				Key:    "icon",
				Header: "🎯",
				Width:  3,
				Render: func(row any, _ string, w int, _ bool) string {
					wf := row.(*testWorkflow)
					switch wf.Priority {
					case "high":
						return "🔥"
					case "medium":
						return "⚡"
					default:
						return "💤"
					}
				},
			},
			{
				Key:      "name",
				Header:   "任务名称", // Chinese: Task Name
				MinWidth: 15,
				Render: func(row any, _ string, w int, _ bool) string {
					wf := row.(*testWorkflow)
					return styles.TruncateString(wf.Name, w)
				},
			},
			{
				Key:    "status",
				Header: "状态", // Chinese: Status
				Width:  6,
				Render: func(row any, _ string, w int, _ bool) string {
					wf := row.(*testWorkflow)
					return styles.TruncateString(wf.Status, w)
				},
			},
		},
		ShowHeader:   true,
		ShowBorder:   true,
		Title:        "🚀 Workflows",
		EmptyMessage: "🤷 No workflows",
	}
	rows := []any{
		&testWorkflow{ID: 1, Name: "构建认证系统", Status: "运行中", Priority: "high"},
		&testWorkflow{ID: 2, Name: "修复支付错误", Status: "已暂停", Priority: "medium"},
		&testWorkflow{ID: 3, Name: "Deploy café app ☕", Status: "done", Priority: "low"},
	}
	tbl := New(cfg).
		SetRows(rows).
		SetSize(60, 10)

	view := tbl.View()
	teatest.RequireEqualOutput(t, []byte(view))
}

// Unit tests for public API

func TestNew_Validation_NoColumns(t *testing.T) {
	cfg := TableConfig{
		Columns: []ColumnConfig{},
	}

	require.Panics(t, func() {
		New(cfg)
	}, "New should panic when Columns is empty")
}

func TestNew_Validation_MissingRender(t *testing.T) {
	cfg := TableConfig{
		Columns: []ColumnConfig{
			{Key: "test", Header: "Test", Render: nil},
		},
	}

	require.Panics(t, func() {
		New(cfg)
	}, "New should panic when column has nil Render")
}

func TestNew_Validation_Valid(t *testing.T) {
	cfg := TableConfig{
		Columns: []ColumnConfig{
			{
				Key:    "test",
				Header: "Test",
				Render: func(row any, _ string, w int, _ bool) string { return "test" },
			},
		},
	}

	require.NotPanics(t, func() {
		New(cfg)
	}, "New should not panic with valid config")
}

func TestNew_DefaultEmptyMessage(t *testing.T) {
	cfg := TableConfig{
		Columns: []ColumnConfig{
			{
				Key:    "test",
				Header: "Test",
				Render: func(row any, _ string, w int, _ bool) string { return "test" },
			},
		},
		EmptyMessage: "", // Empty should use default
	}

	tbl := New(cfg)
	// The config should have "No data" as default
	require.Equal(t, "No data", tbl.config.EmptyMessage)
}

func TestSetRows_Immutable(t *testing.T) {
	cfg := createGoldenTestConfig()
	tbl1 := New(cfg)
	rows := createTestWorkflows()

	tbl2 := tbl1.SetRows(rows)

	require.Equal(t, 0, tbl1.RowCount(), "original model should be unchanged")
	require.Equal(t, 5, tbl2.RowCount(), "new model should have rows")
}

func TestSetSize_Immutable(t *testing.T) {
	cfg := createGoldenTestConfig()
	tbl1 := New(cfg)

	tbl2 := tbl1.SetSize(100, 50)

	require.Equal(t, 0, tbl1.width, "original model width should be unchanged")
	require.Equal(t, 0, tbl1.height, "original model height should be unchanged")
	require.Equal(t, 100, tbl2.width, "new model should have new width")
	require.Equal(t, 50, tbl2.height, "new model should have new height")
}

func TestRowCount(t *testing.T) {
	cfg := createGoldenTestConfig()
	tbl := New(cfg)

	require.Equal(t, 0, tbl.RowCount(), "empty table should have 0 rows")

	rows := createTestWorkflows()
	tbl = tbl.SetRows(rows)
	require.Equal(t, 5, tbl.RowCount(), "table with 5 rows should report 5")
}

func TestView_ZeroDimensions(t *testing.T) {
	cfg := createGoldenTestConfig()
	rows := createTestWorkflows()

	tbl := New(cfg).SetRows(rows)

	// Zero width
	tbl = tbl.SetSize(0, 20)
	require.Empty(t, tbl.View(), "zero width should return empty string")

	// Zero height
	tbl = tbl.SetSize(80, 0)
	require.Empty(t, tbl.View(), "zero height should return empty string")

	// Negative dimensions
	tbl = tbl.SetSize(-10, 20)
	require.Empty(t, tbl.View(), "negative width should return empty string")

	tbl = tbl.SetSize(80, -5)
	require.Empty(t, tbl.View(), "negative height should return empty string")
}

func TestViewWithSelection_OutOfBounds(t *testing.T) {
	cfg := createGoldenTestConfig()
	rows := createTestWorkflows() // 5 rows
	tbl := New(cfg).SetRows(rows).SetSize(80, 15)

	// Negative index - should render without selection (no panic)
	view1 := tbl.ViewWithSelection(-1)
	require.NotEmpty(t, view1, "negative index should still render")

	// Index beyond rows - should render without selection (no panic)
	view2 := tbl.ViewWithSelection(100)
	require.NotEmpty(t, view2, "out-of-bounds index should still render")
}

func TestView_ChainedCalls(t *testing.T) {
	cfg := createGoldenTestConfig()
	rows := createTestWorkflows()

	// Test that chaining works correctly
	view := New(cfg).
		SetRows(rows).
		SetSize(80, 15).
		ViewWithSelection(0)

	require.NotEmpty(t, view, "chained calls should produce valid view")
}
