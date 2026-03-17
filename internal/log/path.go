package log

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	defaultStateSubdir = ".local/state"
	defaultProjectName = "unknown"
	defaultPathHash    = "00000000"
	logDirName         = "perles"
	logSubdirName      = "logs"
)

// DefaultDebugLogPath returns the centralized default runtime log path.
//
// Path shape:
//
//	{stateRoot}/perles/logs/{projectBase}-{shortHash}/{YYYY-MM-DD}-perles.log
//
// State root follows XDG conventions:
//  1. $XDG_STATE_HOME (if set)
//  2. ~/.local/state
//  3. . (safe fallback when home cannot be resolved)
func DefaultDebugLogPath() string {
	return buildDefaultDebugLogPath(time.Now(), os.Getenv, os.Getwd, os.UserHomeDir)
}

func buildDefaultDebugLogPath(
	now time.Time,
	getenv func(string) string,
	getwd func() (string, error),
	userHomeDir func() (string, error),
) string {
	stateRoot := resolveStateRoot(getenv, userHomeDir)
	projectDir := resolveProjectDir(getwd)
	filename := debugLogFilename(now)

	return filepath.Join(stateRoot, logDirName, logSubdirName, projectDir, filename)
}

func resolveStateRoot(getenv func(string) string, userHomeDir func() (string, error)) string {
	if xdg := strings.TrimSpace(getenv("XDG_STATE_HOME")); xdg != "" {
		return xdg
	}

	home, err := userHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return "."
	}

	return filepath.Join(home, defaultStateSubdir)
}

func resolveProjectDir(getwd func() (string, error)) string {
	wd, err := getwd()
	if err != nil || strings.TrimSpace(wd) == "" {
		return fmt.Sprintf("%s-%s", defaultProjectName, defaultPathHash)
	}

	normalized := normalizeWorkDir(wd)
	base := projectBaseName(normalized)
	hash := shortPathHash(normalized)

	return fmt.Sprintf("%s-%s", base, hash)
}

func normalizeWorkDir(wd string) string {
	abs, err := filepath.Abs(wd)
	if err != nil {
		return filepath.Clean(wd)
	}
	return filepath.Clean(abs)
}

func projectBaseName(normalizedPath string) string {
	base := filepath.Base(normalizedPath)
	if base == "." || base == string(filepath.Separator) || strings.TrimSpace(base) == "" {
		return defaultProjectName
	}
	return base
}

func shortPathHash(path string) string {
	if strings.TrimSpace(path) == "" {
		return defaultPathHash
	}
	sum := sha256.Sum256([]byte(path))
	return hex.EncodeToString(sum[:4])
}

func debugLogFilename(now time.Time) string {
	return now.Format("2006-01-02") + "-perles.log"
}
