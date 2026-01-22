package session

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestFactory_Create_Success(t *testing.T) {
	// Create a temp directory for session storage
	baseDir := t.TempDir()

	factory := NewFactory(FactoryConfig{
		BaseDir: baseDir,
	})

	workDir := t.TempDir()
	sess, err := factory.Create(CreateOptions{
		SessionID: "test-session-123",
		WorkDir:   workDir,
	})

	require.NoError(t, err)
	require.NotNil(t, sess)
	require.Equal(t, "test-session-123", sess.ID)
	require.NotEmpty(t, sess.Dir)

	// Verify directory structure was created
	require.DirExists(t, sess.Dir)
	require.DirExists(t, filepath.Join(sess.Dir, "coordinator"))
	require.DirExists(t, filepath.Join(sess.Dir, "workers"))
	require.FileExists(t, filepath.Join(sess.Dir, "metadata.json"))

	// Clean up
	_ = sess.Close(StatusCompleted)
}

func TestFactory_Create_WithApplicationName(t *testing.T) {
	baseDir := t.TempDir()

	factory := NewFactory(FactoryConfig{
		BaseDir: baseDir,
	})

	workDir := t.TempDir()
	sess, err := factory.Create(CreateOptions{
		SessionID:       "test-session-456",
		WorkDir:         workDir,
		ApplicationName: "my-custom-app",
	})

	require.NoError(t, err)
	require.NotNil(t, sess)

	// Verify path includes the custom application name
	require.Contains(t, sess.Dir, "my-custom-app")

	_ = sess.Close(StatusCompleted)
}

func TestFactory_Create_RequiresSessionID(t *testing.T) {
	factory := NewFactory(FactoryConfig{
		BaseDir: t.TempDir(),
	})

	_, err := factory.Create(CreateOptions{
		WorkDir: t.TempDir(),
		// SessionID intentionally omitted
	})

	require.Error(t, err)
	require.Contains(t, err.Error(), "SessionID is required")
}

func TestFactory_Create_RequiresWorkDir(t *testing.T) {
	factory := NewFactory(FactoryConfig{
		BaseDir: t.TempDir(),
	})

	_, err := factory.Create(CreateOptions{
		SessionID: "test-session",
		// WorkDir intentionally omitted
	})

	require.Error(t, err)
	require.Contains(t, err.Error(), "WorkDir is required")
}

func TestFactory_DefaultBaseDir(t *testing.T) {
	// Test that empty BaseDir uses default
	factory := NewFactory(FactoryConfig{
		BaseDir: "", // Empty - should use default
	})

	expectedDefault := DefaultBaseDir()
	require.Equal(t, expectedDefault, factory.BaseDir())
}

func TestFactory_Reopen_RequiresSessionID(t *testing.T) {
	factory := NewFactory(FactoryConfig{
		BaseDir: t.TempDir(),
	})

	_, err := factory.Reopen(CreateOptions{
		WorkDir: t.TempDir(),
	}, "/some/dir")

	require.Error(t, err)
	require.Contains(t, err.Error(), "SessionID is required")
}

func TestFactory_Reopen_RequiresExistingDir(t *testing.T) {
	factory := NewFactory(FactoryConfig{
		BaseDir: t.TempDir(),
	})

	_, err := factory.Reopen(CreateOptions{
		SessionID: "test-session",
		WorkDir:   t.TempDir(),
	}, "") // Empty dir

	require.Error(t, err)
	require.Contains(t, err.Error(), "existingDir is required")
}

func TestFactory_Reopen_Success(t *testing.T) {
	baseDir := t.TempDir()
	factory := NewFactory(FactoryConfig{
		BaseDir: baseDir,
	})

	workDir := t.TempDir()

	// First, create a session
	sess1, err := factory.Create(CreateOptions{
		SessionID: "test-session-reopen",
		WorkDir:   workDir,
	})
	require.NoError(t, err)
	sessionDir := sess1.Dir

	// Write some data
	now := time.Now()
	err = sess1.WriteCoordinatorRawJSON(now, []byte(`{"test": "data"}`))
	require.NoError(t, err)

	// Close it (this flushes buffers)
	err = sess1.Close(StatusCompleted)
	require.NoError(t, err)

	// Verify first write persisted
	rawPath := filepath.Join(sessionDir, "coordinator", "raw.jsonl")
	content1, err := os.ReadFile(rawPath)
	require.NoError(t, err)
	require.Contains(t, string(content1), `{"test": "data"}`, "first write should be persisted after close")

	// Now reopen it
	sess2, err := factory.Reopen(CreateOptions{
		SessionID: "test-session-reopen",
		WorkDir:   workDir,
	}, sessionDir)
	require.NoError(t, err)
	require.NotNil(t, sess2)
	require.Equal(t, sessionDir, sess2.Dir)

	// Verify we can write to it
	err = sess2.WriteCoordinatorRawJSON(now, []byte(`{"test": "more data"}`))
	require.NoError(t, err)

	// Close to flush
	err = sess2.Close(StatusCompleted)
	require.NoError(t, err)

	// Check raw.jsonl has both entries
	content2, err := os.ReadFile(rawPath)
	require.NoError(t, err)
	require.Contains(t, string(content2), `{"test": "data"}`, "first entry should still be present")
	require.Contains(t, string(content2), `{"test": "more data"}`, "second entry should be appended")
}
