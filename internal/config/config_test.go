package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateColumns_Empty(t *testing.T) {
	err := ValidateColumns(nil)
	require.NoError(t, err, "empty columns should be valid (uses defaults)")
}

func TestValidateColumns_Valid(t *testing.T) {
	cols := []ColumnConfig{
		{Name: "Todo", Query: "status = open"},
		{Name: "In Progress", Query: "status = in_progress"},
		{Name: "Done", Query: "status = closed"},
	}
	err := ValidateColumns(cols)
	require.NoError(t, err)
}

func TestValidateColumns_MissingQuery(t *testing.T) {
	cols := []ColumnConfig{
		{Name: "All Issues", Query: ""}, // Missing query
	}
	err := ValidateColumns(cols)
	require.Error(t, err)
	require.Contains(t, err.Error(), "query is required")
}

func TestValidateColumns_ValidComplexQuery(t *testing.T) {
	cols := []ColumnConfig{
		{Name: "Active", Query: "status in (open, in_progress)"},
		{Name: "Done", Query: "status = closed"},
	}
	err := ValidateColumns(cols)
	require.NoError(t, err)
}

func TestValidateColumns_MissingName(t *testing.T) {
	cols := []ColumnConfig{
		{Name: "", Query: "status = open"},
	}
	err := ValidateColumns(cols)
	require.Error(t, err)
	require.Contains(t, err.Error(), "column 0: name is required")
}

func TestValidateColumns_SecondColumnMissingQuery(t *testing.T) {
	cols := []ColumnConfig{
		{Name: "Good", Query: "status = open"},
		{Name: "Bad", Query: ""},
	}
	err := ValidateColumns(cols)
	require.Error(t, err)
	require.Contains(t, err.Error(), "column 1")
	require.Contains(t, err.Error(), "query is required")
}

func TestDefaultColumns(t *testing.T) {
	cols := DefaultColumns()
	require.Len(t, cols, 4)

	require.Equal(t, "Blocked", cols[0].Name)
	require.Equal(t, "status = open and blocked = true", cols[0].Query)

	require.Equal(t, "Ready", cols[1].Name)
	require.Equal(t, "status = open and ready = true", cols[1].Query)

	require.Equal(t, "In Progress", cols[2].Name)
	require.Equal(t, "status = in_progress", cols[2].Query)

	require.Equal(t, "Closed", cols[3].Name)
	require.Equal(t, "status = closed", cols[3].Query)
}

func TestDefaults(t *testing.T) {
	cfg := Defaults()

	require.True(t, cfg.AutoRefresh)
	require.Len(t, cfg.Views, 1)
	require.Equal(t, "Default", cfg.Views[0].Name)
	require.Len(t, cfg.Views[0].Columns, 4)
}

func TestDefaultViews(t *testing.T) {
	views := DefaultViews()
	require.Len(t, views, 1)
	require.Equal(t, "Default", views[0].Name)
	require.Len(t, views[0].Columns, 4)
}

func TestConfig_GetColumns(t *testing.T) {
	cfg := Defaults()
	cols := cfg.GetColumns()
	require.Len(t, cols, 4)
	require.Equal(t, "Blocked", cols[0].Name)
}

func TestConfig_GetColumns_Empty(t *testing.T) {
	cfg := Config{} // No views
	cols := cfg.GetColumns()
	// Should return defaults
	require.Len(t, cols, 4)
}

func TestConfig_SetColumns(t *testing.T) {
	cfg := Defaults()
	newCols := []ColumnConfig{{Name: "Test", Query: "status = open"}}
	cfg.SetColumns(newCols)

	require.Len(t, cfg.Views[0].Columns, 1)
	require.Equal(t, "Test", cfg.Views[0].Columns[0].Name)
}

func TestConfig_SetColumns_NoViews(t *testing.T) {
	cfg := Config{} // No views
	newCols := []ColumnConfig{{Name: "Test", Query: "status = open"}}
	cfg.SetColumns(newCols)

	require.Len(t, cfg.Views, 1)
	require.Equal(t, "Default", cfg.Views[0].Name)
	require.Len(t, cfg.Views[0].Columns, 1)
}

