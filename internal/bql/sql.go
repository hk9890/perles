package bql

import (
	"fmt"
	"strings"
)

const unresolvedBlockerExpr = `(blocker.status IN ('open', 'in_progress', 'blocked', 'hooked')
					  OR EXISTS (
						SELECT 1
						FROM custom_statuses blocker_status
						WHERE blocker_status.name = blocker.status
						  AND blocker_status.category IN ('active', 'wip')
					  ))`

// SQLBuilder converts a BQL AST to SQL.
type SQLBuilder struct {
	query  *Query
	params []any
}

// NewSQLBuilder creates a builder for the query.
func NewSQLBuilder(query *Query) *SQLBuilder {
	return &SQLBuilder{query: query}
}

// Build generates the SQL WHERE clause and ORDER BY.
func (b *SQLBuilder) Build() (whereClause string, orderBy string, params []any) {
	if b.query.Filter != nil {
		whereClause = b.buildExpr(b.query.Filter)
	}

	if len(b.query.OrderBy) > 0 {
		orderBy = b.buildOrderBy()
	}

	return whereClause, orderBy, b.params
}

// buildExpr recursively builds SQL for an expression.
func (b *SQLBuilder) buildExpr(expr Expr) string {
	switch e := expr.(type) {
	case *BinaryExpr:
		left := b.buildExpr(e.Left)
		right := b.buildExpr(e.Right)
		op := "AND"
		if e.Op == TokenOr {
			op = "OR"
		}
		return fmt.Sprintf("(%s %s %s)", left, op, right)

	case *NotExpr:
		return fmt.Sprintf("NOT (%s)", b.buildExpr(e.Expr))

	case *CompareExpr:
		return b.buildCompare(e)

	case *InExpr:
		return b.buildIn(e)
	}

	return ""
}

