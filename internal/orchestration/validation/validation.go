// Package validation provides shared validation functions for the orchestration layer.
package validation

import "regexp"

// taskIDPattern validates bd task IDs to prevent command injection.
// Valid formats: "prefix-xxxx" or "prefix-xxxx.N" (for subtasks)
// Prefix may be a single character for compatibility with beads configs
// that use short project names (for example "p-abc1").
// Suffix segments remain 2+ characters to avoid widening all segment rules.
// Examples: "perles-abc1", "perles-abc1.2", "ms-e52", "pe-perles-xyz9.10", "p-abc1"
var taskIDPattern = regexp.MustCompile(`^[a-z0-9]{1,}(-[a-z0-9]{2,})+(\.[0-9]+)*$`)

// IsValidTaskID validates that a task ID matches the expected format.
// Valid formats: "prefix-xxxx" or "prefix-xxxx.N" (for subtasks)
func IsValidTaskID(taskID string) bool {
	return taskIDPattern.MatchString(taskID)
}
