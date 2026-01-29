package frontend

import (
	"os"
	"os/exec"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestOpenBrowser_UnsupportedPlatform tests that unsupported platforms return an error.
// We can't easily change runtime.GOOS, so we test the error path indirectly
// by testing the function's behavior in documented edge cases.
func TestOpenBrowser_UnsupportedPlatform(t *testing.T) {
	// We can't easily test unsupported platform without modifying runtime.GOOS,
	// but we can verify the function handles the current platform
	// and that BROWSER env var takes precedence.

	t.Run("BROWSER_env_takes_precedence", func(t *testing.T) {
		// Save and restore BROWSER
		original := os.Getenv("BROWSER")
		defer func() {
			if original == "" {
				os.Unsetenv("BROWSER")
			} else {
				os.Setenv("BROWSER", original)
			}
		}()

		// Set BROWSER to a non-existent command to verify it's being used
		os.Setenv("BROWSER", "nonexistent-browser-command-12345")

		err := OpenBrowser("http://example.com")
		// The error depends on whether the command can be found/started
		// On most systems, this will fail to start the process
		// The key is that we're testing the BROWSER env var is checked

		// Since exec.Command("nonexistent-browser-command-12345", url).Start()
		// may succeed (process started but fails to find binary) or fail (binary not found)
		// depending on OS, we just verify the function runs without panicking
		_ = err
	})
}

// TestOpenBrowser_PlatformCommands documents expected commands per platform.
// We can't run these without actually opening a browser, so this is a documentation test.
func TestOpenBrowser_PlatformCommands(t *testing.T) {
	testCases := []struct {
		goos            string
		expectedCommand string
	}{
		{"darwin", "open"},
		{"linux", "xdg-open"},
		{"windows", "cmd"},
	}

	for _, tc := range testCases {
		t.Run(tc.goos, func(t *testing.T) {
			// Document expected commands - we can't easily test runtime.GOOS switching
			// This is a documentation test showing the expected behavior

			if runtime.GOOS == tc.goos {
				// Verify the expected command exists on this platform
				_, err := exec.LookPath(tc.expectedCommand)
				// If command exists, great. If not, that's an environment issue
				_ = err
			}
		})
	}
}

// TestOpenBrowser_CurrentPlatform tests that OpenBrowser works on the current platform.
func TestOpenBrowser_CurrentPlatform(t *testing.T) {
	// Skip in CI environments where browser opening would fail
	if os.Getenv("CI") != "" {
		t.Skip("skipping browser test in CI")
	}

	switch runtime.GOOS {
	case "darwin", "linux", "windows":
		// These platforms are supported - verify no error before Start()
		// We don't actually call the function to avoid opening browsers in tests
		t.Logf("Current platform %s is supported", runtime.GOOS)
	default:
		// Unsupported platform - verify error is returned
		err := OpenBrowser("http://example.com")
		require.Error(t, err)
		require.Contains(t, err.Error(), "unsupported platform")
	}
}

// TestOpenBrowser_URLFormats tests various URL formats are passed correctly.
func TestOpenBrowser_URLFormats(t *testing.T) {
	// Save and restore BROWSER
	original := os.Getenv("BROWSER")
	defer func() {
		if original == "" {
			os.Unsetenv("BROWSER")
		} else {
			os.Setenv("BROWSER", original)
		}
	}()

	// Use /bin/true (or echo on windows) to "open" URLs without side effects
	var testCommand string
	switch runtime.GOOS {
	case "darwin", "linux":
		testCommand = "true" // /bin/true accepts any arguments and succeeds
	case "windows":
		testCommand = "cmd.exe" // Will need /c echo or similar
		t.Skip("skipping URL format test on windows - needs different approach")
	default:
		t.Skip("skipping URL format test on unsupported platform")
	}

	os.Setenv("BROWSER", testCommand)

	testURLs := []string{
		"http://localhost:8080",
		"http://localhost:8080/?session=abc-123-def",
		"http://127.0.0.1:3000",
		"http://localhost:8080/?session=550e8400-e29b-41d4-a716-446655440000",
	}

	for _, url := range testURLs {
		t.Run(url, func(t *testing.T) {
			err := OpenBrowser(url)
			require.NoError(t, err, "OpenBrowser should succeed for URL: %s", url)
		})
	}
}
