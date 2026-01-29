package frontend

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
)

// OpenBrowser attempts to open the given URL in the default browser.
// Returns an error if it fails, allowing the caller to fall back to
// printing the URL to the terminal.
func OpenBrowser(url string) error {
	// Check for BROWSER env var first (common on Linux)
	if browser := os.Getenv("BROWSER"); browser != "" {
		return exec.Command(browser, url).Start() //nolint:gosec // BROWSER is user-controlled env var
	}

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", url)
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}

	return cmd.Start()
}
