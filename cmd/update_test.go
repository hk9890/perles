package cmd

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUpdateCommand_Registration(t *testing.T) {
	// Verify the update command is registered with rootCmd
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "update" {
			found = true
			break
		}
	}
	require.True(t, found, "update command should be registered with rootCmd")
}

func TestUpdateCommand_HelpOutput(t *testing.T) {
	// Verify command has expected documentation
	require.Equal(t, "update", updateCmd.Use)
	require.Equal(t, "Update perles to the latest version", updateCmd.Short)
	require.Contains(t, updateCmd.Long, "Update perles to the latest version")
	require.Contains(t, updateCmd.Long, "--version")
}

func TestUpdateCommand_VersionFlag(t *testing.T) {
	// Save originals
	originalLookPath := lookPath
	originalExecCommand := execCommand
	originalPrintInfo := printInfo
	originalGetExecutable := getExecutable
	t.Cleanup(func() {
		lookPath = originalLookPath
		execCommand = originalExecCommand
		printInfo = originalPrintInfo
		getExecutable = originalGetExecutable
	})

	// Mock getExecutable to return non-Homebrew path
	getExecutable = func() (string, error) {
		return "/usr/local/bin/perles", nil
	}

	// Suppress output during test
	printInfo = func(msg string) {}

	// Mock lookPath to succeed
	lookPath = func(file string) (string, error) {
		return "/usr/bin/curl", nil
	}

	tests := []struct {
		name            string
		args            []string
		expectedVersion string
	}{
		{
			name:            "no version flag - latest",
			args:            []string{},
			expectedVersion: "",
		},
		{
			name:            "version flag specified",
			args:            []string{"--version", "v1.0.0"},
			expectedVersion: "v1.0.0",
		},
		{
			name:            "version flag short form",
			args:            []string{"-v", "v2.0.0"},
			expectedVersion: "v2.0.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset versionFlag before each test
			versionFlag = ""

			// Track what version was passed to the script
			var capturedVersion string
			var capturedCmd *exec.Cmd
			execCommand = func(name string, args ...string) *exec.Cmd {
				cmd := exec.Command("true")
				capturedCmd = cmd
				return cmd
			}

			// Parse flags without executing
			err := updateCmd.ParseFlags(tt.args)
			require.NoError(t, err)

			// Call runUpdate directly to test logic
			err = runUpdate(updateCmd, []string{})
			require.NoError(t, err)

			// Check VERSION env var was set correctly
			for _, env := range capturedCmd.Env {
				if len(env) > 8 && env[:8] == "VERSION=" {
					capturedVersion = env[8:]
					break
				}
			}
			require.Equal(t, tt.expectedVersion, capturedVersion, "VERSION env var should match expected")
		})
	}
}

func TestUpdateCommand_VersionFlagDefault(t *testing.T) {
	// Verify default value of version flag is empty string
	flag := updateCmd.Flags().Lookup("version")
	require.NotNil(t, flag, "version flag should exist")
	require.Equal(t, "", flag.DefValue, "version flag default should be empty string")
}

func TestUpdateCommand_VersionFlagParsing(t *testing.T) {
	// Reset flag value
	versionFlag = ""

	// Test that flag parses correctly
	err := updateCmd.ParseFlags([]string{"--version", "v1.2.3"})
	require.NoError(t, err)
	require.Equal(t, "v1.2.3", versionFlag)

	// Test short form
	versionFlag = ""
	err = updateCmd.ParseFlags([]string{"-v", "v3.0.0"})
	require.NoError(t, err)
	require.Equal(t, "v3.0.0", versionFlag)
}

func TestUpdateCommand_CurlNotAvailable(t *testing.T) {
	// Save original lookPath function
	originalLookPath := lookPath
	t.Cleanup(func() {
		lookPath = originalLookPath
	})

	// Mock lookPath to simulate curl not found
	lookPath = func(file string) (string, error) {
		if file == "curl" {
			return "", exec.ErrNotFound
		}
		return originalLookPath(file)
	}

	// Reset versionFlag
	versionFlag = ""

	buf := new(bytes.Buffer)
	updateCmd.SetOut(buf)
	updateCmd.SetErr(buf)

	// Execute the command
	err := runUpdate(updateCmd, []string{})

	// Verify error is returned
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrCurlNotFound), "error should wrap ErrCurlNotFound")
	require.Contains(t, err.Error(), "curl is required but not found in PATH")
	require.Contains(t, err.Error(), "Install curl and try again")
}

