package testutil

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewTestDB_CreatesSchema(t *testing.T) {
	db := NewTestDB(t)
	defer func() { _ = db.Close() }()

	// Verify all tables exist by querying sqlite_master
	var count int
	err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name IN ('issues', 'labels', 'dependencies', 'comments', 'blocked_issues')`).Scan(&count)
	require.NoError(t, err)
	require.Equal(t, 5, count, "expected 5 tables")

	// Verify ready_issues view exists
	err = db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='view' AND name='ready_issues'`).Scan(&count)
	require.NoError(t, err)
	require.Equal(t, 1, count, "expected ready_issues view")
}

func TestNewTestDB_TablesExist(t *testing.T) {
	db := NewTestDB(t)
	defer func() { _ = db.Close() }()

	// Test each table is queryable via COUNT
	tables := []string{"issues", "labels", "dependencies", "comments", "blocked_issues"}
	for _, table := range tables {
		var count int
		err := db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&count)
		require.NoError(t, err, "table %s should be queryable", table)
	}

	// Test view is queryable
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM ready_issues").Scan(&count)
	require.NoError(t, err, "view ready_issues should be queryable")
}

func TestNewTestDB_IssueColumns(t *testing.T) {
	db := NewTestDB(t)
	defer func() { _ = db.Close() }()

	// Insert a test issue with all columns
	_, err := db.Exec(`INSERT INTO issues
		(id, title, description, design, acceptance_criteria, notes, status, priority, issue_type, assignee, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, datetime('now'), datetime('now'))`,
		"test-id", "Test Title", "Test Desc", "Test Design", "Test AC", "Test Notes", "open", 1, "task", "alice")
	require.NoError(t, err)

	// Verify all columns exist and are readable
	var id, title, desc, design, ac, notes, status, issueType string
	var priority int
	var assignee *string
	err = db.QueryRow(`SELECT id, title, description, design, acceptance_criteria, notes, status, priority, issue_type, assignee FROM issues WHERE id = ?`, "test-id").
		Scan(&id, &title, &desc, &design, &ac, &notes, &status, &priority, &issueType, &assignee)
	require.NoError(t, err)
	require.Equal(t, "test-id", id)
	require.Equal(t, "Test Title", title)
	require.Equal(t, "Test Desc", desc)
	require.Equal(t, "Test Design", design)
	require.Equal(t, "Test AC", ac)
	require.Equal(t, "Test Notes", notes)
	require.Equal(t, "open", status)
	require.Equal(t, 1, priority)
	require.Equal(t, "task", issueType)
	require.NotNil(t, assignee)
	require.Equal(t, "alice", *assignee)
}

func TestNewBeadsV1TestDB_CreatesV1SchemaTables(t *testing.T) {
	db := NewBeadsV1TestDB(t)
	defer func() { _ = db.Close() }()

	var count int
	err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name IN ('issues', 'labels', 'dependencies', 'comments', 'config', 'metadata', 'custom_statuses', 'custom_types', 'blocked_issues')`).Scan(&count)
	require.NoError(t, err)
	require.Equal(t, 9, count, "expected beads v1-compatible tables")

	err = db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='view' AND name='ready_issues'`).Scan(&count)
	require.NoError(t, err)
	require.Equal(t, 1, count)
}

func TestNewBeadsV1TestDB_IssueSchedulingColumns(t *testing.T) {
	db := NewBeadsV1TestDB(t)
	defer func() { _ = db.Close() }()

	_, err := db.Exec(`INSERT INTO issues (id, title, status, defer_until, due_at, created_at, updated_at) VALUES (?, ?, ?, datetime('now', '+1 day'), datetime('now', '+2 day'), datetime('now'), datetime('now'))`,
		"v1-sched", "Scheduling", "open")
	require.NoError(t, err)

	var deferUntil, dueAt string
	err = db.QueryRow(`SELECT defer_until, due_at FROM issues WHERE id = ?`, "v1-sched").Scan(&deferUntil, &dueAt)
	require.NoError(t, err)
	require.NotEmpty(t, deferUntil)
	require.NotEmpty(t, dueAt)
}
