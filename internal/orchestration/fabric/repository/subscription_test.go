package repository

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zjrosen/perles/internal/orchestration/fabric/domain"
)

func TestMemorySubscriptionRepository_Subscribe(t *testing.T) {
	repo := NewMemorySubscriptionRepository()

	sub, err := repo.Subscribe("channel-1", "agent-1", domain.ModeAll)
	require.NoError(t, err)
	require.Equal(t, "channel-1", sub.ChannelID)
	require.Equal(t, "agent-1", sub.AgentID)
	require.Equal(t, domain.ModeAll, sub.Mode)
}

func TestMemorySubscriptionRepository_SubscribeUpdate(t *testing.T) {
	repo := NewMemorySubscriptionRepository()

	_, err := repo.Subscribe("channel-1", "agent-1", domain.ModeAll)
	require.NoError(t, err)

	// Update mode
	updated, err := repo.Subscribe("channel-1", "agent-1", domain.ModeMentions)
	require.NoError(t, err)
	require.Equal(t, domain.ModeMentions, updated.Mode)

	// Verify only one subscription exists
	subs, err := repo.ListForAgent("agent-1")
	require.NoError(t, err)
	require.Len(t, subs, 1)
}

func TestMemorySubscriptionRepository_Unsubscribe(t *testing.T) {
	repo := NewMemorySubscriptionRepository()

	_, err := repo.Subscribe("channel-1", "agent-1", domain.ModeAll)
	require.NoError(t, err)

	err = repo.Unsubscribe("channel-1", "agent-1")
	require.NoError(t, err)

	subs, err := repo.ListForAgent("agent-1")
	require.NoError(t, err)
	require.Len(t, subs, 0)

	// Idempotent
	err = repo.Unsubscribe("channel-1", "agent-1")
	require.NoError(t, err)
}

func TestMemorySubscriptionRepository_ListForAgent(t *testing.T) {
	repo := NewMemorySubscriptionRepository()

	_, err := repo.Subscribe("channel-1", "agent-1", domain.ModeAll)
	require.NoError(t, err)
	_, err = repo.Subscribe("channel-2", "agent-1", domain.ModeMentions)
	require.NoError(t, err)
	_, err = repo.Subscribe("channel-1", "agent-2", domain.ModeAll)
	require.NoError(t, err)

	subs, err := repo.ListForAgent("agent-1")
	require.NoError(t, err)
	require.Len(t, subs, 2)
}

func TestMemorySubscriptionRepository_ListForChannel(t *testing.T) {
	repo := NewMemorySubscriptionRepository()

	_, err := repo.Subscribe("channel-1", "agent-1", domain.ModeAll)
	require.NoError(t, err)
	_, err = repo.Subscribe("channel-1", "agent-2", domain.ModeMentions)
	require.NoError(t, err)
	_, err = repo.Subscribe("channel-2", "agent-1", domain.ModeAll)
	require.NoError(t, err)

	subs, err := repo.ListForChannel("channel-1")
	require.NoError(t, err)
	require.Len(t, subs, 2)
}

func TestMemorySubscriptionRepository_Get(t *testing.T) {
	repo := NewMemorySubscriptionRepository()

	// Not found
	sub, err := repo.Get("channel-1", "agent-1")
	require.NoError(t, err)
	require.Nil(t, sub)

	// Create and get
	_, err = repo.Subscribe("channel-1", "agent-1", domain.ModeAll)
	require.NoError(t, err)

	sub, err = repo.Get("channel-1", "agent-1")
	require.NoError(t, err)
	require.NotNil(t, sub)
	require.Equal(t, domain.ModeAll, sub.Mode)
}
