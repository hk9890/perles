package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/hk9890/perles/internal/app"
	infrabeads "github.com/hk9890/perles/internal/beads/infrastructure"
	"github.com/hk9890/perles/internal/config"
	"github.com/hk9890/perles/internal/keys"

	"github.com/stretchr/testify/require"
)

type testModelNoClose struct{}

func (m testModelNoClose) Init() tea.Cmd                           { return nil }
func (m testModelNoClose) Update(msg tea.Msg) (tea.Model, tea.Cmd) { return m, nil }
func (m testModelNoClose) View() string                            { return "" }

type testModelWithClose struct {
	closed bool
	err    error
}

func (m *testModelWithClose) Init() tea.Cmd                           { return nil }
func (m *testModelWithClose) Update(msg tea.Msg) (tea.Model, tea.Cmd) { return m, nil }
func (m *testModelWithClose) View() string                            { return "" }
func (m *testModelWithClose) Close() error {
	m.closed = true
	return m.err
}

// TestNoBeadsDirectory_BeadsClientFails verifies that appbeads.NewDoltClient returns
// an error when there's no .beads directory. This is the condition that triggers
// the nobeads empty state view.
func TestNoBeadsDirectory_BeadsClientFails(t *testing.T) {
	// Create temp directory without .beads
	tmpDir, err := os.MkdirTemp("", "perles-test-nobeads-*")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })

	// Verify no .beads directory exists
	beadsPath := filepath.Join(tmpDir, ".beads")
	_, err = os.Stat(beadsPath)
	require.True(t, os.IsNotExist(err), "expected .beads to not exist")

	// Verify NewDoltClient fails for this directory
	_, err = infrabeads.NewDoltClient(tmpDir)
	require.Error(t, err, "expected NewDoltClient to fail without .beads directory")
}

// TestNoBeadsDirectory_WithBeadsFailsIfServerUnavailable verifies NewDoltClient
// attempts to connect to the configured Dolt SQL endpoint and fails when unavailable.
func TestNoBeadsDirectory_WithBeadsFailsIfServerUnavailable(t *testing.T) {
	tmpDir := t.TempDir()
	beadsPath := filepath.Join(tmpDir, ".beads")
	require.NoError(t, os.MkdirAll(filepath.Join(beadsPath, "dolt"), 0755))

	require.NoError(t, os.WriteFile(filepath.Join(beadsPath, "metadata.json"), []byte(`{"backend":"dolt","dolt_mode":"server","dolt_database":"perles"}`), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(beadsPath, "dolt", "config.yaml"), []byte("listener:\n  host: 127.0.0.1\n  port: 1\n"), 0644))

	_, err := infrabeads.NewDoltClient(beadsPath)
	require.Error(t, err)
}

func TestClassifyStartupBehavior(t *testing.T) {
	t.Run("nil error proceeds", func(t *testing.T) {
		behavior := classifyStartupBehavior(nil)
		require.Equal(t, startupBehaviorProceed, behavior)
	})

	t.Run("no beads error shows no-beads mode", func(t *testing.T) {
		err := &infrabeads.StartupError{
			Kind: infrabeads.StartupErrorKindNoBeads,
			Err:  fmt.Errorf("missing metadata"),
		}

		behavior := classifyStartupBehavior(err)
		require.Equal(t, startupBehaviorNoBeadsMode, behavior)
	})

	t.Run("server startup error returns actionable error", func(t *testing.T) {
		err := &infrabeads.StartupError{
			Kind: infrabeads.StartupErrorKindServerStartup,
			Err:  fmt.Errorf("connection refused"),
		}

		behavior := classifyStartupBehavior(err)
		require.Equal(t, startupBehaviorReturnError, behavior)
	})

	t.Run("non startup errors return actionable error", func(t *testing.T) {
		behavior := classifyStartupBehavior(fmt.Errorf("unexpected failure"))
		require.Equal(t, startupBehaviorReturnError, behavior)
	})
}

// ============================================================================
// Keybinding Startup Integration Tests
// ============================================================================

