// Package styles contains Lip Gloss style definitions.
package styles

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Border characters (rounded)
const (
	borderTopLeft     = "╭"
	borderTopRight    = "╮"
	borderBottomLeft  = "╰"
	borderBottomRight = "╯"
	borderHorizontal  = "─"
	borderVertical    = "│"
	borderMiddleLeft  = "├"
	borderMiddleRight = "┤"
)

// RenderWithTitleBorder renders content with titles embedded in the top border.
// leftTitle appears on the left, rightTitle appears on the right. Pass "" to omit a title.
// titleColor is used for the title text, focusedBorderColor is used for the border when focused.
func RenderWithTitleBorder(content, leftTitle, rightTitle string, width, height int, focused bool, titleColor, focusedBorderColor lipgloss.TerminalColor) string {
	// Border color: use focusedBorderColor when focused, BorderDefaultColor when not
	var borderColor lipgloss.TerminalColor = BorderDefaultColor
	if focused {
		borderColor = focusedBorderColor
	}

	borderStyle := lipgloss.NewStyle().Foreground(borderColor)
	titleStyle := lipgloss.NewStyle().Foreground(titleColor)

	// Calculate inner width (excluding border characters)
	innerWidth := max(width-2, 1) // -2 for left and right border

	// Build top border with embedded titles (handles empty titles correctly)
	// Format: ╭─ LeftTitle ─────────────────── RightTitle ─╮
	topBorder := buildDualTitleTopBorder(leftTitle, rightTitle, innerWidth, borderStyle, titleStyle)

	// Build bottom border
	bottomBorder := borderStyle.Render(borderBottomLeft + strings.Repeat(borderHorizontal, innerWidth) + borderBottomRight)

	// Calculate content height (excluding top and bottom borders)
	contentHeight := max(height-2, 1)

	// Use lipgloss to constrain content width (handles wrapping/truncation properly)
	contentStyle := lipgloss.NewStyle().Width(innerWidth).Height(contentHeight)
	constrainedContent := contentStyle.Render(content)

	// Split constrained content into lines
	contentLines := strings.Split(constrainedContent, "\n")
	paddedLines := make([]string, contentHeight)

	for i := range contentHeight {
		var line string
		if i < len(contentLines) {
			line = contentLines[i]
		}

		// Pad line to innerWidth to ensure right border aligns
		lineWidth := lipgloss.Width(line)
		if lineWidth < innerWidth {
			line = line + strings.Repeat(" ", innerWidth-lineWidth)
		}

		// Add side borders
		paddedLines[i] = borderStyle.Render(borderVertical) + line + borderStyle.Render(borderVertical)
	}

	// Join all parts
	var result strings.Builder
	result.WriteString(topBorder)
	result.WriteString("\n")
	result.WriteString(strings.Join(paddedLines, "\n"))
	result.WriteString("\n")
	result.WriteString(bottomBorder)

	return result.String()
}

// buildTopBorder creates the top border with embedded title.
// borderStyle is used for border characters, titleStyle for the title text.
func buildTopBorder(title string, innerWidth int, borderStyle, titleStyle lipgloss.Style) string {
	// Format: ╭─ Title ──────╮
	// Minimum: ╭─╮ (3 chars for just borders)

	if innerWidth < 1 {
		return borderStyle.Render(borderTopLeft + borderTopRight)
	}

	// If title is empty, just render a plain top border
	if title == "" {
		return borderStyle.Render(borderTopLeft + strings.Repeat(borderHorizontal, innerWidth) + borderTopRight)
	}

	// Calculate space for title
	// Format: ─ Title ─
	// We need at least 4 chars: "─ " + " ─"
	titlePartMinWidth := 4

	if innerWidth < titlePartMinWidth {
		// Too narrow for title, just render plain border
		return borderStyle.Render(borderTopLeft + strings.Repeat(borderHorizontal, innerWidth) + borderTopRight)
	}

	// Calculate available space for title text
	availableForTitle := innerWidth - 4 // "─ " before and " ─" after (minimum)

	displayTitle := title
	if lipgloss.Width(displayTitle) > availableForTitle {
		// Truncate title with ellipsis
		displayTitle = truncateString(displayTitle, availableForTitle)
	}

	// Calculate remaining width for trailing dashes
	// Inner: "─ " (2) + title + " " (1) + dashes = innerWidth
	// So: dashes = innerWidth - 3 - titleTextWidth
	titleTextWidth := lipgloss.Width(displayTitle)
	remainingWidth := max(innerWidth-3-titleTextWidth, 0)

	// Build: ╭─ Title ──────╮
	// Border parts use borderStyle, title text uses titleStyle
	return borderStyle.Render(borderTopLeft+borderHorizontal+" ") +
		titleStyle.Render(displayTitle) +
		borderStyle.Render(" "+strings.Repeat(borderHorizontal, remainingWidth)+borderTopRight)
}

