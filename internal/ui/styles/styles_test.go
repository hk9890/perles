package styles

import (
	"testing"

	beads "github.com/hk9890/perles/internal/beads/domain"
	"github.com/stretchr/testify/require"
)

func TestGetTypeIndicator_Decision(t *testing.T) {
	require.Equal(t, "[D]", GetTypeIndicator(beads.TypeDecision))
}

func TestGetTypeIndicator_UnknownTypeReadable(t *testing.T) {
	require.Equal(t, "[RA]", GetTypeIndicator(beads.IssueType("risk_assessment")))
	require.Equal(t, "[RFQ]", GetTypeIndicator(beads.IssueType("request-for-quote")))
	require.Equal(t, "[?]", GetTypeIndicator(beads.IssueType("")))
}

func TestGetTypeStyle_CustomTypeNeutral(t *testing.T) {
	style := GetTypeStyle(beads.IssueType("custom_type"))
	rendered := style.Render("X")
	require.Equal(t, "X", rendered)
}
