package styles

import (
	"testing"

	beads "github.com/hk9890/perles/internal/beads/domain"
	"github.com/stretchr/testify/require"
)

func TestGetTypeIndicator_Decision(t *testing.T) {
	require.Equal(t, "[D]", GetTypeIndicator(beads.TypeDecision))
}

func TestGetTypeIndicator_V1AdditionalBuiltins(t *testing.T) {
	require.Equal(t, "[S]", GetTypeIndicator(beads.TypeSpike))
	require.Equal(t, "[ST]", GetTypeIndicator(beads.TypeStory))
	require.Equal(t, "[M]", GetTypeIndicator(beads.TypeMilestone))
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

func TestGetStatusColor_V1AdditionalBuiltins(t *testing.T) {
	require.Equal(t, StatusHookedColor, GetStatusColor(beads.StatusHooked))
	require.Equal(t, StatusPinnedColor, GetStatusColor(beads.StatusPinned))
}
