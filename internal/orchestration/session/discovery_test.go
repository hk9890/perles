package session

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// Helper to create a test session in the application index with specified metadata.
// Returns the session directory path.
func createTestIndexedSession(t *testing.T, pathBuilder *SessionPathBuilder, entry SessionIndexEntry, metadata *Metadata) string {
	t.Helper()

	// Create session directory
	require.NoError(t, os.MkdirAll(entry.SessionDir, 0750))

	// Save metadata
	require.NoError(t, metadata.Save(entry.SessionDir))

	// Load or create application index and add entry
	indexPath := pathBuilder.ApplicationIndexPath()
	appIndex, err := LoadApplicationIndex(indexPath)
	require.NoError(t, err)

	appIndex.Sessions = append(appIndex.Sessions, entry)
	appIndex.ApplicationName = pathBuilder.ApplicationName()

	require.NoError(t, SaveApplicationIndex(indexPath, appIndex))

	return entry.SessionDir
}

// Helper to create a minimal resumable session
func createResumableTestSession(t *testing.T, pathBuilder *SessionPathBuilder, id string, startTime time.Time) string {
	t.Helper()

	sessionDir := pathBuilder.SessionDir(id, startTime)
	entry := SessionIndexEntry{
		ID:              id,
		StartTime:       startTime,
		EndTime:         startTime.Add(time.Hour),
		Status:          StatusCompleted,
		SessionDir:      sessionDir,
		WorkerCount:     2,
		ApplicationName: pathBuilder.ApplicationName(),
		WorkDir:         "/test/project",
	}

	metadata := &Metadata{
		SessionID:             id,
		StartTime:             startTime,
		EndTime:               startTime.Add(time.Hour),
		Status:                StatusCompleted,
		SessionDir:            sessionDir,
		Resumable:             true,
		CoordinatorSessionRef: "coord-ref-" + id,
		ApplicationName:       pathBuilder.ApplicationName(),
		WorkDir:               "/test/project",
	}

	return createTestIndexedSession(t, pathBuilder, entry, metadata)
}

// Helper to create a non-resumable session
func createNonResumableTestSession(t *testing.T, pathBuilder *SessionPathBuilder, id string, startTime time.Time) string {
	t.Helper()

	sessionDir := pathBuilder.SessionDir(id, startTime)
	entry := SessionIndexEntry{
		ID:              id,
		StartTime:       startTime,
		EndTime:         startTime.Add(time.Hour),
		Status:          StatusCompleted,
		SessionDir:      sessionDir,
		WorkerCount:     1,
		ApplicationName: pathBuilder.ApplicationName(),
	}

	metadata := &Metadata{
		SessionID:             id,
		StartTime:             startTime,
		EndTime:               startTime.Add(time.Hour),
		Status:                StatusCompleted,
		SessionDir:            sessionDir,
		Resumable:             false, // Not resumable
		CoordinatorSessionRef: "",    // No ref
		ApplicationName:       pathBuilder.ApplicationName(),
	}

	return createTestIndexedSession(t, pathBuilder, entry, metadata)
}

// --- ListResumableSessions Tests ---

func TestListResumableSessions_Empty(t *testing.T) {
	baseDir := t.TempDir()
	pathBuilder := NewSessionPathBuilder(baseDir, "test-app")

	// No sessions exist - should return empty slice, not error
	sessions, err := ListResumableSessions(pathBuilder)
	require.NoError(t, err)
	require.Empty(t, sessions)
	require.NotNil(t, sessions) // Should be empty slice, not nil
}

