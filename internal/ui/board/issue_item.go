package board

import (
	"fmt"

	beads "github.com/hk9890/perles/internal/beads/domain"
)

// IssueItem wraps an Issue to implement the bubbles list.Item interface.
type IssueItem struct {
	*beads.Issue
}

// Title returns the display title for the list item.
func (i IssueItem) Title() string {
	return i.ID + " " + i.TitleText
}

// Description returns the description for the list item.
func (i IssueItem) Description() string {
	return string(i.Type) + " - P" + fmt.Sprintf("%d", i.Priority)
}

// FilterValue returns the value used for filtering.
func (i IssueItem) FilterValue() string {
	return i.TitleText
}
