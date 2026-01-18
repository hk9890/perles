// Package controlplane provides health monitoring types for the control plane.
package controlplane

import (
	"fmt"
	"time"
)

// HealthPolicy defines health monitoring thresholds and auto-recovery settings.
// It configures how the HealthMonitor detects stuck workflows and what recovery
// actions to take.
type HealthPolicy struct {
	// HeartbeatTimeout is the duration after which a workflow is marked unhealthy
	// if no events are received. Default: 2 minutes.
	HeartbeatTimeout time.Duration

	// ProgressTimeout is the duration after which a workflow is declared stuck
	// if no forward progress is made (e.g., phase transitions, task completions).
	// Default: 5 minutes.
	ProgressTimeout time.Duration

	// MaxRecoveries is the maximum number of recovery attempts before giving up.
	// Default: 3.
	MaxRecoveries int

	// RecoveryBackoff is the minimum time to wait between recovery attempts.
	// Default: 30 seconds.
	RecoveryBackoff time.Duration

	// EnableAutoNudge enables automatic nudging of the coordinator when stuck.
	EnableAutoNudge bool

	// MaxNudges is the maximum number of nudge attempts before escalating to
	// the next recovery action (replace). Default: 2.
	// Only applies when EnableAutoNudge is true.
	MaxNudges int

	// EnableAutoReplace enables automatic replacement of the coordinator after
	// repeated stuck states.
	EnableAutoReplace bool

	// EnableAutoPause enables automatic pausing of workflows after repeated
	// recovery failures.
	EnableAutoPause bool
}

// DefaultHealthPolicy returns a HealthPolicy with sensible defaults.
func DefaultHealthPolicy() HealthPolicy {
	return HealthPolicy{
		HeartbeatTimeout:  2 * time.Minute,
		ProgressTimeout:   5 * time.Minute,
		MaxRecoveries:     3,
		RecoveryBackoff:   30 * time.Second,
		EnableAutoNudge:   false,
		MaxNudges:         2,
		EnableAutoReplace: false,
		EnableAutoPause:   false,
	}
}

// Validate checks that the HealthPolicy has valid values.
// Returns an error describing the first validation failure, or nil if valid.
func (p *HealthPolicy) Validate() error {
	if p.HeartbeatTimeout <= 0 {
		return fmt.Errorf("heartbeat_timeout must be positive: %v", p.HeartbeatTimeout)
	}
	if p.ProgressTimeout <= 0 {
		return fmt.Errorf("progress_timeout must be positive: %v", p.ProgressTimeout)
	}
	if p.MaxRecoveries < 0 {
		return fmt.Errorf("max_recoveries cannot be negative: %d", p.MaxRecoveries)
	}
	if p.RecoveryBackoff < 0 {
		return fmt.Errorf("recovery_backoff cannot be negative: %v", p.RecoveryBackoff)
	}
	return nil
}

// HealthStatus tracks the health of a single workflow instance.
type HealthStatus struct {
	// WorkflowID identifies the workflow being tracked.
	WorkflowID WorkflowID

	// IsHealthy indicates whether the workflow is currently healthy.
	// A workflow is healthy if it has received recent heartbeats.
	IsHealthy bool

	// LastHeartbeatAt is the timestamp of the most recent activity from the workflow.
	LastHeartbeatAt time.Time

	// LastProgressAt is the timestamp of the most recent forward progress.
	// Progress includes phase transitions, task completions, and turn completions.
	// Used to compute "stuck" status: stuck = now - LastProgressAt > ProgressTimeout
	LastProgressAt time.Time

	// RecoveryCount is the number of recovery attempts made for this workflow.
	// Also used to track if "stuck suspected" event was emitted (count > 0 means yes).
	RecoveryCount int

	// LastRecoveryAt is the timestamp of the most recent recovery attempt.
	// Nil if no recovery has been attempted.
	LastRecoveryAt *time.Time
}

// NewHealthStatus creates a new HealthStatus for the given workflow ID.
// The status is initialized as healthy with current timestamps.
func NewHealthStatus(workflowID WorkflowID) HealthStatus {
	now := time.Now()
	return HealthStatus{
		WorkflowID:      workflowID,
		IsHealthy:       true,
		LastHeartbeatAt: now,
		LastProgressAt:  now,
		RecoveryCount:   0,
	}
}

// IsDegraded returns true if the workflow has missed heartbeats but is not yet stuck.
// A degraded workflow is one where:
// - IsHealthy is false (heartbeat timeout exceeded), OR
// - LastHeartbeatAt is older than LastProgressAt by more than the heartbeat interval
func (s *HealthStatus) IsDegraded(policy HealthPolicy) bool {
	if !s.IsHealthy {
		return true
	}
	// Also degraded if heartbeat is lagging significantly behind expected
	timeSinceHeartbeat := time.Since(s.LastHeartbeatAt)
	return timeSinceHeartbeat > policy.HeartbeatTimeout/2
}