func TestListResumableSessions_FilterNonResumable(t *testing.T) {
	baseDir := t.TempDir()
	pathBuilder := NewSessionPathBuilder(baseDir, "test-app")

	now := time.Now().UTC().Truncate(time.Second)

	// Create some resumable and some non-resumable sessions
	createResumableTestSession(t, pathBuilder, "resumable-1", now.Add(-3*time.Hour))
	createNonResumableTestSession(t, pathBuilder, "non-resumable-1", now.Add(-2*time.Hour))
	createResumableTestSession(t, pathBuilder, "resumable-2", now.Add(-time.Hour))
	createNonResumableTestSession(t, pathBuilder, "non-resumable-2", now)

	sessions, err := ListResumableSessions(pathBuilder)
	require.NoError(t, err)

	// Should only include resumable sessions
	require.Len(t, sessions, 2)

	// Verify IDs
	ids := make([]string, len(sessions))
	for i, s := range sessions {
		ids[i] = s.ID
		require.True(t, s.Resumable)
	}
	require.Contains(t, ids, "resumable-1")
	require.Contains(t, ids, "resumable-2")
	require.NotContains(t, ids, "non-resumable-1")
	require.NotContains(t, ids, "non-resumable-2")
}

func TestListResumableSessions_SortByRecent(t *testing.T) {
	baseDir := t.TempDir()
	pathBuilder := NewSessionPathBuilder(baseDir, "test-app")

	now := time.Now().UTC().Truncate(time.Second)

	// Create sessions at different times (order of creation doesn't matter, they're sorted)
	createResumableTestSession(t, pathBuilder, "oldest", now.Add(-3*time.Hour))
	createResumableTestSession(t, pathBuilder, "newest", now)
	createResumableTestSession(t, pathBuilder, "middle", now.Add(-time.Hour))

	sessions, err := ListResumableSessions(pathBuilder)
	require.NoError(t, err)
	require.Len(t, sessions, 3)

	// Should be sorted by StartTime descending (most recent first)
	require.Equal(t, "newest", sessions[0].ID)
	require.Equal(t, "middle", sessions[1].ID)
	require.Equal(t, "oldest", sessions[2].ID)

	// Verify times are actually descending
	require.True(t, sessions[0].StartTime.After(sessions[1].StartTime))
	require.True(t, sessions[1].StartTime.After(sessions[2].StartTime))
}

// --- ListAllSessions Tests ---

func TestListAllSessions_IncludesNonResumable(t *testing.T) {
	baseDir := t.TempDir()
	pathBuilder := NewSessionPathBuilder(baseDir, "test-app")

	now := time.Now().UTC().Truncate(time.Second)

	// Create mix of resumable and non-resumable
	createResumableTestSession(t, pathBuilder, "resumable-1", now.Add(-2*time.Hour))
	createNonResumableTestSession(t, pathBuilder, "non-resumable-1", now.Add(-time.Hour))
	createResumableTestSession(t, pathBuilder, "resumable-2", now)

	sessions, err := ListAllSessions(pathBuilder)
	require.NoError(t, err)

	// Should include all sessions
	require.Len(t, sessions, 3)

	// Verify both resumable and non-resumable are present
	var hasResumable, hasNonResumable bool
	for _, s := range sessions {
		if s.Resumable {
			hasResumable = true
		} else {
			hasNonResumable = true
		}
	}
	require.True(t, hasResumable, "should have resumable sessions")
	require.True(t, hasNonResumable, "should have non-resumable sessions")
}

// --- GetRecentSessions Tests ---

func TestGetRecentSessions_LimitWorks(t *testing.T) {
	baseDir := t.TempDir()
	pathBuilder := NewSessionPathBuilder(baseDir, "test-app")

	now := time.Now().UTC().Truncate(time.Second)

	// Create 5 sessions
	for i := 0; i < 5; i++ {
		createResumableTestSession(t, pathBuilder, "session-"+string(rune('a'+i)), now.Add(time.Duration(-i)*time.Hour))
	}

	// Request only 3
	sessions, err := GetRecentSessions(pathBuilder, 3)
	require.NoError(t, err)
	require.Len(t, sessions, 3)

	// Should be the 3 most recent (session-a is most recent, then b, then c)
	require.Equal(t, "session-a", sessions[0].ID)
	require.Equal(t, "session-b", sessions[1].ID)
	require.Equal(t, "session-c", sessions[2].ID)
}

