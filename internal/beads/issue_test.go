package beads

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIssue_Title(t *testing.T) {
	issue := Issue{ID: "bd-123", TitleText: "Test issue"}
	got := issue.Title()
	want := "bd-123 Test issue"
	require.Equal(t, want, got)
}

func TestIssue_Description(t *testing.T) {
	issue := Issue{Type: TypeTask, Priority: PriorityHigh}
	got := issue.Description()
	want := "task - P1"
	require.Equal(t, want, got)
}

func TestIssue_FilterValue(t *testing.T) {
	issue := Issue{TitleText: "Search this"}
	got := issue.FilterValue()
	want := "Search this"
	require.Equal(t, want, got)
}
