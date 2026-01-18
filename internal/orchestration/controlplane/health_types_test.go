package controlplane

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestHealthPolicy_Validate_AcceptsValidConfig(t *testing.T) {
	policy := HealthPolicy{
		HeartbeatTimeout:  2 * time.Minute,
		ProgressTimeout:   5 * time.Minute,
		MaxRecoveries:     3,
		RecoveryBackoff:   30 * time.Second,
		EnableAutoNudge:   true,
		EnableAutoReplace: true,
		EnableAutoPause:   false,
	}

	err := policy.Validate()
	require.NoError(t, err)
}

func TestHealthPolicy_Validate_RejectsZeroHeartbeatTimeout(t *testing.T) {
	policy := HealthPolicy{
		HeartbeatTimeout: 0,
		ProgressTimeout:  5 * time.Minute,
		MaxRecoveries:    3,
		RecoveryBackoff:  30 * time.Second,
	}

	err := policy.Validate()
	require.Error(t, err)
	require.Contains(t, err.Error(), "heartbeat_timeout must be positive")
}

func TestHealthPolicy_Validate_RejectsNegativeHeartbeatTimeout(t *testing.T) {
	policy := HealthPolicy{
		HeartbeatTimeout: -1 * time.Second,
		ProgressTimeout:  5 * time.Minute,
		MaxRecoveries:    3,
		RecoveryBackoff:  30 * time.Second,
	}

	err := policy.Validate()
	require.Error(t, err)
	require.Contains(t, err.Error(), "heartbeat_timeout must be positive")
}

func TestHealthPolicy_Validate_RejectsZeroProgressTimeout(t *testing.T) {
	policy := HealthPolicy{
		HeartbeatTimeout: 2 * time.Minute,
		ProgressTimeout:  0,
		MaxRecoveries:    3,
		RecoveryBackoff:  30 * time.Second,
	}

	err := policy.Validate()
	require.Error(t, err)
	require.Contains(t, err.Error(), "progress_timeout must be positive")
}

func TestHealthPolicy_Validate_RejectsNegativeProgressTimeout(t *testing.T) {
	policy := HealthPolicy{
		HeartbeatTimeout: 2 * time.Minute,
		ProgressTimeout:  -1 * time.Second,
		MaxRecoveries:    3,
		RecoveryBackoff:  30 * time.Second,
	}

	err := policy.Validate()
	require.Error(t, err)
	require.Contains(t, err.Error(), "progress_timeout must be positive")
}

func TestHealthPolicy_Validate_RejectsNegativeMaxRecoveries(t *testing.T) {
	policy := HealthPolicy{
		HeartbeatTimeout: 2 * time.Minute,
		ProgressTimeout:  5 * time.Minute,
		MaxRecoveries:    -1,
		RecoveryBackoff:  30 * time.Second,
	}

	err := policy.Validate()
	require.Error(t, err)
	require.Contains(t, err.Error(), "max_recoveries cannot be negative")
}

func TestHealthPolicy_Validate_AcceptsZeroMaxRecoveries(t *testing.T) {
	// Zero max recoveries means no auto-recovery
	policy := HealthPolicy{
		HeartbeatTimeout: 2 * time.Minute,
		ProgressTimeout:  5 * time.Minute,
		MaxRecoveries:    0,
		RecoveryBackoff:  30 * time.Second,
	}

	err := policy.Validate()
	require.NoError(t, err)
}

func TestHealthPolicy_Validate_RejectsNegativeRecoveryBackoff(t *testing.T) {
	policy := HealthPolicy{
		HeartbeatTimeout: 2 * time.Minute,
		ProgressTimeout:  5 * time.Minute,
		MaxRecoveries:    3,
		RecoveryBackoff:  -1 * time.Second,
	}

	err := policy.Validate()
	require.Error(t, err)
	require.Contains(t, err.Error(), "recovery_backoff cannot be negative")
}

func TestHealthPolicy_Validate_AcceptsZeroRecoveryBackoff(t *testing.T) {
	// Zero backoff means immediate retry
	policy := HealthPolicy{
		HeartbeatTimeout: 2 * time.Minute,
		ProgressTimeout:  5 * time.Minute,
		MaxRecoveries:    3,
		RecoveryBackoff:  0,
	}

	err := policy.Validate()
	require.NoError(t, err)
}

