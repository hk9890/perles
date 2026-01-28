package repository

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/zjrosen/perles/internal/orchestration/fabric/domain"
)

// MemoryThreadRepository is an in-memory implementation of ThreadRepository.
type MemoryThreadRepository struct {
	mu         sync.RWMutex
	threads    map[string]*domain.Thread // id -> thread
	slugs      map[string]string         // slug -> id (for channels only)
	seqCounter atomic.Int64
}

// NewMemoryThreadRepository creates a new in-memory thread repository.
func NewMemoryThreadRepository() *MemoryThreadRepository {
	return &MemoryThreadRepository{
		threads: make(map[string]*domain.Thread),
		slugs:   make(map[string]string),
	}
}

// Create adds a new thread to the graph.
func (r *MemoryThreadRepository) Create(thread domain.Thread) (*domain.Thread, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if thread.ID == "" {
		thread.ID = uuid.New().String()
	}

	if _, exists := r.threads[thread.ID]; exists {
		return nil, fmt.Errorf("thread already exists: %s", thread.ID)
	}

	if thread.CreatedAt.IsZero() {
		thread.CreatedAt = time.Now()
	}

	thread.Seq = r.seqCounter.Add(1)

	if thread.Type == domain.ThreadChannel && thread.Slug != "" {
		if existingID, exists := r.slugs[thread.Slug]; exists {
			return nil, fmt.Errorf("channel slug already exists: %s (id: %s)", thread.Slug, existingID)
		}
		r.slugs[thread.Slug] = thread.ID
	}

	r.threads[thread.ID] = &thread
	return &thread, nil
}

// Get retrieves a thread by ID.
func (r *MemoryThreadRepository) Get(id string) (*domain.Thread, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	thread, exists := r.threads[id]
	if !exists {
		return nil, fmt.Errorf("thread not found: %s", id)
	}

	copy := *thread
	return &copy, nil
}

// GetBySlug finds a channel thread by its slug.
func (r *MemoryThreadRepository) GetBySlug(slug string) (*domain.Thread, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	id, exists := r.slugs[slug]
	if !exists {
		return nil, fmt.Errorf("channel not found: %s", slug)
	}

	thread := r.threads[id]
	copy := *thread
	return &copy, nil
}

// List returns threads matching the filter criteria.
func (r *MemoryThreadRepository) List(opts ListOptions) ([]domain.Thread, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var results []domain.Thread

	for _, thread := range r.threads {
		if opts.Type != nil && thread.Type != *opts.Type {
			continue
		}
		if opts.AfterSeq > 0 && thread.Seq <= opts.AfterSeq {
			continue
		}
		if opts.CreatedBy != nil && thread.CreatedBy != *opts.CreatedBy {
			continue
		}
		if opts.HasMention != nil && !thread.HasMention(*opts.HasMention) {
			continue
		}

		results = append(results, *thread)
	}

	// Sort by Seq
	for i := 0; i < len(results); i++ {
		for j := i + 1; j < len(results); j++ {
			if results[i].Seq > results[j].Seq {
				results[i], results[j] = results[j], results[i]
			}
		}
	}

	if opts.Limit > 0 && len(results) > opts.Limit {
		results = results[:opts.Limit]
	}

	return results, nil
}

// Update modifies an existing thread.
func (r *MemoryThreadRepository) Update(thread domain.Thread) (*domain.Thread, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	existing, exists := r.threads[thread.ID]
	if !exists {
		return nil, fmt.Errorf("thread not found: %s", thread.ID)
	}

	thread.Seq = existing.Seq
	thread.CreatedAt = existing.CreatedAt

	if existing.Type == domain.ThreadChannel && existing.Slug != "" {
		if thread.Slug != existing.Slug {
			delete(r.slugs, existing.Slug)
			if thread.Slug != "" {
				r.slugs[thread.Slug] = thread.ID
			}
		}
	}

	r.threads[thread.ID] = &thread
	return &thread, nil
}

// Archive soft-deletes a thread.
func (r *MemoryThreadRepository) Archive(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	thread, exists := r.threads[id]
	if !exists {
		return fmt.Errorf("thread not found: %s", id)
	}

	now := time.Now()
	thread.ArchivedAt = &now
	return nil
}

var _ ThreadRepository = (*MemoryThreadRepository)(nil)