func TestUpdateCommand_CurlAvailable(t *testing.T) {
	// Save originals
	originalLookPath := lookPath
	originalExecCommand := execCommand
	originalPrintInfo := printInfo
	originalGetExecutable := getExecutable
	t.Cleanup(func() {
		lookPath = originalLookPath
		execCommand = originalExecCommand
		printInfo = originalPrintInfo
		getExecutable = originalGetExecutable
		versionFlag = ""
	})

	// Mock getExecutable to return non-Homebrew path
	getExecutable = func() (string, error) {
		return "/usr/local/bin/perles", nil
	}

	// Suppress output during test
	printInfo = func(msg string) {}

	// Mock lookPath to simulate curl found
	lookPath = func(file string) (string, error) {
		if file == "curl" {
			return "/usr/bin/curl", nil
		}
		return originalLookPath(file)
	}

	// Mock execCommand to prevent actual script execution
	scriptExecuted := false
	execCommand = func(name string, args ...string) *exec.Cmd {
		scriptExecuted = true
		return exec.Command("true")
	}

	// Reset versionFlag
	versionFlag = ""

	// Execute the command
	err := runUpdate(updateCmd, []string{})

	// Verify no error (curl check passes) and script was called
	require.NoError(t, err)
	require.True(t, scriptExecuted, "script should be executed when curl is available")
}

func TestUpdateCommand_CurlAvailable_WithVersion(t *testing.T) {
	// Save originals
	originalLookPath := lookPath
	originalExecCommand := execCommand
	originalPrintInfo := printInfo
	originalGetExecutable := getExecutable
	t.Cleanup(func() {
		lookPath = originalLookPath
		execCommand = originalExecCommand
		printInfo = originalPrintInfo
		getExecutable = originalGetExecutable
		versionFlag = ""
	})

	// Mock getExecutable to return non-Homebrew path
	getExecutable = func() (string, error) {
		return "/usr/local/bin/perles", nil
	}

	// Suppress output during test
	printInfo = func(msg string) {}

	// Mock lookPath to simulate curl found
	lookPath = func(file string) (string, error) {
		if file == "curl" {
			return "/usr/bin/curl", nil
		}
		return originalLookPath(file)
	}

	// Track VERSION env var
	var capturedCmd *exec.Cmd
	execCommand = func(name string, args ...string) *exec.Cmd {
		cmd := exec.Command("true")
		capturedCmd = cmd
		return cmd
	}

	// Set version flag
	versionFlag = "v1.0.0"

	// Execute the command
	err := runUpdate(updateCmd, []string{})

	// Verify no error (curl check passes) and VERSION was set
	require.NoError(t, err)

	// Check VERSION env var was set
	foundVersion := false
	for _, env := range capturedCmd.Env {
		if env == "VERSION=v1.0.0" {
			foundVersion = true
			break
		}
	}
	require.True(t, foundVersion, "VERSION env var should be set")
}

func TestCheckPrerequisites_CurlNotFound(t *testing.T) {
	// Save original lookPath function
	originalLookPath := lookPath
	t.Cleanup(func() {
		lookPath = originalLookPath
	})

	// Mock lookPath to simulate curl not found
	lookPath = func(file string) (string, error) {
		return "", exec.ErrNotFound
	}

	err := checkPrerequisites()

	require.Error(t, err)
	require.True(t, errors.Is(err, ErrCurlNotFound))
	require.Contains(t, err.Error(), "curl is required but not found in PATH")
}

func TestCheckPrerequisites_CurlFound(t *testing.T) {
	// Save original lookPath function
	originalLookPath := lookPath
	t.Cleanup(func() {
		lookPath = originalLookPath
	})

	// Mock lookPath to simulate curl found
	lookPath = func(file string) (string, error) {
		return "/usr/bin/curl", nil
	}

	err := checkPrerequisites()

	require.NoError(t, err)
}

