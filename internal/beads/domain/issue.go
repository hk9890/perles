package domain

import (
	"encoding/json"
	"time"
)

// Status represents the issue lifecycle state.
type Status string

const (
	StatusOpen       Status = "open"
	StatusInProgress Status = "in_progress"
	StatusClosed     Status = "closed"
	StatusDeferred   Status = "deferred"
	StatusBlocked    Status = "blocked"
)

// Priority levels (0-4, lower is more urgent).
type Priority int

const (
	PriorityCritical Priority = 0
	PriorityHigh     Priority = 1
	PriorityMedium   Priority = 2
	PriorityLow      Priority = 3
	PriorityBacklog  Priority = 4
)

// IssueType categorizes the nature of work.
type IssueType string

const (
	TypeBug      IssueType = "bug"
	TypeFeature  IssueType = "feature"
	TypeTask     IssueType = "task"
	TypeEpic     IssueType = "epic"
	TypeChore    IssueType = "chore"
	TypeDecision IssueType = "decision"

	// Legacy/custom issue types retained for compatibility.
	// These are not treated as guaranteed built-in core types in UI rendering.
	TypeMolecule IssueType = "molecule"
	TypeConvoy   IssueType = "convoy"
	TypeAgent    IssueType = "agent"
)

// Comment represents a comment on an issue.
type Comment struct {
	ID        string    `json:"id"`
	Author    string    `json:"author"`
	Text      string    `json:"text"`
	CreatedAt time.Time `json:"created_at"`
}

// Issue represents a beads issue.
type Issue struct {
	ID                 string    `json:"id"`
	TitleText          string    `json:"title"`
	DescriptionText    string    `json:"description"`
	Design             string    `json:"design"`
	AcceptanceCriteria string    `json:"acceptance_criteria"`
	Notes              string    `json:"notes"`
	Status             Status    `json:"status"`
	Priority           Priority  `json:"priority"`
	Type               IssueType `json:"issue_type"`
	Assignee           string    `json:"assignee"`
	Sender             string    `json:"sender,omitempty"`
	Ephemeral          bool      `json:"ephemeral,omitempty"`
	Pinned             *bool     `json:"pinned,omitempty"`
	IsTemplate         *bool     `json:"is_template,omitempty"`
	Labels             []string  `json:"labels"`
	CreatedAt          time.Time `json:"created_at"`
	CreatedBy          string    `json:"created_by,omitempty"`
	UpdatedAt          time.Time `json:"updated_at"`
	ClosedAt           time.Time `json:"closed_at"`
	CloseReason        string    `json:"close_reason,omitempty"`

	// Agent fields (agent-as-bead pattern)
	HookBead     string    `json:"hook_bead,omitempty"`
	RoleBead     string    `json:"role_bead,omitempty"`
	AgentState   string    `json:"agent_state,omitempty"`
	LastActivity time.Time `json:"last_activity,omitzero"`
	RoleType     string    `json:"role_type,omitempty"`
	Rig          string    `json:"rig,omitempty"`
	MolType      string    `json:"mol_type,omitempty"`

	// Dependency tracking
	BlockedBy      []string `json:"blocked_by"`
	Blocks         []string `json:"blocks"`
	Children       []string `json:"children"`
	DiscoveredFrom []string `json:"discovered_from"`
	Discovered     []string `json:"discovered"`
	ParentID       string   `json:"parent_id"`

	// Comments (populated on demand)
	Comments []Comment `json:"comments,omitempty"`

	// CommentCount is populated by BQL queries for display without loading full comments
	CommentCount int `json:"comment_count,omitempty"`
}

type issueDependency struct {
	ID             string `json:"id"`
	DependencyType string `json:"dependency_type"`
	Type           string `json:"type"`
}

