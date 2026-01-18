package controlplane

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/zjrosen/perles/internal/orchestration/events"
	"github.com/zjrosen/perles/internal/pubsub"
)

// mockClock is a controllable clock for testing.
type mockClock struct {
	mu  sync.Mutex
	now time.Time
}

func newMockClock(t time.Time) *mockClock {
	return &mockClock{now: t}
}

func (c *mockClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *mockClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

func (c *mockClock) Set(t time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = t
}

func TestHealthMonitor_RecordHeartbeat_UpdatesLastHeartbeatAt(t *testing.T) {
	clock := newMockClock(time.Now())
	monitor := NewHealthMonitor(HealthMonitorConfig{
		Policy: DefaultHealthPolicy(),
		Clock:  clock,
	})

	workflowID := WorkflowID("workflow-1")
	monitor.TrackWorkflow(workflowID)

	// Advance clock and record heartbeat
	clock.Advance(30 * time.Second)
	monitor.RecordHeartbeat(workflowID)

	status, ok := monitor.GetStatus(workflowID)
	require.True(t, ok)
	require.Equal(t, clock.Now(), status.LastHeartbeatAt)
	require.True(t, status.IsHealthy)
}

func TestHealthMonitor_RecordProgress_UpdatesLastProgressAt(t *testing.T) {
	clock := newMockClock(time.Now())
	monitor := NewHealthMonitor(HealthMonitorConfig{
		Policy: DefaultHealthPolicy(),
		Clock:  clock,
	})

	workflowID := WorkflowID("workflow-1")
	monitor.TrackWorkflow(workflowID)

	// Advance clock and record progress
	clock.Advance(30 * time.Second)
	monitor.RecordProgress(workflowID)

	status, ok := monitor.GetStatus(workflowID)
	require.True(t, ok)
	require.Equal(t, clock.Now(), status.LastProgressAt)
	require.Equal(t, clock.Now(), status.LastHeartbeatAt, "progress should also update heartbeat")
	require.True(t, status.IsHealthy)
}

func TestHealthMonitor_GetStatus_ReturnsCorrectStatus(t *testing.T) {
	clock := newMockClock(time.Now())
	monitor := NewHealthMonitor(HealthMonitorConfig{
		Policy: DefaultHealthPolicy(),
		Clock:  clock,
	})

	workflowID := WorkflowID("workflow-1")

	// Not tracked initially
	_, ok := monitor.GetStatus(workflowID)
	require.False(t, ok)

	// Track workflow
	monitor.TrackWorkflow(workflowID)

	// Now should be found
	status, ok := monitor.GetStatus(workflowID)
	require.True(t, ok)
	require.Equal(t, workflowID, status.WorkflowID)
	require.True(t, status.IsHealthy)
}

func TestHealthMonitor_CheckLoop_MarksUnhealthyAfterHeartbeatTimeout(t *testing.T) {
	clock := newMockClock(time.Now())
	policy := HealthPolicy{
		HeartbeatTimeout: 100 * time.Millisecond,
		ProgressTimeout:  500 * time.Millisecond,
		MaxRecoveries:    3,
		RecoveryBackoff:  10 * time.Millisecond,
	}

	var receivedEvents []HealthEvent
	var eventMu sync.Mutex

	monitor := NewHealthMonitor(HealthMonitorConfig{
		Policy:        policy,
		CheckInterval: 20 * time.Millisecond,
		Clock:         clock,
		OnHealthEvent: func(event HealthEvent) {
			eventMu.Lock()
			receivedEvents = append(receivedEvents, event)
			eventMu.Unlock()
		},
	})

	workflowID := WorkflowID("workflow-1")
	monitor.TrackWorkflow(workflowID)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := monitor.Start(ctx)
	require.NoError(t, err)

	// Advance clock past heartbeat timeout
	clock.Advance(150 * time.Millisecond)

	// Wait for check loop to run
	time.Sleep(50 * time.Millisecond)

	status, ok := monitor.GetStatus(workflowID)
	require.True(t, ok)
	require.False(t, status.IsHealthy)

	// Should have received heartbeat missed event
	eventMu.Lock()
	require.Len(t, receivedEvents, 1)
	require.Equal(t, HealthHeartbeatMissed, receivedEvents[0].Type)
	eventMu.Unlock()

	monitor.Stop()
}

func TestHealthMonitor_CheckLoop_MarksStuckAfterProgressTimeout(t *testing.T) {
	clock := newMockClock(time.Now())
	policy := HealthPolicy{
		HeartbeatTimeout: 50 * time.Millisecond,
		ProgressTimeout:  100 * time.Millisecond,
		MaxRecoveries:    3,
		RecoveryBackoff:  10 * time.Millisecond,
	}

	var receivedEvents []HealthEvent
	var eventMu sync.Mutex

	monitor := NewHealthMonitor(HealthMonitorConfig{
		Policy:        policy,
		CheckInterval: 20 * time.Millisecond,
		Clock:         clock,
		OnHealthEvent: func(event HealthEvent) {
			eventMu.Lock()
			receivedEvents = append(receivedEvents, event)
			eventMu.Unlock()
		},
	})

	workflowID := WorkflowID("workflow-1")
	monitor.TrackWorkflow(workflowID)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := monitor.Start(ctx)
	require.NoError(t, err)

	// Advance clock past progress timeout (also past heartbeat timeout)
	clock.Advance(150 * time.Millisecond)

	// Wait for check loop to run
	time.Sleep(50 * time.Millisecond)

	status, ok := monitor.GetStatus(workflowID)
	require.True(t, ok)
	require.True(t, status.IsStuckAt(policy, clock.Now()))

	// Should have received both heartbeat missed and stuck suspected events
	eventMu.Lock()
	require.GreaterOrEqual(t, len(receivedEvents), 2)
	eventTypes := make(map[HealthEventType]bool)
	for _, e := range receivedEvents {
		eventTypes[e.Type] = true
	}
	require.True(t, eventTypes[HealthHeartbeatMissed])
	require.True(t, eventTypes[HealthStuckSuspected])
	eventMu.Unlock()

	monitor.Stop()
}

func TestHealthMonitor_EventBus_TriggersHeartbeatOnAnyEvent(t *testing.T) {
	clock := newMockClock(time.Now())
	eventBus := pubsub.NewBroker[ControlPlaneEvent]()
	defer eventBus.Close()

	monitor := NewHealthMonitor(HealthMonitorConfig{
		Policy:        DefaultHealthPolicy(),
		CheckInterval: 1 * time.Second, // Long interval so we test event handling
		Clock:         clock,
		EventBus:      eventBus,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := monitor.Start(ctx)
	require.NoError(t, err)

	// Wait for subscription to be established
	time.Sleep(10 * time.Millisecond)

	// Publish a process event wrapped in ControlPlaneEvent
	processEvent := events.ProcessEvent{
		Type:      events.ProcessOutput,
		ProcessID: "workflow-1",
		Role:      events.RoleCoordinator,
		Output:    "some output",
	}
	cpEvent := ControlPlaneEvent{
		WorkflowID: "workflow-1",
		Payload:    processEvent,
	}
	eventBus.Publish(pubsub.UpdatedEvent, cpEvent)

	// Wait for event to be processed
	time.Sleep(20 * time.Millisecond)

	// Should have auto-tracked and recorded heartbeat
	status, ok := monitor.GetStatus(WorkflowID("workflow-1"))
	require.True(t, ok)
	require.True(t, status.IsHealthy)

	monitor.Stop()
}

func TestHealthMonitor_EventBus_TriggersProgressOnPhaseTransition(t *testing.T) {
	clock := newMockClock(time.Now())
	eventBus := pubsub.NewBroker[ControlPlaneEvent]()
	defer eventBus.Close()

	monitor := NewHealthMonitor(HealthMonitorConfig{
		Policy:        DefaultHealthPolicy(),
		CheckInterval: 1 * time.Second,
		Clock:         clock,
		EventBus:      eventBus,
	})

	workflowID := WorkflowID("workflow-1")
	monitor.TrackWorkflow(workflowID)

	// Record initial timestamps
	initialStatus, _ := monitor.GetStatus(workflowID)
	initialProgressAt := initialStatus.LastProgressAt

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := monitor.Start(ctx)
	require.NoError(t, err)

	// Wait for subscription
	time.Sleep(10 * time.Millisecond)

	// Advance clock so we can detect the progress update
	clock.Advance(1 * time.Second)

	// Publish a phase transition event (indicates progress)
	implementingPhase := events.ProcessPhaseImplementing
	processEvent := events.ProcessEvent{
		Type:      events.ProcessStatusChange,
		ProcessID: string(workflowID),
		Role:      events.RoleWorker,
		Status:    events.ProcessStatusWorking,
		Phase:     &implementingPhase,
	}
	cpEvent := ControlPlaneEvent{
		WorkflowID: workflowID,
		Payload:    processEvent,
	}
	eventBus.Publish(pubsub.UpdatedEvent, cpEvent)

	// Wait for event to be processed
	time.Sleep(20 * time.Millisecond)

	// Should have updated progress timestamp
	status, ok := monitor.GetStatus(workflowID)
	require.True(t, ok)
	require.True(t, status.LastProgressAt.After(initialProgressAt))

	monitor.Stop()
}

func TestHealthMonitor_Stop_CleanlyShutsDownCheckLoop(t *testing.T) {
	monitor := NewHealthMonitor(HealthMonitorConfig{
		Policy:        DefaultHealthPolicy(),
		CheckInterval: 10 * time.Millisecond,
	})

	ctx := context.Background()

	err := monitor.Start(ctx)
	require.NoError(t, err)

	// Wait a bit for loop to run
	time.Sleep(30 * time.Millisecond)

	// Stop should return without hanging
	done := make(chan struct{})
	go func() {
		monitor.Stop()
		close(done)
	}()

	select {
	case <-done:
		// Success - Stop returned
	case <-time.After(1 * time.Second):
		t.Fatal("Stop did not return within timeout")
	}
}

func TestHealthMonitor_MultipleWorkflows_TrackedIndependently(t *testing.T) {
	clock := newMockClock(time.Now())
	monitor := NewHealthMonitor(HealthMonitorConfig{
		Policy: DefaultHealthPolicy(),
		Clock:  clock,
	})

	workflow1 := WorkflowID("workflow-1")
	workflow2 := WorkflowID("workflow-2")

	monitor.TrackWorkflow(workflow1)
	monitor.TrackWorkflow(workflow2)

	// Advance clock
	clock.Advance(30 * time.Second)

	// Record heartbeat for workflow1 only
	monitor.RecordHeartbeat(workflow1)

	status1, ok := monitor.GetStatus(workflow1)
	require.True(t, ok)
	require.Equal(t, clock.Now(), status1.LastHeartbeatAt)

	status2, ok := monitor.GetStatus(workflow2)
	require.True(t, ok)
	// workflow2's heartbeat should still be at the initial time
	require.True(t, status2.LastHeartbeatAt.Before(status1.LastHeartbeatAt))
}

func TestHealthMonitor_GetAllStatuses_ReturnsAllTracked(t *testing.T) {
	monitor := NewHealthMonitor(HealthMonitorConfig{
		Policy: DefaultHealthPolicy(),
	})

	workflow1 := WorkflowID("workflow-1")
	workflow2 := WorkflowID("workflow-2")
	workflow3 := WorkflowID("workflow-3")

	monitor.TrackWorkflow(workflow1)
	monitor.TrackWorkflow(workflow2)
	monitor.TrackWorkflow(workflow3)

	statuses := monitor.GetAllStatuses()
	require.Len(t, statuses, 3)

	ids := make(map[WorkflowID]bool)
	for _, s := range statuses {
		ids[s.WorkflowID] = true
	}
	require.True(t, ids[workflow1])
	require.True(t, ids[workflow2])
	require.True(t, ids[workflow3])
}

func TestHealthMonitor_UntrackWorkflow_RemovesWorkflow(t *testing.T) {
	monitor := NewHealthMonitor(HealthMonitorConfig{
		Policy: DefaultHealthPolicy(),
	})

	workflowID := WorkflowID("workflow-1")

	monitor.TrackWorkflow(workflowID)
	_, ok := monitor.GetStatus(workflowID)
	require.True(t, ok)

	monitor.UntrackWorkflow(workflowID)
	_, ok = monitor.GetStatus(workflowID)
	require.False(t, ok)
}

func TestHealthMonitor_RecordProgress_ResetsRecoveryCounter(t *testing.T) {
	clock := newMockClock(time.Now())
	policy := HealthPolicy{
		HeartbeatTimeout: 50 * time.Millisecond,
		ProgressTimeout:  100 * time.Millisecond,
		MaxRecoveries:    3,
		RecoveryBackoff:  10 * time.Millisecond,
	}
	monitor := NewHealthMonitor(HealthMonitorConfig{
		Policy: policy,
		Clock:  clock,
	})

	workflowID := WorkflowID("workflow-1")
	monitor.TrackWorkflow(workflowID)

	// Make workflow stuck by setting LastProgressAt in the past
	m := monitor.(*defaultHealthMonitor)
	m.mu.Lock()
	m.statuses[workflowID].LastProgressAt = clock.Now().Add(-150 * time.Millisecond)
	m.statuses[workflowID].RecoveryCount = 2 // Simulate previous recovery attempts
	m.mu.Unlock()

	status, _ := monitor.GetStatus(workflowID)
	require.True(t, status.IsStuck(policy))
	require.Equal(t, 2, status.RecoveryCount)

	// Record progress should reset recovery counter and update LastProgressAt
	monitor.RecordProgress(workflowID)

	status, _ = monitor.GetStatus(workflowID)
	require.False(t, status.IsStuck(policy))
	require.Equal(t, 0, status.RecoveryCount)
}

func TestHealthMonitor_SetPolicy_UpdatesPolicy(t *testing.T) {
	clock := newMockClock(time.Now())
	initialPolicy := DefaultHealthPolicy()

	monitor := NewHealthMonitor(HealthMonitorConfig{
		Policy: initialPolicy,
		Clock:  clock,
	})

	newPolicy := HealthPolicy{
		HeartbeatTimeout: 1 * time.Minute,
		ProgressTimeout:  3 * time.Minute,
		MaxRecoveries:    5,
		RecoveryBackoff:  1 * time.Minute,
	}

	monitor.SetPolicy(newPolicy)

	// The internal policy should be updated
	m := monitor.(*defaultHealthMonitor)
	m.mu.RLock()
	require.Equal(t, newPolicy.HeartbeatTimeout, m.policy.HeartbeatTimeout)
	require.Equal(t, newPolicy.ProgressTimeout, m.policy.ProgressTimeout)
	require.Equal(t, newPolicy.MaxRecoveries, m.policy.MaxRecoveries)
	m.mu.RUnlock()
}

func TestHealthMonitor_Start_Idempotent(t *testing.T) {
	monitor := NewHealthMonitor(HealthMonitorConfig{
		Policy:        DefaultHealthPolicy(),
		CheckInterval: 10 * time.Millisecond,
	})

	ctx := context.Background()

	err1 := monitor.Start(ctx)
	require.NoError(t, err1)

	err2 := monitor.Start(ctx)
	require.NoError(t, err2, "second Start should be no-op")

	monitor.Stop()
}

func TestHealthMonitor_RecordHeartbeat_AutoTracksUnknownWorkflow(t *testing.T) {
	clock := newMockClock(time.Now())
	monitor := NewHealthMonitor(HealthMonitorConfig{
		Policy: DefaultHealthPolicy(),
		Clock:  clock,
	})

	workflowID := WorkflowID("unknown-workflow")

	// Not tracked initially
	_, ok := monitor.GetStatus(workflowID)
	require.False(t, ok)

	// Record heartbeat should auto-track
	monitor.RecordHeartbeat(workflowID)

	status, ok := monitor.GetStatus(workflowID)
	require.True(t, ok)
	require.Equal(t, workflowID, status.WorkflowID)
}

func TestHealthMonitor_RecordProgress_AutoTracksUnknownWorkflow(t *testing.T) {
	clock := newMockClock(time.Now())
	monitor := NewHealthMonitor(HealthMonitorConfig{
		Policy: DefaultHealthPolicy(),
		Clock:  clock,
	})

	workflowID := WorkflowID("unknown-workflow")

	// Not tracked initially
	_, ok := monitor.GetStatus(workflowID)
	require.False(t, ok)

	// Record progress should auto-track
	monitor.RecordProgress(workflowID)

	status, ok := monitor.GetStatus(workflowID)
	require.True(t, ok)
	require.Equal(t, workflowID, status.WorkflowID)
}

func TestHealthMonitor_EventBus_StatusChangeToWorkingIsProgress(t *testing.T) {
	clock := newMockClock(time.Now())
	eventBus := pubsub.NewBroker[ControlPlaneEvent]()
	defer eventBus.Close()

	monitor := NewHealthMonitor(HealthMonitorConfig{
		Policy:        DefaultHealthPolicy(),
		CheckInterval: 1 * time.Second,
		Clock:         clock,
		EventBus:      eventBus,
	})

	workflowID := WorkflowID("workflow-1")
	monitor.TrackWorkflow(workflowID)

	initialStatus, _ := monitor.GetStatus(workflowID)
	initialProgressAt := initialStatus.LastProgressAt

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := monitor.Start(ctx)
	require.NoError(t, err)
	time.Sleep(10 * time.Millisecond)

	clock.Advance(1 * time.Second)

	// Status change to Working is progress
	processEvent := events.ProcessEvent{
		Type:      events.ProcessStatusChange,
		ProcessID: string(workflowID),
		Role:      events.RoleCoordinator,
		Status:    events.ProcessStatusWorking,
	}
	cpEvent := ControlPlaneEvent{
		WorkflowID: workflowID,
		Payload:    processEvent,
	}
	eventBus.Publish(pubsub.UpdatedEvent, cpEvent)
	time.Sleep(20 * time.Millisecond)

	status, _ := monitor.GetStatus(workflowID)
	require.True(t, status.LastProgressAt.After(initialProgressAt))

	monitor.Stop()
}

func TestHealthMonitor_EventBus_WorkflowCompleteIsProgress(t *testing.T) {
	clock := newMockClock(time.Now())
	eventBus := pubsub.NewBroker[ControlPlaneEvent]()
	defer eventBus.Close()

	monitor := NewHealthMonitor(HealthMonitorConfig{
		Policy:        DefaultHealthPolicy(),
		CheckInterval: 1 * time.Second,
		Clock:         clock,
		EventBus:      eventBus,
	})

	workflowID := WorkflowID("workflow-1")
	monitor.TrackWorkflow(workflowID)

	initialStatus, _ := monitor.GetStatus(workflowID)
	initialProgressAt := initialStatus.LastProgressAt

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := monitor.Start(ctx)
	require.NoError(t, err)
	time.Sleep(10 * time.Millisecond)

	clock.Advance(1 * time.Second)

	// Workflow complete is progress
	processEvent2 := events.ProcessEvent{
		Type:      events.ProcessWorkflowComplete,
		ProcessID: string(workflowID),
		Role:      events.RoleCoordinator,
	}
	cpEvent2 := ControlPlaneEvent{
		WorkflowID: workflowID,
		Payload:    processEvent2,
	}
	eventBus.Publish(pubsub.UpdatedEvent, cpEvent2)
	time.Sleep(20 * time.Millisecond)

	status, _ := monitor.GetStatus(workflowID)
	require.True(t, status.LastProgressAt.After(initialProgressAt))

	monitor.Stop()
}

func TestHealthMonitor_Stop_SafeToCallBeforeStart(t *testing.T) {
	monitor := NewHealthMonitor(HealthMonitorConfig{
		Policy: DefaultHealthPolicy(),
	})

	// Should not panic
	monitor.Stop()
}

func TestHealthMonitor_Stop_SafeToCallMultipleTimes(t *testing.T) {
	monitor := NewHealthMonitor(HealthMonitorConfig{
		Policy:        DefaultHealthPolicy(),
		CheckInterval: 10 * time.Millisecond,
	})

	ctx := context.Background()
	err := monitor.Start(ctx)
	require.NoError(t, err)

	// Multiple stops should not panic
	monitor.Stop()
	monitor.Stop()
	monitor.Stop()
}

func TestIsProgressEvent(t *testing.T) {
	tests := []struct {
		name     string
		event    events.ProcessEvent
		expected bool
	}{
		{
			name: "status change to working is progress",
			event: events.ProcessEvent{
				Type:   events.ProcessStatusChange,
				Status: events.ProcessStatusWorking,
			},
			expected: true,
		},
		{
			name: "status change to ready is progress",
			event: events.ProcessEvent{
				Type:   events.ProcessStatusChange,
				Status: events.ProcessStatusReady,
			},
			expected: true,
		},
		{
			name: "workflow complete is progress",
			event: events.ProcessEvent{
				Type: events.ProcessWorkflowComplete,
			},
			expected: true,
		},
		{
			name: "phase transition is progress",
			event: events.ProcessEvent{
				Type:  events.ProcessStatusChange,
				Phase: func() *events.ProcessPhase { p := events.ProcessPhaseImplementing; return &p }(),
			},
			expected: true,
		},
		{
			name: "output is not progress",
			event: events.ProcessEvent{
				Type: events.ProcessOutput,
			},
			expected: false,
		},
		{
			name: "spawned is not progress",
			event: events.ProcessEvent{
				Type: events.ProcessSpawned,
			},
			expected: false,
		},
		{
			name: "error is not progress",
			event: events.ProcessEvent{
				Type: events.ProcessError,
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isProgressEvent(tt.event)
			require.Equal(t, tt.expected, result)
		})
	}
}
