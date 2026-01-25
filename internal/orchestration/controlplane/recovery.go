// Package controlplane provides recovery actions for stuck workflows.
package controlplane

import (
	"context"
	"fmt"
	"time"

	"github.com/zjrosen/perles/internal/orchestration/v2/command"
	"github.com/zjrosen/perles/internal/orchestration/v2/repository"
)

// RecoveryAction represents the type of recovery action to take for a stuck workflow.
type RecoveryAction int

const (
	// RecoveryNudge sends a gentle reminder message to the coordinator.
	// This is the first-line recovery action for mildly stuck workflows.
	RecoveryNudge RecoveryAction = iota

	// RecoveryReplace terminates the coordinator and spawns a fresh replacement.
	// Used when nudging hasn't helped and the workflow remains stuck.
	RecoveryReplace

	// RecoveryPause suspends the workflow, preserving state for later resumption.
	// Used when replacement hasn't resolved the issue.
	RecoveryPause

	// RecoveryFail terminates the workflow with a Failed state.
	// This is the final recovery action after max retries are exhausted.
	RecoveryFail
)

// String returns the string representation of a RecoveryAction.
func (a RecoveryAction) String() string {
	switch a {
	case RecoveryNudge:
		return "nudge"
	case RecoveryReplace:
		return "replace"
	case RecoveryPause:
		return "pause"
	case RecoveryFail:
		return "fail"
	default:
		return fmt.Sprintf("unknown(%d)", a)
	}
}

// IsValid returns true if this is a recognized RecoveryAction value.
func (a RecoveryAction) IsValid() bool {
	return a >= RecoveryNudge && a <= RecoveryFail
}

// RecoveryResult contains the outcome of a recovery action.
type RecoveryResult struct {
	// Action is the recovery action that was executed.
	Action RecoveryAction
	// Success indicates whether the action completed without error.
	Success bool
	// Error is the error that occurred, if any.
	Error error
	// Timestamp is when the recovery was attempted.
	Timestamp time.Time
}

// RecoveryExecutor executes recovery actions for stuck workflows.
// It bridges the HealthMonitor's stuck detection to concrete recovery operations.
type RecoveryExecutor interface {
	// ExecuteRecovery performs the specified recovery action for a workflow.
	// Returns an error if the action could not be executed.
	ExecuteRecovery(ctx context.Context, id WorkflowID, action RecoveryAction) error
}

// CommandSubmitter abstracts command submission for recovery operations.
// This interface enables testing by allowing mock implementations.
type CommandSubmitter interface {
	// SubmitAndWait submits a command and waits for its result.
	SubmitAndWait(ctx context.Context, cmd command.Command) (*command.CommandResult, error)
}

// WorkflowProvider provides access to workflow instances for recovery operations.
// This interface enables testing by decoupling from concrete Registry/ControlPlane.
type WorkflowProvider interface {
	// Get retrieves a workflow by ID.
	Get(id WorkflowID) (*WorkflowInstance, bool)
}

// CommandSubmitterFactory creates a CommandSubmitter for a given workflow.
// This allows tests to inject mock command submitters while production code
// uses the Infrastructure's processor.
type CommandSubmitterFactory func(inst *WorkflowInstance) CommandSubmitter

// InfrastructureCommandSubmitter returns the workflow's infrastructure processor.
// This is the default factory for production use.
func InfrastructureCommandSubmitter(inst *WorkflowInstance) CommandSubmitter {
	if inst.Infrastructure == nil {
		return nil
	}
	return inst.Infrastructure.Core.Processor
}

// RecoveryExecutorConfig configures the RecoveryExecutor.
type RecoveryExecutorConfig struct {
	// WorkflowProvider provides access to workflow instances.
	WorkflowProvider WorkflowProvider
	// Supervisor provides workflow lifecycle operations (pause, resume, shutdown).
	// Required for executing pause recovery actions.
	Supervisor Supervisor
	// OnHealthEvent is called when recovery events are emitted.
	// Can be nil if event emission is not needed.
	OnHealthEvent HealthEventCallback
	// CommandSubmitterFactory creates command submitters for workflows.
	// If nil, uses InfrastructureCommandSubmitter.
	CommandSubmitterFactory CommandSubmitterFactory
	// Clock is used for timestamps (for testing).
	// If nil, uses time.Now().
	Clock Clock
}

// defaultRecoveryExecutor is the default implementation of RecoveryExecutor.
type defaultRecoveryExecutor struct {
	workflowProvider        WorkflowProvider
	supervisor              Supervisor
	onHealthEvent           HealthEventCallback
	commandSubmitterFactory CommandSubmitterFactory
	clock                   Clock
}