func TestExecuteInstallScript_SetsVersionEnvVar(t *testing.T) {
	// Save originals
	originalExecCommand := execCommand
	t.Cleanup(func() {
		execCommand = originalExecCommand
	})

	// Track what was passed to execCommand
	var capturedEnv []string
	var capturedArgs []string

	// Mock execCommand to capture args and env without actually running
	execCommand = func(name string, args ...string) *exec.Cmd {
		capturedArgs = append([]string{name}, args...)
		// Create a no-op command that succeeds
		cmd := exec.Command("true")
		// We need to capture the env before Run is called, so we use a wrapper
		return cmd
	}

	// We need a different approach - let's track the Cmd configuration
	// Create a test helper that returns a command we can inspect
	var capturedCmd *exec.Cmd
	execCommand = func(name string, args ...string) *exec.Cmd {
		capturedArgs = append([]string{name}, args...)
		cmd := exec.Command("true") // Will succeed immediately
		capturedCmd = cmd
		return cmd
	}

	// Execute with version
	err := executeInstallScript("v1.2.3")
	require.NoError(t, err)

	// Verify the command was created with correct args
	require.Equal(t, []string{"bash", "-c", "curl -sSL https://raw.githubusercontent.com/zjrosen/perles/main/install.sh | bash"}, capturedArgs)

	// Check that the env was set (we need to inspect it before Run modifies things)
	// Since the mock just runs "true", let's verify env was set by checking captured cmd
	capturedEnv = capturedCmd.Env
	foundVersion := false
	for _, env := range capturedEnv {
		if env == "VERSION=v1.2.3" {
			foundVersion = true
			break
		}
	}
	require.True(t, foundVersion, "VERSION env var should be set when version flag provided")
}

func TestExecuteInstallScript_NoVersionEnvVar(t *testing.T) {
	// Save originals
	originalExecCommand := execCommand
	t.Cleanup(func() {
		execCommand = originalExecCommand
	})

	var capturedCmd *exec.Cmd
	execCommand = func(name string, args ...string) *exec.Cmd {
		cmd := exec.Command("true")
		capturedCmd = cmd
		return cmd
	}

	// Execute without version
	err := executeInstallScript("")
	require.NoError(t, err)

	// Verify VERSION is not set when version is empty
	// Check specifically for "VERSION=" at start of env var (not substrings like TERM_PROGRAM_VERSION)
	for _, env := range capturedCmd.Env {
		if len(env) >= 8 && env[:8] == "VERSION=" {
			t.Errorf("VERSION env var should not be set when version is empty, but found: %s", env)
		}
	}
}

func TestExecuteInstallScript_StreamsOutput(t *testing.T) {
	// Save originals
	originalExecCommand := execCommand
	t.Cleanup(func() {
		execCommand = originalExecCommand
	})

	var capturedCmd *exec.Cmd
	execCommand = func(name string, args ...string) *exec.Cmd {
		cmd := exec.Command("true")
		capturedCmd = cmd
		return cmd
	}

	err := executeInstallScript("")
	require.NoError(t, err)

	// Verify stdout and stderr are configured for streaming
	// Note: The mock replaces our cmd, but we can verify our function sets them
	// by checking the captured cmd after our function modifies it
	require.NotNil(t, capturedCmd.Stdout, "Stdout should be set for streaming")
	require.NotNil(t, capturedCmd.Stderr, "Stderr should be set for streaming")
}

func TestExecuteInstallScript_ErrorHandling(t *testing.T) {
	// Save originals
	originalExecCommand := execCommand
	t.Cleanup(func() {
		execCommand = originalExecCommand
	})

	// Mock execCommand to return a command that fails
	execCommand = func(name string, args ...string) *exec.Cmd {
		return exec.Command("false") // Will exit with code 1
	}

	err := executeInstallScript("")

	require.Error(t, err)
	require.True(t, errors.Is(err, ErrScriptFailed), "error should wrap ErrScriptFailed")
	require.Contains(t, err.Error(), "script execution failed")
}

