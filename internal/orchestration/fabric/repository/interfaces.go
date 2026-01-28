// Package repository provides storage interfaces and implementations for Fabric.
package repository

import (
	"github.com/zjrosen/perles/internal/orchestration/fabric/domain"
)

// ListOptions configures thread listing.
type ListOptions struct {
	Type       *domain.ThreadType // Filter by type (channel, message, artifact)
	AfterSeq   int64              // Pagination: threads after this sequence
	Limit      int                // Max results (0 = no limit)
	CreatedBy  *string            // Filter by creator
	HasMention *string            // Filter to threads mentioning this agent
	ChannelID  *string            // Filter to messages in this channel (requires dependency lookup)
}

// UnackedSummary contains unacked message info for a channel.
type UnackedSummary struct {
	Count     int
	ThreadIDs []string
}

// ThreadRepository manages all thread nodes (channels, messages, artifacts).
type ThreadRepository interface {
	// Create adds a new thread to the graph.
	// ID and Seq are assigned automatically if empty/zero.
	Create(thread domain.Thread) (*domain.Thread, error)

	// Get retrieves a thread by ID.
	Get(id string) (*domain.Thread, error)

	// GetBySlug finds a channel thread by its slug (e.g., "tasks").
	GetBySlug(slug string) (*domain.Thread, error)

	// List returns threads matching the filter criteria.
	List(opts ListOptions) ([]domain.Thread, error)

	// Update modifies an existing thread.
	Update(thread domain.Thread) (*domain.Thread, error)

	// Archive soft-deletes a thread by setting ArchivedAt.
	Archive(id string) error
}

// DependencyRepository manages edges between threads.
type DependencyRepository interface {
	// Add creates a dependency edge.
	Add(dep domain.Dependency) error

	// Remove deletes a dependency edge.
	Remove(threadID, dependsOnID string) error

	// GetParents returns threads this thread depends on.
	// If relation is nil, returns all parent relations.
	// For messages: the channel (child_of) and reply target (reply_to).
	// For artifacts: the message/channel it references.
	GetParents(threadID string, relation *domain.RelationType) ([]domain.Dependency, error)

	// GetChildren returns threads that depend on this thread.
	// If relation is nil, returns all child relations.
	// For channels: messages (child_of) and sub-channels (child_of).
	// For messages: replies (reply_to) and attached artifacts (references).
	GetChildren(threadID string, relation *domain.RelationType) ([]domain.Dependency, error)

	// GetRoots returns thread IDs with no child_of dependency (root channels).
	GetRoots() ([]string, error)
}

// SubscriptionRepository manages agent subscriptions to channels.
type SubscriptionRepository interface {
	// Subscribe creates or updates a subscription.
	Subscribe(channelID, agentID string, mode domain.SubscriptionMode) (*domain.Subscription, error)

	// Unsubscribe removes a subscription.
	Unsubscribe(channelID, agentID string) error

	// ListForAgent returns all subscriptions for an agent.
	ListForAgent(agentID string) ([]domain.Subscription, error)

	// ListForChannel returns all subscriptions for a channel.
	ListForChannel(channelID string) ([]domain.Subscription, error)

	// Get returns a specific subscription if it exists.
	Get(channelID, agentID string) (*domain.Subscription, error)
}

// AckRepository tracks which messages agents have acknowledged.
type AckRepository interface {
	// Ack marks message threads as acknowledged by an agent.
	Ack(agentID string, threadIDs ...string) error

	// IsAcked checks if an agent has acknowledged a message.
	IsAcked(threadID, agentID string) (bool, error)

	// GetUnacked returns all unacked messages for an agent, grouped by channel.
	// Key is channelID, value contains count and thread IDs.
	// Requires cooperation with DependencyRepository to resolve channels.
	GetUnacked(agentID string) (map[string]UnackedSummary, error)

	// GetAckedThreadIDs returns all thread IDs that an agent has acknowledged.
	GetAckedThreadIDs(agentID string) ([]string, error)
}