// NewRecoveryExecutor creates a new RecoveryExecutor with the given configuration.
func NewRecoveryExecutor(cfg RecoveryExecutorConfig) (RecoveryExecutor, error) {
	if cfg.WorkflowProvider == nil {
		return nil, fmt.Errorf("WorkflowProvider is required")
	}

	clock := cfg.Clock
	if clock == nil {
		clock = realClock{}
	}

	cmdSubmitterFactory := cfg.CommandSubmitterFactory
	if cmdSubmitterFactory == nil {
		cmdSubmitterFactory = InfrastructureCommandSubmitter
	}

	return &defaultRecoveryExecutor{
		workflowProvider:        cfg.WorkflowProvider,
		supervisor:              cfg.Supervisor,
		onHealthEvent:           cfg.OnHealthEvent,
		commandSubmitterFactory: cmdSubmitterFactory,
		clock:                   clock,
	}, nil
}

// ExecuteRecovery performs the specified recovery action for a workflow.
func (e *defaultRecoveryExecutor) ExecuteRecovery(ctx context.Context, id WorkflowID, action RecoveryAction) error {
	// Validate the action
	if !action.IsValid() {
		return fmt.Errorf("invalid recovery action: %d", action)
	}

	// Get the workflow instance
	inst, ok := e.workflowProvider.Get(id)
	if !ok {
		return fmt.Errorf("workflow not found: %s", id)
	}

	// Emit recovery started event
	e.emitEvent(NewHealthEvent(HealthRecoveryStarted, id).
		WithRecoveryAction(action.String()).
		WithDetails(fmt.Sprintf("Starting %s recovery", action)))

	// Execute the recovery action
	var err error
	switch action {
	case RecoveryNudge:
		err = e.executeNudge(ctx, inst)
	case RecoveryReplace:
		err = e.executeReplace(ctx, inst)
	case RecoveryPause:
		err = e.executePause(ctx, inst)
	case RecoveryFail:
		err = e.executeFail(ctx, inst)
	default:
		err = fmt.Errorf("unhandled recovery action: %s", action)
	}

	// Emit success or failure event
	if err != nil {
		e.emitEvent(NewHealthEvent(HealthRecoveryFailed, id).
			WithRecoveryAction(action.String()).
			WithDetails(fmt.Sprintf("Recovery %s failed: %v", action, err)))
		return err
	}

	e.emitEvent(NewHealthEvent(HealthRecoverySuccess, id).
		WithRecoveryAction(action.String()).
		WithDetails(fmt.Sprintf("Recovery %s succeeded", action)))

	return nil
}

// executeNudge sends a gentle reminder message to the coordinator.
func (e *defaultRecoveryExecutor) executeNudge(ctx context.Context, inst *WorkflowInstance) error {
	// Workflow must be running to nudge
	if inst.State != WorkflowRunning {
		return fmt.Errorf("cannot nudge workflow in state %s", inst.State)
	}

	// Get command submitter
	cmdSubmitter := e.commandSubmitterFactory(inst)
	if cmdSubmitter == nil {
		return fmt.Errorf("workflow infrastructure not available")
	}

	// Create nudge message with actionable guidance
	nudgeMessage := `[SYSTEM] Automatic System Health Check: No worker output detected recently.

Please diagnose using these tools:
1. Use query_workflow_state to check worker statuses
2. Use read_message_log to review recent activity

Based on what you find:
- If workers are still in "working" state → No action needed, they're actively processing
- If waiting for user input or action → You MUST call the notify_user to alert the user, then end your turn
- If workers are idle/stuck → if they were supposed to be working on a task investigate and determine if we need to send a message to a worker.`

	// Submit send-to-process command for the coordinator
	cmd := command.NewSendToProcessCommand(
		command.SourceInternal,
		repository.CoordinatorID,
		nudgeMessage,
	)

	result, err := cmdSubmitter.SubmitAndWait(ctx, cmd)
	if err != nil {
		return fmt.Errorf("submitting nudge command: %w", err)
	}
	if !result.Success {
		return fmt.Errorf("nudge command failed: %w", result.Error)
	}

	return nil
}

// executeReplace terminates the coordinator and spawns a replacement.
func (e *defaultRecoveryExecutor) executeReplace(ctx context.Context, inst *WorkflowInstance) error {
	// Workflow must be running to replace coordinator
	if inst.State != WorkflowRunning {
		return fmt.Errorf("cannot replace coordinator in workflow state %s", inst.State)
	}

	// Get command submitter
	cmdSubmitter := e.commandSubmitterFactory(inst)
	if cmdSubmitter == nil {
		return fmt.Errorf("workflow infrastructure not available")
	}

	// Submit replace process command for the coordinator
	cmd := command.NewReplaceProcessCommand(
		command.SourceInternal,
		repository.CoordinatorID,
		"Coordinator replaced due to stuck workflow recovery",
	)

	result, err := cmdSubmitter.SubmitAndWait(ctx, cmd)
	if err != nil {
		return fmt.Errorf("submitting replace command: %w", err)
	}
	if !result.Success {
		return fmt.Errorf("replace command failed: %w", result.Error)
	}

	return nil
}

