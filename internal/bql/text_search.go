package bql

import (
	"database/sql"
	"fmt"
	"strings"

	beads "github.com/hk9890/perles/internal/beads/domain"
)

// ExecuteSimpleTextSearch performs a broad LIKE-based search for v1 text mode.
// It intentionally stays simple (no FTS index) and searches across:
// title, description, notes, labels, id, comments, design, acceptance criteria.
func ExecuteSimpleTextSearch(db *sql.DB, input string) ([]beads.Issue, error) {
	query := strings.TrimSpace(input)
	if query == "" {
		return []beads.Issue{}, nil
	}

	like := "%" + strings.ToLower(query) + "%"

	sqlQuery := `
		SELECT DISTINCT
			i.id,
			i.title,
			i.description,
			i.design,
			i.acceptance_criteria,
			i.notes,
			i.status,
			i.priority,
			i.issue_type,
			i.assignee,
			i.sender,
			i.ephemeral,
			i.pinned,
			i.is_template,
			i.created_at,
			i.created_by,
			i.updated_at,
			i.closed_at,
			i.close_reason,
			i.hook_bead,
			i.role_bead,
			i.agent_state,
			i.last_activity,
			i.role_type,
			i.rig,
			i.mol_type
		FROM issues i
		WHERE i.status NOT IN ('deleted', 'tombstone')
		  AND (
			LOWER(i.id) LIKE ?
			OR LOWER(i.title) LIKE ?
			OR LOWER(COALESCE(i.description, '')) LIKE ?
			OR LOWER(COALESCE(i.notes, '')) LIKE ?
			OR LOWER(COALESCE(i.design, '')) LIKE ?
			OR LOWER(COALESCE(i.acceptance_criteria, '')) LIKE ?
			OR EXISTS (
				SELECT 1
				FROM labels l
				WHERE l.issue_id = i.id
				  AND LOWER(l.label) LIKE ?
			)
			OR EXISTS (
				SELECT 1
				FROM comments c
				WHERE c.issue_id = i.id
				  AND LOWER(COALESCE(c.text, '')) LIKE ?
			)
		  )
		ORDER BY i.updated_at DESC
	`

	rows, err := db.Query(sqlQuery, like, like, like, like, like, like, like, like)
	if err != nil {
		return nil, fmt.Errorf("text search query error: %w", err)
	}
	defer func() { _ = rows.Close() }()

	executor := &Executor{}
	issues, err := executor.scanIssuesBase(rows)
	if err != nil {
		return nil, err
	}
	if len(issues) == 0 {
		return issues, nil
	}

	ids := make([]string, len(issues))
	for i, issue := range issues {
		ids[i] = issue.ID
	}

	labels, err := executor.loadLabelsForIssuesWithDB(db, ids)
	if err != nil {
		return nil, fmt.Errorf("load labels: %w", err)
	}
	commentCounts, err := executor.loadCommentCountsForIssuesWithDB(db, ids)
	if err != nil {
		return nil, fmt.Errorf("load comment counts: %w", err)
	}
	deps, err := executor.loadDependenciesForIssuesWithDB(db, ids)
	if err != nil {
		return nil, fmt.Errorf("load dependencies: %w", err)
	}

	for i := range issues {
		id := issues[i].ID
		if l, ok := labels[id]; ok {
			issues[i].Labels = l
		}
		if c, ok := commentCounts[id]; ok {
			issues[i].CommentCount = c
		}
		if d, ok := deps[id]; ok {
			issues[i].ParentID = d.ParentID
			issues[i].BlockedBy = d.BlockedBy
			issues[i].Blocks = d.Blocks
			issues[i].Children = d.Children
			issues[i].DiscoveredFrom = d.DiscoveredFrom
			issues[i].Discovered = d.Discovered
		}
	}

	return issues, nil
}

func (e *Executor) loadDependenciesForIssuesWithDB(db *sql.DB, ids []string) (map[string]IssueDeps, error) {
	if e != nil && e.db != nil {
		return e.loadDependenciesForIssues(ids)
	}
	tmp := &Executor{db: db}
	return tmp.loadDependenciesForIssues(ids)
}

func (e *Executor) loadLabelsForIssuesWithDB(db *sql.DB, ids []string) (map[string][]string, error) {
	if e != nil && e.db != nil {
		return e.loadLabelsForIssues(ids)
	}
	tmp := &Executor{db: db}
	return tmp.loadLabelsForIssues(ids)
}

func (e *Executor) loadCommentCountsForIssuesWithDB(db *sql.DB, ids []string) (map[string]int, error) {
	if e != nil && e.db != nil {
		return e.loadCommentCountsForIssues(ids)
	}
	tmp := &Executor{db: db}
	return tmp.loadCommentCountsForIssues(ids)
}
