package table

// calculateColumnWidths distributes available width across columns using a two-pass algorithm.
//
// Algorithm:
//  1. First pass: Allocate fixed widths (columns with Width > 0)
//  2. Second pass: Distribute remaining width evenly to flex columns (Width = 0)
//  3. Apply MinWidth/MaxWidth constraints to flex columns
//  4. Enforce minimum width of 3 for all columns (for ellipsis "…")
//  5. Account for column separators (1 char between each column)
//
// Parameters:
//   - cols: Column configurations
//   - totalWidth: Total available width for the table content (inside borders)
//
// Returns:
//   - Slice of calculated widths for each column
func calculateColumnWidths(cols []ColumnConfig, totalWidth int) []int {
	if len(cols) == 0 {
		return []int{}
	}

	widths := make([]int, len(cols))
	flexCols := []int{}

	// Calculate separator space: (n-1) separators for n columns
	separatorSpace := len(cols) - 1
	availableWidth := totalWidth - separatorSpace

	// First pass: allocate fixed widths and identify flex columns
	for i, col := range cols {
		if col.Width > 0 {
			widths[i] = col.Width
			availableWidth -= col.Width
		} else {
			flexCols = append(flexCols, i)
		}
	}

	// Second pass: distribute remaining width to flex columns
	if len(flexCols) > 0 {
		if availableWidth <= 0 {
			// Not enough space - give each flex column minimum width
			for _, i := range flexCols {
				widths[i] = minColumnWidth
			}
		} else {
			perCol := availableWidth / len(flexCols)
			remainder := availableWidth % len(flexCols)

			for j, i := range flexCols {
				w := perCol
				// Distribute remainder to first columns
				if j < remainder {
					w++
				}

				// Apply MinWidth constraint
				minW := max(cols[i].MinWidth, minColumnWidth)
				if w < minW {
					w = minW
				}

				// Apply MaxWidth constraint
				if cols[i].MaxWidth > 0 && w > cols[i].MaxWidth {
					w = cols[i].MaxWidth
				}

				widths[i] = w
			}
		}
	}

	// Final pass: enforce minimum width of 3 for all columns
	for i := range widths {
		if widths[i] < minColumnWidth {
			widths[i] = minColumnWidth
		}
	}

	return widths
}

// minColumnWidth is the minimum width for any column to ensure at least "…" can be displayed.
const minColumnWidth = 3