func TestGetRecentSessions_LimitExceedsCount(t *testing.T) {
	baseDir := t.TempDir()
	pathBuilder := NewSessionPathBuilder(baseDir, "test-app")

	now := time.Now().UTC().Truncate(time.Second)

	// Create only 2 sessions
	createResumableTestSession(t, pathBuilder, "session-1", now.Add(-time.Hour))
	createResumableTestSession(t, pathBuilder, "session-2", now)

	// Request 10
	sessions, err := GetRecentSessions(pathBuilder, 10)
	require.NoError(t, err)

	// Should return all 2 (not error)
	require.Len(t, sessions, 2)
}

// --- GetLatestResumableSession Tests ---

func TestGetLatestResumableSession_ReturnsNewest(t *testing.T) {
	baseDir := t.TempDir()
	pathBuilder := NewSessionPathBuilder(baseDir, "test-app")

	now := time.Now().UTC().Truncate(time.Second)

	// Create multiple resumable sessions at different times
	createResumableTestSession(t, pathBuilder, "oldest", now.Add(-3*time.Hour))
	createResumableTestSession(t, pathBuilder, "middle", now.Add(-2*time.Hour))
	createResumableTestSession(t, pathBuilder, "newest", now)
	// Also create a non-resumable session that's even newer - should be ignored
	createNonResumableTestSession(t, pathBuilder, "newest-non-resumable", now.Add(time.Hour))

	session, err := GetLatestResumableSession(pathBuilder)
	require.NoError(t, err)
	require.NotNil(t, session)

	// Should return the most recent RESUMABLE session
	require.Equal(t, "newest", session.ID)
	require.True(t, session.Resumable)
	require.Equal(t, "coord-ref-newest", session.CoordinatorSessionRef)
}

func TestGetLatestResumableSession_NoneAvailable(t *testing.T) {
	baseDir := t.TempDir()
	pathBuilder := NewSessionPathBuilder(baseDir, "test-app")

	now := time.Now().UTC().Truncate(time.Second)

	// Create only non-resumable sessions
	createNonResumableTestSession(t, pathBuilder, "non-resumable-1", now.Add(-time.Hour))
	createNonResumableTestSession(t, pathBuilder, "non-resumable-2", now)

	session, err := GetLatestResumableSession(pathBuilder)
	require.NoError(t, err)
	require.Nil(t, session) // Should return nil, not error
}

func TestGetLatestResumableSession_EmptyIndex(t *testing.T) {
	baseDir := t.TempDir()
	pathBuilder := NewSessionPathBuilder(baseDir, "test-app")

	// No sessions at all
	session, err := GetLatestResumableSession(pathBuilder)
	require.NoError(t, err)
	require.Nil(t, session)
}

// --- FindSessionByID Tests ---

func TestFindSessionByID_Found(t *testing.T) {
	baseDir := t.TempDir()
	pathBuilder := NewSessionPathBuilder(baseDir, "test-app")

	now := time.Now().UTC().Truncate(time.Second)

	// Create several sessions
	createResumableTestSession(t, pathBuilder, "session-aaa", now.Add(-2*time.Hour))
	createResumableTestSession(t, pathBuilder, "session-bbb", now.Add(-time.Hour))
	createNonResumableTestSession(t, pathBuilder, "session-ccc", now)

	// Find specific session
	session, err := FindSessionByID(pathBuilder, "session-bbb")
	require.NoError(t, err)
	require.NotNil(t, session)
	require.Equal(t, "session-bbb", session.ID)
	require.True(t, session.Resumable)
}

