package controlplane

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

// === Sentinel Error Tests ===

func TestErrUncommittedChanges_IsSentinelError(t *testing.T) {
	// Verify error is not nil
	require.NotNil(t, ErrUncommittedChanges)

	// Verify errors.Is works for identity
	require.True(t, errors.Is(ErrUncommittedChanges, ErrUncommittedChanges))

	// Verify errors.Is works when wrapped
	wrapped := fmt.Errorf("failed to stop workflow: %w", ErrUncommittedChanges)
	require.True(t, errors.Is(wrapped, ErrUncommittedChanges))

	// Verify error message
	require.Equal(t, "worktree has uncommitted changes", ErrUncommittedChanges.Error())
}
