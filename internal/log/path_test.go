package log

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestDefaultDebugLogPath_XDGStateHomePreferred(t *testing.T) {
	now := time.Date(2026, 3, 12, 9, 30, 0, 0, time.UTC)

	path := buildDefaultDebugLogPath(
		now,
		func(key string) string {
			if key == "XDG_STATE_HOME" {
				return "/state/xdg"
			}
			return ""
		},
		func() (string, error) { return "/home/alice/src/perles", nil },
		func() (string, error) { return "/home/alice", nil },
	)

	require.Equal(t, filepath.Join("/state/xdg", "perles", "logs", "perles-32de9822", "2026-03-12-perles.log"), path)
}

func TestDefaultDebugLogPath_FallsBackToHomeState(t *testing.T) {
	now := time.Date(2026, 3, 12, 0, 0, 0, 0, time.UTC)

	path := buildDefaultDebugLogPath(
		now,
		func(string) string { return "" },
		func() (string, error) { return "/workspace/repo", nil },
		func() (string, error) { return "/home/alice", nil },
	)

	require.Equal(t, filepath.Join("/home/alice", ".local/state", "perles", "logs", "repo-b996db19", "2026-03-12-perles.log"), path)
}

func TestDefaultDebugLogPath_FallsBackSafelyWhenHomeUnavailable(t *testing.T) {
	now := time.Date(2026, 3, 12, 0, 0, 0, 0, time.UTC)

	path := buildDefaultDebugLogPath(
		now,
		func(string) string { return "" },
		func() (string, error) { return "", errors.New("cwd unavailable") },
		func() (string, error) { return "", errors.New("home unavailable") },
	)

	require.Equal(t, filepath.Join(".", "perles", "logs", "unknown-00000000", "2026-03-12-perles.log"), path)
}

func TestShortPathHash_Stable(t *testing.T) {
	require.Equal(t, "b26a7b07", shortPathHash("/tmp/foo/bar"))
	require.Equal(t, "b26a7b07", shortPathHash("/tmp/foo/bar"))
	require.Equal(t, "00000000", shortPathHash(""))
}

func TestDebugLogFilename_DateBased(t *testing.T) {
	now := time.Date(2026, 3, 12, 23, 59, 59, 0, time.UTC)
	require.Equal(t, "2026-03-12-perles.log", debugLogFilename(now))
}