func TestFindSessionByID_NotFound(t *testing.T) {
	baseDir := t.TempDir()
	pathBuilder := NewSessionPathBuilder(baseDir, "test-app")

	now := time.Now().UTC().Truncate(time.Second)

	// Create a session
	createResumableTestSession(t, pathBuilder, "existing-session", now)

	// Try to find non-existent session
	session, err := FindSessionByID(pathBuilder, "non-existent-session")
	require.NoError(t, err)
	require.Nil(t, session) // Should return nil, not error
}

func TestFindSessionByID_EmptyIndex(t *testing.T) {
	baseDir := t.TempDir()
	pathBuilder := NewSessionPathBuilder(baseDir, "test-app")

	// No sessions at all
	session, err := FindSessionByID(pathBuilder, "any-id")
	require.NoError(t, err)
	require.Nil(t, session)
}

// --- Edge Cases ---

func TestListResumableSessions_CorruptMetadata(t *testing.T) {
	baseDir := t.TempDir()
	pathBuilder := NewSessionPathBuilder(baseDir, "test-app")

	now := time.Now().UTC().Truncate(time.Second)

	// Create a valid resumable session
	createResumableTestSession(t, pathBuilder, "valid-session", now.Add(-time.Hour))

	// Create session with corrupt metadata
	corruptDir := pathBuilder.SessionDir("corrupt-session", now)
	require.NoError(t, os.MkdirAll(corruptDir, 0750))

	// Write corrupt JSON to metadata file
	metadataPath := filepath.Join(corruptDir, "metadata.json")
	require.NoError(t, os.WriteFile(metadataPath, []byte("{ invalid json"), 0600))

	// Add corrupt session to index
	indexPath := pathBuilder.ApplicationIndexPath()
	appIndex, err := LoadApplicationIndex(indexPath)
	require.NoError(t, err)

	appIndex.Sessions = append(appIndex.Sessions, SessionIndexEntry{
		ID:         "corrupt-session",
		StartTime:  now,
		Status:     StatusCompleted,
		SessionDir: corruptDir,
	})
	require.NoError(t, SaveApplicationIndex(indexPath, appIndex))

	// List should succeed and skip corrupt session
	sessions, err := ListResumableSessions(pathBuilder)
	require.NoError(t, err)

	// Should only have the valid session
	require.Len(t, sessions, 1)
	require.Equal(t, "valid-session", sessions[0].ID)
}

func TestListAllSessions_MissingMetadata(t *testing.T) {
	baseDir := t.TempDir()
	pathBuilder := NewSessionPathBuilder(baseDir, "test-app")

	now := time.Now().UTC().Truncate(time.Second)

	// Create a valid session
	createResumableTestSession(t, pathBuilder, "valid-session", now.Add(-time.Hour))

	// Create session directory without metadata
	missingDir := pathBuilder.SessionDir("missing-metadata", now)
	require.NoError(t, os.MkdirAll(missingDir, 0750))
	// Don't create metadata.json

	// Add to index
	indexPath := pathBuilder.ApplicationIndexPath()
	appIndex, err := LoadApplicationIndex(indexPath)
	require.NoError(t, err)

	appIndex.Sessions = append(appIndex.Sessions, SessionIndexEntry{
		ID:         "missing-metadata",
		StartTime:  now,
		Status:     StatusCompleted,
		SessionDir: missingDir,
	})
	require.NoError(t, SaveApplicationIndex(indexPath, appIndex))

	// List should succeed and skip session with missing metadata
	sessions, err := ListAllSessions(pathBuilder)
	require.NoError(t, err)

	// Should only have the valid session
	require.Len(t, sessions, 1)
	require.Equal(t, "valid-session", sessions[0].ID)
}

