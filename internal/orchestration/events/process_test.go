package events

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestProcessWorkflowComplete_ConstantValue(t *testing.T) {
	// Verify ProcessWorkflowComplete has the correct string value
	require.Equal(t, "workflow_complete", string(ProcessWorkflowComplete))
}

func TestProcessWorkflowComplete_UsableInProcessEvent(t *testing.T) {
	// Verify the event type can be used in ProcessEvent struct
	event := ProcessEvent{
		Type:      ProcessWorkflowComplete,
		ProcessID: "coordinator-1",
		Role:      RoleCoordinator,
	}

	require.Equal(t, ProcessWorkflowComplete, event.Type)
	require.Equal(t, "workflow_complete", string(event.Type))
}

func TestProcessPhase_Values(t *testing.T) {
	// Verify all ProcessPhase constants have correct string values
	tests := []struct {
		phase    ProcessPhase
		expected string
	}{
		{ProcessPhaseIdle, "idle"},
		{ProcessPhaseImplementing, "implementing"},
		{ProcessPhaseAwaitingReview, "awaiting_review"},
		{ProcessPhaseReviewing, "reviewing"},
		{ProcessPhaseAddressingFeedback, "addressing_feedback"},
		{ProcessPhaseCommitting, "committing"},
	}

	for _, tt := range tests {
		t.Run(string(tt.phase), func(t *testing.T) {
			require.Equal(t, tt.expected, string(tt.phase))
		})
	}
}

func TestProcessPhase_AllPhasesAreDefined(t *testing.T) {
	// Verify we have exactly 6 phases as specified in the proposal
	phases := []ProcessPhase{
		ProcessPhaseIdle,
		ProcessPhaseImplementing,
		ProcessPhaseAwaitingReview,
		ProcessPhaseReviewing,
		ProcessPhaseAddressingFeedback,
		ProcessPhaseCommitting,
	}

	// Each phase should be distinct
	seen := make(map[ProcessPhase]bool)
	for _, phase := range phases {
		require.False(t, seen[phase], "Duplicate phase: %s", phase)
		seen[phase] = true
	}

	require.Len(t, phases, 6, "Expected exactly 6 workflow phases")
}