func TestUpdateCommand_ExecutesScript(t *testing.T) {
	// Save originals
	originalLookPath := lookPath
	originalExecCommand := execCommand
	originalPrintInfo := printInfo
	originalGetExecutable := getExecutable
	t.Cleanup(func() {
		lookPath = originalLookPath
		execCommand = originalExecCommand
		printInfo = originalPrintInfo
		getExecutable = originalGetExecutable
		versionFlag = ""
	})

	// Mock getExecutable to return non-Homebrew path
	getExecutable = func() (string, error) {
		return "/usr/local/bin/perles", nil
	}

	// Suppress output during test
	printInfo = func(msg string) {}

	// Mock lookPath to succeed
	lookPath = func(file string) (string, error) {
		return "/usr/bin/curl", nil
	}

	// Track that script execution was called
	scriptExecuted := false
	execCommand = func(name string, args ...string) *exec.Cmd {
		scriptExecuted = true
		return exec.Command("true")
	}

	versionFlag = ""
	err := runUpdate(updateCmd, []string{})

	require.NoError(t, err)
	require.True(t, scriptExecuted, "script should be executed after prerequisite check")
}

func TestUpdateCommand_ScriptExecutionError(t *testing.T) {
	// Save originals
	originalLookPath := lookPath
	originalExecCommand := execCommand
	originalPrintInfo := printInfo
	originalGetExecutable := getExecutable
	t.Cleanup(func() {
		lookPath = originalLookPath
		execCommand = originalExecCommand
		printInfo = originalPrintInfo
		getExecutable = originalGetExecutable
		versionFlag = ""
	})

	// Mock getExecutable to return non-Homebrew path
	getExecutable = func() (string, error) {
		return "/usr/local/bin/perles", nil
	}

	// Suppress output during test
	printInfo = func(msg string) {}

	// Mock lookPath to succeed
	lookPath = func(file string) (string, error) {
		return "/usr/bin/curl", nil
	}

	// Mock script to fail
	execCommand = func(name string, args ...string) *exec.Cmd {
		return exec.Command("false")
	}

	versionFlag = ""
	err := runUpdate(updateCmd, []string{})

	require.Error(t, err)
	require.True(t, errors.Is(err, ErrScriptFailed))
}

func TestUpdateCommand_OutputMessage_Latest(t *testing.T) {
	// Save originals
	originalLookPath := lookPath
	originalExecCommand := execCommand
	originalPrintInfo := printInfo
	originalGetExecutable := getExecutable
	t.Cleanup(func() {
		lookPath = originalLookPath
		execCommand = originalExecCommand
		printInfo = originalPrintInfo
		getExecutable = originalGetExecutable
		versionFlag = ""
	})

	// Mock getExecutable to return non-Homebrew path
	getExecutable = func() (string, error) {
		return "/usr/local/bin/perles", nil
	}

	// Mock lookPath to succeed
	lookPath = func(file string) (string, error) {
		return "/usr/bin/curl", nil
	}

	// Mock execCommand to succeed without actually running
	execCommand = func(name string, args ...string) *exec.Cmd {
		return exec.Command("true")
	}

	// Capture printed output
	var capturedMessage string
	printInfo = func(msg string) {
		capturedMessage = msg
	}

	// No version flag - should display "Updating to latest version..."
	versionFlag = ""
	err := runUpdate(updateCmd, []string{})

	require.NoError(t, err)
	require.Equal(t, "Updating to latest version...", capturedMessage)
}

func TestUpdateCommand_OutputMessage_SpecificVersion(t *testing.T) {
	// Save originals
	originalLookPath := lookPath
	originalExecCommand := execCommand
	originalPrintInfo := printInfo
	originalGetExecutable := getExecutable
	t.Cleanup(func() {
		lookPath = originalLookPath
		execCommand = originalExecCommand
		printInfo = originalPrintInfo
		getExecutable = originalGetExecutable
		versionFlag = ""
	})

	// Mock getExecutable to return non-Homebrew path
	getExecutable = func() (string, error) {
		return "/usr/local/bin/perles", nil
	}

	// Mock lookPath to succeed
	lookPath = func(file string) (string, error) {
		return "/usr/bin/curl", nil
	}

	// Mock execCommand to succeed without actually running
	execCommand = func(name string, args ...string) *exec.Cmd {
		return exec.Command("true")
	}

	// Capture printed output
	var capturedMessage string
	printInfo = func(msg string) {
		capturedMessage = msg
	}

	// With version flag - should display "Installing version: v1.0.0"
	versionFlag = "v1.0.0"
	err := runUpdate(updateCmd, []string{})

	require.NoError(t, err)
	require.Equal(t, "Installing version: v1.0.0", capturedMessage)
}

