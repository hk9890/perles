package repository

import (
	"sync"
	"time"

	"github.com/zjrosen/perles/internal/orchestration/fabric/domain"
)

// MemoryParticipantRepository is an in-memory implementation of ParticipantRepository.
type MemoryParticipantRepository struct {
	mu           sync.RWMutex
	participants map[string]*domain.Participant
}

// NewMemoryParticipantRepository creates a new in-memory participant repository.
func NewMemoryParticipantRepository() *MemoryParticipantRepository {
	return &MemoryParticipantRepository{
		participants: make(map[string]*domain.Participant),
	}
}

// Join adds a participant to the registry.
func (r *MemoryParticipantRepository) Join(agentID string, role domain.ParticipantRole) (*domain.Participant, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	p := &domain.Participant{
		AgentID:  agentID,
		Role:     role,
		JoinedAt: time.Now(),
	}
	r.participants[agentID] = p
	return p, nil
}

// Leave removes a participant from the registry.
func (r *MemoryParticipantRepository) Leave(agentID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.participants, agentID)
	return nil
}

// Get returns a participant by ID, or nil if not found.
func (r *MemoryParticipantRepository) Get(agentID string) (*domain.Participant, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.participants[agentID]
	if !ok {
		return nil, nil
	}
	return p, nil
}

// List returns all active participants.
func (r *MemoryParticipantRepository) List() ([]domain.Participant, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]domain.Participant, 0, len(r.participants))
	for _, p := range r.participants {
		result = append(result, *p)
	}
	return result, nil
}

// ListByRole returns participants with the given role.
func (r *MemoryParticipantRepository) ListByRole(role domain.ParticipantRole) ([]domain.Participant, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []domain.Participant
	for _, p := range r.participants {
		if p.Role == role {
			result = append(result, *p)
		}
	}
	return result, nil
}
