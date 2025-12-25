package mcp

import (
	"github.com/zjrosen/perles/internal/orchestration/message"
	"github.com/zjrosen/perles/internal/orchestration/pool"
)

// GetPool returns the worker pool for test access.
func (cs *CoordinatorServer) GetPool() *pool.WorkerPool {
	return cs.pool
}

// GetMessageIssue returns the message issue for test access.
func (cs *CoordinatorServer) GetMessageIssue() *message.Issue {
	return cs.msgIssue
}

// IsValidTaskID exposes the validation function for testing.
func IsValidTaskID(taskID string) bool {
	return isValidTaskID(taskID)
}
