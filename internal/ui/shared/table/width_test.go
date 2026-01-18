package table

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCalculateColumnWidths_AllFixed(t *testing.T) {
	// All columns have fixed widths
	cols := []ColumnConfig{
		{Key: "id", Width: 5},
		{Key: "name", Width: 20},
		{Key: "status", Width: 10},
	}

	// Total width: 5 + 20 + 10 = 35, plus 2 separators = 37 minimum
	widths := calculateColumnWidths(cols, 100)

	require.Len(t, widths, 3)
	require.Equal(t, 5, widths[0], "first column should have fixed width 5")
	require.Equal(t, 20, widths[1], "second column should have fixed width 20")
	require.Equal(t, 10, widths[2], "third column should have fixed width 10")
}

func TestCalculateColumnWidths_AllFlex(t *testing.T) {
	// All columns are flex (Width=0)
	cols := []ColumnConfig{
		{Key: "col1", Width: 0},
		{Key: "col2", Width: 0},
		{Key: "col3", Width: 0},
	}

	// Total 100, minus 2 separators = 98 available
	// 98 / 3 = 32 each, remainder 2 goes to first 2 columns
	widths := calculateColumnWidths(cols, 100)

	require.Len(t, widths, 3)
	require.Equal(t, 33, widths[0], "first flex column gets +1 from remainder")
	require.Equal(t, 33, widths[1], "second flex column gets +1 from remainder")
	require.Equal(t, 32, widths[2], "third flex column gets base allocation")
}

func TestCalculateColumnWidths_Mixed(t *testing.T) {
	// Mix of fixed and flex columns
	cols := []ColumnConfig{
		{Key: "id", Width: 5},     // Fixed
		{Key: "name", Width: 0},   // Flex
		{Key: "status", Width: 8}, // Fixed
		{Key: "desc", Width: 0},   // Flex
	}

	// Total 100, minus 3 separators = 97
	// Fixed: 5 + 8 = 13, remaining: 97 - 13 = 84
	// 84 / 2 = 42 each
	widths := calculateColumnWidths(cols, 100)

	require.Len(t, widths, 4)
	require.Equal(t, 5, widths[0], "first column (fixed) should be 5")
	require.Equal(t, 42, widths[1], "second column (flex) should get half remaining")
	require.Equal(t, 8, widths[2], "third column (fixed) should be 8")
	require.Equal(t, 42, widths[3], "fourth column (flex) should get half remaining")
}

func TestCalculateColumnWidths_MinMaxConstraints(t *testing.T) {
	t.Run("MinWidth enforced", func(t *testing.T) {
		cols := []ColumnConfig{
			{Key: "col1", Width: 0, MinWidth: 20},
			{Key: "col2", Width: 0, MinWidth: 20},
		}

		// Total 30, minus 1 separator = 29
		// 29 / 2 = 14 each, but MinWidth is 20
		widths := calculateColumnWidths(cols, 30)

		require.Len(t, widths, 2)
		require.Equal(t, 20, widths[0], "first column should respect MinWidth")
		require.Equal(t, 20, widths[1], "second column should respect MinWidth")
	})

	t.Run("MaxWidth enforced", func(t *testing.T) {
		cols := []ColumnConfig{
			{Key: "col1", Width: 0, MaxWidth: 10},
			{Key: "col2", Width: 0, MaxWidth: 15},
		}

		// Total 100, minus 1 separator = 99
		// 99 / 2 = 49-50 each, but MaxWidths are 10 and 15
		widths := calculateColumnWidths(cols, 100)

		require.Len(t, widths, 2)
		require.Equal(t, 10, widths[0], "first column should respect MaxWidth")
		require.Equal(t, 15, widths[1], "second column should respect MaxWidth")
	})

	t.Run("MinWidth takes precedence over base minimum", func(t *testing.T) {
		cols := []ColumnConfig{
			{Key: "col1", Width: 0, MinWidth: 10},
		}

		// MinWidth (10) > minColumnWidth (3)
		widths := calculateColumnWidths(cols, 50)

		require.Len(t, widths, 1)
		require.GreaterOrEqual(t, widths[0], 10, "should respect MinWidth over base minimum")
	})
}