func TestSessionSummary_FieldsPopulated(t *testing.T) {
	baseDir := t.TempDir()
	pathBuilder := NewSessionPathBuilder(baseDir, "test-app")

	now := time.Now().UTC().Truncate(time.Second)

	// Create a session with all fields populated
	sessionDir := createResumableTestSession(t, pathBuilder, "full-session", now)

	sessions, err := ListAllSessions(pathBuilder)
	require.NoError(t, err)
	require.Len(t, sessions, 1)

	s := sessions[0]

	// Verify all fields are populated
	require.Equal(t, "full-session", s.ID)
	require.Equal(t, "test-app", s.ApplicationName)
	require.Equal(t, "/test/project", s.WorkDir)
	require.Equal(t, now, s.StartTime)
	require.Equal(t, now.Add(time.Hour), s.EndTime)
	require.Equal(t, StatusCompleted, s.Status)
	require.Equal(t, 2, s.WorkerCount)
	require.True(t, s.Resumable)
	require.Equal(t, sessionDir, s.SessionDir)
	require.Equal(t, "coord-ref-full-session", s.CoordinatorSessionRef)
}

// Test that running sessions are not resumable
func TestListResumableSessions_RunningSessionNotResumable(t *testing.T) {
	baseDir := t.TempDir()
	pathBuilder := NewSessionPathBuilder(baseDir, "test-app")

	now := time.Now().UTC().Truncate(time.Second)

	// Create a running session (even if it has all resumption fields, running status prevents it)
	sessionDir := pathBuilder.SessionDir("running-session", now)
	entry := SessionIndexEntry{
		ID:              "running-session",
		StartTime:       now,
		Status:          StatusRunning, // Still running
		SessionDir:      sessionDir,
		ApplicationName: pathBuilder.ApplicationName(),
	}

	metadata := &Metadata{
		SessionID:             "running-session",
		StartTime:             now,
		Status:                StatusRunning, // Still running
		SessionDir:            sessionDir,
		Resumable:             true,                // Has flag
		CoordinatorSessionRef: "coord-ref-running", // Has ref
		ApplicationName:       pathBuilder.ApplicationName(),
	}

	createTestIndexedSession(t, pathBuilder, entry, metadata)

	// Also create a valid resumable session
	createResumableTestSession(t, pathBuilder, "completed-session", now.Add(-time.Hour))

	sessions, err := ListResumableSessions(pathBuilder)
	require.NoError(t, err)

	// Should only have the completed session (running is not resumable)
	require.Len(t, sessions, 1)
	require.Equal(t, "completed-session", sessions[0].ID)
}

// --- ListAllApplications Tests ---

func TestListAllApplications_MultipleApps(t *testing.T) {
	baseDir := t.TempDir()

	// Create multiple application directories with sessions.json
	for _, appName := range []string{"app-alpha", "app-beta", "app-gamma"} {
		appDir := filepath.Join(baseDir, appName)
		require.NoError(t, os.MkdirAll(appDir, 0750))
		indexPath := filepath.Join(appDir, "sessions.json")
		require.NoError(t, os.WriteFile(indexPath, []byte(`{"version":1,"sessions":[]}`), 0600))
	}

	apps, err := ListAllApplications(baseDir)
	require.NoError(t, err)
	require.Len(t, apps, 3)
	require.Equal(t, []string{"app-alpha", "app-beta", "app-gamma"}, apps)
}

func TestListAllApplications_EmptyDir(t *testing.T) {
	baseDir := t.TempDir()

	// Empty directory - no apps
	apps, err := ListAllApplications(baseDir)
	require.NoError(t, err)
	require.Empty(t, apps)
	require.NotNil(t, apps) // Should be empty slice, not nil
}

func TestListAllApplications_NoIndex(t *testing.T) {
	baseDir := t.TempDir()

	// Create directories, some with sessions.json and some without
	appWithIndex := filepath.Join(baseDir, "app-with-index")
	require.NoError(t, os.MkdirAll(appWithIndex, 0750))
	require.NoError(t, os.WriteFile(filepath.Join(appWithIndex, "sessions.json"), []byte(`{"version":1}`), 0600))

	appWithoutIndex := filepath.Join(baseDir, "app-without-index")
	require.NoError(t, os.MkdirAll(appWithoutIndex, 0750))
	// No sessions.json file here

	apps, err := ListAllApplications(baseDir)
	require.NoError(t, err)

	// Should only include app with sessions.json
	require.Len(t, apps, 1)
	require.Equal(t, "app-with-index", apps[0])
}

