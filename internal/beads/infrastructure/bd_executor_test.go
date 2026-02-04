package infrastructure

import (
	"testing"

	"github.com/stretchr/testify/require"
	appbeads "github.com/zjrosen/perles/internal/beads/application"
)

// TestBDExecutor_ImplementsIssueExecutor verifies BDExecutor implements IssueExecutor.
func TestBDExecutor_ImplementsIssueExecutor(t *testing.T) {
	var _ appbeads.IssueExecutor = (*BDExecutor)(nil)
}

// TestBDExecutor_NewBDExecutor tests the constructor.
func TestBDExecutor_NewBDExecutor(t *testing.T) {
	workDir := "/some/work/dir"
	beadsDir := "/some/beads/dir"
	executor := NewBDExecutor(workDir, beadsDir)

	require.NotNil(t, executor, "NewBDExecutor returned nil")
	require.Equal(t, workDir, executor.workDir)
	require.Equal(t, beadsDir, executor.beadsDir)
}

// TestBDExecutor_UpdateTitle_MethodExists verifies UpdateTitle exists with correct signature.
// This is a compile-time check that ensures the method is implemented.
func TestBDExecutor_UpdateTitle_MethodExists(t *testing.T) {
	executor := NewBDExecutor("", "")

	// Verify the method exists and has the correct signature.
	// We call it with empty values - it will fail due to missing bd CLI,
	// but this confirms the method exists and compiles.
	var updateTitleFunc func(issueID, title string) error = executor.UpdateTitle

	require.NotNil(t, updateTitleFunc, "UpdateTitle method should exist")
}

// TestBDExecutor_UpdateDescription_MethodExists verifies UpdateDescription exists with correct signature.
func TestBDExecutor_UpdateDescription_MethodExists(t *testing.T) {
	executor := NewBDExecutor("", "")

	var updateDescFunc func(issueID, description string) error = executor.UpdateDescription

	require.NotNil(t, updateDescFunc, "UpdateDescription method should exist")
}

// TestBDExecutor_MethodSignatureConsistency verifies UpdateTitle has same signature as UpdateDescription.
func TestBDExecutor_MethodSignatureConsistency(t *testing.T) {
	executor := NewBDExecutor("", "")

	// Both methods should have signature: func(string, string) error
	var titleFunc func(string, string) error = executor.UpdateTitle
	var descFunc func(string, string) error = executor.UpdateDescription

	require.NotNil(t, titleFunc, "UpdateTitle should have signature func(string, string) error")
	require.NotNil(t, descFunc, "UpdateDescription should have signature func(string, string) error")
}