// UnmarshalJSON supports both legacy and beads v1 show payloads.
//
// beads v1 emits:
//   - issue_type (not type)
//   - owner (not assignee)
//   - dependencies/dependents arrays with dependency_type metadata
func (i *Issue) UnmarshalJSON(data []byte) error {
	type issueAlias Issue
	var aux struct {
		issueAlias
		LegacyType       IssueType         `json:"type"`
		V1IssueType      IssueType         `json:"issue_type"`
		Owner            string            `json:"owner"`
		Dependencies     []issueDependency `json:"dependencies"`
		Dependents       []issueDependency `json:"dependents"`
		LegacyBlockedBy  []string          `json:"blocked_by"`
		LegacyBlocks     []string          `json:"blocks"`
		LegacyChildren   []string          `json:"children"`
		LegacyDiscovered []string          `json:"discovered"`
		LegacyFrom       []string          `json:"discovered_from"`
		LegacyParentID   string            `json:"parent_id"`
	}

	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	*i = Issue(aux.issueAlias)

	if aux.V1IssueType != "" {
		i.Type = aux.V1IssueType
	} else if aux.LegacyType != "" {
		i.Type = aux.LegacyType
	}

	if i.Assignee == "" && aux.Owner != "" {
		i.Assignee = aux.Owner
	}

	i.BlockedBy = appendUnique(append([]string(nil), aux.LegacyBlockedBy...), nil...)
	i.Blocks = appendUnique(append([]string(nil), aux.LegacyBlocks...), nil...)
	i.Children = appendUnique(append([]string(nil), aux.LegacyChildren...), nil...)
	i.Discovered = appendUnique(append([]string(nil), aux.LegacyDiscovered...), nil...)
	i.DiscoveredFrom = appendUnique(append([]string(nil), aux.LegacyFrom...), nil...)
	if i.ParentID == "" {
		i.ParentID = aux.LegacyParentID
	}

	for _, dep := range aux.Dependencies {
		depType := dep.DependencyType
		if depType == "" {
			depType = dep.Type
		}
		switch depType {
		case "blocks":
			i.BlockedBy = appendUnique(i.BlockedBy, dep.ID)
		case "parent-child":
			if i.ParentID == "" {
				i.ParentID = dep.ID
			}
		case "discovered-from":
			i.DiscoveredFrom = appendUnique(i.DiscoveredFrom, dep.ID)
		}
	}

	for _, dep := range aux.Dependents {
		depType := dep.DependencyType
		if depType == "" {
			depType = dep.Type
		}
		switch depType {
		case "blocks":
			i.Blocks = appendUnique(i.Blocks, dep.ID)
		case "parent-child":
			i.Children = appendUnique(i.Children, dep.ID)
		case "discovered-from":
			i.Discovered = appendUnique(i.Discovered, dep.ID)
		}
	}

	return nil
}

func appendUnique(base []string, items ...string) []string {
	if len(items) == 0 {
		return base
	}
	seen := make(map[string]struct{}, len(base)+len(items))
	for _, item := range base {
		seen[item] = struct{}{}
	}
	for _, item := range items {
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		base = append(base, item)
	}
	return base
}

// CreateResult holds the result of a create operation.
type CreateResult struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

// UpdateIssueOptions specifies which fields to update on an issue.
// Nil pointer fields are skipped (not sent to bd CLI).
// This enables a single bd update call with only changed fields.
type UpdateIssueOptions struct {
	Title       *string
	Description *string
	Notes       *string
	Priority    *Priority
	Status      *Status
	Labels      *[]string  // nil = unchanged, &[]string{} = clear all
	Assignee    *string    // proactive; not used by current editor
	Type        *IssueType // proactive; not used by current editor
}

// IsEmpty reports whether no update fields were set.
func (o UpdateIssueOptions) IsEmpty() bool {
	return o.Title == nil &&
		o.Description == nil &&
		o.Notes == nil &&
		o.Priority == nil &&
		o.Status == nil &&
		o.Labels == nil &&
		o.Assignee == nil &&
		o.Type == nil
}
