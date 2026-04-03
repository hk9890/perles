package shared

import (
	"database/sql"
	"strings"
	"unicode"

	beads "github.com/hk9890/perles/internal/beads/domain"
	"github.com/hk9890/perles/internal/ui/shared/picker"
	"github.com/hk9890/perles/internal/ui/styles"
)

// PriorityOptions returns picker options for priority levels.
func PriorityOptions() []picker.Option {
	return []picker.Option{
		{Label: "P0 - Critical", Value: "P0", Color: styles.PriorityCriticalColor},
		{Label: "P1 - High", Value: "P1", Color: styles.PriorityHighColor},
		{Label: "P2 - Medium", Value: "P2", Color: styles.PriorityMediumColor},
		{Label: "P3 - Low", Value: "P3", Color: styles.PriorityLowColor},
		{Label: "P4 - Backlog", Value: "P4", Color: styles.PriorityBacklogColor},
	}
}

// StatusOptions returns picker options for status values.
func StatusOptions(issue beads.Issue, db *sql.DB) []picker.Option {
	builtins := []beads.Status{
		beads.StatusOpen,
		beads.StatusInProgress,
		beads.StatusBlocked,
		beads.StatusHooked,
		beads.StatusPinned,
		beads.StatusDeferred,
		beads.StatusClosed,
	}

	options := make([]picker.Option, 0, len(builtins)+4)
	seen := make(map[string]struct{})

	for _, status := range builtins {
		name := strings.TrimSpace(string(status))
		if name == "" {
			continue
		}
		options = append(options, picker.Option{
			Label: formatValueLabel(name),
			Value: name,
			Color: styles.GetStatusColor(status),
		})
		seen[name] = struct{}{}
	}

	if issueStatus := strings.TrimSpace(string(issue.Status)); issueStatus != "" {
		if _, ok := seen[issueStatus]; !ok {
			options = append(options, picker.Option{
				Label: formatValueLabel(issueStatus),
				Value: issueStatus,
				Color: styles.GetStatusColor(beads.Status(issueStatus)),
			})
			seen[issueStatus] = struct{}{}
		}
	}

	for _, custom := range discoverCustomStatuses(db) {
		if _, ok := seen[custom]; ok {
			continue
		}
		options = append(options, picker.Option{
			Label: formatValueLabel(custom),
			Value: custom,
			Color: styles.GetStatusColor(beads.Status(custom)),
		})
		seen[custom] = struct{}{}
	}

	return options
}

// TypeOptions returns picker options for issue types.
func TypeOptions(issue beads.Issue, db *sql.DB) []picker.Option {
	builtins := []beads.IssueType{
		beads.TypeTask,
		beads.TypeBug,
		beads.TypeFeature,
		beads.TypeChore,
		beads.TypeEpic,
		beads.TypeDecision,
		beads.TypeSpike,
		beads.TypeStory,
		beads.TypeMilestone,
	}

	options := make([]picker.Option, 0, len(builtins)+4)
	seen := make(map[string]struct{})

	for _, issueType := range builtins {
		name := strings.TrimSpace(string(issueType))
		if name == "" {
			continue
		}
		options = append(options, picker.Option{
			Label: formatValueLabel(name),
			Value: name,
			Color: styles.GetTypeColor(issueType),
		})
		seen[name] = struct{}{}
	}

	if currentType := strings.TrimSpace(string(issue.Type)); currentType != "" {
		if _, ok := seen[currentType]; !ok {
			options = append(options, picker.Option{
				Label: formatValueLabel(currentType),
				Value: currentType,
				Color: styles.GetTypeColor(beads.IssueType(currentType)),
			})
			seen[currentType] = struct{}{}
		}
	}

	for _, custom := range discoverCustomTypes(db) {
		if _, ok := seen[custom]; ok {
			continue
		}
		options = append(options, picker.Option{
			Label: formatValueLabel(custom),
			Value: custom,
			Color: styles.GetTypeColor(beads.IssueType(custom)),
		})
		seen[custom] = struct{}{}
	}

	return options
}

func discoverCustomStatuses(db *sql.DB) []string {
	if db == nil {
		return nil
	}

	rows, err := db.Query(`SELECT name FROM custom_statuses ORDER BY name`) //nolint:gosec // static query
	if err != nil {
		return nil
	}
	defer func() { _ = rows.Close() }()

	var statuses []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			continue
		}
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		statuses = append(statuses, name)
	}

	return statuses
}

func discoverCustomTypes(db *sql.DB) []string {
	if db == nil {
		return nil
	}

	rows, err := db.Query(`SELECT name FROM custom_types ORDER BY name`) //nolint:gosec // static query
	if err != nil {
		return nil
	}
	defer func() { _ = rows.Close() }()

	var issueTypes []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			continue
		}
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		issueTypes = append(issueTypes, name)
	}

	return issueTypes
}

func formatValueLabel(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "Unknown"
	}

	parts := strings.FieldsFunc(trimmed, func(r rune) bool {
		switch r {
		case '_', '-', '/':
			return true
		default:
			return unicode.IsSpace(r)
		}
	})
	if len(parts) == 0 {
		return "Unknown"
	}

	for i, part := range parts {
		if part == "" {
			continue
		}
		lower := []rune(strings.ToLower(part))
		lower[0] = unicode.ToUpper(lower[0])
		parts[i] = string(lower)
	}

	return strings.Join(parts, " ")
}
