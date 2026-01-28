package repository

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zjrosen/perles/internal/orchestration/fabric/domain"
)

func TestMemoryDependencyRepository_Add(t *testing.T) {
	repo := NewMemoryDependencyRepository()

	dep := domain.NewDependency("msg-1", "channel-1", domain.RelationChildOf)
	err := repo.Add(dep)
	require.NoError(t, err)

	// Idempotent - adding again should not error
	err = repo.Add(dep)
	require.NoError(t, err)
}

func TestMemoryDependencyRepository_GetParents(t *testing.T) {
	repo := NewMemoryDependencyRepository()

	// msg-1 is child_of channel-1
	err := repo.Add(domain.NewDependency("msg-1", "channel-1", domain.RelationChildOf))
	require.NoError(t, err)

	// msg-2 is reply_to msg-1
	err = repo.Add(domain.NewDependency("msg-2", "msg-1", domain.RelationReplyTo))
	require.NoError(t, err)

	// msg-2 is also child_of channel-1 (for channel membership)
	err = repo.Add(domain.NewDependency("msg-2", "channel-1", domain.RelationChildOf))
	require.NoError(t, err)

	// Get all parents of msg-2
	parents, err := repo.GetParents("msg-2", nil)
	require.NoError(t, err)
	require.Len(t, parents, 2)

	// Get only child_of parents
	childOf := domain.RelationChildOf
	parents, err = repo.GetParents("msg-2", &childOf)
	require.NoError(t, err)
	require.Len(t, parents, 1)
	require.Equal(t, "channel-1", parents[0].DependsOnID)

	// Get only reply_to parents
	replyTo := domain.RelationReplyTo
	parents, err = repo.GetParents("msg-2", &replyTo)
	require.NoError(t, err)
	require.Len(t, parents, 1)
	require.Equal(t, "msg-1", parents[0].DependsOnID)
}

func TestMemoryDependencyRepository_GetChildren(t *testing.T) {
	repo := NewMemoryDependencyRepository()

	// msg-1 and msg-2 are children of channel-1
	err := repo.Add(domain.NewDependency("msg-1", "channel-1", domain.RelationChildOf))
	require.NoError(t, err)
	err = repo.Add(domain.NewDependency("msg-2", "channel-1", domain.RelationChildOf))
	require.NoError(t, err)

	// artifact-1 references msg-1
	err = repo.Add(domain.NewDependency("artifact-1", "msg-1", domain.RelationReferences))
	require.NoError(t, err)

	// Get all children of channel-1
	children, err := repo.GetChildren("channel-1", nil)
	require.NoError(t, err)
	require.Len(t, children, 2)

	// Get children of msg-1 (just the artifact)
	refs := domain.RelationReferences
	children, err = repo.GetChildren("msg-1", &refs)
	require.NoError(t, err)
	require.Len(t, children, 1)
	require.Equal(t, "artifact-1", children[0].ThreadID)
}

func TestMemoryDependencyRepository_GetRoots(t *testing.T) {
	repo := NewMemoryDependencyRepository()

	// root channel has no parent
	// but we need to establish it in the graph
	err := repo.Add(domain.NewDependency("channel-1", "root", domain.RelationChildOf))
	require.NoError(t, err)
	err = repo.Add(domain.NewDependency("msg-1", "channel-1", domain.RelationChildOf))
	require.NoError(t, err)

	roots, err := repo.GetRoots()
	require.NoError(t, err)
	require.Len(t, roots, 1)
	require.Equal(t, "root", roots[0])
}

func TestMemoryDependencyRepository_Remove(t *testing.T) {
	repo := NewMemoryDependencyRepository()

	err := repo.Add(domain.NewDependency("msg-1", "channel-1", domain.RelationChildOf))
	require.NoError(t, err)

	parents, err := repo.GetParents("msg-1", nil)
	require.NoError(t, err)
	require.Len(t, parents, 1)

	err = repo.Remove("msg-1", "channel-1")
	require.NoError(t, err)

	parents, err = repo.GetParents("msg-1", nil)
	require.NoError(t, err)
	require.Len(t, parents, 0)
}

func TestMemoryDependencyRepository_GetChannelForMessage(t *testing.T) {
	repo := NewMemoryDependencyRepository()

	err := repo.Add(domain.NewDependency("msg-1", "channel-tasks", domain.RelationChildOf))
	require.NoError(t, err)

	channelID, err := repo.GetChannelForMessage("msg-1")
	require.NoError(t, err)
	require.Equal(t, "channel-tasks", channelID)
}
