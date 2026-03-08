package editor

import (
	"fmt"
	"regexp"
	"strings"

	beads "github.com/hk9890/perles/internal/beads/domain"
)

// IssueEdit contains editable issue fields supported by the issue-level external editor flow.
type IssueEdit struct {
	Title       string
	Description string
	Notes       string
	Labels      []string
	Status      beads.Status
	Priority    beads.Priority
}

var issueSectionRegex = regexp.MustCompile(`(?m)^## (Title|Description|Notes|Labels|Status|Priority)\s*$`)

// MarshalIssueMarkdown serializes the issue into a structured markdown format for editing.
func MarshalIssueMarkdown(issue beads.Issue) string {
	labels := make([]string, 0, len(issue.Labels))
	for _, label := range issue.Labels {
		if strings.TrimSpace(label) == "" {
			continue
		}
		labels = append(labels, strings.TrimSpace(label))
	}
	labelsBlock := ""
	for _, label := range labels {
		labelsBlock += "- " + label + "\n"
	}
	labelsBlock = strings.TrimRight(labelsBlock, "\n")

	return fmt.Sprintf(`# Perles Issue External Edit

Edit the sections below and save. Keep section headings unchanged.

## Title
%s

## Description
%s

## Notes
%s

## Labels
%s

## Status
%s

## Priority
P%d
`, strings.TrimSpace(issue.TitleText), issue.DescriptionText, issue.Notes, labelsBlock, issue.Status, issue.Priority)
}

// ParseIssueMarkdown parses structured markdown content produced by MarshalIssueMarkdown.
func ParseIssueMarkdown(content string) (IssueEdit, error) {
	sections := parseIssueSections(content)
	required := []string{"Title", "Description", "Notes", "Labels", "Status", "Priority"}
	for _, key := range required {
		if _, ok := sections[key]; !ok {
			return IssueEdit{}, fmt.Errorf("missing section: %s", key)
		}
	}

	status, err := parseStatus(sections["Status"])
	if err != nil {
		return IssueEdit{}, err
	}

	priority, err := parsePriority(sections["Priority"])
	if err != nil {
		return IssueEdit{}, err
	}

	return IssueEdit{
		Title:       strings.TrimSpace(sections["Title"]),
		Description: sections["Description"],
		Notes:       sections["Notes"],
		Labels:      parseLabels(sections["Labels"]),
		Status:      status,
		Priority:    priority,
	}, nil
}

func parseIssueSections(content string) map[string]string {
	matches := issueSectionRegex.FindAllStringSubmatchIndex(content, -1)
	sections := make(map[string]string, len(matches))

	for i, match := range matches {
		name := content[match[2]:match[3]]
		start := match[1]
		end := len(content)
		if i+1 < len(matches) {
			end = matches[i+1][0]
		}
		body := content[start:end]
		body = strings.TrimPrefix(body, "\n")
		if i+1 < len(matches) {
			// Interior sections are separated by exactly two newlines before the next heading.
			// Remove only that separator so we don't introduce a trailing newline on round-trip.
			body = strings.TrimSuffix(body, "\n\n")
		} else {
			// Last section may end with a single trailing newline from serialization.
			body = strings.TrimSuffix(body, "\n")
		}
		sections[name] = body
	}

	return sections
}

func parseLabels(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return []string{}
	}

	seen := make(map[string]struct{})
	labels := make([]string, 0)
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "- ") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "- "))
		}
		for _, piece := range strings.Split(line, ",") {
			label := strings.TrimSpace(piece)
			if label == "" {
				continue
			}
			if _, exists := seen[label]; exists {
				continue
			}
			seen[label] = struct{}{}
			labels = append(labels, label)
		}
	}
	return labels
}

func parseStatus(raw string) (beads.Status, error) {
	status := beads.Status(strings.ToLower(strings.TrimSpace(raw)))
	switch status {
	case beads.StatusOpen, beads.StatusInProgress, beads.StatusClosed, beads.StatusDeferred, beads.StatusBlocked:
		return status, nil
	default:
		return "", fmt.Errorf("invalid status %q (expected one of: open, in_progress, closed, deferred, blocked)", strings.TrimSpace(raw))
	}
}

func parsePriority(raw string) (beads.Priority, error) {
	value := strings.ToLower(strings.TrimSpace(raw))
	if strings.HasPrefix(value, "p") {
		value = strings.TrimPrefix(value, "p")
	}

	switch value {
	case "0":
		return beads.PriorityCritical, nil
	case "1":
		return beads.PriorityHigh, nil
	case "2":
		return beads.PriorityMedium, nil
	case "3":
		return beads.PriorityLow, nil
	case "4":
		return beads.PriorityBacklog, nil
	default:
		return beads.PriorityMedium, fmt.Errorf("invalid priority %q (expected P0..P4)", strings.TrimSpace(raw))
	}
}
