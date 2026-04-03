// Package testutil provides test utilities for database setup.
package testutil

import (
	"database/sql"
	"testing"

	_ "github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/embed"
	"github.com/stretchr/testify/require"
)

// Schema contains the legacy internal test schema used by existing tests.
const Schema = `
CREATE TABLE issues (
	id TEXT PRIMARY KEY,
	title TEXT NOT NULL,
	description TEXT,
	design TEXT NOT NULL DEFAULT '',
	acceptance_criteria TEXT NOT NULL DEFAULT '',
	notes TEXT NOT NULL DEFAULT '',
	status TEXT NOT NULL DEFAULT 'open',
	priority INTEGER NOT NULL DEFAULT 2,
	issue_type TEXT NOT NULL DEFAULT 'task',
	assignee TEXT,
	sender TEXT,
	ephemeral INTEGER,
	pinned INTEGER,
	is_template INTEGER,
	defer_until DATETIME,
	due_at DATETIME,
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	created_by TEXT DEFAULT '',
	updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	closed_at DATETIME,
	close_reason TEXT DEFAULT '',
	deleted_at DATETIME,
	hook_bead TEXT DEFAULT '',
	role_bead TEXT DEFAULT '',
	agent_state TEXT DEFAULT '',
	last_activity DATETIME,
	role_type TEXT DEFAULT '',
	rig TEXT DEFAULT '',
	mol_type TEXT DEFAULT '',
	CHECK ((status = 'closed') = (closed_at IS NOT NULL) OR status IN ('deleted', 'tombstone'))
);

CREATE TABLE labels (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	issue_id TEXT NOT NULL,
	label TEXT NOT NULL,
	FOREIGN KEY (issue_id) REFERENCES issues(id)
);

CREATE TABLE dependencies (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	issue_id TEXT NOT NULL,
	depends_on_id TEXT NOT NULL,
	type TEXT NOT NULL DEFAULT 'blocks',
	FOREIGN KEY (issue_id) REFERENCES issues(id),
	FOREIGN KEY (depends_on_id) REFERENCES issues(id)
);

CREATE TABLE comments (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	issue_id TEXT NOT NULL,
	author TEXT NOT NULL,
	text TEXT NOT NULL,
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY (issue_id) REFERENCES issues(id)
);

CREATE TABLE blocked_issues (
	id TEXT PRIMARY KEY,
	blocked_by_count INTEGER NOT NULL DEFAULT 1
);

CREATE VIEW ready_issues AS
SELECT i.id
FROM issues i
WHERE i.status IN ('open', 'in_progress')
  AND i.id NOT IN (SELECT id FROM blocked_issues);
`

// BeadsV1Schema models the beads v1.0-compatible schema contract used by
// compatibility-focused tests.
const BeadsV1Schema = `
CREATE TABLE issues (
	id TEXT PRIMARY KEY,
	title TEXT NOT NULL,
	description TEXT,
	design TEXT NOT NULL DEFAULT '',
	acceptance_criteria TEXT NOT NULL DEFAULT '',
	notes TEXT NOT NULL DEFAULT '',
	status TEXT NOT NULL DEFAULT 'open',
	priority INTEGER NOT NULL DEFAULT 2,
	issue_type TEXT NOT NULL DEFAULT 'task',
	assignee TEXT,
	sender TEXT,
	ephemeral INTEGER,
	pinned INTEGER,
	is_template INTEGER,
	defer_until DATETIME,
	due_at DATETIME,
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	created_by TEXT DEFAULT '',
	updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	closed_at DATETIME,
	close_reason TEXT DEFAULT '',
	deleted_at DATETIME,
	hook_bead TEXT DEFAULT '',
	role_bead TEXT DEFAULT '',
	agent_state TEXT DEFAULT '',
	last_activity DATETIME,
	role_type TEXT DEFAULT '',
	rig TEXT DEFAULT '',
	mol_type TEXT DEFAULT ''
);

CREATE TABLE labels (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	issue_id TEXT NOT NULL,
	label TEXT NOT NULL,
	FOREIGN KEY (issue_id) REFERENCES issues(id)
);

CREATE TABLE dependencies (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	issue_id TEXT NOT NULL,
	depends_on_id TEXT NOT NULL,
	type TEXT NOT NULL DEFAULT 'blocks',
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	created_by TEXT DEFAULT '',
	FOREIGN KEY (issue_id) REFERENCES issues(id),
	FOREIGN KEY (depends_on_id) REFERENCES issues(id)
);

CREATE TABLE comments (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	issue_id TEXT NOT NULL,
	author TEXT NOT NULL,
	text TEXT NOT NULL,
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY (issue_id) REFERENCES issues(id)
);

CREATE TABLE config (
	key TEXT PRIMARY KEY,
	value TEXT NOT NULL
);

CREATE TABLE metadata (
	key TEXT PRIMARY KEY,
	value TEXT NOT NULL
);

CREATE TABLE custom_statuses (
	name TEXT PRIMARY KEY,
	category TEXT NOT NULL
);

CREATE TABLE custom_types (
	name TEXT PRIMARY KEY
);

CREATE TABLE blocked_issues (
	id TEXT PRIMARY KEY,
	blocked_by_count INTEGER NOT NULL DEFAULT 1
);

CREATE VIEW ready_issues AS
SELECT i.id
FROM issues i
WHERE (i.status = 'open'
	OR EXISTS (
		SELECT 1 FROM custom_statuses cs
		WHERE cs.name = i.status AND cs.category = 'active'
	)
)
AND COALESCE(i.ephemeral, 0) = 0
AND (i.defer_until IS NULL OR i.defer_until <= CURRENT_TIMESTAMP)
AND i.status <> 'deferred'
AND NOT EXISTS (
	SELECT 1
	FROM dependencies d
	JOIN issues blocker ON blocker.id = d.depends_on_id
	WHERE d.issue_id = i.id
	  AND d.type = 'blocks'
	  AND blocker.status NOT IN ('closed', 'pinned')
);
`

// NewTestDB creates an in-memory SQLite database with the full test schema.
// The caller is responsible for closing the database.
func NewTestDB(t *testing.T) *sql.DB {
	t.Helper()
	return newTestDBWithSchema(t, Schema)
}

// NewBeadsV1TestDB creates an in-memory SQLite database with a beads v1.0
// compatibility schema.
func NewBeadsV1TestDB(t *testing.T) *sql.DB {
	t.Helper()
	return newTestDBWithSchema(t, BeadsV1Schema)
}

func newTestDBWithSchema(t *testing.T, schema string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	_, err = db.Exec(schema)
	require.NoError(t, err)
	return db
}