func TestListAllApplications_SortedAlphabetically(t *testing.T) {
	baseDir := t.TempDir()

	// Create apps in non-alphabetical order
	for _, appName := range []string{"zulu", "alpha", "mike", "bravo"} {
		appDir := filepath.Join(baseDir, appName)
		require.NoError(t, os.MkdirAll(appDir, 0750))
		require.NoError(t, os.WriteFile(filepath.Join(appDir, "sessions.json"), []byte(`{}`), 0600))
	}

	apps, err := ListAllApplications(baseDir)
	require.NoError(t, err)

	// Should be sorted alphabetically
	require.Equal(t, []string{"alpha", "bravo", "mike", "zulu"}, apps)
}

func TestListAllApplications_BaseDirNotExist(t *testing.T) {
	// Non-existent directory should return empty slice, not error
	apps, err := ListAllApplications("/non/existent/path/that/does/not/exist")
	require.NoError(t, err)
	require.Empty(t, apps)
	require.NotNil(t, apps)
}

func TestListAllApplications_SkipsFiles(t *testing.T) {
	baseDir := t.TempDir()

	// Create a valid app directory
	appDir := filepath.Join(baseDir, "valid-app")
	require.NoError(t, os.MkdirAll(appDir, 0750))
	require.NoError(t, os.WriteFile(filepath.Join(appDir, "sessions.json"), []byte(`{}`), 0600))

	// Create a file (not a directory) at baseDir level
	require.NoError(t, os.WriteFile(filepath.Join(baseDir, "not-a-dir"), []byte("file"), 0600))

	apps, err := ListAllApplications(baseDir)
	require.NoError(t, err)

	// Should only include directories
	require.Len(t, apps, 1)
	require.Equal(t, "valid-app", apps[0])
}

// --- ListGlobalResumableSessions Tests ---

func TestListGlobalResumableSessions_CrossApp(t *testing.T) {
	baseDir := t.TempDir()
	now := time.Now().UTC().Truncate(time.Second)

	// Create sessions in two different apps
	app1 := NewSessionPathBuilder(baseDir, "app-one")
	app2 := NewSessionPathBuilder(baseDir, "app-two")

	createResumableTestSession(t, app1, "session-1a", now.Add(-4*time.Hour))
	createResumableTestSession(t, app1, "session-1b", now.Add(-2*time.Hour))
	createResumableTestSession(t, app2, "session-2a", now.Add(-3*time.Hour))
	createResumableTestSession(t, app2, "session-2b", now.Add(-time.Hour))

	sessions, err := ListGlobalResumableSessions(baseDir)
	require.NoError(t, err)

	// Should have all 4 sessions from both apps
	require.Len(t, sessions, 4)

	// Verify sessions from both apps are present
	appNames := make(map[string]int)
	for _, s := range sessions {
		appNames[s.ApplicationName]++
	}
	require.Equal(t, 2, appNames["app-one"])
	require.Equal(t, 2, appNames["app-two"])
}

