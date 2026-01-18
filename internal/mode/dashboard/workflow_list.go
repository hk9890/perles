package dashboard

import (
	"sort"

	"github.com/zjrosen/perles/internal/orchestration/controlplane"
)

// SortField defines which field to sort workflows by.
type SortField int

const (
	SortByIndex SortField = iota
	SortByState
	SortByName
	SortByWorkers
	SortByTokens
	SortByHealth
)

// SortOrder defines the sort direction.
type SortOrder int

const (
	SortAscending SortOrder = iota
	SortDescending
)

// WorkflowList manages the workflow list display state.
type WorkflowList struct {
	workflows []*controlplane.WorkflowInstance
	sortField SortField
	sortOrder SortOrder
}

// NewWorkflowList creates a new workflow list component.
func NewWorkflowList() WorkflowList {
	return WorkflowList{
		workflows: make([]*controlplane.WorkflowInstance, 0),
		sortField: SortByIndex,
		sortOrder: SortAscending,
	}
}

// SetWorkflows updates the workflow list and applies current sorting.
func (l WorkflowList) SetWorkflows(workflows []*controlplane.WorkflowInstance) WorkflowList {
	// Copy the slice to avoid mutating the original
	l.workflows = make([]*controlplane.WorkflowInstance, len(workflows))
	copy(l.workflows, workflows)
	l.sort()
	return l
}

// Workflows returns the current sorted workflow list.
func (l WorkflowList) Workflows() []*controlplane.WorkflowInstance {
	return l.workflows
}

// SetSort updates the sort field and order.
func (l WorkflowList) SetSort(field SortField, order SortOrder) WorkflowList {
	l.sortField = field
	l.sortOrder = order
	l.sort()
	return l
}

// ToggleSort toggles sort order if same field, or sets new field with ascending order.
func (l WorkflowList) ToggleSort(field SortField) WorkflowList {
	if l.sortField == field {
		if l.sortOrder == SortAscending {
			l.sortOrder = SortDescending
		} else {
			l.sortOrder = SortAscending
		}
	} else {
		l.sortField = field
		l.sortOrder = SortAscending
	}
	l.sort()
	return l
}

// MoveDown moves selection down, wrapping at the end.
func (l WorkflowList) MoveDown(current, total int) int {
	if total == 0 {
		return 0
	}
	return (current + 1) % total
}

// MoveUp moves selection up, wrapping at the beginning.
func (l WorkflowList) MoveUp(current int) int {
	if current <= 0 {
		return 0
	}
	return current - 1
}

// sort applies the current sort field and order to the workflow list.
func (l *WorkflowList) sort() {
	if len(l.workflows) == 0 {
		return
	}

	sort.SliceStable(l.workflows, func(i, j int) bool {
		less := l.compareLess(l.workflows[i], l.workflows[j])
		if l.sortOrder == SortDescending {
			return !less
		}
		return less
	})
}

// compareLess returns true if a should come before b in ascending order.
func (l *WorkflowList) compareLess(a, b *controlplane.WorkflowInstance) bool {
	switch l.sortField {
	case SortByState:
		return stateOrder(a.State) < stateOrder(b.State)
	case SortByName:
		return a.Name < b.Name
	case SortByWorkers:
		return a.ActiveWorkers < b.ActiveWorkers
	case SortByTokens:
		return a.TokensUsed < b.TokensUsed
	case SortByHealth:
		return healthOrder(a) < healthOrder(b)
	default: // SortByIndex - maintain original order
		return false
	}
}

// stateOrder returns a numeric order for workflow states.
// Running workflows come first, then pending, paused, and finally terminal states.
func stateOrder(state controlplane.WorkflowState) int {
	switch state {
	case controlplane.WorkflowRunning:
		return 0
	case controlplane.WorkflowPending:
		return 1
	case controlplane.WorkflowPaused:
		return 2
	case controlplane.WorkflowCompleted:
		return 3
	case controlplane.WorkflowFailed:
		return 4
	case controlplane.WorkflowStopped:
		return 5
	default:
		return 6
	}
}

// healthOrder returns a numeric order for workflow health.
// Unhealthy workflows come first (to surface problems).
func healthOrder(wf *controlplane.WorkflowInstance) int {
	if !wf.IsActive() {
		return 3 // Terminal workflows last
	}
	if wf.IsPaused() {
		return 1 // Paused might need attention
	}
	if wf.IsRunning() {
		return 2 // Running and healthy
	}
	return 3 // Unknown
}

// Count returns the number of workflows in the list.
func (l WorkflowList) Count() int {
	return len(l.workflows)
}

// IsEmpty returns true if the workflow list is empty.
func (l WorkflowList) IsEmpty() bool {
	return len(l.workflows) == 0
}

// CountByState returns counts of workflows grouped by state.
func (l WorkflowList) CountByState() map[controlplane.WorkflowState]int {
	counts := make(map[controlplane.WorkflowState]int)
	for _, wf := range l.workflows {
		counts[wf.State]++
	}
	return counts
}