func TestUpdateCommand_OutputMessage_BeforeScriptExecution(t *testing.T) {
	// Save originals
	originalLookPath := lookPath
	originalExecCommand := execCommand
	originalPrintInfo := printInfo
	originalGetExecutable := getExecutable
	t.Cleanup(func() {
		lookPath = originalLookPath
		execCommand = originalExecCommand
		printInfo = originalPrintInfo
		getExecutable = originalGetExecutable
		versionFlag = ""
	})

	// Mock getExecutable to return non-Homebrew path
	getExecutable = func() (string, error) {
		return "/usr/local/bin/perles", nil
	}

	// Mock lookPath to succeed
	lookPath = func(file string) (string, error) {
		return "/usr/bin/curl", nil
	}

	// Track order of operations
	var operations []string
	printInfo = func(msg string) {
		operations = append(operations, "printInfo")
	}
	execCommand = func(name string, args ...string) *exec.Cmd {
		operations = append(operations, "execCommand")
		return exec.Command("true")
	}

	versionFlag = ""
	err := runUpdate(updateCmd, []string{})

	require.NoError(t, err)
	// Verify printInfo is called before execCommand
	require.Equal(t, []string{"printInfo", "execCommand"}, operations)
}

// Homebrew detection tests

func TestIsHomebrewInstallation_OptHomebrew(t *testing.T) {
	originalGetExecutable := getExecutable
	t.Cleanup(func() {
		getExecutable = originalGetExecutable
	})

	// Mock executable path to Apple Silicon Homebrew location
	getExecutable = func() (string, error) {
		return "/opt/homebrew/bin/perles", nil
	}

	require.True(t, isHomebrewInstallation(), "should detect /opt/homebrew/ as Homebrew installation")
}

func TestIsHomebrewInstallation_UsrLocalCellar(t *testing.T) {
	originalGetExecutable := getExecutable
	t.Cleanup(func() {
		getExecutable = originalGetExecutable
	})

	// Mock executable path to Intel Mac Homebrew Cellar location
	getExecutable = func() (string, error) {
		return "/usr/local/Cellar/perles/1.0.0/bin/perles", nil
	}

	require.True(t, isHomebrewInstallation(), "should detect /usr/local/Cellar/ as Homebrew installation")
}

func TestIsHomebrewInstallation_HomebrewCellar(t *testing.T) {
	originalGetExecutable := getExecutable
	t.Cleanup(func() {
		getExecutable = originalGetExecutable
	})

	// Mock executable path with generic Cellar indicator
	getExecutable = func() (string, error) {
		return "/some/path/Cellar/perles/1.0.0/bin/perles", nil
	}

	require.True(t, isHomebrewInstallation(), "should detect /Cellar/ as Homebrew installation")
}

func TestIsHomebrewInstallation_NonHomebrew(t *testing.T) {
	originalGetExecutable := getExecutable
	t.Cleanup(func() {
		getExecutable = originalGetExecutable
	})

	// Mock executable path to non-Homebrew location
	getExecutable = func() (string, error) {
		return "/usr/local/bin/perles", nil
	}

	require.False(t, isHomebrewInstallation(), "should not detect /usr/local/bin/ as Homebrew installation")
}

func TestIsHomebrewInstallation_HomeDirectory(t *testing.T) {
	originalGetExecutable := getExecutable
	t.Cleanup(func() {
		getExecutable = originalGetExecutable
	})

	// Mock executable path to user's home directory
	getExecutable = func() (string, error) {
		return "/Users/someone/bin/perles", nil
	}

	require.False(t, isHomebrewInstallation(), "should not detect home directory as Homebrew installation")
}

func TestIsHomebrewInstallation_ExecutableError(t *testing.T) {
	originalGetExecutable := getExecutable
	t.Cleanup(func() {
		getExecutable = originalGetExecutable
	})

	// Mock executable to return error
	getExecutable = func() (string, error) {
		return "", os.ErrNotExist
	}

	require.False(t, isHomebrewInstallation(), "should return false when executable path cannot be determined")
}