// TestStartup_ValidKeybindings verifies that validation passes and ApplyConfig
// is called for valid keybinding configuration.
func TestStartup_ValidKeybindings(t *testing.T) {
	kb := config.KeybindingsConfig{
		Search:    "ctrl+k",
		Dashboard: "ctrl+d",
		Editor:    "ctrl+shift+e",
	}

	// Validation should pass
	err := config.ValidateKeybindings(kb)
	require.NoError(t, err, "valid keybindings should pass validation")

	// ApplyConfig with these keys should work (tested via keys package)
	keys.ResetForTesting()
	defer keys.ResetForTesting()

	searchKey := kb.Search
	dashboardKey := kb.Dashboard
	keys.ApplyConfig(searchKey, dashboardKey, "ctrl+q", "ctrl+shift+e")

	// Verify keys were applied
	require.Equal(t, []string{"ctrl+k"}, keys.Kanban.SwitchMode.Keys())
	require.Equal(t, []string{"ctrl+d"}, keys.Kanban.Dashboard.Keys())
	require.Equal(t, []string{"ctrl+shift+e"}, keys.Component.Editor.Keys())
}

// TestStartup_InvalidKeybindings verifies that invalid keybindings cause
// validation failure with a clear error message.
func TestStartup_InvalidKeybindings(t *testing.T) {
	tests := []struct {
		name        string
		kb          config.KeybindingsConfig
		errContains string
	}{
		{
			name:        "invalid format - typo in ctrl",
			kb:          config.KeybindingsConfig{Search: "crtl+k"},
			errContains: "invalid key format",
		},
		{
			name:        "reserved key - q",
			kb:          config.KeybindingsConfig{Dashboard: "q"},
			errContains: "reserved",
		},
		{
			name:        "reserved key - enter",
			kb:          config.KeybindingsConfig{Search: "enter"},
			errContains: "reserved",
		},
		{
			name:        "duplicate keys",
			kb:          config.KeybindingsConfig{Search: "ctrl+k", Dashboard: "ctrl+k"},
			errContains: "same key",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := config.ValidateKeybindings(tt.kb)
			require.Error(t, err, "invalid keybindings should fail validation")
			require.Contains(t, err.Error(), tt.errContains,
				"error message should contain '%s'", tt.errContains)
		})
	}
}

// TestStartup_NoKeybindings verifies that empty keybindings configuration
// uses default values (ctrl+space and ctrl+o).
func TestStartup_NoKeybindings(t *testing.T) {
	kb := config.KeybindingsConfig{
		Search:    "", // Empty
		Dashboard: "", // Empty
		Editor:    "", // Empty
	}

	// Validation should pass for empty values
	err := config.ValidateKeybindings(kb)
	require.NoError(t, err, "empty keybindings should pass validation")

	// Simulate startup logic: empty strings get defaults
	keys.ResetForTesting()
	defer keys.ResetForTesting()

	searchKey := kb.Search
	if searchKey == "" {
		searchKey = "ctrl+space" // Default
	}
	dashboardKey := kb.Dashboard
	if dashboardKey == "" {
		dashboardKey = "ctrl+o" // Default
	}
	editorKey := kb.Editor
	if editorKey == "" {
		editorKey = "ctrl+shift+e" // Default
	}
	keys.ApplyConfig(searchKey, dashboardKey, "ctrl+q", editorKey)

	// Verify defaults were applied
	// ctrl+space translates to ctrl+@ for terminal
	require.Equal(t, []string{"ctrl+@"}, keys.Kanban.SwitchMode.Keys(),
		"default search key should be ctrl+@ (ctrl+space)")
	require.Equal(t, []string{"ctrl+o"}, keys.Kanban.Dashboard.Keys(),
		"default dashboard key should be ctrl+o")
	require.Equal(t, []string{"ctrl+shift+e"}, keys.Component.Editor.Keys(),
		"default editor key should be ctrl+shift+e")
}