// buildCompare builds SQL for a comparison expression.
func (b *SQLBuilder) buildCompare(e *CompareExpr) string {
	// Handle special fields
	switch e.Field {
	case "blocked":
		// blocked = true means issue has an unresolved blocker dependency.
		// Unresolved blockers are active/wip work (built-ins + categorized custom
		// statuses). Done/frozen blockers are treated as resolved.
		// Use direct dependency predicates instead of blocked_issues view to avoid
		// Dolt field-index errors when selecting through wide i.* views.
		blockedExpr := `EXISTS (
			SELECT 1
			FROM dependencies d
			WHERE d.issue_id = i.id
			  AND d.type = 'blocks'
			  AND EXISTS (
				SELECT 1
				FROM issues blocker
				WHERE blocker.id = d.depends_on_id
				  AND ` + unresolvedBlockerExpr + `
			  )
		)`
		if e.Value.Bool {
			return blockedExpr
		}
		return "NOT " + blockedExpr

	case "ready":
		// ready = true tracks beads v1 ready semantics:
		//   - status is open OR a custom status in category=active
		//   - not ephemeral
		//   - not deferred by defer_until
		//   - not explicitly deferred status
		//   - parent is not deferred/frozen
		//   - no unresolved blockers (active/wip blockers)
		// Keep this independent of ready_issues view to avoid Dolt view-scan bugs.
		readyExpr := `((i.status = 'open'
			OR EXISTS (
				SELECT 1
				FROM custom_statuses issue_status
				WHERE issue_status.name = i.status
				  AND issue_status.category = 'active'
			))
			AND COALESCE(i.ephemeral, 0) = 0
			AND (i.defer_until IS NULL OR i.defer_until <= CURRENT_TIMESTAMP)
			AND i.status != 'deferred'
			AND NOT EXISTS (
				SELECT 1
				FROM dependencies pd
				JOIN issues parent ON parent.id = pd.depends_on_id
				WHERE pd.issue_id = i.id
				  AND pd.type = 'parent-child'
				  AND (
					parent.status IN ('deferred', 'pinned')
					OR EXISTS (
						SELECT 1
						FROM custom_statuses parent_status
						WHERE parent_status.name = parent.status
						  AND parent_status.category = 'frozen'
					)
					OR (parent.defer_until IS NOT NULL AND parent.defer_until > CURRENT_TIMESTAMP)
				  )
			)
			AND NOT EXISTS (
				SELECT 1
				FROM dependencies d
				WHERE d.issue_id = i.id
				  AND d.type = 'blocks'
				  AND EXISTS (
					SELECT 1
					FROM issues blocker
					WHERE blocker.id = d.depends_on_id
					  AND ` + unresolvedBlockerExpr + `
				  )
			)
		)`
		if e.Value.Bool {
			return readyExpr
		}
		return "NOT " + readyExpr

	case "pinned", "is_template":
		// Nullable boolean columns (INTEGER in SQLite)
		// field = true -> column = 1
		// field = false -> column = 0 (won't match NULL)
		column := b.fieldToColumn(e.Field)
		if e.Value.Bool {
			return fmt.Sprintf("%s = 1", column)
		}
		return fmt.Sprintf("%s = 0", column)

	case "label":
		// Label check via labels table
		// Supports exact match (=, !=) and partial match (~, !~)
		switch e.Op {
		case TokenContains:
			b.params = append(b.params, "%"+e.Value.String+"%")
			return "i.id IN (SELECT issue_id FROM labels WHERE label LIKE ?)"
		case TokenNotContains:
			b.params = append(b.params, "%"+e.Value.String+"%")
			return "i.id NOT IN (SELECT issue_id FROM labels WHERE label LIKE ?)"
		case TokenNeq:
			b.params = append(b.params, e.Value.String)
			return "i.id NOT IN (SELECT issue_id FROM labels WHERE label = ?)"
		default: // TokenEq
			b.params = append(b.params, e.Value.String)
			return "i.id IN (SELECT issue_id FROM labels WHERE label = ?)"
		}
	}

	if e.Field == "status_category" {
		categoryExpr := b.statusCategoryExpr()
		b.params = append(b.params, e.Value.String)
		return fmt.Sprintf("%s %s ?", categoryExpr, b.opToSQL(e.Op))
	}

	// Map BQL fields to SQL columns
	column := b.fieldToColumn(e.Field)

	// Handle priority comparisons
	if e.Field == "priority" {
		b.params = append(b.params, e.Value.Int)
		return fmt.Sprintf("%s %s ?", column, b.opToSQL(e.Op))
	}

	// Handle date comparisons using Dolt/MySQL-compatible functions.
	if e.Value.Type == ValueDate {
		dateSQL := b.dateToSQL(e.Value.String)
		return fmt.Sprintf("%s %s %s", column, b.opToSQL(e.Op), dateSQL)
	}

	// Handle nullable string fields (use COALESCE so NULL matches empty string)
	if e.Field == "assignee" {
		switch e.Op {
		case TokenContains:
			b.params = append(b.params, "%"+e.Value.String+"%")
			return fmt.Sprintf("COALESCE(%s, '') LIKE ?", column)
		case TokenNotContains:
			b.params = append(b.params, "%"+e.Value.String+"%")
			return fmt.Sprintf("COALESCE(%s, '') NOT LIKE ?", column)
		default:
			b.params = append(b.params, e.Value.String)
			return fmt.Sprintf("COALESCE(%s, '') %s ?", column, b.opToSQL(e.Op))
		}
	}

	// Handle contains/not contains operators
	switch e.Op {
	case TokenContains:
		b.params = append(b.params, "%"+e.Value.String+"%")
		return fmt.Sprintf("%s LIKE ?", column)
	case TokenNotContains:
		b.params = append(b.params, "%"+e.Value.String+"%")
		return fmt.Sprintf("%s NOT LIKE ?", column)
	}

	// Standard comparison
	b.params = append(b.params, e.Value.String)
	return fmt.Sprintf("%s %s ?", column, b.opToSQL(e.Op))
}

