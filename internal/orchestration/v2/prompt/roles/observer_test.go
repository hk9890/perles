package roles

import (
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// ============================================================================
// Observer Prompt Tests
// ============================================================================

// TestObserverPrompt_ContainsPassiveInstruction verifies prompt contains
// "passive" or "observe" instruction as required by acceptance criteria.
func TestObserverPrompt_ContainsPassiveInstruction(t *testing.T) {
	prompt := ObserverSystemPrompt()
	promptLower := strings.ToLower(prompt)

	hasPassive := strings.Contains(promptLower, "passive")
	hasObserve := strings.Contains(promptLower, "observe")

	require.True(t, hasPassive || hasObserve,
		"ObserverSystemPrompt should contain 'passive' or 'observe' instruction")
}

// TestObserverPrompt_ContainsNeverRespondInstruction verifies prompt contains
// instruction to never respond to coordinator/worker as required by acceptance criteria.
func TestObserverPrompt_ContainsNeverRespondInstruction(t *testing.T) {
	prompt := ObserverSystemPrompt()

	require.Contains(t, prompt, "NEVER respond to coordinator or worker",
		"ObserverSystemPrompt should explicitly state 'NEVER respond to coordinator or worker messages'")
}

// TestObserverPrompt_ContainsObserverChannelOnly verifies prompt mentions
// #observer as the only allowed response channel as required by acceptance criteria.
func TestObserverPrompt_ContainsObserverChannelOnly(t *testing.T) {
	prompt := ObserverSystemPrompt()

	require.Contains(t, prompt, "ONLY respond",
		"ObserverSystemPrompt should contain 'ONLY respond' instruction")
	require.Contains(t, prompt, "#observer",
		"ObserverSystemPrompt should mention #observer channel")
	require.Contains(t, prompt, "ONLY WRITE CHANNEL",
		"ObserverSystemPrompt should emphasize #observer is the only write channel")
}

// TestObserverPrompt_ContainsChannelDescriptions verifies prompt includes
// descriptions of all fabric channels as required by acceptance criteria.
func TestObserverPrompt_ContainsChannelDescriptions(t *testing.T) {
	prompt := ObserverSystemPrompt()

	channels := []string{
		"#system",
		"#tasks",
		"#planning",
		"#general",
		"#observer",
	}

	for _, channel := range channels {
		require.Contains(t, prompt, channel,
			"ObserverSystemPrompt should contain description for %s", channel)
	}

	// Verify the channel descriptions section header exists
	require.Contains(t, prompt, "FABRIC CHANNEL DESCRIPTIONS",
		"ObserverSystemPrompt should have a channel descriptions section")
}

// TestObserverPrompt_ContainsActionLimitation verifies prompt explains
// Observer cannot take orchestration actions as required by acceptance criteria.
func TestObserverPrompt_ContainsActionLimitation(t *testing.T) {
	prompt := ObserverSystemPrompt()

	require.Contains(t, prompt, "CANNOT take orchestration actions",
		"ObserverSystemPrompt should explain Observer cannot take actions")
	require.Contains(t, prompt, "spawn workers",
		"ObserverSystemPrompt should mention inability to spawn workers")
	require.Contains(t, prompt, "assign tasks",
		"ObserverSystemPrompt should mention inability to assign tasks")
}

// TestObserverSystemPrompt_ReturnsNonEmpty verifies prompt is not empty.
func TestObserverSystemPrompt_ReturnsNonEmpty(t *testing.T) {
	prompt := ObserverSystemPrompt()
	require.NotEmpty(t, prompt,
		"ObserverSystemPrompt should return non-empty string")
}

// TestObserverIdlePrompt_ReturnsNonEmpty verifies idle prompt is not empty.
func TestObserverIdlePrompt_ReturnsNonEmpty(t *testing.T) {
	prompt := ObserverIdlePrompt()
	require.NotEmpty(t, prompt,
		"ObserverIdlePrompt should return non-empty string")
}

// TestObserverSystemPromptVersion_IsSemver verifies version follows semver format.
func TestObserverSystemPromptVersion_IsSemver(t *testing.T) {
	// Semver regex pattern (simplified for major.minor.patch)
	semverPattern := regexp.MustCompile(`^\d+\.\d+\.\d+$`)
	require.True(t, semverPattern.MatchString(ObserverSystemPromptVersion),
		"ObserverSystemPromptVersion %q should follow semver format (x.y.z)",
		ObserverSystemPromptVersion)
}

// TestObserverIdlePrompt_ContainsSubscriptionInstructions verifies idle prompt
// instructs the Observer to subscribe to channels on startup.
func TestObserverIdlePrompt_ContainsSubscriptionInstructions(t *testing.T) {
	prompt := ObserverIdlePrompt()

	require.Contains(t, prompt, "fabric_subscribe",
		"ObserverIdlePrompt should contain subscription instructions")
	// Observer now subscribes to #observer first, then remaining channels
	require.Contains(t, prompt, "Subscribe to #observer channel first",
		"ObserverIdlePrompt should instruct subscribing to #observer first")
	require.Contains(t, prompt, "Subscribe to remaining channels",
		"ObserverIdlePrompt should instruct subscribing to remaining channels")
}

// TestObserverIdlePrompt_IdentifiesAsObserver verifies role identification.
func TestObserverIdlePrompt_IdentifiesAsObserver(t *testing.T) {
	prompt := ObserverIdlePrompt()
	require.Contains(t, prompt, "Observer",
		"ObserverIdlePrompt should identify as Observer")
}

// TestObserverSystemPrompt_MentionsReadOnlyTools verifies prompt lists read-only tools.
func TestObserverSystemPrompt_MentionsReadOnlyTools(t *testing.T) {
	prompt := ObserverSystemPrompt()

	readOnlyTools := []string{
		"fabric_inbox",
		"fabric_history",
		"fabric_read_thread",
		"fabric_subscribe",
		"fabric_ack",
	}

	for _, tool := range readOnlyTools {
		require.Contains(t, prompt, tool,
			"ObserverSystemPrompt should mention read-only tool %s", tool)
	}
}

// TestObserverSystemPrompt_MentionsRestrictedWriteTools verifies prompt lists
// restricted write tools with their limitations.
func TestObserverSystemPrompt_MentionsRestrictedWriteTools(t *testing.T) {
	prompt := ObserverSystemPrompt()

	require.Contains(t, prompt, "fabric_send",
		"ObserverSystemPrompt should mention fabric_send as restricted")
	require.Contains(t, prompt, "fabric_reply",
		"ObserverSystemPrompt should mention fabric_reply as restricted")
	require.Contains(t, prompt, "Restricted write tools",
		"ObserverSystemPrompt should have a restricted write tools section")
}

// ============================================================================
// Observer v1.1.0 Tests - Artifact and Inbox Management
// ============================================================================

// TestObserverSystemPromptVersion_Is_1_2_0 verifies version constant is "1.2.0".
func TestObserverSystemPromptVersion_Is_1_2_0(t *testing.T) {
	require.Equal(t, "1.2.0", ObserverSystemPromptVersion,
		"ObserverSystemPromptVersion should be 1.2.0")
}

// ============================================================================
// Observer v1.2.0 Tests - User Message Response via fabric_reply
// ============================================================================

// TestObserverSystemPrompt_ContainsFabricReplyInstruction verifies prompt instructs
// the observer to use fabric_reply when responding to user messages.
func TestObserverSystemPrompt_ContainsFabricReplyInstruction(t *testing.T) {
	prompt := ObserverSystemPrompt()

	require.Contains(t, prompt, "ALWAYS use fabric_reply to respond",
		"ObserverSystemPrompt should instruct to ALWAYS use fabric_reply for user responses")
	require.Contains(t, prompt, "never use fabric_send for user message responses",
		"ObserverSystemPrompt should instruct against using fabric_send for user message responses")
}

// TestObserverIdlePrompt_ContainsArtifactInstructions verifies idle prompt includes
// {{SESSION_DIR}} placeholder and artifact creation steps.
func TestObserverIdlePrompt_ContainsArtifactInstructions(t *testing.T) {
	prompt := ObserverIdlePrompt()

	// Verify {{SESSION_DIR}} placeholder exists
	require.Contains(t, prompt, "{{SESSION_DIR}}",
		"ObserverIdlePrompt should contain {{SESSION_DIR}} placeholder")

	// Verify observer_notes.md creation instructions
	require.Contains(t, prompt, "observer_notes.md",
		"ObserverIdlePrompt should instruct creating observer_notes.md")

	// Verify fabric_attach instructions
	require.Contains(t, prompt, "fabric_attach",
		"ObserverIdlePrompt should instruct using fabric_attach")

	// Verify channel_id note for fabric_attach
	require.Contains(t, prompt, "channel_id",
		"ObserverIdlePrompt should mention channel_id for fabric_attach")

	// Verify "append, don't overwrite" instruction
	require.Contains(t, prompt, "append, don't overwrite",
		"ObserverIdlePrompt should instruct to append, not overwrite notes")
}

// TestObserverSystemPrompt_ContainsInboxManagement verifies system prompt includes
// fabric_ack and fabric_history guidance.
func TestObserverSystemPrompt_ContainsInboxManagement(t *testing.T) {
	prompt := ObserverSystemPrompt()

	// Verify INBOX MANAGEMENT section exists
	require.Contains(t, prompt, "INBOX MANAGEMENT",
		"ObserverSystemPrompt should have INBOX MANAGEMENT section")

	// Verify fabric_ack guidance
	require.Contains(t, prompt, "fabric_ack",
		"ObserverSystemPrompt should contain fabric_ack guidance")
	require.Contains(t, prompt, "message_ids",
		"ObserverSystemPrompt should explain message_ids parameter for fabric_ack")

	// Verify REVIEWING HISTORY section
	require.Contains(t, prompt, "REVIEWING HISTORY",
		"ObserverSystemPrompt should have REVIEWING HISTORY section")

	// Verify fabric_history usage examples
	require.Contains(t, prompt, "fabric_history(channel=",
		"ObserverSystemPrompt should contain fabric_history usage examples")
}

// TestObserverIdlePrompt_NoExtraWhitespace verifies prompts render correctly
// without leading/trailing whitespace issues.
func TestObserverIdlePrompt_NoExtraWhitespace(t *testing.T) {
	prompt := ObserverIdlePrompt()

	// Should not start with newline
	require.False(t, strings.HasPrefix(prompt, "\n"),
		"ObserverIdlePrompt should not start with newline")

	// Should not end with excessive newlines (more than 1)
	trimmed := strings.TrimRight(prompt, "\n")
	trailing := len(prompt) - len(trimmed)
	require.LessOrEqual(t, trailing, 1,
		"ObserverIdlePrompt should not have more than 1 trailing newline")
}

// TestObserverSystemPrompt_NoExtraWhitespace verifies prompts render correctly
// without leading/trailing whitespace issues.
func TestObserverSystemPrompt_NoExtraWhitespace(t *testing.T) {
	prompt := ObserverSystemPrompt()

	// Should not start with newline
	require.False(t, strings.HasPrefix(prompt, "\n"),
		"ObserverSystemPrompt should not start with newline")

	// Should not end with excessive newlines (more than 1)
	trimmed := strings.TrimRight(prompt, "\n")
	trailing := len(prompt) - len(trimmed)
	require.LessOrEqual(t, trailing, 1,
		"ObserverSystemPrompt should not have more than 1 trailing newline")
}
