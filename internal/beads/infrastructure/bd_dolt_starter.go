package infrastructure

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

const delegatedDoltStartTimeout = 15 * time.Second

type beadsCommandRunner func(ctx context.Context, command string, args []string, env []string) (stdout string, stderr string, err error)

type bdDoltStarter struct {
	beadsDir string
	timeout  time.Duration

	lookPathFunc func(file string) (string, error)
	runFunc      beadsCommandRunner
}

func newBDDoltStarter(beadsDir string) *bdDoltStarter {
	return &bdDoltStarter{
		beadsDir:     beadsDir,
		timeout:      delegatedDoltStartTimeout,
		lookPathFunc: exec.LookPath,
		runFunc:      runBeadsCommand,
	}
}

func (s *bdDoltStarter) Start(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}

	bdPath, err := s.lookPathFunc("bd")
	if err != nil {
		return fmt.Errorf("delegated startup requires 'bd' CLI in PATH; install beads CLI and retry: %w", err)
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	stdout, stderr, err := s.runFunc(timeoutCtx, bdPath, []string{"dolt", "start"}, beadsCommandEnv(s.beadsDir))
	if err == nil {
		return nil
	}

	combinedOutput := formatBeadsCommandOutput(stdout, stderr)
	if timeoutCtx.Err() == context.DeadlineExceeded {
		if combinedOutput != "" {
			return fmt.Errorf("delegated 'bd dolt start' timed out after %s: %s", s.timeout, combinedOutput)
		}
		return fmt.Errorf("delegated 'bd dolt start' timed out after %s", s.timeout)
	}

	if combinedOutput != "" {
		return fmt.Errorf("delegated 'bd dolt start' failed: %s", combinedOutput)
	}

	return fmt.Errorf("delegated 'bd dolt start' failed: %w", err)
}

func beadsCommandEnv(beadsDir string) []string {
	if beadsDir == "" {
		return os.Environ()
	}

	return append(os.Environ(), "BEADS_DIR="+beadsDir)
}

func formatBeadsCommandOutput(stdout, stderr string) string {
	parts := make([]string, 0, 2)
	if trimmed := strings.TrimSpace(stdout); trimmed != "" {
		parts = append(parts, "stdout: "+trimmed)
	}
	if trimmed := strings.TrimSpace(stderr); trimmed != "" {
		parts = append(parts, "stderr: "+trimmed)
	}

	return strings.Join(parts, " | ")
}

func runBeadsCommand(ctx context.Context, command string, args []string, env []string) (string, string, error) {
	//nolint:gosec // G204: command and args come from controlled startup flow
	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Env = env

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	return strings.TrimSpace(stdout.String()), strings.TrimSpace(stderr.String()), err
}