// TestStartup_PartialKeybindings verifies that specifying only one keybinding
// uses the default for the other.
func TestStartup_PartialKeybindings(t *testing.T) {
	t.Run("only search specified", func(t *testing.T) {
		kb := config.KeybindingsConfig{
			Search:    "ctrl+k",
			Dashboard: "", // Use default
			Editor:    "", // Use default
		}

		// Validation should pass
		err := config.ValidateKeybindings(kb)
		require.NoError(t, err, "partial keybindings should pass validation")

		// Simulate startup logic
		keys.ResetForTesting()
		defer keys.ResetForTesting()

		searchKey := kb.Search
		if searchKey == "" {
			searchKey = "ctrl+space"
		}
		dashboardKey := kb.Dashboard
		if dashboardKey == "" {
			dashboardKey = "ctrl+o" // Default
		}
		editorKey := kb.Editor
		if editorKey == "" {
			editorKey = "ctrl+shift+e"
		}
		keys.ApplyConfig(searchKey, dashboardKey, "ctrl+q", editorKey)

		// Verify custom search and default dashboard
		require.Equal(t, []string{"ctrl+k"}, keys.Kanban.SwitchMode.Keys(),
			"search key should be ctrl+k")
		require.Equal(t, []string{"ctrl+o"}, keys.Kanban.Dashboard.Keys(),
			"dashboard key should default to ctrl+o")
		require.Equal(t, []string{"ctrl+shift+e"}, keys.Component.Editor.Keys(),
			"editor key should default to ctrl+shift+e")
	})

	t.Run("only dashboard specified", func(t *testing.T) {
		kb := config.KeybindingsConfig{
			Search:    "", // Use default
			Dashboard: "ctrl+d",
			Editor:    "ctrl+shift+f",
		}

		// Validation should pass
		err := config.ValidateKeybindings(kb)
		require.NoError(t, err, "partial keybindings should pass validation")

		// Simulate startup logic
		keys.ResetForTesting()
		defer keys.ResetForTesting()

		searchKey := kb.Search
		if searchKey == "" {
			searchKey = "ctrl+space" // Default
		}
		dashboardKey := kb.Dashboard
		if dashboardKey == "" {
			dashboardKey = "ctrl+o"
		}
		editorKey := kb.Editor
		if editorKey == "" {
			editorKey = "ctrl+shift+e"
		}
		keys.ApplyConfig(searchKey, dashboardKey, "ctrl+q", editorKey)

		// Verify default search and custom dashboard
		require.Equal(t, []string{"ctrl+@"}, keys.Kanban.SwitchMode.Keys(),
			"search key should default to ctrl+@ (ctrl+space)")
		require.Equal(t, []string{"ctrl+d"}, keys.Kanban.Dashboard.Keys(),
			"dashboard key should be ctrl+d")
		require.Equal(t, []string{"ctrl+shift+f"}, keys.Component.Editor.Keys(),
			"editor key should be ctrl+shift+f")
	})
}

