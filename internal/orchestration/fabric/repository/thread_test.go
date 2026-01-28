package repository

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zjrosen/perles/internal/orchestration/fabric/domain"
)

func TestMemoryThreadRepository_Create(t *testing.T) {
	repo := NewMemoryThreadRepository()

	thread := domain.Thread{
		Type:      domain.ThreadChannel,
		Slug:      "test",
		Title:     "Test Channel",
		CreatedBy: "system",
	}

	created, err := repo.Create(thread)
	require.NoError(t, err)
	require.NotEmpty(t, created.ID)
	require.Equal(t, int64(1), created.Seq)
	require.False(t, created.CreatedAt.IsZero())
}

func TestMemoryThreadRepository_Get(t *testing.T) {
	repo := NewMemoryThreadRepository()

	thread := domain.Thread{
		Type:      domain.ThreadMessage,
		Content:   "Hello",
		CreatedBy: "agent-1",
	}

	created, err := repo.Create(thread)
	require.NoError(t, err)

	retrieved, err := repo.Get(created.ID)
	require.NoError(t, err)
	require.Equal(t, created.ID, retrieved.ID)
	require.Equal(t, "Hello", retrieved.Content)
}

func TestMemoryThreadRepository_GetNotFound(t *testing.T) {
	repo := NewMemoryThreadRepository()

	_, err := repo.Get("nonexistent")
	require.Error(t, err)
	require.Contains(t, err.Error(), "not found")
}

func TestMemoryThreadRepository_GetBySlug(t *testing.T) {
	repo := NewMemoryThreadRepository()

	thread := domain.Thread{
		Type:      domain.ThreadChannel,
		Slug:      "tasks",
		Title:     "Tasks",
		CreatedBy: "system",
	}

	created, err := repo.Create(thread)
	require.NoError(t, err)

	retrieved, err := repo.GetBySlug("tasks")
	require.NoError(t, err)
	require.Equal(t, created.ID, retrieved.ID)
}

func TestMemoryThreadRepository_DuplicateSlug(t *testing.T) {
	repo := NewMemoryThreadRepository()

	thread1 := domain.Thread{
		Type: domain.ThreadChannel,
		Slug: "tasks",
	}
	_, err := repo.Create(thread1)
	require.NoError(t, err)

	thread2 := domain.Thread{
		Type: domain.ThreadChannel,
		Slug: "tasks",
	}
	_, err = repo.Create(thread2)
	require.Error(t, err)
	require.Contains(t, err.Error(), "slug already exists")
}

func TestMemoryThreadRepository_List(t *testing.T) {
	repo := NewMemoryThreadRepository()

	for i := 0; i < 5; i++ {
		_, err := repo.Create(domain.Thread{
			Type:      domain.ThreadMessage,
			Content:   "Message",
			CreatedBy: "agent-1",
		})
		require.NoError(t, err)
	}

	// Create a channel too
	_, err := repo.Create(domain.Thread{
		Type: domain.ThreadChannel,
		Slug: "test",
	})
	require.NoError(t, err)

	// List all
	all, err := repo.List(ListOptions{})
	require.NoError(t, err)
	require.Len(t, all, 6)

	// List only messages
	msgType := domain.ThreadMessage
	messages, err := repo.List(ListOptions{Type: &msgType})
	require.NoError(t, err)
	require.Len(t, messages, 5)

	// List with limit
	limited, err := repo.List(ListOptions{Limit: 3})
	require.NoError(t, err)
	require.Len(t, limited, 3)

	// List after seq
	afterSeq, err := repo.List(ListOptions{AfterSeq: 3})
	require.NoError(t, err)
	require.Len(t, afterSeq, 3)
}

func TestMemoryThreadRepository_Archive(t *testing.T) {
	repo := NewMemoryThreadRepository()

	thread, err := repo.Create(domain.Thread{
		Type:    domain.ThreadMessage,
		Content: "Hello",
	})
	require.NoError(t, err)
	require.False(t, thread.IsArchived())

	err = repo.Archive(thread.ID)
	require.NoError(t, err)

	retrieved, err := repo.Get(thread.ID)
	require.NoError(t, err)
	require.True(t, retrieved.IsArchived())
}

func TestMemoryThreadRepository_ArtifactMetadata(t *testing.T) {
	repo := NewMemoryThreadRepository()

	// Artifacts now store file path reference, not content
	artifact, err := repo.Create(domain.Thread{
		Type:       domain.ThreadArtifact,
		Name:       "test.txt",
		MediaType:  "text/plain",
		SizeBytes:  100,
		StorageURI: "file:///path/to/test.txt",
		Sha256:     "abc123",
	})
	require.NoError(t, err)

	retrieved, err := repo.Get(artifact.ID)
	require.NoError(t, err)
	require.Equal(t, "file:///path/to/test.txt", retrieved.StorageURI)
	require.Equal(t, "abc123", retrieved.Sha256)
	require.Equal(t, int64(100), retrieved.SizeBytes)
}
