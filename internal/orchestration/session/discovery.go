// Package session provides session tracking for orchestration mode.
// discovery.go provides functions for discovering and listing sessions for session picker UI.
package session

import (
	"os"
	"path/filepath"
	"sort"
	"time"
)

// SessionSummary provides a lightweight view of a session for listing purposes.
// Contains just enough information to display in a session picker UI.
type SessionSummary struct {
	// ID is the unique session identifier (UUID).
	ID string

	// ApplicationName is the derived or configured name for the application.
	ApplicationName string

	// WorkDir is the project working directory where the session was initiated.
	WorkDir string

	// StartTime is when the session was created.
	StartTime time.Time

	// EndTime is when the session ended (zero if still running).
	EndTime time.Time

	// Status is the session's current or final status.
	Status Status

	// WorkerCount is the number of workers that participated in this session.
	WorkerCount int

	// Resumable indicates whether this session can be resumed.
	Resumable bool

	// SessionDir is the full path to the session directory.
	SessionDir string

	// CoordinatorSessionRef is the headless client session reference for resuming.
	CoordinatorSessionRef string
}

// ListResumableSessions returns all sessions that can be resumed for the given application.
// Sessions are sorted by StartTime descending (most recent first).
// Corrupt metadata in individual sessions is skipped gracefully without failing the entire listing.
func ListResumableSessions(pathBuilder *SessionPathBuilder) ([]SessionSummary, error) {
	return listSessions(pathBuilder, true)
}

// ListAllSessions returns all sessions (resumable or not) for the given application.
// Sessions are sorted by StartTime descending (most recent first).
// Useful for displaying session history.
// Corrupt metadata in individual sessions is skipped gracefully without failing the entire listing.
func ListAllSessions(pathBuilder *SessionPathBuilder) ([]SessionSummary, error) {
	return listSessions(pathBuilder, false)
}

// listSessions is the internal implementation that loads sessions from the application index.
// If filterResumable is true, only resumable sessions are returned.
func listSessions(pathBuilder *SessionPathBuilder, filterResumable bool) ([]SessionSummary, error) {
	// Load application index
	indexPath := pathBuilder.ApplicationIndexPath()
	appIndex, err := LoadApplicationIndex(indexPath)
	if err != nil {
		return nil, err
	}

	var summaries []SessionSummary
	for _, entry := range appIndex.Sessions {
		// Load metadata for each session to check resumability and get coordinator ref
		metadata, err := Load(entry.SessionDir)
		if err != nil {
			// Skip sessions with missing/corrupt metadata - graceful degradation
			continue
		}

		resumable := IsResumable(metadata)

		// If filtering for resumable only, skip non-resumable sessions
		if filterResumable && !resumable {
			continue
		}

		summaries = append(summaries, SessionSummary{
			ID:                    entry.ID,
			ApplicationName:       entry.ApplicationName,
			WorkDir:               entry.WorkDir,
			StartTime:             entry.StartTime,
			EndTime:               entry.EndTime,
			Status:                entry.Status,
			WorkerCount:           entry.WorkerCount,
			Resumable:             resumable,
			SessionDir:            entry.SessionDir,
			CoordinatorSessionRef: metadata.CoordinatorSessionRef,
		})
	}

	// Sort by StartTime descending (most recent first)
	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].StartTime.After(summaries[j].StartTime)
	})

	// Ensure we return an empty slice, not nil
	if summaries == nil {
		summaries = []SessionSummary{}
	}

	return summaries, nil
}

// GetRecentSessions returns the N most recent sessions for the application.
// Sessions are sorted by StartTime descending (most recent first).
func GetRecentSessions(pathBuilder *SessionPathBuilder, limit int) ([]SessionSummary, error) {
	all, err := ListAllSessions(pathBuilder)
	if err != nil {
		return nil, err
	}

	if len(all) <= limit {
		return all, nil
	}
	return all[:limit], nil
}

// GetLatestResumableSession returns the most recent resumable session, if any.
// Returns nil if no resumable sessions exist.
func GetLatestResumableSession(pathBuilder *SessionPathBuilder) (*SessionSummary, error) {
	resumable, err := ListResumableSessions(pathBuilder)
	if err != nil {
		return nil, err
	}

	if len(resumable) == 0 {
		return nil, nil
	}

	// First element is the most recent (sorted by StartTime desc)
	return &resumable[0], nil
}

// FindSessionByID searches for a session by ID across all sessions for the application.
// Returns nil if no session with the given ID is found.
func FindSessionByID(pathBuilder *SessionPathBuilder, sessionID string) (*SessionSummary, error) {
	all, err := ListAllSessions(pathBuilder)
	if err != nil {
		return nil, err
	}

	for i := range all {
		if all[i].ID == sessionID {
			return &all[i], nil
		}
	}

	return nil, nil // Not found
}

// ListAllApplications scans baseDir for subdirectories that contain a sessions.json file.
// Returns a sorted list of application names (directory names).
// If baseDir doesn't exist or is empty, returns an empty slice (not an error).
func ListAllApplications(baseDir string) ([]string, error) {
	entries, err := os.ReadDir(baseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}

	var apps []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		// Check if this directory contains sessions.json
		indexPath := filepath.Join(baseDir, entry.Name(), "sessions.json")
		if _, err := os.Stat(indexPath); err == nil {
			apps = append(apps, entry.Name())
		}
	}

	// Sort alphabetically
	sort.Strings(apps)

	// Ensure we return empty slice, not nil
	if apps == nil {
		apps = []string{}
	}

	return apps, nil
}

// ListGlobalResumableSessions returns all resumable sessions across all applications.
// Sessions are aggregated from all applications found in baseDir and sorted by
// StartTime descending (most recent first) globally.
// If baseDir doesn't exist or is empty, returns an empty slice (not an error).
// Individual application errors are skipped gracefully - one app failing doesn't fail all.
func ListGlobalResumableSessions(baseDir string) ([]SessionSummary, error) {
	apps, err := ListAllApplications(baseDir)
	if err != nil {
		return nil, err
	}

	var allSessions []SessionSummary
	for _, appName := range apps {
		pathBuilder := NewSessionPathBuilder(baseDir, appName)
		sessions, err := ListResumableSessions(pathBuilder)
		if err != nil {
			// Skip apps with errors - graceful degradation
			continue
		}
		allSessions = append(allSessions, sessions...)
	}

	// Sort all sessions globally by StartTime descending (most recent first)
	sort.Slice(allSessions, func(i, j int) bool {
		return allSessions[i].StartTime.After(allSessions[j].StartTime)
	})

	// Ensure we return empty slice, not nil
	if allSessions == nil {
		allSessions = []SessionSummary{}
	}

	return allSessions, nil
}
