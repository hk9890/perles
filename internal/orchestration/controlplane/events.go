// Package controlplane provides event types for control plane operations.
package controlplane

import (
	"slices"
	"time"

	"github.com/zjrosen/perles/internal/orchestration/events"
)

// EventType categorizes control plane events.
type EventType string

const (
	// Workflow lifecycle events
	EventWorkflowCreated   EventType = "workflow.created"
	EventWorkflowStarted   EventType = "workflow.started"
	EventWorkflowPaused    EventType = "workflow.paused"
	EventWorkflowResumed   EventType = "workflow.resumed"
	EventWorkflowCompleted EventType = "workflow.completed"
	EventWorkflowFailed    EventType = "workflow.failed"

	// Coordinator events
	EventCoordinatorSpawned  EventType = "coordinator.spawned"
	EventCoordinatorReplaced EventType = "coordinator.replaced"
	EventCoordinatorOutput   EventType = "coordinator.output"
	EventCoordinatorIncoming EventType = "coordinator.incoming"

	// Worker events
	EventWorkerSpawned  EventType = "worker.spawned"
	EventWorkerRetired  EventType = "worker.retired"
	EventWorkerOutput   EventType = "worker.output"
	EventWorkerIncoming EventType = "worker.incoming"

	// Task events
	EventTaskAssigned  EventType = "task.assigned"
	EventTaskCompleted EventType = "task.completed"
	EventTaskFailed    EventType = "task.failed"

	// Message events
	EventMessagePosted EventType = "message.posted"

	// User notification events
	EventUserNotification EventType = "user.notification"

	// Health events
	EventHealthUnhealthy  EventType = "health.unhealthy"
	EventHealthStuck      EventType = "health.stuck"
	EventHealthRecovering EventType = "health.recovering"
	EventHealthRecovered  EventType = "health.recovered"

	// Unknown event type for unclassified events
	EventUnknown EventType = "unknown"
)

// ControlPlaneEvent is the envelope for all control plane events.
// It provides a consistent structure for events with workflow context.
type ControlPlaneEvent struct {
	// Type identifies the kind of event.
	Type EventType
	// Timestamp when the event occurred.
	Timestamp time.Time

	// Workflow context (always present for workflow-related events)
	WorkflowID   WorkflowID
	TemplateID   string
	WorkflowName string
	State        WorkflowState

	// Optional correlation IDs (present for process/task events)
	ProcessID string
	TaskID    string

	// Event-specific payload (depends on Type)
	Payload any
}

// NewControlPlaneEvent creates a new event with the current timestamp.
func NewControlPlaneEvent(eventType EventType, payload any) ControlPlaneEvent {
	return ControlPlaneEvent{
		Type:      eventType,
		Timestamp: time.Now(),
		Payload:   payload,
	}
}

// WithWorkflow adds workflow context to the event.
func (e ControlPlaneEvent) WithWorkflow(inst *WorkflowInstance) ControlPlaneEvent {
	e.WorkflowID = inst.ID
	e.TemplateID = inst.TemplateID
	e.WorkflowName = inst.Name
	e.State = inst.State
	return e
}

// WithProcess adds process context to the event.
func (e ControlPlaneEvent) WithProcess(processID string) ControlPlaneEvent {
	e.ProcessID = processID
	return e
}

// WithTask adds task context to the event.
func (e ControlPlaneEvent) WithTask(taskID string) ControlPlaneEvent {
	e.TaskID = taskID
	return e
}

// WorkflowPausedPayload contains details about workflow pausing.
type WorkflowPausedPayload struct {
	// Reason why the workflow was paused.
	Reason string
	// TriggeredBy indicates what triggered the pause.
	// Examples: "user", "health_monitor"
	TriggeredBy string
}