// buildIn builds SQL for an IN expression.
func (b *SQLBuilder) buildIn(e *InExpr) string {
	// Handle label field specially
	if e.Field == "label" {
		placeholders := make([]string, len(e.Values))
		for i, v := range e.Values {
			placeholders[i] = "?"
			b.params = append(b.params, v.String)
		}
		subquery := fmt.Sprintf("i.id IN (SELECT issue_id FROM labels WHERE label IN (%s))",
			strings.Join(placeholders, ", "))
		if e.Not {
			return "NOT " + subquery
		}
		return subquery
	}

	if e.Field == "status_category" {
		placeholders := make([]string, len(e.Values))
		for i, v := range e.Values {
			placeholders[i] = "?"
			b.params = append(b.params, v.String)
		}
		op := "IN"
		if e.Not {
			op = "NOT IN"
		}
		return fmt.Sprintf("%s %s (%s)", b.statusCategoryExpr(), op, strings.Join(placeholders, ", "))
	}

	column := b.fieldToColumn(e.Field)
	placeholders := make([]string, len(e.Values))

	for i, v := range e.Values {
		placeholders[i] = "?"
		if e.Field == "priority" {
			b.params = append(b.params, v.Int)
		} else {
			b.params = append(b.params, v.String)
		}
	}

	op := "IN"
	if e.Not {
		op = "NOT IN"
	}

	return fmt.Sprintf("%s %s (%s)", column, op, strings.Join(placeholders, ", "))
}

func (b *SQLBuilder) statusCategoryExpr() string {
	return `COALESCE((CASE
		WHEN i.status = 'open' THEN 'active'
		WHEN i.status = 'in_progress' THEN 'wip'
		WHEN i.status = 'blocked' THEN 'wip'
		WHEN i.status = 'hooked' THEN 'wip'
		WHEN i.status = 'deferred' THEN 'frozen'
		WHEN i.status = 'pinned' THEN 'frozen'
		WHEN i.status = 'closed' THEN 'done'
		ELSE NULL
	END), (
		SELECT cs.category
		FROM custom_statuses cs
		WHERE cs.name = i.status
		LIMIT 1
	), '')`
}

// fieldToColumn maps BQL field names to SQL column names.
func (b *SQLBuilder) fieldToColumn(field string) string {
	// Only map fields where BQL name differs from column name
	mapping := map[string]string{
		"type":          "i.issue_type",
		"created":       "i.created_at",
		"updated":       "i.updated_at",
		"last_activity": "i.last_activity",
	}
	if col, ok := mapping[field]; ok {
		return col
	}
	return "i." + field
}

// opToSQL converts a token operator to SQL.
func (b *SQLBuilder) opToSQL(op TokenType) string {
	switch op {
	case TokenEq:
		return "="
	case TokenNeq:
		return "!="
	case TokenLt:
		return "<"
	case TokenGt:
		return ">"
	case TokenLte:
		return "<="
	case TokenGte:
		return ">="
	default:
		return "="
	}
}

// dateToSQL converts a date value to SQL expression.
func (b *SQLBuilder) dateToSQL(dateStr string) string {
	switch dateStr {
	case "today":
		return "CURDATE()"
	case "yesterday":
		return "DATE_SUB(CURDATE(), INTERVAL 1 DAY)"
	default:
		// Handle relative time formats: -Nd (days), -Nh (hours), -Nm (months)
		if len(dateStr) > 1 && dateStr[0] == '-' {
			suffix := dateStr[len(dateStr)-1]
			value := dateStr[1 : len(dateStr)-1] // strip - and suffix

			switch suffix {
			case 'd', 'D':
				return fmt.Sprintf("DATE_SUB(CURDATE(), INTERVAL %s DAY)", value)
			case 'h', 'H':
				return fmt.Sprintf("DATE_SUB(NOW(), INTERVAL %s HOUR)", value)
			case 'm', 'M':
				return fmt.Sprintf("DATE_SUB(CURDATE(), INTERVAL %s MONTH)", value)
			}
		}
		// Assume ISO date, pass through as string
		b.params = append(b.params, dateStr)
		return "?"
	}
}

// buildOrderBy builds the ORDER BY clause.
func (b *SQLBuilder) buildOrderBy() string {
	var parts []string
	for _, term := range b.query.OrderBy {
		col := b.fieldToColumn(term.Field)
		dir := "ASC"
		if term.Desc {
			dir = "DESC"
		}
		parts = append(parts, fmt.Sprintf("%s %s", col, dir))
	}
	return strings.Join(parts, ", ")
}
