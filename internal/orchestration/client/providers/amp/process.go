package amp

import (
	"context"
	"fmt"

	"github.com/zjrosen/perles/internal/orchestration/client"
)

// Process represents a headless Amp process.
// Process implements client.HeadlessProcess by embedding BaseProcess.
type Process struct {
	*client.BaseProcess
}

// ErrTimeout is returned when an Amp process exceeds its configured timeout.
var ErrTimeout = fmt.Errorf("amp process timed out")

// parser is the shared Amp event parser instance.
var parser = NewParser()

// Spawn creates and starts a new headless Amp process.
// Context is used for cancellation and timeout control.
func Spawn(ctx context.Context, cfg Config) (*Process, error) {
	return spawnProcess(ctx, cfg, false)
}

// Resume continues an existing Amp thread.
func Resume(ctx context.Context, threadID string, cfg Config) (*Process, error) {
	cfg.ThreadID = threadID
	return spawnProcess(ctx, cfg, true)
}

// spawnProcess is the internal implementation for both Spawn and Resume.
// Uses SpawnBuilder for clean process lifecycle management.
func spawnProcess(ctx context.Context, cfg Config, isResume bool) (*Process, error) {
	// Find executable via PATH only (no known paths to maintain backwards compatibility)
	execPath, err := client.NewExecutableFinder("amp").Find()
	if err != nil {
		return nil, err
	}

	args := buildArgs(cfg, isResume)

	// Build environment variables (BEADS_DIR if set)
	env := client.BuildEnvVars(client.Config{BeadsDir: cfg.BeadsDir})

	base, err := client.NewSpawnBuilder(ctx).
		WithExecutable(execPath, args).
		WithWorkDir(cfg.WorkDir).
		WithSessionRef(cfg.ThreadID).
		WithTimeout(cfg.Timeout).
		WithParser(parser).
		WithStderrCapture(false). // Amp logs but doesn't capture stderr
		WithProviderName("amp").
		WithEnv(env).
		Build()
	if err != nil {
		return nil, fmt.Errorf("amp: %w", err)
	}

	return &Process{BaseProcess: base}, nil
}

// ThreadID returns the thread ID (Amp's equivalent of session ID).
func (p *Process) ThreadID() string {
	return p.SessionRef()
}

// Ensure Process implements client.HeadlessProcess at compile time.
var _ client.HeadlessProcess = (*Process)(nil)
