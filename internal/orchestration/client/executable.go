// Package client provides shared infrastructure for headless AI CLI clients.
// This file implements ExecutableFinder, a utility for locating CLI executables
// with priority-ordered path checking and cross-platform support.
package client

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/zjrosen/perles/internal/log"
)

// ErrExecutableNotFound is returned when the executable cannot be located
// in known paths or PATH.
var ErrExecutableNotFound = errors.New("executable not found")

// FinderOption configures an ExecutableFinder.
type FinderOption func(*ExecutableFinder)

// ExecutableFinder locates CLI executables with priority-ordered path checking.
// It checks an optional environment variable override first, then known paths
// in order, and finally falls back to PATH lookup via exec.LookPath.
type ExecutableFinder struct {
	execName    string   // Base executable name: "claude", "gemini", etc.
	knownPaths  []string // Priority-ordered path templates
	envOverride string   // Environment variable to check first (e.g., "CLAUDE_PATH")
	goos        string   // Operating system (defaults to runtime.GOOS)

	// Function injection for testability (defaults to os/exec functions)
	statFn     func(string) (os.FileInfo, error)
	lookPathFn func(string) (string, error)
	userHomeFn func() (string, error)
}

// NewExecutableFinder creates a finder for the given executable name.
// Use options like WithKnownPaths and WithEnvOverride to configure behavior.
func NewExecutableFinder(execName string, opts ...FinderOption) *ExecutableFinder {
	f := &ExecutableFinder{
		execName:   execName,
		goos:       runtime.GOOS,
		statFn:     os.Stat,
		lookPathFn: exec.LookPath,
		userHomeFn: os.UserHomeDir,
	}
	for _, opt := range opts {
		opt(f)
	}
	return f
}

// WithKnownPaths sets the priority-ordered paths to check before PATH lookup.
// Paths support template variables:
//   - {name}: executable name (with .exe suffix on Windows)
//   - ~: user home directory
//   - $VAR or ${VAR}: Unix environment variable
//   - %VAR%: Windows environment variable
func WithKnownPaths(paths ...string) FinderOption {
	return func(f *ExecutableFinder) {
		f.knownPaths = paths
	}
}

// WithEnvOverride sets an environment variable to check before known paths.
// If the variable is set and points to an existing executable, it takes
// precedence over all other paths.
func WithEnvOverride(envVar string) FinderOption {
	return func(f *ExecutableFinder) {
		f.envOverride = envVar
	}
}

// Find locates the executable, checking in priority order:
//  1. Environment variable override (if WithEnvOverride was used)
//  2. Known paths (if WithKnownPaths was used)
//  3. PATH lookup via exec.LookPath
//
// Returns the absolute path to the executable or an error listing all paths checked.
func (f *ExecutableFinder) Find() (string, error) {
	var checkedPaths []string

	// 1. Check environment override first
	if f.envOverride != "" {
		if envPath := os.Getenv(f.envOverride); envPath != "" {
			checkedPaths = append(checkedPaths, envPath+" (from $"+f.envOverride+")")
			if f.isValidExecutable(envPath) {
				log.Debug(log.CatOrch, "Found executable via env override",
					"name", f.execName, "path", envPath, "envVar", f.envOverride)
				return envPath, nil
			}
		}
	}

	// 2. Check known paths in priority order
	for _, template := range f.knownPaths {
		path, err := f.expandPath(template)
		if err != nil {
			log.Debug(log.CatOrch, "Skipping path template expansion failure",
				"template", template, "error", err)
			continue
		}
		checkedPaths = append(checkedPaths, path)

		if f.isValidExecutable(path) {
			log.Debug(log.CatOrch, "Found executable in known path",
				"name", f.execName, "path", path)
			return path, nil
		}
	}

	// 3. Fall back to PATH lookup
	execName := f.platformExecName()
	path, err := f.lookPathFn(execName)
	if err == nil {
		log.Debug(log.CatOrch, "Found executable via PATH",
			"name", f.execName, "path", path)
		return path, nil
	}

	// Build informative error message
	pathDesc := "PATH"
	if len(checkedPaths) > 0 {
		pathDesc = strings.Join(checkedPaths, ", ") + ", PATH"
	}
	return "", fmt.Errorf("%w: %s not found in %s", ErrExecutableNotFound, f.execName, pathDesc)
}

// platformExecName returns the executable name with platform-appropriate suffix.
func (f *ExecutableFinder) platformExecName() string {
	if f.goos == "windows" {
		return f.execName + ".exe"
	}
	return f.execName
}

// expandPath expands template variables in a path.
func (f *ExecutableFinder) expandPath(template string) (string, error) {
	path := template

	// Replace {name} placeholder with platform-appropriate executable name
	path = strings.ReplaceAll(path, "{name}", f.platformExecName())

	// Expand ~ to home directory (only at path start)
	if strings.HasPrefix(path, "~") {
		home, err := f.userHomeFn()
		if err != nil {
			return "", fmt.Errorf("cannot expand ~: %w", err)
		}
		path = home + path[1:]
	}

	// Expand Windows %VAR% syntax
	if f.goos == "windows" {
		path = f.expandWindowsEnv(path)
	}

	// Expand Unix $VAR and ${VAR} syntax (os.ExpandEnv works on all platforms)
	path = os.ExpandEnv(path)

	return filepath.Clean(path), nil
}

// expandWindowsEnv expands Windows-style %VAR% environment variables.
func (f *ExecutableFinder) expandWindowsEnv(path string) string {
	re := regexp.MustCompile(`%([^%]+)%`)
	return re.ReplaceAllStringFunc(path, func(match string) string {
		varName := match[1 : len(match)-1]
		if val := os.Getenv(varName); val != "" {
			return val
		}
		return match // Keep original if not set
	})
}

// isValidExecutable checks if the path points to a valid executable file.
func (f *ExecutableFinder) isValidExecutable(path string) bool {
	info, err := f.statFn(path)
	if err != nil {
		return false
	}
	if info.IsDir() {
		return false
	}
	return f.isExecutable(info)
}

// isExecutable checks if a file has executable permissions.
// On Unix, checks execute bit. On Windows, checks .exe extension.
func (f *ExecutableFinder) isExecutable(info os.FileInfo) bool {
	if f.goos == "windows" {
		// On Windows, .exe extension is sufficient (no execute bits)
		return strings.HasSuffix(strings.ToLower(info.Name()), ".exe")
	}
	// On Unix, check execute bit for any user/group/other
	return info.Mode().Perm()&0111 != 0
}
