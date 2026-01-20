package paths

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResolveBeadsDir_ProjectDir(t *testing.T) {
	// Regular project directory should have .beads appended
	result := ResolveBeadsDir(filepath.FromSlash("/path/to/project"))
	require.Equal(t, filepath.FromSlash("/path/to/project/.beads"), result)
}

func TestResolveBeadsDir_BeadsDir(t *testing.T) {
	// .beads suffix should be preserved
	result := ResolveBeadsDir(filepath.FromSlash("/path/to/project/.beads"))
	require.Equal(t, filepath.FromSlash("/path/to/project/.beads"), result)
}

func TestResolveBeadsDir_BeadsDirWithTrailingSlash(t *testing.T) {
	// .beads/ with trailing slash should be normalized
	result := ResolveBeadsDir(filepath.FromSlash("/path/to/project/.beads/"))
	require.Equal(t, filepath.FromSlash("/path/to/project/.beads"), result)
}

func TestResolveBeadsDir_RelativeBeads(t *testing.T) {
	// Relative .beads should stay as .beads
	result := ResolveBeadsDir(".beads")
	require.Equal(t, ".beads", result)
}

func TestResolveBeadsDir_EmptyString(t *testing.T) {
	// Empty string should return "./.beads"
	result := ResolveBeadsDir("")
	require.Equal(t, ".beads", result)
}

func TestResolveBeadsDir_CurrentDir(t *testing.T) {
	// Current directory should append .beads
	result := ResolveBeadsDir(".")
	require.Equal(t, ".beads", result)
}

func TestResolveBeadsDir_TableDriven(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{"absolute project path", "/home/user/project", "/home/user/project/.beads"},
		{"absolute with .beads", "/home/user/project/.beads", "/home/user/project/.beads"},
		{"absolute with trailing slash", "/home/user/project/.beads/", "/home/user/project/.beads"},
		{"relative .beads", ".beads", ".beads"},
		{"empty string", "", ".beads"},
		{"relative project", "./my-project", "my-project/.beads"},
		{"relative with .beads", "./my-project/.beads", "my-project/.beads"},
		{"nested path", "/a/b/c/d", "/a/b/c/d/.beads"},
		{"nested with .beads", "/a/b/c/.beads", "/a/b/c/.beads"},
		{"single dir", "project", "project/.beads"},
		{"current dir", ".", ".beads"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			input := filepath.FromSlash(tc.input)
			expected := filepath.FromSlash(tc.expected)
			result := ResolveBeadsDir(input)
			require.Equal(t, expected, result)
		})
	}
}

func TestResolveBeadsDir_FollowsRedirect(t *testing.T) {
	// Create a temp directory structure with redirect
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "project")
	beadsDir := filepath.Join(projectDir, ".beads")
	targetDir := filepath.Join(tmpDir, "actual-beads")

	require.NoError(t, os.MkdirAll(beadsDir, 0755))
	require.NoError(t, os.MkdirAll(targetDir, 0755))

	// Create redirect file pointing to actual location (relative path)
	redirectPath := filepath.Join(beadsDir, "redirect")
	relPath, err := filepath.Rel(beadsDir, targetDir)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(redirectPath, []byte(relPath), 0644))

	// ResolveBeadsDir should follow the redirect
	result := ResolveBeadsDir(projectDir)
	require.Equal(t, targetDir, result)
}

func TestResolveBeadsDir_NoRedirect(t *testing.T) {
	// Create a temp directory structure without redirect
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "project")
	beadsDir := filepath.Join(projectDir, ".beads")

	require.NoError(t, os.MkdirAll(beadsDir, 0755))

	// ResolveBeadsDir should return the .beads dir directly
	result := ResolveBeadsDir(projectDir)
	require.Equal(t, beadsDir, result)
}

func TestResolveBeadsDir_EmptyRedirect(t *testing.T) {
	// Create a temp directory structure with empty redirect file
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "project")
	beadsDir := filepath.Join(projectDir, ".beads")

	require.NoError(t, os.MkdirAll(beadsDir, 0755))

	// Create empty redirect file
	redirectPath := filepath.Join(beadsDir, "redirect")
	require.NoError(t, os.WriteFile(redirectPath, []byte(""), 0644))

	// ResolveBeadsDir should return the .beads dir (empty redirect is ignored)
	result := ResolveBeadsDir(projectDir)
	require.Equal(t, beadsDir, result)
}