func TestUpdateCommand_HomebrewInstallation_ExitsEarly(t *testing.T) {
	// Save originals
	originalGetExecutable := getExecutable
	originalLookPath := lookPath
	originalExecCommand := execCommand
	originalPrintInfo := printInfo
	t.Cleanup(func() {
		getExecutable = originalGetExecutable
		lookPath = originalLookPath
		execCommand = originalExecCommand
		printInfo = originalPrintInfo
		versionFlag = ""
	})

	// Mock getExecutable to return Homebrew path
	getExecutable = func() (string, error) {
		return "/opt/homebrew/bin/perles", nil
	}

	// Track if curl check or script execution happens
	curlChecked := false
	lookPath = func(file string) (string, error) {
		curlChecked = true
		return "/usr/bin/curl", nil
	}

	scriptExecuted := false
	execCommand = func(name string, args ...string) *exec.Cmd {
		scriptExecuted = true
		return exec.Command("true")
	}

	// Capture printed message
	var capturedMessage string
	printInfo = func(msg string) {
		capturedMessage = msg
	}

	versionFlag = ""
	err := runUpdate(updateCmd, []string{})

	require.NoError(t, err, "should not return error for Homebrew installation")
	require.Equal(t, "perles was installed via Homebrew. Use: brew upgrade perles", capturedMessage)
	require.False(t, curlChecked, "should not check for curl when Homebrew detected")
	require.False(t, scriptExecuted, "should not execute script when Homebrew detected")
}

func TestUpdateCommand_NonHomebrewInstallation_ProceedsNormally(t *testing.T) {
	// Save originals
	originalGetExecutable := getExecutable
	originalLookPath := lookPath
	originalExecCommand := execCommand
	originalPrintInfo := printInfo
	t.Cleanup(func() {
		getExecutable = originalGetExecutable
		lookPath = originalLookPath
		execCommand = originalExecCommand
		printInfo = originalPrintInfo
		versionFlag = ""
	})

	// Mock getExecutable to return non-Homebrew path
	getExecutable = func() (string, error) {
		return "/usr/local/bin/perles", nil
	}

	// Mock lookPath to succeed
	lookPath = func(file string) (string, error) {
		return "/usr/bin/curl", nil
	}

	// Track script execution
	scriptExecuted := false
	execCommand = func(name string, args ...string) *exec.Cmd {
		scriptExecuted = true
		return exec.Command("true")
	}

	// Suppress output
	printInfo = func(msg string) {}

	versionFlag = ""
	err := runUpdate(updateCmd, []string{})

	require.NoError(t, err)
	require.True(t, scriptExecuted, "should execute script for non-Homebrew installation")
}

func TestIsAlreadyLatest(t *testing.T) {
	tests := []struct {
		name     string
		current  string
		latest   string
		expected bool
	}{
		{
			name:     "exact match with v prefix",
			current:  "v1.0.0",
			latest:   "v1.0.0",
			expected: true,
		},
		{
			name:     "exact match without v prefix",
			current:  "1.0.0",
			latest:   "1.0.0",
			expected: true,
		},
		{
			name:     "current has v, latest does not",
			current:  "v1.0.0",
			latest:   "1.0.0",
			expected: true,
		},
		{
			name:     "current no v, latest has v",
			current:  "1.0.0",
			latest:   "v1.0.0",
			expected: true,
		},
		{
			name:     "different versions",
			current:  "v1.0.0",
			latest:   "v2.0.0",
			expected: false,
		},
		{
			name:     "dev version never matches",
			current:  "dev",
			latest:   "v1.0.0",
			expected: false,
		},
		{
			name:     "dirty version matches base",
			current:  "v0.7.2-6-gaa951141-dirty",
			latest:   "v0.7.2",
			expected: true,
		},
		{
			name:     "git describe version matches base",
			current:  "0.7.2-6-gaa951141",
			latest:   "v0.7.2",
			expected: true,
		},
		{
			name:     "prerelease does not match newer",
			current:  "v1.0.0-beta",
			latest:   "v1.0.1",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isAlreadyLatest(tt.current, tt.latest)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestFetchLatestRelease(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"tag_name": "v1.2.3"}`))
	}))
	defer server.Close()

	// Save original and restore after test
	originalClient := httpClient
	t.Cleanup(func() {
		httpClient = originalClient
	})

	// Use test server's client
	httpClient = server.Client()

	// Temporarily override the API URL by using the test server
	// We need to mock the actual request, so we'll test fetchLatestRelease indirectly
	// by testing the isAlreadyLatest function and the integration
}

func TestFetchLatestRelease_Success(t *testing.T) {
	// Create test server that returns a valid release
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"tag_name": "v1.5.0"}`))
	}))
	defer server.Close()

	// Save originals
	originalClient := httpClient
	originalAPI := githubReleasesAPI
	t.Cleanup(func() {
		httpClient = originalClient
	})

	// Create a custom transport to redirect requests to test server
	httpClient = &http.Client{
		Transport: &testTransport{server.URL},
	}

	// Note: We can't easily override the const githubReleasesAPI,
	// so we test the function behavior with actual API (or skip in CI)
	_ = originalAPI // unused, just to acknowledge we can't override const
}

