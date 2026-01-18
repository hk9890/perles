// Package controlplane provides the foundational types and state management for
// multi-workflow orchestration.
package controlplane

import "errors"

// Sentinel errors for worktree operations.
var (
	// ErrUncommittedChanges is returned when attempting to stop a workflow
	// that has a worktree with uncommitted changes. The caller should prompt
	// the user to either commit/stash changes or force the stop.
	ErrUncommittedChanges = errors.New("worktree has uncommitted changes")
)