func TestDefaultHealthPolicy(t *testing.T) {
	policy := DefaultHealthPolicy()

	require.Equal(t, 2*time.Minute, policy.HeartbeatTimeout)
	require.Equal(t, 5*time.Minute, policy.ProgressTimeout)
	require.Equal(t, 3, policy.MaxRecoveries)
	require.Equal(t, 30*time.Second, policy.RecoveryBackoff)
	require.False(t, policy.EnableAutoNudge)
	require.False(t, policy.EnableAutoReplace)
	require.False(t, policy.EnableAutoPause)

	err := policy.Validate()
	require.NoError(t, err, "default policy should be valid")
}

func TestHealthStatus_IsDegraded_TrueWhenUnhealthy(t *testing.T) {
	status := NewHealthStatus(WorkflowID("test-id"))
	status.IsHealthy = false

	policy := DefaultHealthPolicy()
	require.True(t, status.IsDegraded(policy))
}

func TestHealthStatus_IsDegraded_TrueWhenHeartbeatLagging(t *testing.T) {
	status := NewHealthStatus(WorkflowID("test-id"))
	status.IsHealthy = true
	// Set heartbeat to be older than half the timeout
	status.LastHeartbeatAt = time.Now().Add(-1*time.Minute - 1*time.Second)

	policy := DefaultHealthPolicy() // HeartbeatTimeout = 2m
	require.True(t, status.IsDegraded(policy))
}

func TestHealthStatus_IsDegraded_FalseWhenHealthyAndRecent(t *testing.T) {
	status := NewHealthStatus(WorkflowID("test-id"))
	status.IsHealthy = true
	status.LastHeartbeatAt = time.Now()

	policy := DefaultHealthPolicy()
	require.False(t, status.IsDegraded(policy))
}

func TestHealthStatus_NeedsRecovery_TrueWhenStuck(t *testing.T) {
	status := NewHealthStatus(WorkflowID("test-id"))
	// Make it stuck by setting LastProgressAt in the past
	status.LastProgressAt = time.Now().Add(-6 * time.Minute)

	policy := DefaultHealthPolicy() // ProgressTimeout = 5 minutes
	require.True(t, status.NeedsRecovery(policy))
}

func TestHealthStatus_NeedsRecovery_FalseWhenNotStuck(t *testing.T) {
	status := NewHealthStatus(WorkflowID("test-id"))
	// LastProgressAt is recent (set by NewHealthStatus)

	policy := DefaultHealthPolicy()
	require.False(t, status.NeedsRecovery(policy))
}

func TestHealthStatus_NeedsRecovery_TrueWhenMaxRecoveriesReached_ForFailAction(t *testing.T) {
	// When RecoveryCount == MaxRecoveries, we still need to trigger one more
	// recovery action: the "fail" action that terminates the workflow.
	status := NewHealthStatus(WorkflowID("test-id"))
	status.LastProgressAt = time.Now().Add(-6 * time.Minute) // stuck
	status.RecoveryCount = 3

	policy := DefaultHealthPolicy() // MaxRecoveries = 3
	// NeedsRecovery returns true because we need to execute the fail action
	require.True(t, status.NeedsRecovery(policy))
	// DetermineRecoveryAction will return RecoveryFail for this case
}

func TestHealthStatus_NeedsRecovery_FalseAfterAllRecoveriesExhausted(t *testing.T) {
	// When RecoveryCount > MaxRecoveries, all recovery attempts including
	// the fail action have been executed. No more recovery is needed.
	status := NewHealthStatus(WorkflowID("test-id"))
	status.LastProgressAt = time.Now().Add(-6 * time.Minute) // stuck
	status.RecoveryCount = 4                                 // One more than MaxRecoveries

	policy := DefaultHealthPolicy() // MaxRecoveries = 3
	require.False(t, status.NeedsRecovery(policy))
}

func TestHealthStatus_NeedsRecovery_FalseWhenInBackoff(t *testing.T) {
	status := NewHealthStatus(WorkflowID("test-id"))
	status.LastProgressAt = time.Now().Add(-6 * time.Minute) // stuck
	status.RecordRecoveryAttempt()                           // Sets LastRecoveryAt to now

	policy := DefaultHealthPolicy() // RecoveryBackoff = 30s
	require.False(t, status.NeedsRecovery(policy))
}

func TestHealthStatus_NeedsRecovery_TrueAfterBackoff(t *testing.T) {
	status := NewHealthStatus(WorkflowID("test-id"))
	status.LastProgressAt = time.Now().Add(-6 * time.Minute) // stuck
	// Simulate a recovery attempt that happened in the past
	pastRecovery := time.Now().Add(-31 * time.Second)
	status.LastRecoveryAt = &pastRecovery
	status.RecoveryCount = 1

	policy := DefaultHealthPolicy() // RecoveryBackoff = 30s
	require.True(t, status.NeedsRecovery(policy))
}

