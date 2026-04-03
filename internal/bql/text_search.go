package bql

import (
	"context"
	"errors"
	"fmt"
	"strings"

	appbeads "github.com/hk9890/perles/internal/beads/application"
	beads "github.com/hk9890/perles/internal/beads/domain"
	"github.com/hk9890/perles/internal/beads/infrastructure"
)

// ExecuteSimpleTextSearch performs a broad LIKE-based search for v1 text mode.
// It intentionally stays simple (no FTS index) and searches across:
// title, description, notes, labels, id, comments, design, acceptance criteria.
func ExecuteSimpleTextSearch(provider appbeads.DBProvider, input string) ([]beads.Issue, error) {
	if provider == nil {
		return nil, errors.New("database provider unavailable")
	}

	query := strings.TrimSpace(input)
	if query == "" {
		return []beads.Issue{}, nil
	}

	like := "%" + strings.ToLower(query) + "%"
	executor := &Executor{provider: provider}

	executeOnce := func() ([]beads.Issue, error) {
		selectColumns, err := executor.issueSelectColumnsSQL()
		if err != nil {
			return nil, fmt.Errorf("build issue select columns: %w", err)
		}

		sqlQuery := fmt.Sprintf(`
			SELECT DISTINCT
				%s
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
		`, strings.Join(selectColumns, ",\n\t\t\t"))

		db := provider.DB()
		if db == nil {
			return nil, errors.New("database connection unavailable")
		}

		rows, err := db.Query(sqlQuery, like, like, like, like, like, like, like, like)
		if err != nil {
			return nil, fmt.Errorf("text search query error: %w", err)
		}
		defer func() { _ = rows.Close() }()

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

		labels, err := executor.loadLabelsForIssues(ids)
		if err != nil {
			return nil, fmt.Errorf("load labels: %w", err)
		}
		commentCounts, err := executor.loadCommentCountsForIssues(ids)
		if err != nil {
			return nil, fmt.Errorf("load comment counts: %w", err)
		}
		deps, err := executor.loadDependenciesForIssues(ids)
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

	retryPolicy := infrastructure.DefaultQueryRetryPolicy()
	maxAttempts := max(1, retryPolicy.MaxAttempts)
	attempt := 0

	var issues []beads.Issue
	execErr := retryPolicy.Execute(context.Background(), func() error {
		attempt++

		loaded, queryErr := executeOnce()
		if queryErr == nil {
			issues = loaded
			return nil
		}

		reconnector, ok := provider.(appbeads.Reconnector)
		if !ok || !appbeads.IsRecoverableConnectivityError(queryErr) {
			return queryErr
		}

		if attempt < maxAttempts {
			if _, reconnectErr := reconnector.ReconnectIfRecoverable(queryErr); reconnectErr != nil {
				return reconnectErr
			}
		}

		return queryErr
	})

	return issues, execErr
}