func TestCalculateColumnWidths_NarrowTotal(t *testing.T) {
	t.Run("Insufficient width for fixed columns", func(t *testing.T) {
		cols := []ColumnConfig{
			{Key: "col1", Width: 30},
			{Key: "col2", Width: 30},
			{Key: "col3", Width: 0}, // Flex column
		}

		// Total 50, minus 2 separators = 48
		// Fixed: 30 + 30 = 60, but only 48 available
		// Flex column gets minimum width (3)
		widths := calculateColumnWidths(cols, 50)

		require.Len(t, widths, 3)
		require.Equal(t, 30, widths[0], "fixed columns retain their width")
		require.Equal(t, 30, widths[1], "fixed columns retain their width")
		require.Equal(t, 3, widths[2], "flex column gets minimum width")
	})

	t.Run("Very narrow total gives minimum to all flex", func(t *testing.T) {
		cols := []ColumnConfig{
			{Key: "col1", Width: 0},
			{Key: "col2", Width: 0},
		}

		// Total 5, minus 1 separator = 4
		// 4 / 2 = 2 each, but minimum is 3
		widths := calculateColumnWidths(cols, 5)

		require.Len(t, widths, 2)
		require.Equal(t, 3, widths[0], "should enforce minimum width")
		require.Equal(t, 3, widths[1], "should enforce minimum width")
	})

	t.Run("Zero total gives minimum to all", func(t *testing.T) {
		cols := []ColumnConfig{
			{Key: "col1", Width: 0},
			{Key: "col2", Width: 10}, // Fixed but total is 0
		}

		widths := calculateColumnWidths(cols, 0)

		require.Len(t, widths, 2)
		require.Equal(t, 3, widths[0], "flex column gets minimum")
		require.Equal(t, 10, widths[1], "fixed column retains width")
	})
}

func TestCalculateColumnWidths_EmptyColumns(t *testing.T) {
	widths := calculateColumnWidths([]ColumnConfig{}, 100)
	require.Empty(t, widths, "empty columns should return empty widths")
}

func TestCalculateColumnWidths_SingleColumn(t *testing.T) {
	t.Run("Single fixed column", func(t *testing.T) {
		cols := []ColumnConfig{
			{Key: "only", Width: 25},
		}

		// No separators for single column
		widths := calculateColumnWidths(cols, 100)

		require.Len(t, widths, 1)
		require.Equal(t, 25, widths[0], "single fixed column keeps its width")
	})

	t.Run("Single flex column", func(t *testing.T) {
		cols := []ColumnConfig{
			{Key: "only", Width: 0},
		}

		// No separators for single column, gets all available width
		widths := calculateColumnWidths(cols, 100)

		require.Len(t, widths, 1)
		require.Equal(t, 100, widths[0], "single flex column gets all width")
	})
}

func TestCalculateColumnWidths_RemainderDistribution(t *testing.T) {
	// Test that remainder is distributed evenly to first columns
	cols := []ColumnConfig{
		{Key: "col1", Width: 0},
		{Key: "col2", Width: 0},
		{Key: "col3", Width: 0},
		{Key: "col4", Width: 0},
		{Key: "col5", Width: 0},
	}

	// Total 100, minus 4 separators = 96
	// 96 / 5 = 19 with remainder 1
	// First column gets 20, rest get 19
	widths := calculateColumnWidths(cols, 100)

	require.Len(t, widths, 5)
	require.Equal(t, 20, widths[0], "first column gets remainder")
	require.Equal(t, 19, widths[1])
	require.Equal(t, 19, widths[2])
	require.Equal(t, 19, widths[3])
	require.Equal(t, 19, widths[4])

	// Verify total (excluding separators) matches available
	total := 0
	for _, w := range widths {
		total += w
	}
	require.Equal(t, 96, total, "total widths should equal available space")
}

func TestCalculateColumnWidths_FixedTooSmall(t *testing.T) {
	// Fixed columns smaller than minimum should be enforced to minimum
	cols := []ColumnConfig{
		{Key: "tiny", Width: 1},
		{Key: "normal", Width: 10},
	}

	widths := calculateColumnWidths(cols, 50)

	require.Len(t, widths, 2)
	require.Equal(t, 3, widths[0], "fixed width below minimum should be enforced to minimum")
	require.Equal(t, 10, widths[1], "normal fixed width unchanged")
}
