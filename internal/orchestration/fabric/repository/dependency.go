package repository

import (
	"fmt"
	"sync"

	"github.com/zjrosen/perles/internal/orchestration/fabric/domain"
)

// MemoryDependencyRepository is an in-memory implementation of DependencyRepository.
type MemoryDependencyRepository struct {
	mu   sync.RWMutex
	deps map[string]domain.Dependency // key -> dependency

	// Indexes for efficient lookups
	byThread    map[string][]string // threadID -> list of dep keys (outgoing edges)
	byDependsOn map[string][]string // dependsOnID -> list of dep keys (incoming edges)
}

// NewMemoryDependencyRepository creates a new in-memory dependency repository.
func NewMemoryDependencyRepository() *MemoryDependencyRepository {
	return &MemoryDependencyRepository{
		deps:        make(map[string]domain.Dependency),
		byThread:    make(map[string][]string),
		byDependsOn: make(map[string][]string),
	}
}

// Add creates a dependency edge.
func (r *MemoryDependencyRepository) Add(dep domain.Dependency) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := dep.Key()
	if _, exists := r.deps[key]; exists {
		return nil // Idempotent - already exists
	}

	r.deps[key] = dep
	r.byThread[dep.ThreadID] = append(r.byThread[dep.ThreadID], key)
	r.byDependsOn[dep.DependsOnID] = append(r.byDependsOn[dep.DependsOnID], key)

	return nil
}

// Remove deletes a dependency edge.
func (r *MemoryDependencyRepository) Remove(threadID, dependsOnID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Find and remove all relations between these two threads
	var keysToRemove []string
	for key, dep := range r.deps {
		if dep.ThreadID == threadID && dep.DependsOnID == dependsOnID {
			keysToRemove = append(keysToRemove, key)
		}
	}

	for _, key := range keysToRemove {
		dep := r.deps[key]
		delete(r.deps, key)
		r.byThread[dep.ThreadID] = removeFromSlice(r.byThread[dep.ThreadID], key)
		r.byDependsOn[dep.DependsOnID] = removeFromSlice(r.byDependsOn[dep.DependsOnID], key)
	}

	return nil
}

// GetParents returns dependencies where this thread is the dependent.
func (r *MemoryDependencyRepository) GetParents(threadID string, relation *domain.RelationType) ([]domain.Dependency, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	keys := r.byThread[threadID]
	var results []domain.Dependency

	for _, key := range keys {
		dep, exists := r.deps[key]
		if !exists {
			continue
		}
		if relation != nil && dep.Relation != *relation {
			continue
		}
		results = append(results, dep)
	}

	return results, nil
}

// GetChildren returns dependencies where this thread is depended upon.
func (r *MemoryDependencyRepository) GetChildren(threadID string, relation *domain.RelationType) ([]domain.Dependency, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	keys := r.byDependsOn[threadID]
	var results []domain.Dependency

	for _, key := range keys {
		dep, exists := r.deps[key]
		if !exists {
			continue
		}
		if relation != nil && dep.Relation != *relation {
			continue
		}
		results = append(results, dep)
	}

	return results, nil
}

// GetRoots returns thread IDs with no child_of dependency.
func (r *MemoryDependencyRepository) GetRoots() ([]string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	hasChildOf := make(map[string]bool)

	for _, dep := range r.deps {
		if dep.Relation == domain.RelationChildOf {
			hasChildOf[dep.ThreadID] = true
		}
	}

	var allThreads = make(map[string]bool)
	for _, dep := range r.deps {
		allThreads[dep.ThreadID] = true
		allThreads[dep.DependsOnID] = true
	}

	var roots []string
	for threadID := range allThreads {
		if !hasChildOf[threadID] {
			roots = append(roots, threadID)
		}
	}

	return roots, nil
}

// GetChannelForMessage finds the channel a message belongs to by traversing child_of.
func (r *MemoryDependencyRepository) GetChannelForMessage(messageID string) (string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	relation := domain.RelationChildOf
	keys := r.byThread[messageID]

	for _, key := range keys {
		dep, exists := r.deps[key]
		if !exists {
			continue
		}
		if dep.Relation == relation {
			return dep.DependsOnID, nil
		}
	}

	return "", fmt.Errorf("no channel found for message: %s", messageID)
}

func removeFromSlice(slice []string, item string) []string {
	result := make([]string, 0, len(slice))
	for _, s := range slice {
		if s != item {
			result = append(result, s)
		}
	}
	return result
}

var _ DependencyRepository = (*MemoryDependencyRepository)(nil)