func TestListGlobalResumableSessions_SortByRecent(t *testing.T) {
	baseDir := t.TempDir()
	now := time.Now().UTC().Truncate(time.Second)

	// Create sessions across apps with interleaved times
	app1 := NewSessionPathBuilder(baseDir, "app-one")
	app2 := NewSessionPathBuilder(baseDir, "app-two")

	// app-one has oldest and newest
	createResumableTestSession(t, app1, "oldest", now.Add(-4*time.Hour))
	createResumableTestSession(t, app1, "newest", now)

	// app-two has middle times
	createResumableTestSession(t, app2, "middle-old", now.Add(-3*time.Hour))
	createResumableTestSession(t, app2, "middle-new", now.Add(-time.Hour))

	sessions, err := ListGlobalResumableSessions(baseDir)
	require.NoError(t, err)
	require.Len(t, sessions, 4)

	// Should be sorted by StartTime descending globally (not per-app)
	require.Equal(t, "newest", sessions[0].ID)     // now
	require.Equal(t, "middle-new", sessions[1].ID) // now - 1h
	require.Equal(t, "middle-old", sessions[2].ID) // now - 3h
	require.Equal(t, "oldest", sessions[3].ID)     // now - 4h

	// Verify times are actually descending
	for i := 0; i < len(sessions)-1; i++ {
		require.True(t, sessions[i].StartTime.After(sessions[i+1].StartTime),
			"session %d (%s) should be after session %d (%s)",
			i, sessions[i].ID, i+1, sessions[i+1].ID)
	}
}

func TestListGlobalResumableSessions_SkipsFailedApps(t *testing.T) {
	baseDir := t.TempDir()
	now := time.Now().UTC().Truncate(time.Second)

	// Create a valid app with sessions
	validApp := NewSessionPathBuilder(baseDir, "valid-app")
	createResumableTestSession(t, validApp, "valid-session", now)

	// Create a broken app with corrupt sessions.json
	brokenAppDir := filepath.Join(baseDir, "broken-app")
	require.NoError(t, os.MkdirAll(brokenAppDir, 0750))
	require.NoError(t, os.WriteFile(filepath.Join(brokenAppDir, "sessions.json"), []byte("{ invalid json }"), 0600))

	sessions, err := ListGlobalResumableSessions(baseDir)
	require.NoError(t, err)

	// Should still have the valid session, broken app is skipped
	require.Len(t, sessions, 1)
	require.Equal(t, "valid-session", sessions[0].ID)
	require.Equal(t, "valid-app", sessions[0].ApplicationName)
}

func TestListGlobalResumableSessions_EmptyBase(t *testing.T) {
	baseDir := t.TempDir()

	// Empty base directory
	sessions, err := ListGlobalResumableSessions(baseDir)
	require.NoError(t, err)
	require.Empty(t, sessions)
	require.NotNil(t, sessions) // Should be empty slice, not nil
}

func TestListGlobalResumableSessions_BaseDirNotExist(t *testing.T) {
	// Non-existent directory should return empty slice, not error
	sessions, err := ListGlobalResumableSessions("/non/existent/path/that/does/not/exist")
	require.NoError(t, err)
	require.Empty(t, sessions)
	require.NotNil(t, sessions)
}

func TestListGlobalResumableSessions_OnlyResumable(t *testing.T) {
	baseDir := t.TempDir()
	now := time.Now().UTC().Truncate(time.Second)

	// Create mix of resumable and non-resumable across apps
	app1 := NewSessionPathBuilder(baseDir, "app-one")
	app2 := NewSessionPathBuilder(baseDir, "app-two")

	createResumableTestSession(t, app1, "resumable-1", now.Add(-2*time.Hour))
	createNonResumableTestSession(t, app1, "non-resumable-1", now.Add(-time.Hour))
	createResumableTestSession(t, app2, "resumable-2", now)
	createNonResumableTestSession(t, app2, "non-resumable-2", now.Add(-3*time.Hour))

	sessions, err := ListGlobalResumableSessions(baseDir)
	require.NoError(t, err)

	// Should only have resumable sessions
	require.Len(t, sessions, 2)
	for _, s := range sessions {
		require.True(t, s.Resumable, "session %s should be resumable", s.ID)
	}

	ids := []string{sessions[0].ID, sessions[1].ID}
	require.Contains(t, ids, "resumable-1")
	require.Contains(t, ids, "resumable-2")
}
