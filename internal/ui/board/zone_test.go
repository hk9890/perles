package board

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMakeZoneID(t *testing.T) {
	tests := []struct {
		name     string
		colIdx   int
		issueID  string
		expected string
	}{
		{
			name:     "basic",
			colIdx:   2,
			issueID:  "bd-42",
			expected: "col:2:issue:bd-42",
		},
		{
			name:     "zero column index",
			colIdx:   0,
			issueID:  "bd-123",
			expected: "col:0:issue:bd-123",
		},
		{
			name:     "high column index",
			colIdx:   5,
			issueID:  "perles-abc",
			expected: "col:5:issue:perles-abc",
		},
		{
			name:     "issue ID with hyphen",
			colIdx:   1,
			issueID:  "my-project-123",
			expected: "col:1:issue:my-project-123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := makeZoneID(tt.colIdx, tt.issueID)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestParseZoneID(t *testing.T) {
	tests := []struct {
		name           string
		zoneID         string
		expectedColIdx int
		expectedID     string
		expectedOK     bool
	}{
		{
			name:           "valid input col:0:issue:bd-123",
			zoneID:         "col:0:issue:bd-123",
			expectedColIdx: 0,
			expectedID:     "bd-123",
			expectedOK:     true,
		},
		{
			name:           "valid input col:5:issue:perles-abc",
			zoneID:         "col:5:issue:perles-abc",
			expectedColIdx: 5,
			expectedID:     "perles-abc",
			expectedOK:     true,
		},
		{
			name:           "valid input with high column index",
			zoneID:         "col:99:issue:test-id",
			expectedColIdx: 99,
			expectedID:     "test-id",
			expectedOK:     true,
		},
		{
			name:           "invalid - missing parts",
			zoneID:         "invalid",
			expectedColIdx: 0,
			expectedID:     "",
			expectedOK:     false,
		},
		{
			name:           "invalid - wrong prefix",
			zoneID:         "row:1:issue:bd-1",
			expectedColIdx: 0,
			expectedID:     "",
			expectedOK:     false,
		},
		{
			name:           "invalid - wrong middle part",
			zoneID:         "col:1:item:bd-1",
			expectedColIdx: 0,
			expectedID:     "",
			expectedOK:     false,
		},
		{
			name:           "invalid - non-numeric column index",
			zoneID:         "col:x:issue:bd-1",
			expectedColIdx: 0,
			expectedID:     "",
			expectedOK:     false,
		},
		{
			name:           "invalid - too few parts",
			zoneID:         "col:1:issue",
			expectedColIdx: 0,
			expectedID:     "",
			expectedOK:     false,
		},
		{
			name:           "invalid - empty string",
			zoneID:         "",
			expectedColIdx: 0,
			expectedID:     "",
			expectedOK:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			colIdx, issueID, ok := parseZoneID(tt.zoneID)
			require.Equal(t, tt.expectedOK, ok)
			require.Equal(t, tt.expectedColIdx, colIdx)
			require.Equal(t, tt.expectedID, issueID)
		})
	}
}

func TestMakeAndParseRoundTrip(t *testing.T) {
	// Verify that makeZoneID and parseZoneID are inverses
	testCases := []struct {
		colIdx  int
		issueID string
	}{
		{0, "bd-1"},
		{5, "perles-xyz"},
		{10, "my-issue-999"},
	}

	for _, tc := range testCases {
		zoneID := makeZoneID(tc.colIdx, tc.issueID)
		parsedColIdx, parsedIssueID, ok := parseZoneID(zoneID)
		require.True(t, ok, "parseZoneID should succeed for makeZoneID output")
		require.Equal(t, tc.colIdx, parsedColIdx)
		require.Equal(t, tc.issueID, parsedIssueID)
	}
}