func TestValidateViews_Empty(t *testing.T) {
	err := ValidateViews(nil)
	require.NoError(t, err, "empty views should be valid (uses defaults)")
}

func TestValidateViews_Valid(t *testing.T) {
	views := []ViewConfig{
		{
			Name: "Test",
			Columns: []ColumnConfig{
				{Name: "Open", Query: "status = open"},
			},
		},
	}
	err := ValidateViews(views)
	require.NoError(t, err)
}

func TestValidateViews_MissingName(t *testing.T) {
	views := []ViewConfig{
		{
			Name: "",
			Columns: []ColumnConfig{
				{Name: "Open", Query: "status = open"},
			},
		},
	}
	err := ValidateViews(views)
	require.Error(t, err)
	require.Contains(t, err.Error(), "view 0: name is required")
}

func TestValidateViews_EmptyColumns(t *testing.T) {
	// Empty columns array is valid - will show empty state UI
	views := []ViewConfig{
		{
			Name:    "Empty",
			Columns: []ColumnConfig{},
		},
	}
	err := ValidateViews(views)
	require.NoError(t, err)
}

func TestValidateViews_InvalidColumn(t *testing.T) {
	views := []ViewConfig{
		{
			Name: "Bad",
			Columns: []ColumnConfig{
				{Name: "Missing Query", Query: ""},
			},
		},
	}
	err := ValidateViews(views)
	require.Error(t, err)
	require.Contains(t, err.Error(), "query is required")
}

func TestConfig_GetColumnsForView(t *testing.T) {
	cfg := Config{
		Views: []ViewConfig{
			{Name: "View1", Columns: []ColumnConfig{{Name: "Col1", Query: "q1"}}},
			{Name: "View2", Columns: []ColumnConfig{{Name: "Col2", Query: "q2"}}},
		},
	}

	cols0 := cfg.GetColumnsForView(0)
	require.Len(t, cols0, 1)
	require.Equal(t, "Col1", cols0[0].Name)

	cols1 := cfg.GetColumnsForView(1)
	require.Len(t, cols1, 1)
	require.Equal(t, "Col2", cols1[0].Name)
}

func TestConfig_GetColumnsForView_OutOfRange(t *testing.T) {
	cfg := Config{
		Views: []ViewConfig{
			{Name: "View1", Columns: []ColumnConfig{{Name: "Col1", Query: "q1"}}},
		},
	}

	// Out of range should return defaults
	cols := cfg.GetColumnsForView(5)
	require.Len(t, cols, 4) // DefaultColumns has 4
}

func TestConfig_SetColumnsForView(t *testing.T) {
	cfg := Config{
		Views: []ViewConfig{
			{Name: "View1", Columns: []ColumnConfig{{Name: "Col1", Query: "q1"}}},
			{Name: "View2", Columns: []ColumnConfig{{Name: "Col2", Query: "q2"}}},
		},
	}

	newCols := []ColumnConfig{{Name: "Updated", Query: "updated"}}
	cfg.SetColumnsForView(1, newCols)

	// View1 unchanged
	require.Equal(t, "Col1", cfg.Views[0].Columns[0].Name)
	// View2 updated
	require.Equal(t, "Updated", cfg.Views[1].Columns[0].Name)
}

func TestConfig_SetColumnsForView_OutOfRange(t *testing.T) {
	cfg := Config{
		Views: []ViewConfig{
			{Name: "View1", Columns: []ColumnConfig{{Name: "Col1", Query: "q1"}}},
		},
	}

	newCols := []ColumnConfig{{Name: "Updated", Query: "updated"}}
	cfg.SetColumnsForView(5, newCols) // Out of range - should do nothing

	// Original unchanged
	require.Equal(t, "Col1", cfg.Views[0].Columns[0].Name)
}