func TestHealthStatus_IsStuck_TrueWhenProgressTimeoutExceeded(t *testing.T) {
	status := NewHealthStatus(WorkflowID("test-id"))
	status.LastProgressAt = time.Now().Add(-6 * time.Minute)

	policy := DefaultHealthPolicy() // ProgressTimeout = 5 minutes
	require.True(t, status.IsStuck(policy))
}

func TestHealthStatus_IsStuck_FalseWhenProgressRecent(t *testing.T) {
	status := NewHealthStatus(WorkflowID("test-id"))
	// LastProgressAt is recent (set by NewHealthStatus)

	policy := DefaultHealthPolicy()
	require.False(t, status.IsStuck(policy))
}

func TestHealthStatus_ResetRecovery_ClearsCountAndTimestamp(t *testing.T) {
	status := NewHealthStatus(WorkflowID("test-id"))
	status.RecoveryCount = 2
	now := time.Now()
	status.LastRecoveryAt = &now

	status.ResetRecovery()

	require.Equal(t, 0, status.RecoveryCount)
	require.Nil(t, status.LastRecoveryAt)
}

func TestHealthStatus_RecordRecoveryAttempt_IncrementsCount(t *testing.T) {
	status := NewHealthStatus(WorkflowID("test-id"))
	require.Equal(t, 0, status.RecoveryCount)
	require.Nil(t, status.LastRecoveryAt)

	status.RecordRecoveryAttempt()

	require.Equal(t, 1, status.RecoveryCount)
	require.NotNil(t, status.LastRecoveryAt)
}

func TestHealthStatus_RecordRecoveryAttempt_MultipleTimes(t *testing.T) {
	status := NewHealthStatus(WorkflowID("test-id"))

	status.RecordRecoveryAttempt()
	status.RecordRecoveryAttempt()
	status.RecordRecoveryAttempt()

	require.Equal(t, 3, status.RecoveryCount)
}

func TestNewHealthStatus(t *testing.T) {
	workflowID := WorkflowID("test-workflow-123")
	status := NewHealthStatus(workflowID)

	require.Equal(t, workflowID, status.WorkflowID)
	require.True(t, status.IsHealthy)
	require.Equal(t, 0, status.RecoveryCount)
	require.Nil(t, status.LastRecoveryAt)
	// Timestamps should be recent
	require.WithinDuration(t, time.Now(), status.LastHeartbeatAt, time.Second)
	require.WithinDuration(t, time.Now(), status.LastProgressAt, time.Second)
}

func TestHealthEventType_Constants(t *testing.T) {
	// Verify the enum values are as expected (for documentation/stability)
	require.Equal(t, HealthEventType("health.heartbeat.missed"), HealthHeartbeatMissed)
	require.Equal(t, HealthEventType("health.stuck.suspected"), HealthStuckSuspected)
	require.Equal(t, HealthEventType("health.recovery.started"), HealthRecoveryStarted)
	require.Equal(t, HealthEventType("health.recovery.succeeded"), HealthRecoverySuccess)
	require.Equal(t, HealthEventType("health.recovery.failed"), HealthRecoveryFailed)
}

func TestNewHealthEvent(t *testing.T) {
	workflowID := WorkflowID("test-workflow-123")
	event := NewHealthEvent(HealthHeartbeatMissed, workflowID)

	require.Equal(t, HealthHeartbeatMissed, event.Type)
	require.Equal(t, workflowID, event.WorkflowID)
	require.WithinDuration(t, time.Now(), event.Timestamp, time.Second)
	require.Empty(t, event.Details)
	require.Empty(t, event.RecoveryAction)
}

func TestHealthEvent_WithDetails(t *testing.T) {
	event := NewHealthEvent(HealthStuckSuspected, WorkflowID("test-id"))
	event = event.WithDetails("No progress for 5 minutes")

	require.Equal(t, "No progress for 5 minutes", event.Details)
}

func TestHealthEvent_WithRecoveryAction(t *testing.T) {
	event := NewHealthEvent(HealthRecoveryStarted, WorkflowID("test-id"))
	event = event.WithRecoveryAction("nudge")

	require.Equal(t, "nudge", event.RecoveryAction)
}

func TestHealthEvent_Chaining(t *testing.T) {
	event := NewHealthEvent(HealthRecoveryStarted, WorkflowID("test-id")).
		WithDetails("Workflow stuck for 10 minutes").
		WithRecoveryAction("replace")

	require.Equal(t, HealthRecoveryStarted, event.Type)
	require.Equal(t, "Workflow stuck for 10 minutes", event.Details)
	require.Equal(t, "replace", event.RecoveryAction)
}