// buildDualTitleTopBorder creates the top border with titles on both left and right.
// Format: ╭─ LeftTitle ─────────────────── RightTitle ─╮
func buildDualTitleTopBorder(leftTitle, rightTitle string, innerWidth int, borderStyle, titleStyle lipgloss.Style) string {
	if innerWidth < 1 {
		return borderStyle.Render(borderTopLeft + borderTopRight)
	}

	// If both titles are empty, just render a plain top border
	if leftTitle == "" && rightTitle == "" {
		return borderStyle.Render(borderTopLeft + strings.Repeat(borderHorizontal, innerWidth) + borderTopRight)
	}

	// Calculate widths
	leftTitleWidth := lipgloss.Width(leftTitle)
	rightTitleWidth := lipgloss.Width(rightTitle)

	// Minimum format: "─ Left ─ Right ─" = 2 + left + 1 + middle + 1 + right + 1
	// We need space for: "─ " + leftTitle + " " + middleDashes + " " + rightTitle + " ─"
	minRequired := 2 + leftTitleWidth + 1 + 1 + 1 + rightTitleWidth + 2
	if rightTitle == "" {
		// Just left title: "─ " + leftTitle + " " + dashes
		minRequired = 2 + leftTitleWidth + 1 + 1
	}
	if leftTitle == "" {
		// Just right title: dashes + " " + rightTitle + " ─"
		minRequired = 1 + 1 + rightTitleWidth + 2
	}

	if innerWidth < minRequired {
		// Too narrow, fall back to simple border or just left title
		if leftTitle != "" {
			return buildTopBorder(leftTitle, innerWidth, borderStyle, titleStyle)
		}
		return borderStyle.Render(borderTopLeft + strings.Repeat(borderHorizontal, innerWidth) + borderTopRight)
	}

	// Calculate middle dashes
	// Format: ╭─ Left ───────── Right ─╮
	// innerWidth = 2 + leftWidth + 1 + middleDashes + 1 + rightWidth + 2
	// middleDashes = innerWidth - 2 - leftWidth - 1 - 1 - rightWidth - 2
	//              = innerWidth - leftWidth - rightWidth - 6
	var middleDashes int
	if leftTitle != "" && rightTitle != "" {
		middleDashes = innerWidth - leftTitleWidth - rightTitleWidth - 6
	} else if leftTitle != "" {
		// No right title: ╭─ Left ─────────╮
		middleDashes = innerWidth - leftTitleWidth - 3
	} else {
		// No left title: ╭───────── Right ─╮
		middleDashes = innerWidth - rightTitleWidth - 3
	}
	middleDashes = max(middleDashes, 1)

	// Build the border
	var result strings.Builder
	result.WriteString(borderStyle.Render(borderTopLeft))

	if leftTitle != "" {
		result.WriteString(borderStyle.Render(borderHorizontal + " "))
		result.WriteString(titleStyle.Render(leftTitle))
		result.WriteString(borderStyle.Render(" "))
	}

	result.WriteString(borderStyle.Render(strings.Repeat(borderHorizontal, middleDashes)))

	if rightTitle != "" {
		result.WriteString(borderStyle.Render(" "))
		result.WriteString(titleStyle.Render(rightTitle))
		result.WriteString(borderStyle.Render(" " + borderHorizontal))
	}

	result.WriteString(borderStyle.Render(borderTopRight))

	return result.String()
}

// truncateString truncates a string to fit within maxWidth, adding ellipsis if needed.
func truncateString(s string, maxWidth int) string {
	if maxWidth < 1 {
		return ""
	}

	if lipgloss.Width(s) <= maxWidth {
		return s
	}

	// Need to truncate - leave room for ellipsis
	if maxWidth <= 3 {
		return strings.Repeat(".", maxWidth)
	}

	// Truncate rune by rune
	result := ""
	for _, r := range s {
		test := result + string(r)
		if lipgloss.Width(test) > maxWidth-3 {
			break
		}
		result = test
	}

	return result + "..."
}
