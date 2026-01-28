package prompt

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/zjrosen/perles/internal/orchestration/workflow"
)

// ============================================================================
// BuildWorkflowContinuationPrompt Tests
// ============================================================================

// TestBuildWorkflowContinuationPrompt_IncludesWorkflowContent verifies the prompt includes workflow content.
func TestBuildWorkflowContinuationPrompt_IncludesWorkflowContent(t *testing.T) {
	workflowState := &workflow.WorkflowState{
		WorkflowID:      "cook",
		WorkflowName:    "Cook Workflow",
		WorkflowContent: "## Step 1\nDo the first step\n\n## Step 2\nDo the second step",
		StartedAt:       time.Now(),
	}

	prompt := BuildWorkflowContinuationPrompt(workflowState)

	require.Contains(t, prompt, "## Step 1", "Prompt should include workflow content")
	require.Contains(t, prompt, "Do the first step", "Prompt should include workflow content")
	require.Contains(t, prompt, "## Step 2", "Prompt should include workflow content")
}

// TestBuildWorkflowContinuationPrompt_IncludesWorkflowNameInHeader verifies the prompt includes workflow name.
func TestBuildWorkflowContinuationPrompt_IncludesWorkflowNameInHeader(t *testing.T) {
	workflowState := &workflow.WorkflowState{
		WorkflowID:      "research-to-tasks",
		WorkflowName:    "Research to Tasks",
		WorkflowContent: "Some workflow content",
		StartedAt:       time.Now(),
	}

	prompt := BuildWorkflowContinuationPrompt(workflowState)

	require.Contains(t, prompt, "ACTIVE WORKFLOW: Research to Tasks",
		"Prompt should include workflow name in header")
}

// TestBuildWorkflowContinuationPrompt_OmitsOriginalPromptSectionWhenEmpty verifies section is omitted.
func TestBuildWorkflowContinuationPrompt_OmitsOriginalPromptSectionWhenEmpty(t *testing.T) {
	workflowState := &workflow.WorkflowState{
		WorkflowID:      "cook",
		WorkflowName:    "Cook Workflow",
		WorkflowContent: "Workflow content here",
		StartedAt:       time.Now(),
	}

	prompt := BuildWorkflowContinuationPrompt(workflowState)

	require.NotContains(t, prompt, "ORIGINAL USER REQUEST:",
		"Prompt should NOT include original request section when empty")
}

// TestBuildWorkflowContinuationPrompt_IncludesContinuationInstructions verifies recovery steps.
func TestBuildWorkflowContinuationPrompt_IncludesContinuationInstructions(t *testing.T) {
	workflowState := &workflow.WorkflowState{
		WorkflowID:      "cook",
		WorkflowName:    "Cook Workflow",
		WorkflowContent: "Workflow content here",
		StartedAt:       time.Now(),
	}

	prompt := BuildWorkflowContinuationPrompt(workflowState)

	require.Contains(t, prompt, "AVAILABLE TOOLS FOR RECOVERY:",
		"Prompt should include tools section")
	require.Contains(t, prompt, "query_worker_state",
		"Prompt should mention query_worker_state tool")
	require.Contains(t, prompt, "fabric_inbox",
		"Prompt should mention fabric_inbox tool")
	require.Contains(t, prompt, "RECOVERY STEPS:",
		"Prompt should include recovery steps section")
}

// TestBuildWorkflowContinuationPrompt_InstructsAutonomousResumption verifies autonomous instruction.
func TestBuildWorkflowContinuationPrompt_InstructsAutonomousResumption(t *testing.T) {
	workflowState := &workflow.WorkflowState{
		WorkflowID:      "cook",
		WorkflowName:    "Cook Workflow",
		WorkflowContent: "Workflow content here",
		StartedAt:       time.Now(),
	}

	prompt := BuildWorkflowContinuationPrompt(workflowState)

	require.Contains(t, prompt, "Do NOT wait for user input",
		"Prompt should instruct not waiting for user input")
	require.Contains(t, prompt, "resume the workflow autonomously",
		"Prompt should instruct autonomous resumption")
}

// TestBuildWorkflowContinuationPrompt_IncludesContinuationMarker verifies the header marker.
func TestBuildWorkflowContinuationPrompt_IncludesContinuationMarker(t *testing.T) {
	workflowState := &workflow.WorkflowState{
		WorkflowID:      "cook",
		WorkflowName:    "Cook Workflow",
		WorkflowContent: "Workflow content here",
		StartedAt:       time.Now(),
	}

	prompt := BuildWorkflowContinuationPrompt(workflowState)

	require.Contains(t, prompt, "[CONTEXT REFRESH - WORKFLOW CONTINUATION]",
		"Prompt should include continuation marker")
}

// TestBuildWorkflowContinuationPrompt_IncludesContextExplanation verifies context explanation.
func TestBuildWorkflowContinuationPrompt_IncludesContextExplanation(t *testing.T) {
	workflowState := &workflow.WorkflowState{
		WorkflowID:      "cook",
		WorkflowName:    "Cook Workflow",
		WorkflowContent: "Workflow content here",
		StartedAt:       time.Now(),
	}

	prompt := BuildWorkflowContinuationPrompt(workflowState)

	require.Contains(t, prompt, "context window was exhausted",
		"Prompt should explain context exhaustion")
	require.Contains(t, prompt, "automatically refreshed",
		"Prompt should explain automatic refresh")
}

// TestBuildWorkflowContinuationPrompt_NilWorkflowState verifies graceful handling of nil.
func TestBuildWorkflowContinuationPrompt_NilWorkflowState(t *testing.T) {
	prompt := BuildWorkflowContinuationPrompt(nil)

	// Should still include basic sections
	require.Contains(t, prompt, "[CONTEXT REFRESH - WORKFLOW CONTINUATION]",
		"Prompt should include continuation marker even with nil state")
	require.Contains(t, prompt, "RECOVERY STEPS:",
		"Prompt should include recovery steps even with nil state")
	// Should NOT include workflow section
	require.NotContains(t, prompt, "ACTIVE WORKFLOW:",
		"Prompt should NOT include workflow section with nil state")
}
