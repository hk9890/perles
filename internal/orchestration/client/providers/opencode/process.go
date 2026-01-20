package opencode

import (
	"context"
	"fmt"

	"github.com/zjrosen/perles/internal/orchestration/client"
)

// defaultKnownPaths defines the priority-ordered paths to check for the opencode executable.
// These are checked before falling back to PATH lookup.
var defaultKnownPaths = []string{
	"~/.local/bin/{name}",      // Common Go binary location (go install)
	"/opt/homebrew/bin/{name}", // Apple Silicon Mac (Homebrew)
	"/usr/local/bin/{name}",    // Intel Mac / Linux
}

// Process represents a headless OpenCode CLI process.
// Process implements client.HeadlessProcess by embedding BaseProcess.
type Process struct {
	*client.BaseProcess
}

// ErrTimeout is returned when an OpenCode process exceeds its configured timeout.
var ErrTimeout = fmt.Errorf("opencode process timed out")

// Spawn creates and starts a new headless OpenCode process.
// Context is used for cancellation and timeout control.
func Spawn(ctx context.Context, cfg Config) (*Process, error) {
	return spawnProcess(ctx, cfg, false)
}

// Resume continues an existing OpenCode session using --session flag.
func Resume(ctx context.Context, sessionID string, cfg Config) (*Process, error) {
	cfg.SessionID = sessionID
	return spawnProcess(ctx, cfg, true)
}

// spawnProcess is the internal implementation for both Spawn and Resume.
// Uses SpawnBuilder for clean process lifecycle management.
func spawnProcess(ctx context.Context, cfg Config, isResume bool) (*Process, error) {
	// Find the opencode executable using ExecutableFinder
	execPath, err := client.NewExecutableFinder("opencode",
		client.WithKnownPaths(defaultKnownPaths...),
	).Find()
	if err != nil {
		return nil, err
	}

	args := buildArgs(cfg, isResume)

	// Build environment variables for MCP config
	// OPENCODE_CONFIG_CONTENT allows per-process MCP config without file conflicts
	var env []string
	if cfg.MCPConfig != "" {
		env = append(env, "OPENCODE_CONFIG_CONTENT="+cfg.MCPConfig)
	}
	// Append common environment variables (BEADS_DIR if set)
	env = append(env, client.BuildEnvVars(client.Config{BeadsDir: cfg.BeadsDir})...)

	base, err := client.NewSpawnBuilder(ctx).
		WithExecutable(execPath, args).
		WithWorkDir(cfg.WorkDir).
		WithSessionRef(cfg.SessionID).
		WithTimeout(cfg.Timeout).
		WithParser(NewParser()).
		WithEnv(env).
		WithProviderName("opencode").
		WithStderrCapture(true).
		Build()
	if err != nil {
		return nil, fmt.Errorf("opencode: %w", err)
	}

	return &Process{BaseProcess: base}, nil
}

// SessionID returns the session ID (may be empty until first event with sessionID is received).
// This is a convenience method that wraps SessionRef for backwards compatibility.
func (p *Process) SessionID() string {
	return p.SessionRef()
}

// Ensure Process implements client.HeadlessProcess at compile time.
var _ client.HeadlessProcess = (*Process)(nil)
