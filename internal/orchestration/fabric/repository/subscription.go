package repository

import (
	"sync"
	"time"

	"github.com/zjrosen/perles/internal/orchestration/fabric/domain"
)

// MemorySubscriptionRepository is an in-memory implementation of SubscriptionRepository.
type MemorySubscriptionRepository struct {
	mu   sync.RWMutex
	subs map[string]*domain.Subscription // key -> subscription

	// Indexes for efficient lookups
	byAgent   map[string][]string // agentID -> list of sub keys
	byChannel map[string][]string // channelID -> list of sub keys
}

// NewMemorySubscriptionRepository creates a new in-memory subscription repository.
func NewMemorySubscriptionRepository() *MemorySubscriptionRepository {
	return &MemorySubscriptionRepository{
		subs:      make(map[string]*domain.Subscription),
		byAgent:   make(map[string][]string),
		byChannel: make(map[string][]string),
	}
}

// Subscribe creates or updates a subscription.
func (r *MemorySubscriptionRepository) Subscribe(channelID, agentID string, mode domain.SubscriptionMode) (*domain.Subscription, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	sub := domain.Subscription{
		ChannelID: channelID,
		AgentID:   agentID,
		Mode:      mode,
		CreatedAt: time.Now(),
	}
	key := sub.Key()

	existing, exists := r.subs[key]
	if exists {
		existing.Mode = mode
		return existing, nil
	}

	r.subs[key] = &sub
	r.byAgent[agentID] = append(r.byAgent[agentID], key)
	r.byChannel[channelID] = append(r.byChannel[channelID], key)

	return &sub, nil
}

// Unsubscribe removes a subscription.
func (r *MemorySubscriptionRepository) Unsubscribe(channelID, agentID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := channelID + ":" + agentID
	sub, exists := r.subs[key]
	if !exists {
		return nil // Idempotent
	}

	delete(r.subs, key)
	r.byAgent[sub.AgentID] = removeFromSlice(r.byAgent[sub.AgentID], key)
	r.byChannel[sub.ChannelID] = removeFromSlice(r.byChannel[sub.ChannelID], key)

	return nil
}

// ListForAgent returns all subscriptions for an agent.
func (r *MemorySubscriptionRepository) ListForAgent(agentID string) ([]domain.Subscription, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	keys := r.byAgent[agentID]
	results := make([]domain.Subscription, 0, len(keys))

	for _, key := range keys {
		if sub, exists := r.subs[key]; exists {
			results = append(results, *sub)
		}
	}

	return results, nil
}

// ListForChannel returns all subscriptions for a channel.
func (r *MemorySubscriptionRepository) ListForChannel(channelID string) ([]domain.Subscription, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	keys := r.byChannel[channelID]
	results := make([]domain.Subscription, 0, len(keys))

	for _, key := range keys {
		if sub, exists := r.subs[key]; exists {
			results = append(results, *sub)
		}
	}

	return results, nil
}

// Get returns a specific subscription if it exists.
func (r *MemorySubscriptionRepository) Get(channelID, agentID string) (*domain.Subscription, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	key := channelID + ":" + agentID
	sub, exists := r.subs[key]
	if !exists {
		return nil, nil
	}

	copy := *sub
	return &copy, nil
}

var _ SubscriptionRepository = (*MemorySubscriptionRepository)(nil)
