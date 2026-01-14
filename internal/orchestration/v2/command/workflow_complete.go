// Package command provides concrete command types for the v2 orchestration architecture.
package command

import "fmt"

// WorkflowStatus represents the completion status of a workflow.
type WorkflowStatus string

const (
	// WorkflowStatusSuccess indicates the workflow completed successfully.
	WorkflowStatusSuccess WorkflowStatus = "success"
	// WorkflowStatusPartial indicates the workflow completed partially.
	WorkflowStatusPartial WorkflowStatus = "partial"
	// WorkflowStatusAborted indicates the workflow was aborted.
	WorkflowStatusAborted WorkflowStatus = "aborted"
)

// IsValid returns true if the workflow status is a valid value.
func (s WorkflowStatus) IsValid() bool {
	return s == WorkflowStatusSuccess || s == WorkflowStatusPartial || s == WorkflowStatusAborted
}

// String returns the string representation of the WorkflowStatus.
func (s WorkflowStatus) String() string {
	return string(s)
}

// ===========================================================================
// Workflow Lifecycle Commands
// ===========================================================================

// SignalWorkflowCompleteCommand signals that the workflow has completed.
// This command updates session metadata with completion status and summary.
type SignalWorkflowCompleteCommand struct {
	*BaseCommand
	Status      WorkflowStatus // Required: "success", "partial", or "aborted"
	Summary     string         // Required: summary of what was accomplished
	EpicID      string         // Optional: epic ID that was completed
	TasksClosed int            // Optional: number of tasks closed during workflow
}

// NewSignalWorkflowCompleteCommand creates a new SignalWorkflowCompleteCommand.
func NewSignalWorkflowCompleteCommand(source CommandSource, status WorkflowStatus, summary, epicID string, tasksClosed int) *SignalWorkflowCompleteCommand {
	base := NewBaseCommand(CmdSignalWorkflowComplete, source)
	return &SignalWorkflowCompleteCommand{
		BaseCommand: &base,
		Status:      status,
		Summary:     summary,
		EpicID:      epicID,
		TasksClosed: tasksClosed,
	}
}

// Validate checks that Status and Summary are provided, and Status is valid.
func (c *SignalWorkflowCompleteCommand) Validate() error {
	if !c.Status.IsValid() {
		return fmt.Errorf("status must be success, partial, or aborted, got: %s", c.Status)
	}
	if c.Summary == "" {
		return fmt.Errorf("summary is required")
	}
	return nil
}

// String returns a readable representation of the command.
func (c *SignalWorkflowCompleteCommand) String() string {
	if c.EpicID != "" {
		return fmt.Sprintf("SignalWorkflowComplete{status=%s, epic=%s, tasks_closed=%d}", c.Status, c.EpicID, c.TasksClosed)
	}
	return fmt.Sprintf("SignalWorkflowComplete{status=%s, tasks_closed=%d}", c.Status, c.TasksClosed)
}
