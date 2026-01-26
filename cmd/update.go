package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var versionFlag string

// lookPath is the function used to check if executables are in PATH.
// It defaults to exec.LookPath and can be overridden in tests.
var lookPath = exec.LookPath

// execCommand is the function used to create exec.Cmd instances.
// It defaults to exec.Command and can be overridden in tests.
var execCommand = exec.Command

// printInfo is the function used to print informational messages.
// It defaults to fmt.Println and can be overridden in tests.
var printInfo = func(msg string) {
	fmt.Println(msg)
}

// getExecutable is the function used to get the current executable path.
// It defaults to os.Executable and can be overridden in tests.
var getExecutable = os.Executable

// ErrCurlNotFound is returned when curl is not available in PATH.
var ErrCurlNotFound = errors.New("curl not found")

// ErrScriptFailed is returned when the install script execution fails.
var ErrScriptFailed = errors.New("script execution failed")

const installScriptURL = "https://raw.githubusercontent.com/zjrosen/perles/main/install.sh"
const githubReleasesAPI = "https://api.github.com/repos/zjrosen/perles/releases/latest"

// httpClient is the HTTP client used to fetch release info.
// It can be overridden in tests.
var httpClient = &http.Client{Timeout: 10 * time.Second}

// getVersion returns the current version of perles.
// It can be overridden in tests.
var getVersion = func() string {
	return version
}

// githubRelease represents the GitHub API response for a release.
type githubRelease struct {
	TagName string `json:"tag_name"`
}

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update perles to the latest version",
	Long: `Update perles to the latest version by running the install script.

By default, updates to the latest release. Use --version to install a specific version.

Examples:
  perles update              # Update to latest version
  perles update --version v1.0.0  # Install specific version`,
	RunE: runUpdate,
}

func init() {
	rootCmd.AddCommand(updateCmd)
	updateCmd.Flags().StringVarP(&versionFlag, "version", "v", "", "specific version to install (e.g., v1.0.0)")
}

func runUpdate(cmd *cobra.Command, args []string) error {
	// Check if installed via Homebrew first
	if isHomebrewInstallation() {
		printInfo("perles was installed via Homebrew. Use: brew upgrade perles")
		return nil
	}

	// Check prerequisites before attempting update
	if err := checkPrerequisites(); err != nil {
		return err
	}

	// If no specific version requested, check if already on latest
	if versionFlag == "" {
		currentVersion := getVersion()
		latestVersion, err := fetchLatestRelease()
		if err == nil && isAlreadyLatest(currentVersion, latestVersion) {
			printInfo(fmt.Sprintf("Already on the latest version (%s)", latestVersion))
			return nil
		}
		// If we can't fetch latest, proceed with update anyway
	}

	// Display informational message before update
	if versionFlag != "" {
		printInfo(fmt.Sprintf("Installing version: %s", versionFlag))
	} else {
		printInfo("Updating to latest version...")
	}

	// Execute the install script
	return executeInstallScript(versionFlag)
}

// executeInstallScript runs the install.sh script via curl | bash.
// If version is non-empty, it sets the VERSION environment variable.
func executeInstallScript(version string) error {
	script := fmt.Sprintf("curl -sSL %s | bash", installScriptURL)
	bashCmd := execCommand("bash", "-c", script)

	// Set VERSION environment variable if specified
	bashCmd.Env = os.Environ()
	if version != "" {
		bashCmd.Env = append(bashCmd.Env, "VERSION="+version)
	}

	// Stream stdout and stderr to terminal for progress visibility
	bashCmd.Stdout = os.Stdout
	bashCmd.Stderr = os.Stderr

	if err := bashCmd.Run(); err != nil {
		return fmt.Errorf("%w: %s", ErrScriptFailed, err.Error())
	}

	return nil
}

// checkPrerequisites verifies required tools are available before attempting update.
func checkPrerequisites() error {
	_, err := lookPath("curl")
	if err != nil {
		return fmt.Errorf("%w: curl is required but not found in PATH. Install curl and try again", ErrCurlNotFound)
	}
	return nil
}

// isHomebrewInstallation checks if the binary was installed via Homebrew.
// It looks for Homebrew indicators in the executable path such as:
// - /opt/homebrew/ (Apple Silicon Macs)
// - /usr/local/Cellar/ (Intel Macs)
// - Cellar directory in path (Homebrew's installation structure)
func isHomebrewInstallation() bool {
	execPath, err := getExecutable()
	if err != nil {
		// If we can't determine the path, assume not Homebrew
		return false
	}

	// Homebrew indicators in path
	homebrewIndicators := []string{
		"/opt/homebrew/",
		"/usr/local/Cellar/",
		"/Cellar/",
		"/homebrew/",
	}

	for _, indicator := range homebrewIndicators {
		if strings.Contains(execPath, indicator) {
			return true
		}
	}

	return false
}

// fetchLatestRelease fetches the latest release tag from GitHub.
func fetchLatestRelease() (string, error) {
	resp, err := httpClient.Get(githubReleasesAPI)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", err
	}

	return release.TagName, nil
}

// isAlreadyLatest compares current and latest versions.
// Returns true if current matches latest (with or without 'v' prefix).
// Handles dev versions like "v0.7.2-6-gaa951141-dirty" by extracting base version.
func isAlreadyLatest(current, latest string) bool {
	current = strings.TrimPrefix(current, "v")
	latest = strings.TrimPrefix(latest, "v")

	// Extract base version (before any -suffix like -6-gaa951141-dirty)
	if idx := strings.Index(current, "-"); idx != -1 {
		current = current[:idx]
	}
	if idx := strings.Index(latest, "-"); idx != -1 {
		latest = latest[:idx]
	}

	return current == latest
}
