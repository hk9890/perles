package editor

import (
	"testing"

	"github.com/stretchr/testify/require"

	beads "github.com/hk9890/perles/internal/beads/domain"
)

func TestIssueMarkdown_RoundTrip(t *testing.T) {
	issue := beads.Issue{
		ID:              "perles-123",
		TitleText:       "Original title",
		DescriptionText: "Line 1\n\nLine 2",
		Notes:           "Note line A\nNote line B",
		Labels:          []string{"bug", "needs:discussion"},
		Type:            beads.TypeStory,
		Status:          beads.StatusInProgress,
		Priority:        beads.PriorityHigh,
	}

	serialized := MarshalIssueMarkdown(issue)
	parsed, err := ParseIssueMarkdown(serialized)
	require.NoError(t, err)

	require.Equal(t, issue.TitleText, parsed.Title)
	require.Equal(t, issue.DescriptionText, parsed.Description)
	require.Equal(t, issue.Notes, parsed.Notes)
	require.Equal(t, issue.Type, parsed.Type)
	require.Equal(t, issue.Status, parsed.Status)
	require.Equal(t, issue.Priority, parsed.Priority)
	require.Equal(t, issue.Labels, parsed.Labels)
}

func TestIssueMarkdown_ParseValidation(t *testing.T) {
	_, err := ParseIssueMarkdown("## Title\nX\n## Description\nY")
	require.Error(t, err)
	require.Contains(t, err.Error(), "missing section")

	_, err = ParseIssueMarkdown(`# Perles Issue External Edit

## Title
X

## Description
Y

## Notes
Z

## Labels
- bug

## Type
task

## Status
hooked

## Priority
P2
`)
	require.NoError(t, err)

	_, err = ParseIssueMarkdown(`# Perles Issue External Edit

## Title
X

## Description
Y

## Notes
Z

## Labels
- bug

## Type
task

## Status
invalid

## Priority
P2
`)
	require.NoError(t, err)

	_, err = ParseIssueMarkdown(`# Perles Issue External Edit

## Title
X

## Description
Y

## Notes
Z

## Labels
- bug

## Type
task

## Status


## Priority
P2
`)
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid status")

	parsed, err := ParseIssueMarkdown(`# Perles Issue External Edit

## Title
X

## Description
Y

## Notes
Z

## Labels
- bug

## Type
custom_flow

## Status
qa_testing

## Priority
P2
`)
	require.NoError(t, err)
	require.Equal(t, beads.IssueType("custom_flow"), parsed.Type)
	require.Equal(t, beads.Status("qa_testing"), parsed.Status)
}