func TestCleanupFinalModel(t *testing.T) {
	t.Run("nil final model returns run error", func(t *testing.T) {
		runErr := fmt.Errorf("run failed")
		err := cleanupFinalModel(nil, runErr, false)
		require.ErrorIs(t, err, runErr)
	})

	t.Run("non closable model keeps run error", func(t *testing.T) {
		runErr := fmt.Errorf("run failed")
		err := cleanupFinalModel(testModelNoClose{}, runErr, true)
		require.ErrorIs(t, err, runErr)
	})

	t.Run("closable pointer model is closed", func(t *testing.T) {
		m := &testModelWithClose{}
		err := cleanupFinalModel(m, nil, false)
		require.NoError(t, err)
		require.True(t, m.closed, "expected Close() to be called")
	})

	t.Run("app.Model value final model is closed", func(t *testing.T) {
		oldCloseAppModel := closeAppModel
		t.Cleanup(func() { closeAppModel = oldCloseAppModel })

		called := 0
		closeAppModel = func(m *app.Model) error {
			called++
			require.NotNil(t, m)
			return nil
		}

		finalModel := app.Model{}
		err := cleanupFinalModel(finalModel, nil, false)
		require.NoError(t, err)
		require.Equal(t, 1, called, "expected app.Model Close() path to be called")
	})

	t.Run("*app.Model pointer final model is closed", func(t *testing.T) {
		oldCloseAppModel := closeAppModel
		t.Cleanup(func() { closeAppModel = oldCloseAppModel })

		called := 0
		closeAppModel = func(m *app.Model) error {
			called++
			require.NotNil(t, m)
			return nil
		}

		finalModel := &app.Model{}
		err := cleanupFinalModel(finalModel, nil, false)
		require.NoError(t, err)
		require.Equal(t, 1, called, "expected *app.Model Close() path to be called")
	})

	t.Run("typed nil *app.Model is ignored", func(t *testing.T) {
		oldCloseAppModel := closeAppModel
		t.Cleanup(func() { closeAppModel = oldCloseAppModel })

		called := 0
		closeAppModel = func(m *app.Model) error {
			called++
			return nil
		}

		var finalModel *app.Model
		err := cleanupFinalModel(finalModel, nil, false)
		require.NoError(t, err)
		require.Equal(t, 0, called, "expected typed nil *app.Model to skip Close()")
	})

	t.Run("close error returned when run succeeded", func(t *testing.T) {
		closeErr := fmt.Errorf("close failed")
		m := &testModelWithClose{err: closeErr}
		err := cleanupFinalModel(m, nil, true)
		require.ErrorIs(t, err, closeErr)
		require.True(t, m.closed, "expected Close() to be called")
	})

	t.Run("run error preserved when close also fails", func(t *testing.T) {
		runErr := fmt.Errorf("run failed")
		closeErr := fmt.Errorf("close failed")
		m := &testModelWithClose{err: closeErr}
		err := cleanupFinalModel(m, runErr, true)
		require.ErrorIs(t, err, runErr)
		require.True(t, m.closed, "expected Close() to be called")
	})

	t.Run("app.Model close error is returned when run succeeded", func(t *testing.T) {
		oldCloseAppModel := closeAppModel
		t.Cleanup(func() { closeAppModel = oldCloseAppModel })

		closeErr := fmt.Errorf("app close failed")
		closeAppModel = func(m *app.Model) error {
			return closeErr
		}

		finalModel := app.Model{}
		err := cleanupFinalModel(finalModel, nil, true)
		require.ErrorIs(t, err, closeErr)
	})
}

func TestResolveDebugLogPath_UsesPERLESLOGOverride(t *testing.T) {
	original := os.Getenv("PERLES_LOG")
	t.Cleanup(func() {
		if original == "" {
			_ = os.Unsetenv("PERLES_LOG")
			return
		}
		_ = os.Setenv("PERLES_LOG", original)
	})

	require.NoError(t, os.Setenv("PERLES_LOG", "/tmp/custom-perles.log"))

	resolved := resolveDebugLogPath()
	require.Equal(t, "/tmp/custom-perles.log", resolved)
}

func TestResolveDebugLogPath_UsesCentralizedDefaultWhenUnset(t *testing.T) {
	originalLog := os.Getenv("PERLES_LOG")
	originalXDG := os.Getenv("XDG_STATE_HOME")
	t.Cleanup(func() {
		if originalLog == "" {
			_ = os.Unsetenv("PERLES_LOG")
		} else {
			_ = os.Setenv("PERLES_LOG", originalLog)
		}

		if originalXDG == "" {
			_ = os.Unsetenv("XDG_STATE_HOME")
		} else {
			_ = os.Setenv("XDG_STATE_HOME", originalXDG)
		}
	})

	projectDir := filepath.Join(t.TempDir(), "test-project")
	require.NoError(t, os.MkdirAll(projectDir, 0o755))

	stateRoot := t.TempDir()
	require.NoError(t, os.Setenv("XDG_STATE_HOME", stateRoot))
	require.NoError(t, os.Unsetenv("PERLES_LOG"))

	originalWD, err := os.Getwd()
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, os.Chdir(originalWD))
	})
	require.NoError(t, os.Chdir(projectDir))

	resolved := resolveDebugLogPath()
	require.NotContains(t, resolved, string(filepath.Separator)+"debug.log")
	require.True(t, strings.HasPrefix(resolved, filepath.Join(stateRoot, "perles", "logs")+string(filepath.Separator)))
	require.Contains(t, resolved, string(filepath.Separator)+"test-project-")
	require.True(t, strings.HasSuffix(resolved, "-perles.log"))
}
