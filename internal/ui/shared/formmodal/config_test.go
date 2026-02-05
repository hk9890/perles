package formmodal

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// --- Column Configuration Schema Tests ---

func TestFieldConfig_DefaultColumn(t *testing.T) {
	// Verify that Column defaults to 0 (zero value)
	fc := FieldConfig{
		Key:   "test",
		Type:  FieldTypeText,
		Label: "Test Field",
	}

	require.Equal(t, 0, fc.Column, "Column should default to 0")
}

func TestFormConfig_EmptyColumns_SingleColumn(t *testing.T) {
	// Verify that empty/nil Columns slice means single-column mode
	cfg := FormConfig{
		Title: "Test Form",
		Fields: []FieldConfig{
			{Key: "field1", Type: FieldTypeText, Label: "Field 1"},
		},
	}

	require.Nil(t, cfg.Columns, "Columns should be nil by default")
	require.Len(t, cfg.Columns, 0, "Columns length should be 0")
}

func TestFormConfig_ColumnGap_Default(t *testing.T) {
	// Verify that ColumnGap defaults to 0 (zero value), with expected default of 3
	cfg := FormConfig{
		Title: "Test Form",
	}

	require.Equal(t, 0, cfg.ColumnGap, "ColumnGap zero value should be 0")

	// The rendering code should treat 0 as defaulting to 3
	// This test verifies the struct's zero value behavior
}

func TestFormConfig_MinMultiColumnWidth_Default(t *testing.T) {
	// Verify that MinMultiColumnWidth defaults to 0 (zero value), with expected default of 100
	cfg := FormConfig{
		Title: "Test Form",
	}

	require.Equal(t, 0, cfg.MinMultiColumnWidth, "MinMultiColumnWidth zero value should be 0")

	// The rendering code should treat 0 as defaulting to 100
	// This test verifies the struct's zero value behavior
}

func TestColumnConfig_MinWidth_Default(t *testing.T) {
	// Verify that ColumnConfig.MinWidth defaults to 0 (flexible)
	cc := ColumnConfig{}

	require.Equal(t, 0, cc.MinWidth, "MinWidth should default to 0 (flexible)")
}

func TestFormConfig_WithColumnsConfigured(t *testing.T) {
	// Verify multi-column configuration works correctly
	cfg := FormConfig{
		Title: "Multi-Column Form",
		Fields: []FieldConfig{
			{Key: "field1", Type: FieldTypeText, Label: "Field 1", Column: 0},
			{Key: "field2", Type: FieldTypeText, Label: "Field 2", Column: 1},
			{Key: "field3", Type: FieldTypeText, Label: "Field 3", Column: 0},
		},
		Columns: []ColumnConfig{
			{MinWidth: 40},
			{MinWidth: 30},
		},
		ColumnGap:           3,
		MinMultiColumnWidth: 100,
	}

	require.Len(t, cfg.Columns, 2, "Should have 2 columns configured")
	require.Equal(t, 40, cfg.Columns[0].MinWidth, "First column MinWidth")
	require.Equal(t, 30, cfg.Columns[1].MinWidth, "Second column MinWidth")
	require.Equal(t, 3, cfg.ColumnGap, "ColumnGap should be 3")
	require.Equal(t, 100, cfg.MinMultiColumnWidth, "MinMultiColumnWidth should be 100")

	// Verify fields have correct column assignments
	require.Equal(t, 0, cfg.Fields[0].Column, "field1 Column")
	require.Equal(t, 1, cfg.Fields[1].Column, "field2 Column")
	require.Equal(t, 0, cfg.Fields[2].Column, "field3 Column")
}
