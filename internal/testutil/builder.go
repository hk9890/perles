package testutil

import (
	"database/sql"
	"testing"

	"github.com/stretchr/testify/require"
)

// depData holds data for a dependency to be inserted.
type depData struct {
	issueID     string
	dependsOnID string
	depType     string
}

type customStatusData struct {
	name     string
	category string
}

// Builder accumulates test data and inserts it in the correct order.
type Builder struct {
	t              *testing.T
	db             *sql.DB
	issues         []issueData
	deps           []depData
	blocked        []string
	customStatuses []customStatusData
	customTypes    []string
	metadata       map[string]string
	config         map[string]string
}

// NewBuilder creates a builder for the given test database.
func NewBuilder(t *testing.T, db *sql.DB) *Builder {
	t.Helper()
	return &Builder{
		t:        t,
		db:       db,
		metadata: make(map[string]string),
		config:   make(map[string]string),
	}
}

// WithIssue adds an issue with optional configuration.
func (b *Builder) WithIssue(id string, opts ...IssueOption) *Builder {
	issue := defaultIssue(id)
	for _, opt := range opts {
		opt(&issue)
	}
	b.issues = append(b.issues, issue)
	return b
}

// WithDependency adds a dependency relationship between issues.
func (b *Builder) WithDependency(issueID, dependsOnID, depType string) *Builder {
	b.deps = append(b.deps, depData{issueID, dependsOnID, depType})
	return b
}

// WithBlockedCache marks an issue as blocked in blocked_issues.
func (b *Builder) WithBlockedCache(issueID string) *Builder {
	b.blocked = append(b.blocked, issueID)
	return b
}

// WithCustomStatus adds a custom_statuses row used by beads v1 compatibility tests.
func (b *Builder) WithCustomStatus(name, category string) *Builder {
	b.customStatuses = append(b.customStatuses, customStatusData{name: name, category: category})
	return b
}

// WithCustomType adds a custom_types row used by beads v1 compatibility tests.
func (b *Builder) WithCustomType(name string) *Builder {
	b.customTypes = append(b.customTypes, name)
	return b
}

// WithMetadata sets a metadata key/value row.
func (b *Builder) WithMetadata(key, value string) *Builder {
	b.metadata[key] = value
	return b
}

// WithConfig sets a config key/value row.
func (b *Builder) WithConfig(key, value string) *Builder {
	b.config[key] = value
	return b
}

// Build inserts all accumulated data into the database.
func (b *Builder) Build() {
	b.t.Helper()
	for _, customStatus := range b.customStatuses {
		b.insertCustomStatus(customStatus)
	}
	for _, customType := range b.customTypes {
		b.insertCustomType(customType)
	}
	for key, value := range b.metadata {
		b.insertMetadata(key, value)
	}
	for key, value := range b.config {
		b.insertConfig(key, value)
	}

	// Insert in dependency order: issues → labels → deps → comments → blocked.
	for _, issue := range b.issues {
		b.insertIssue(issue)
		b.insertLabels(issue.id, issue.labels)
		b.insertComments(issue.id, issue.comments)
	}
	for _, dep := range b.deps {
		b.insertDependency(dep)
	}
	for _, id := range b.blocked {
		b.insertBlockedCache(id)
	}
}

func (b *Builder) insertIssue(issue issueData) {
	b.t.Helper()
	_, err := b.db.Exec(
		`INSERT INTO issues (id, title, description, status, priority, issue_type, assignee, sender, ephemeral, pinned, is_template, created_at, created_by, updated_at, defer_until, due_at, closed_at, close_reason, deleted_at, hook_bead, role_bead, agent_state, last_activity, role_type, rig, mol_type)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		issue.id, issue.title, issue.description, issue.status, issue.priority,
		issue.issueType, issue.assignee, issue.sender, issue.ephemeral, issue.pinned, issue.isTemplate, issue.createdAt, issue.createdBy, issue.updatedAt, issue.deferUntil, issue.dueAt, issue.closedAt, issue.closeReason, issue.deletedAt,
		issue.hookBead, issue.roleBead, issue.agentState, issue.lastActivity, issue.roleType, issue.rig, issue.molType,
	)
	require.NoError(b.t, err)
}

func (b *Builder) insertLabels(issueID string, labels []string) {
	b.t.Helper()
	for _, label := range labels {
		_, err := b.db.Exec(`INSERT INTO labels (issue_id, label) VALUES (?, ?)`, issueID, label)
		require.NoError(b.t, err)
	}
}

func (b *Builder) insertComments(issueID string, comments []CommentData) {
	b.t.Helper()
	for _, c := range comments {
		_, err := b.db.Exec(
			`INSERT INTO comments (issue_id, author, text) VALUES (?, ?, ?)`,
			issueID, c.Author, c.Text,
		)
		require.NoError(b.t, err)
	}
}

func (b *Builder) insertDependency(dep depData) {
	b.t.Helper()
	_, err := b.db.Exec(
		`INSERT INTO dependencies (issue_id, depends_on_id, type) VALUES (?, ?, ?)`,
		dep.issueID, dep.dependsOnID, dep.depType,
	)
	require.NoError(b.t, err)
}

func (b *Builder) insertBlockedCache(issueID string) {
	b.t.Helper()
	_, err := b.db.Exec(`INSERT INTO blocked_issues (id, blocked_by_count) VALUES (?, 1)`, issueID)
	require.NoError(b.t, err)
}

func (b *Builder) insertCustomStatus(status customStatusData) {
	b.t.Helper()
	_, err := b.db.Exec(`INSERT INTO custom_statuses (name, category) VALUES (?, ?)`, status.name, status.category)
	require.NoError(b.t, err)
}

func (b *Builder) insertCustomType(name string) {
	b.t.Helper()
	_, err := b.db.Exec(`INSERT INTO custom_types (name) VALUES (?)`, name)
	require.NoError(b.t, err)
}

func (b *Builder) insertMetadata(key, value string) {
	b.t.Helper()
	_, err := b.db.Exec(`INSERT INTO metadata (key, value) VALUES (?, ?)`, key, value)
	require.NoError(b.t, err)
}

func (b *Builder) insertConfig(key, value string) {
	b.t.Helper()
	_, err := b.db.Exec(`INSERT INTO config (key, value) VALUES (?, ?)`, key, value)
	require.NoError(b.t, err)
}
