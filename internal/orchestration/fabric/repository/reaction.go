package repository

import (
	"sync"
	"time"

	"github.com/zjrosen/perles/internal/orchestration/fabric/domain"
)

// ReactionRepository manages emoji reactions on threads.
type ReactionRepository interface {
	// Add adds a reaction to a thread. If the same agent+emoji already exists, it's a no-op.
	Add(threadID, agentID, emoji string) (*domain.Reaction, error)

	// Remove removes a reaction from a thread.
	Remove(threadID, agentID, emoji string) error

	// ListForThread returns all reactions for a thread.
	ListForThread(threadID string) ([]domain.Reaction, error)

	// GetSummary returns aggregated reaction counts for a thread.
	GetSummary(threadID string) ([]domain.ReactionSummary, error)
}

// InMemoryReactionRepository is an in-memory implementation of ReactionRepository.
type InMemoryReactionRepository struct {
	mu        sync.RWMutex
	reactions map[string]*domain.Reaction // key = reaction.Key()
}

// NewInMemoryReactionRepository creates a new in-memory reaction repository.
func NewInMemoryReactionRepository() *InMemoryReactionRepository {
	return &InMemoryReactionRepository{
		reactions: make(map[string]*domain.Reaction),
	}
}

// Add adds a reaction to a thread.
func (r *InMemoryReactionRepository) Add(threadID, agentID, emoji string) (*domain.Reaction, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	reaction := &domain.Reaction{
		ThreadID:  threadID,
		AgentID:   agentID,
		Emoji:     emoji,
		CreatedAt: time.Now(),
	}

	key := reaction.Key()
	if existing, ok := r.reactions[key]; ok {
		return existing, nil // Already exists, return existing
	}

	r.reactions[key] = reaction
	return reaction, nil
}

// Remove removes a reaction from a thread.
func (r *InMemoryReactionRepository) Remove(threadID, agentID, emoji string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	reaction := &domain.Reaction{
		ThreadID: threadID,
		AgentID:  agentID,
		Emoji:    emoji,
	}

	delete(r.reactions, reaction.Key())
	return nil
}

// ListForThread returns all reactions for a thread.
func (r *InMemoryReactionRepository) ListForThread(threadID string) ([]domain.Reaction, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var reactions []domain.Reaction
	for _, reaction := range r.reactions {
		if reaction.ThreadID == threadID {
			reactions = append(reactions, *reaction)
		}
	}

	return reactions, nil
}

// GetSummary returns aggregated reaction counts for a thread.
func (r *InMemoryReactionRepository) GetSummary(threadID string) ([]domain.ReactionSummary, error) {
	reactions, err := r.ListForThread(threadID)
	if err != nil {
		return nil, err
	}

	// Group by emoji
	emojiMap := make(map[string][]string) // emoji -> []agentIDs
	for _, reaction := range reactions {
		emojiMap[reaction.Emoji] = append(emojiMap[reaction.Emoji], reaction.AgentID)
	}

	// Convert to summaries
	summaries := make([]domain.ReactionSummary, 0, len(emojiMap))
	for emoji, agentIDs := range emojiMap {
		summaries = append(summaries, domain.ReactionSummary{
			Emoji:    emoji,
			Count:    len(agentIDs),
			AgentIDs: agentIDs,
		})
	}

	return summaries, nil
}