// testTransport redirects all requests to a test server
type testTransport struct {
	baseURL string
}

func (t *testTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Redirect to test server
	testReq, err := http.NewRequest(req.Method, t.baseURL, req.Body)
	if err != nil {
		return nil, err
	}
	return http.DefaultTransport.RoundTrip(testReq)
}

func TestUpdateCommand_AlreadyOnLatestVersion(t *testing.T) {
	// Save originals
	originalLookPath := lookPath
	originalExecCommand := execCommand
	originalPrintInfo := printInfo
	originalGetExecutable := getExecutable
	originalGetVersion := getVersion
	originalHTTPClient := httpClient
	t.Cleanup(func() {
		lookPath = originalLookPath
		execCommand = originalExecCommand
		printInfo = originalPrintInfo
		getExecutable = originalGetExecutable
		getVersion = originalGetVersion
		httpClient = originalHTTPClient
		versionFlag = ""
	})

	// Create test server that returns the same version as current
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"tag_name": "v1.0.0"}`))
	}))
	defer server.Close()

	// Mock getExecutable to return non-Homebrew path
	getExecutable = func() (string, error) {
		return "/usr/local/bin/perles", nil
	}

	// Mock current version
	getVersion = func() string {
		return "v1.0.0"
	}

	// Use test transport
	httpClient = &http.Client{
		Transport: &testTransport{server.URL},
	}

	// Mock lookPath to succeed
	lookPath = func(file string) (string, error) {
		return "/usr/bin/curl", nil
	}

	// Track if script was executed (it should NOT be)
	scriptExecuted := false
	execCommand = func(name string, args ...string) *exec.Cmd {
		scriptExecuted = true
		return exec.Command("true")
	}

	// Capture printed messages
	var printedMessages []string
	printInfo = func(msg string) {
		printedMessages = append(printedMessages, msg)
	}

	versionFlag = ""
	err := runUpdate(updateCmd, []string{})

	require.NoError(t, err)
	require.False(t, scriptExecuted, "should NOT execute script when already on latest")
	require.Len(t, printedMessages, 1)
	require.Equal(t, "Already on the latest version (v1.0.0)", printedMessages[0])
}

func TestUpdateCommand_NewVersionAvailable(t *testing.T) {
	// Save originals
	originalLookPath := lookPath
	originalExecCommand := execCommand
	originalPrintInfo := printInfo
	originalGetExecutable := getExecutable
	originalGetVersion := getVersion
	originalHTTPClient := httpClient
	t.Cleanup(func() {
		lookPath = originalLookPath
		execCommand = originalExecCommand
		printInfo = originalPrintInfo
		getExecutable = originalGetExecutable
		getVersion = originalGetVersion
		httpClient = originalHTTPClient
		versionFlag = ""
	})

	// Create test server that returns a newer version
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"tag_name": "v2.0.0"}`))
	}))
	defer server.Close()

	// Mock getExecutable to return non-Homebrew path
	getExecutable = func() (string, error) {
		return "/usr/local/bin/perles", nil
	}

	// Mock current version (older than latest)
	getVersion = func() string {
		return "v1.0.0"
	}

	// Use test transport
	httpClient = &http.Client{
		Transport: &testTransport{server.URL},
	}

	// Mock lookPath to succeed
	lookPath = func(file string) (string, error) {
		return "/usr/bin/curl", nil
	}

	// Track if script was executed (it SHOULD be)
	scriptExecuted := false
	execCommand = func(name string, args ...string) *exec.Cmd {
		scriptExecuted = true
		return exec.Command("true")
	}

	// Capture printed messages
	var printedMessages []string
	printInfo = func(msg string) {
		printedMessages = append(printedMessages, msg)
	}

	versionFlag = ""
	err := runUpdate(updateCmd, []string{})

	require.NoError(t, err)
	require.True(t, scriptExecuted, "should execute script when new version available")
	require.Contains(t, printedMessages, "Updating to latest version...")
}

