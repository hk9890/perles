package mcp

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGenerateWorkerConfig(t *testing.T) {
	// GenerateWorkerConfig now returns HTTP config
	configJSON, err := GenerateWorkerConfig("worker-1", "/work")
	require.NoError(t, err, "GenerateWorkerConfig failed")

	var config MCPConfig
	require.NoError(t, json.Unmarshal([]byte(configJSON), &config), "Failed to parse config JSON")

	server, ok := config.MCPServers["perles-worker"]
	require.True(t, ok, "Missing perles-worker server in config")

	// Check it's HTTP transport
	if server.Type != "http" {
		t.Errorf("Type should be 'http', got %q", server.Type)
	}

	// Check URL includes worker ID
	expectedURL := "http://localhost:8765/worker/worker-1"
	if server.URL != expectedURL {
		t.Errorf("URL should be %q, got %q", expectedURL, server.URL)
	}
}

func TestGenerateWorkerConfigHTTP(t *testing.T) {
	configJSON, err := GenerateWorkerConfigHTTP(9000, "WORKER.3")
	require.NoError(t, err, "GenerateWorkerConfigHTTP failed")

	var config MCPConfig
	require.NoError(t, json.Unmarshal([]byte(configJSON), &config), "Failed to parse config JSON")

	server, ok := config.MCPServers["perles-worker"]
	require.True(t, ok, "Missing perles-worker server in config")

	if server.Type != "http" {
		t.Errorf("Type should be 'http', got %q", server.Type)
	}

	expectedURL := "http://localhost:9000/worker/WORKER.3"
	if server.URL != expectedURL {
		t.Errorf("URL should be %q, got %q", expectedURL, server.URL)
	}
}

func TestConfigToFlag(t *testing.T) {
	input := `{"mcpServers":{"test":{"command":"test"}}}`
	result := ConfigToFlag(input)
	if result != input {
		t.Errorf("ConfigToFlag = %q, want %q", result, input)
	}
}

func TestParseMCPConfig(t *testing.T) {
	input := `{
		"mcpServers": {
			"server1": {
				"command": "/bin/server1",
				"args": ["--flag"],
				"env": {"KEY": "VALUE"}
			},
			"server2": {
				"command": "/bin/server2"
			}
		}
	}`

	config, err := ParseMCPConfig(input)
	require.NoError(t, err, "ParseMCPConfig failed")

	if len(config.MCPServers) != 2 {
		t.Errorf("Server count = %d, want 2", len(config.MCPServers))
	}

	server1, ok := config.MCPServers["server1"]
	require.True(t, ok, "Missing server1")
	if server1.Command != "/bin/server1" {
		t.Errorf("server1.Command = %q, want %q", server1.Command, "/bin/server1")
	}
	if len(server1.Args) != 1 || server1.Args[0] != "--flag" {
		t.Errorf("server1.Args = %v, want [--flag]", server1.Args)
	}
	if server1.Env["KEY"] != "VALUE" {
		t.Errorf("server1.Env[KEY] = %q, want %q", server1.Env["KEY"], "VALUE")
	}
}

func TestParseMCPConfigInvalid(t *testing.T) {
	_, err := ParseMCPConfig("not valid json")
	if err == nil {
		t.Error("Expected error for invalid JSON")
	}
}

func TestMCPConfigSerialization(t *testing.T) {
	config := MCPConfig{
		MCPServers: map[string]MCPServerConfig{
			"test-server": {
				Command: "/usr/local/bin/mcp-server",
				Args:    []string{"--verbose", "--port", "8080"},
				Env: map[string]string{
					"LOG_LEVEL": "debug",
				},
			},
		},
	}

	data, err := json.Marshal(config)
	require.NoError(t, err, "Marshal failed")

	var parsed MCPConfig
	require.NoError(t, json.Unmarshal(data, &parsed), "Unmarshal failed")

	server, ok := parsed.MCPServers["test-server"]
	require.True(t, ok, "Missing test-server")
	if server.Command != config.MCPServers["test-server"].Command {
		t.Errorf("Command = %q, want %q", server.Command, config.MCPServers["test-server"].Command)
	}
}

func TestGenerateCoordinatorConfigHTTP(t *testing.T) {
	configJSON, err := GenerateCoordinatorConfigHTTP(9000)
	require.NoError(t, err, "GenerateCoordinatorConfigHTTP failed")

	var config MCPConfig
	require.NoError(t, json.Unmarshal([]byte(configJSON), &config), "Failed to parse config")

	server := config.MCPServers["perles-orchestrator"]
	if server.Type != "http" {
		t.Errorf("Type = %q, want \"http\"", server.Type)
	}
	expectedURL := "http://localhost:9000/mcp"
	if server.URL != expectedURL {
		t.Errorf("URL = %q, want %q", server.URL, expectedURL)
	}
}