// executePause suspends the workflow by delegating to Supervisor.Pause().
func (e *defaultRecoveryExecutor) executePause(ctx context.Context, inst *WorkflowInstance) error {
	// If no supervisor is configured, fall back to legacy behavior
	if e.supervisor == nil {
		// Legacy behavior for backward compatibility
		if inst.State != WorkflowRunning {
			return fmt.Errorf("cannot pause workflow in state %s", inst.State)
		}
		if err := inst.TransitionTo(WorkflowPaused); err != nil {
			return fmt.Errorf("transitioning to paused: %w", err)
		}
		if inst.Infrastructure != nil {
			if inst.Infrastructure.Internal.CoordinatorNudger != nil {
				inst.Infrastructure.Internal.CoordinatorNudger.Stop()
			}
			if inst.Infrastructure.Internal.ProcessRegistry != nil {
				inst.Infrastructure.Internal.ProcessRegistry.StopAll()
			}
		}
		return nil
	}

	// Delegate to Supervisor.Pause() for unified pause behavior
	return e.supervisor.Pause(ctx, inst)
}

// executeFail terminates the workflow with Failed state.
func (e *defaultRecoveryExecutor) executeFail(_ context.Context, inst *WorkflowInstance) error {
	// Workflow must be in a non-terminal state
	if inst.State.IsTerminal() {
		return fmt.Errorf("cannot fail workflow already in terminal state %s", inst.State)
	}

	// Transition to failed state
	if err := inst.TransitionTo(WorkflowFailed); err != nil {
		return fmt.Errorf("transitioning to failed: %w", err)
	}

	// Full infrastructure shutdown
	if inst.Infrastructure != nil {
		inst.Infrastructure.Shutdown()
	}

	// Cancel the workflow context
	if inst.Cancel != nil {
		inst.Cancel()
	}

	return nil
}

// emitEvent emits a health event if a callback is configured.
// Note: This runs synchronously since the recovery executor is already called
// asynchronously from the health monitor, and we need ordered event delivery.
func (e *defaultRecoveryExecutor) emitEvent(event HealthEvent) {
	if e.onHealthEvent != nil {
		e.onHealthEvent(event)
	}
}

// DetermineRecoveryAction determines the appropriate recovery action based on
// the current health status and policy.
// Returns the action to take, or -1 if no recovery is needed.
//
// Recovery escalation:
// 1. Nudge (up to MaxNudges times, default 2)
// 2. Replace (once, if enabled)
// 3. Pause (once, if enabled)
// 4. Fail (when max recoveries reached)
func DetermineRecoveryAction(status *HealthStatus, policy HealthPolicy) RecoveryAction {
	return DetermineRecoveryActionAt(status, policy, time.Now())
}

// DetermineRecoveryActionAt determines the appropriate recovery action at the given time.
// This variant allows testing with a mock clock.
func DetermineRecoveryActionAt(status *HealthStatus, policy HealthPolicy, now time.Time) RecoveryAction {
	// No recovery needed if not stuck (computed from LastProgressAt vs ProgressTimeout)
	if !status.IsStuckAt(policy, now) {
		return RecoveryAction(-1)
	}

	// Check if max recoveries exceeded
	if status.RecoveryCount >= policy.MaxRecoveries {
		if policy.EnableAutoFail {
			return RecoveryFail
		}
		// No auto-fail - enter limbo state (no action, will emit HealthStillStuck)
		return RecoveryAction(-1)
	}

	// Calculate MaxNudges with default
	maxNudges := policy.MaxNudges
	if maxNudges <= 0 {
		maxNudges = 2 // Default: 2 nudges before escalation
	}

	// Phase 1: Nudge attempts (0 to maxNudges-1)
	if status.RecoveryCount < maxNudges {
		if policy.EnableAutoNudge {
			return RecoveryNudge
		}
		// Skip nudge phase if disabled - fall through to escalation logic
	}

	// Phase 2: Replace (at count == maxNudges)
	if status.RecoveryCount == maxNudges {
		if policy.EnableAutoReplace {
			return RecoveryReplace
		}
		// Skip to pause if replace is disabled
		if policy.EnableAutoPause {
			return RecoveryPause
		}
		// No escalation available - return no action
		return RecoveryAction(-1)
	}

	// Phase 3: Pause (at count == maxNudges + 1)
	if status.RecoveryCount == maxNudges+1 {
		if policy.EnableAutoPause {
			return RecoveryPause
		}
		// No pause available - return no action
		return RecoveryAction(-1)
	}

	// Phase 4: Fail (beyond pause attempts, only if enabled)
	if policy.EnableAutoFail {
		return RecoveryFail
	}
	return RecoveryAction(-1)
}