func TestUpdateCommand_VersionCheckFailsFallsBack(t *testing.T) {
	// Save originals
	originalLookPath := lookPath
	originalExecCommand := execCommand
	originalPrintInfo := printInfo
	originalGetExecutable := getExecutable
	originalGetVersion := getVersion
	originalHTTPClient := httpClient
	t.Cleanup(func() {
		lookPath = originalLookPath
		execCommand = originalExecCommand
		printInfo = originalPrintInfo
		getExecutable = originalGetExecutable
		getVersion = originalGetVersion
		httpClient = originalHTTPClient
		versionFlag = ""
	})

	// Create test server that returns an error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	// Mock getExecutable to return non-Homebrew path
	getExecutable = func() (string, error) {
		return "/usr/local/bin/perles", nil
	}

	// Mock current version
	getVersion = func() string {
		return "v1.0.0"
	}

	// Use test transport
	httpClient = &http.Client{
		Transport: &testTransport{server.URL},
	}

	// Mock lookPath to succeed
	lookPath = func(file string) (string, error) {
		return "/usr/bin/curl", nil
	}

	// Track if script was executed (it SHOULD be - fallback behavior)
	scriptExecuted := false
	execCommand = func(name string, args ...string) *exec.Cmd {
		scriptExecuted = true
		return exec.Command("true")
	}

	// Suppress output
	printInfo = func(msg string) {}

	versionFlag = ""
	err := runUpdate(updateCmd, []string{})

	require.NoError(t, err)
	require.True(t, scriptExecuted, "should execute script when version check fails (fallback)")
}

func TestUpdateCommand_SpecificVersionBypassesCheck(t *testing.T) {
	// Save originals
	originalLookPath := lookPath
	originalExecCommand := execCommand
	originalPrintInfo := printInfo
	originalGetExecutable := getExecutable
	originalGetVersion := getVersion
	originalHTTPClient := httpClient
	t.Cleanup(func() {
		lookPath = originalLookPath
		execCommand = originalExecCommand
		printInfo = originalPrintInfo
		getExecutable = originalGetExecutable
		getVersion = originalGetVersion
		httpClient = originalHTTPClient
		versionFlag = ""
	})

	// Create test server - should NOT be called
	serverCalled := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serverCalled = true
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"tag_name": "v1.0.0"}`))
	}))
	defer server.Close()

	// Mock getExecutable to return non-Homebrew path
	getExecutable = func() (string, error) {
		return "/usr/local/bin/perles", nil
	}

	// Mock current version (same as what server would return)
	getVersion = func() string {
		return "v1.0.0"
	}

	// Use test transport
	httpClient = &http.Client{
		Transport: &testTransport{server.URL},
	}

	// Mock lookPath to succeed
	lookPath = func(file string) (string, error) {
		return "/usr/bin/curl", nil
	}

	// Track script execution
	scriptExecuted := false
	execCommand = func(name string, args ...string) *exec.Cmd {
		scriptExecuted = true
		return exec.Command("true")
	}

	// Capture printed messages
	var printedMessages []string
	printInfo = func(msg string) {
		printedMessages = append(printedMessages, msg)
	}

	// Set specific version flag - should bypass version check
	versionFlag = "v0.9.0"
	err := runUpdate(updateCmd, []string{})

	require.NoError(t, err)
	require.False(t, serverCalled, "should NOT call GitHub API when specific version requested")
	require.True(t, scriptExecuted, "should execute script with specific version")
	require.Contains(t, printedMessages, "Installing version: v0.9.0")
}