// NeedsRecovery returns true if the workflow is stuck and needs recovery action.
// A workflow needs recovery when:
// - It is currently stuck (LastProgressAt exceeds ProgressTimeout), AND
// - Recovery count is below max recoveries, AND
// - Enough time has passed since the last recovery attempt (backoff)
func (s *HealthStatus) NeedsRecovery(policy HealthPolicy) bool {
	return s.NeedsRecoveryAt(policy, time.Now())
}

// NeedsRecoveryAt returns true if the workflow is stuck and needs recovery action at the given time.
// This variant allows testing with a mock clock.
func (s *HealthStatus) NeedsRecoveryAt(policy HealthPolicy, now time.Time) bool {
	if !s.IsStuckAt(policy, now) {
		return false
	}
	// Note: We check >= because once we hit max recoveries, the next action should be to fail
	// which is handled by DetermineRecoveryAction
	if s.RecoveryCount > policy.MaxRecoveries {
		return false
	}
	if s.LastRecoveryAt != nil {
		timeSinceLastRecovery := now.Sub(*s.LastRecoveryAt)
		if timeSinceLastRecovery < policy.RecoveryBackoff {
			return false
		}
	}
	return true
}

// IsStuck returns true if the workflow is currently stuck (no progress within ProgressTimeout).
func (s *HealthStatus) IsStuck(policy HealthPolicy) bool {
	return s.IsStuckAt(policy, time.Now())
}

// IsStuckAt returns true if the workflow is stuck at the given time.
func (s *HealthStatus) IsStuckAt(policy HealthPolicy, now time.Time) bool {
	return now.Sub(s.LastProgressAt) > policy.ProgressTimeout
}

// ResetRecovery resets the recovery counter when progress is made.
func (s *HealthStatus) ResetRecovery() {
	s.RecoveryCount = 0
	s.LastRecoveryAt = nil
}

// RecordRecoveryAttempt increments the recovery count and updates the last recovery time.
func (s *HealthStatus) RecordRecoveryAttempt() {
	s.RecordRecoveryAttemptAt(time.Now())
}

// RecordRecoveryAttemptAt increments the recovery count and updates the last recovery time to the given time.
// This variant allows testing with a mock clock.
func (s *HealthStatus) RecordRecoveryAttemptAt(now time.Time) {
	s.RecoveryCount++
	s.LastRecoveryAt = &now
}

// HealthEventType categorizes health-related events.
type HealthEventType string

const (
	// HealthHeartbeatMissed indicates the workflow has not sent any events
	// within the heartbeat timeout period.
	HealthHeartbeatMissed HealthEventType = "health.heartbeat.missed"

	// HealthStuckSuspected indicates the workflow is making no forward progress
	// and may be stuck.
	HealthStuckSuspected HealthEventType = "health.stuck.suspected"

	// HealthRecoveryStarted indicates a recovery action has been initiated.
	HealthRecoveryStarted HealthEventType = "health.recovery.started"

	// HealthRecoverySuccess indicates a recovery action succeeded and the
	// workflow is progressing again.
	HealthRecoverySuccess HealthEventType = "health.recovery.succeeded"

	// HealthRecoveryFailed indicates a recovery action failed and the workflow
	// remains stuck.
	HealthRecoveryFailed HealthEventType = "health.recovery.failed"

	// HealthStillStuck indicates the workflow remains stuck but no recovery
	// action is available (e.g., max nudges reached and escalation disabled).
	// Emitted periodically to provide visibility into limbo state.
	HealthStillStuck HealthEventType = "health.still.stuck"
)

// HealthEvent represents a health-related event for a workflow.
type HealthEvent struct {
	// Type identifies the kind of health event.
	Type HealthEventType

	// WorkflowID identifies the affected workflow.
	WorkflowID WorkflowID

	// Timestamp is when the event occurred.
	Timestamp time.Time

	// Details provides a human-readable description of the event.
	Details string

	// RecoveryAction describes the recovery action taken (if any).
	// Examples: "nudge", "replace", "pause", "fail"
	RecoveryAction string
}

// NewHealthEvent creates a new HealthEvent with the given type and workflow ID.
func NewHealthEvent(eventType HealthEventType, workflowID WorkflowID) HealthEvent {
	return HealthEvent{
		Type:       eventType,
		WorkflowID: workflowID,
		Timestamp:  time.Now(),
	}
}

// WithDetails adds details to the health event.
func (e HealthEvent) WithDetails(details string) HealthEvent {
	e.Details = details
	return e
}

// WithRecoveryAction adds recovery action information to the health event.
func (e HealthEvent) WithRecoveryAction(action string) HealthEvent {
	e.RecoveryAction = action
	return e
}