// ClassifyEvent maps a v2 ProcessEvent to the appropriate ControlPlane EventType.
// It inspects the event's Type and Role to determine the correct classification.
// Unknown events are mapped to EventUnknown.
func ClassifyEvent(v2Event any) EventType {
	processEvent, ok := v2Event.(events.ProcessEvent)
	if !ok {
		return EventUnknown
	}

	// Classify based on event type and role
	switch processEvent.Type {
	case events.ProcessSpawned:
		if processEvent.Role == events.RoleCoordinator {
			return EventCoordinatorSpawned
		}
		return EventWorkerSpawned

	case events.ProcessOutput:
		if processEvent.Role == events.RoleCoordinator {
			return EventCoordinatorOutput
		}
		return EventWorkerOutput

	case events.ProcessStatusChange:
		// Map status changes to more specific events
		if processEvent.Status == events.ProcessStatusRetired {
			if processEvent.Role == events.RoleCoordinator {
				return EventCoordinatorReplaced
			}
			return EventWorkerRetired
		}
		// Generic status change - classify by role
		if processEvent.Role == events.RoleCoordinator {
			return EventCoordinatorOutput
		}
		return EventWorkerOutput

	case events.ProcessReady, events.ProcessWorking, events.ProcessTokenUsage:
		// Ready/Working/TokenUsage state transitions - classify by role
		if processEvent.Role == events.RoleCoordinator {
			return EventCoordinatorOutput
		}
		return EventWorkerOutput

	case events.ProcessWorkflowComplete:
		return EventWorkflowCompleted

	case events.ProcessError:
		if processEvent.Role == events.RoleWorker {
			// Worker errors might indicate task failure
			return EventTaskFailed
		}
		return EventUnknown

	case events.ProcessUserNotification:
		return EventUserNotification

	case events.ProcessIncoming:
		if processEvent.Role == events.RoleCoordinator {
			return EventCoordinatorIncoming
		}
		return EventWorkerIncoming

	default:
		return EventUnknown
	}
}

// IsLifecycleEvent returns true if the event type is a workflow lifecycle event.
func (t EventType) IsLifecycleEvent() bool {
	switch t {
	case EventWorkflowCreated,
		EventWorkflowStarted,
		EventWorkflowPaused,
		EventWorkflowResumed,
		EventWorkflowCompleted,
		EventWorkflowFailed:
		return true
	default:
		return false
	}
}

// IsCoordinatorEvent returns true if the event type is a coordinator event.
func (t EventType) IsCoordinatorEvent() bool {
	switch t {
	case EventCoordinatorSpawned,
		EventCoordinatorReplaced,
		EventCoordinatorOutput,
		EventCoordinatorIncoming:
		return true
	default:
		return false
	}
}

// IsWorkerEvent returns true if the event type is a worker event.
func (t EventType) IsWorkerEvent() bool {
	switch t {
	case EventWorkerSpawned,
		EventWorkerRetired,
		EventWorkerOutput,
		EventWorkerIncoming:
		return true
	default:
		return false
	}
}

// IsTaskEvent returns true if the event type is a task event.
func (t EventType) IsTaskEvent() bool {
	switch t {
	case EventTaskAssigned,
		EventTaskCompleted,
		EventTaskFailed:
		return true
	default:
		return false
	}
}

// IsMessageEvent returns true if the event type is a message event.
func (t EventType) IsMessageEvent() bool {
	return t == EventMessagePosted
}

// IsHealthEvent returns true if the event type is a health event.
func (t EventType) IsHealthEvent() bool {
	switch t {
	case EventHealthUnhealthy,
		EventHealthStuck,
		EventHealthRecovering,
		EventHealthRecovered:
		return true
	default:
		return false
	}
}

// String returns the string representation of the EventType.
func (t EventType) String() string {
	return string(t)
}

// EventFilter defines criteria for filtering ControlPlaneEvents in subscriptions.
// All criteria are AND'd together - an event must match all specified criteria
// to pass the filter.
type EventFilter struct {
	// Types limits events to these specific types. If empty, all types are allowed.
	Types []EventType

	// WorkflowIDs limits events to these specific workflows. If empty, all workflows are allowed.
	WorkflowIDs []WorkflowID

	// ExcludeTypes excludes events of these types. Applied after Types filter.
	ExcludeTypes []EventType
}

// Matches returns true if the event matches the filter criteria.
// An empty filter matches all events.
func (f *EventFilter) Matches(event ControlPlaneEvent) bool {
	// Check type inclusion
	if len(f.Types) > 0 {
		if !f.containsType(f.Types, event.Type) {
			return false
		}
	}

	// Check workflow inclusion
	if len(f.WorkflowIDs) > 0 {
		if !f.containsWorkflowID(f.WorkflowIDs, event.WorkflowID) {
			return false
		}
	}

	// Check type exclusion (applied after inclusion filters)
	if len(f.ExcludeTypes) > 0 {
		if f.containsType(f.ExcludeTypes, event.Type) {
			return false
		}
	}

	return true
}

// containsType checks if the event type is in the list.
func (f *EventFilter) containsType(types []EventType, t EventType) bool {
	return slices.Contains(types, t)
}

// containsWorkflowID checks if the workflow ID is in the list.
func (f *EventFilter) containsWorkflowID(ids []WorkflowID, id WorkflowID) bool {
	return slices.Contains(ids, id)
}

// IsEmpty returns true if the filter has no criteria set.
func (f *EventFilter) IsEmpty() bool {
	return len(f.Types) == 0 && len(f.WorkflowIDs) == 0 && len(f.ExcludeTypes) == 0
}
