package roles

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"
)

// ============================================================================
// Generic Prompt Tests
// ============================================================================

// TestGenericSystemPrompt_ContainsWorkerID verifies workerID appears in output.
func TestGenericSystemPrompt_ContainsWorkerID(t *testing.T) {
	workerID := "worker-42"
	prompt := GenericSystemPrompt(workerID)
	require.Contains(t, prompt, workerID,
		"GenericSystemPrompt should contain the workerID")
}

// TestGenericSystemPrompt_ReturnsNonEmpty verifies prompt is not empty.
func TestGenericSystemPrompt_ReturnsNonEmpty(t *testing.T) {
	prompt := GenericSystemPrompt("worker-1")
	require.NotEmpty(t, prompt,
		"GenericSystemPrompt should return non-empty string")
}

// TestGenericIdlePrompt_ContainsWorkerID verifies workerID appears in output.
func TestGenericIdlePrompt_ContainsWorkerID(t *testing.T) {
	workerID := "worker-42"
	prompt := GenericIdlePrompt(workerID)
	require.Contains(t, prompt, workerID,
		"GenericIdlePrompt should contain the workerID")
}

// TestGenericIdlePrompt_ReturnsNonEmpty verifies prompt is not empty.
func TestGenericIdlePrompt_ReturnsNonEmpty(t *testing.T) {
	prompt := GenericIdlePrompt("worker-1")
	require.NotEmpty(t, prompt,
		"GenericIdlePrompt should return non-empty string")
}

// TestGenericSystemPromptVersion_IsSemver verifies version follows semver format.
func TestGenericSystemPromptVersion_IsSemver(t *testing.T) {
	// Semver regex pattern (simplified for major.minor.patch)
	semverPattern := regexp.MustCompile(`^\d+\.\d+\.\d+$`)
	require.True(t, semverPattern.MatchString(GenericSystemPromptVersion),
		"GenericSystemPromptVersion %q should follow semver format (x.y.z)",
		GenericSystemPromptVersion)
}

// TestGenericSystemPrompt_ContainsMCPTools verifies the prompt documents MCP tools.
func TestGenericSystemPrompt_ContainsMCPTools(t *testing.T) {
	prompt := GenericSystemPrompt("worker-1")

	requiredTools := []string{
		"signal_ready",
		"fabric_inbox",
		"fabric_send",
		"report_implementation_complete",
		"report_review_verdict",
	}

	for _, tool := range requiredTools {
		require.Contains(t, prompt, tool,
			"GenericSystemPrompt should mention MCP tool %q", tool)
	}
}

// TestGenericIdlePrompt_ContainsIdleInstructions verifies idle state instructions.
func TestGenericIdlePrompt_ContainsIdleInstructions(t *testing.T) {
	prompt := GenericIdlePrompt("worker-1")

	// Should mention idle state
	require.Contains(t, prompt, "IDLE",
		"GenericIdlePrompt should mention IDLE state")

	// Should mention signal_ready
	require.Contains(t, prompt, "signal_ready",
		"GenericIdlePrompt should mention signal_ready")
}
