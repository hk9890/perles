package table

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// dummyRender is a simple render callback for testing
func dummyRender(row any, key string, width int, selected bool) string {
	return "test"
}

func TestValidateConfig_NoColumns(t *testing.T) {
	cfg := TableConfig{
		Columns: []ColumnConfig{},
	}

	err := ValidateConfig(cfg)
	require.Error(t, err, "expected error for empty columns")
	require.Contains(t, err.Error(), "at least one column is required")
}

func TestValidateConfig_MissingRender(t *testing.T) {
	t.Run("single column with nil render", func(t *testing.T) {
		cfg := TableConfig{
			Columns: []ColumnConfig{
				{Key: "name", Header: "Name", Render: nil},
			},
		}

		err := ValidateConfig(cfg)
		require.Error(t, err, "expected error for nil Render callback")
		require.Contains(t, err.Error(), "name")
		require.Contains(t, err.Error(), "nil Render callback")
	})

	t.Run("second column with nil render", func(t *testing.T) {
		cfg := TableConfig{
			Columns: []ColumnConfig{
				{Key: "id", Header: "#", Render: dummyRender},
				{Key: "status", Header: "Status", Render: nil},
			},
		}

		err := ValidateConfig(cfg)
		require.Error(t, err, "expected error for nil Render callback on second column")
		require.Contains(t, err.Error(), "status")
		require.Contains(t, err.Error(), "nil Render callback")
	})

	t.Run("column without key has nil render", func(t *testing.T) {
		cfg := TableConfig{
			Columns: []ColumnConfig{
				{Header: "No Key", Render: nil},
			},
		}

		err := ValidateConfig(cfg)
		require.Error(t, err, "expected error for nil Render callback")
		require.Contains(t, err.Error(), "nil Render callback")
	})
}

func TestValidateConfig_Valid(t *testing.T) {
	t.Run("single column with render", func(t *testing.T) {
		cfg := TableConfig{
			Columns: []ColumnConfig{
				{Key: "name", Header: "Name", Render: dummyRender},
			},
		}

		err := ValidateConfig(cfg)
		require.NoError(t, err, "expected valid config to pass validation")
	})

	t.Run("multiple columns with render", func(t *testing.T) {
		cfg := TableConfig{
			Columns: []ColumnConfig{
				{Key: "id", Header: "#", Width: 3, Render: dummyRender},
				{Key: "name", Header: "Name", MinWidth: 10, Render: dummyRender},
				{Key: "status", Header: "Status", Type: ColumnTypeIcon, Render: dummyRender},
			},
			ShowHeader: true,
			ShowBorder: true,
			Title:      "Test Table",
		}

		err := ValidateConfig(cfg)
		require.NoError(t, err, "expected valid config with multiple columns to pass validation")
	})

	t.Run("with all column types", func(t *testing.T) {
		cfg := TableConfig{
			Columns: []ColumnConfig{
				{Key: "text", Header: "Text", Type: ColumnTypeText, Render: dummyRender},
				{Key: "icon", Header: "Icon", Type: ColumnTypeIcon, Render: dummyRender},
				{Key: "date", Header: "Date", Type: ColumnTypeDate, Render: dummyRender},
				{Key: "num", Header: "Number", Type: ColumnTypeNumber, Render: dummyRender},
				{Key: "custom", Header: "Custom", Type: ColumnTypeCustom, Render: dummyRender},
			},
		}

		err := ValidateConfig(cfg)
		require.NoError(t, err, "expected config with all column types to pass validation")
	})
}

func TestValidateConfig_MixedValidAndInvalid(t *testing.T) {
	// Test that validation iterates through all columns, not just the first
	cfg := TableConfig{
		Columns: []ColumnConfig{
			{Key: "a", Header: "A", Render: dummyRender},
			{Key: "b", Header: "B", Render: dummyRender},
			{Key: "c", Header: "C", Render: nil}, // Invalid
			{Key: "d", Header: "D", Render: dummyRender},
		},
	}

	err := ValidateConfig(cfg)
	require.Error(t, err, "expected error when any column has nil Render")
	require.Contains(t, err.Error(), "c", "expected error message to reference the invalid column key")
}
