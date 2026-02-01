package handler

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/zjrosen/perles/internal/orchestration/client"
	"github.com/zjrosen/perles/internal/orchestration/mock"
	"github.com/zjrosen/perles/internal/orchestration/v2/command"
	"github.com/zjrosen/perles/internal/orchestration/v2/prompt/roles"
	"github.com/zjrosen/perles/internal/orchestration/v2/repository"
	"github.com/zjrosen/perles/internal/pubsub"
)

// mockCommandSubmitter implements process.CommandSubmitter for testing.
type mockCommandSubmitter struct {
	commands []command.Command
}

func (m *mockCommandSubmitter) Submit(cmd command.Command) {
	m.commands = append(m.commands, cmd)
}

func TestUnifiedProcessSpawner_SpawnProcess_Worker(t *testing.T) {
	mockClient := mock.NewClient()
	eventBus := pubsub.NewBroker[any]()
	submitter := &mockCommandSubmitter{}

	spawner := NewUnifiedProcessSpawner(UnifiedSpawnerConfig{
		CoordinatorClient: mockClient,
		WorkerClient:      mockClient,
		WorkDir:           "/test/workdir",
		Port:              8080,
		Submitter:         submitter,
		EventBus:          eventBus,
	})

	proc, err := spawner.SpawnProcess(context.Background(), "worker-1", repository.RoleWorker, SpawnOptions{})
	require.NoError(t, err)
	require.NotNil(t, proc)
	assert.Equal(t, "worker-1", proc.ID)
	assert.Equal(t, repository.RoleWorker, proc.Role)

	// Cleanup
	proc.Stop()
}

func TestUnifiedProcessSpawner_SpawnProcess_Coordinator(t *testing.T) {
	mockClient := mock.NewClient()
	eventBus := pubsub.NewBroker[any]()
	submitter := &mockCommandSubmitter{}

	spawner := NewUnifiedProcessSpawner(UnifiedSpawnerConfig{
		CoordinatorClient: mockClient,
		WorkerClient:      mockClient,
		WorkDir:           "/test/workdir",
		Port:              8080,
		Submitter:         submitter,
		EventBus:          eventBus,
	})

	proc, err := spawner.SpawnProcess(context.Background(), repository.CoordinatorID, repository.RoleCoordinator, SpawnOptions{})
	require.NoError(t, err)
	require.NotNil(t, proc)
	assert.Equal(t, repository.CoordinatorID, proc.ID)
	assert.Equal(t, repository.RoleCoordinator, proc.Role)

	// Cleanup
	proc.Stop()
}

func TestUnifiedProcessSpawner_SpawnProcess_NilClient(t *testing.T) {
	eventBus := pubsub.NewBroker[any]()
	submitter := &mockCommandSubmitter{}

	spawner := NewUnifiedProcessSpawner(UnifiedSpawnerConfig{
		CoordinatorClient: nil,
		WorkerClient:      nil,
		WorkDir:           "/test/workdir",
		Port:              8080,
		Submitter:         submitter,
		EventBus:          eventBus,
	})

	proc, err := spawner.SpawnProcess(context.Background(), "worker-1", repository.RoleWorker, SpawnOptions{})
	require.Error(t, err)
	require.Nil(t, proc)
	assert.Contains(t, err.Error(), "client is nil")
}

func TestSpawnOptions_AgentType(t *testing.T) {
	opts := SpawnOptions{
		AgentType: roles.AgentTypeImplementer,
	}
	assert.Equal(t, roles.AgentTypeImplementer, opts.AgentType)
}

func TestSpawnOptions_DefaultAgentType(t *testing.T) {
	opts := SpawnOptions{}
	// Default (zero value) should be AgentTypeGeneric (empty string)
	assert.Equal(t, roles.AgentTypeGeneric, opts.AgentType)
}

func TestUnifiedProcessSpawner_SpawnProcess_WithAgentType(t *testing.T) {
	testCases := []struct {
		name      string
		agentType roles.AgentType
	}{
		{"generic", roles.AgentTypeGeneric},
		{"implementer", roles.AgentTypeImplementer},
		{"reviewer", roles.AgentTypeReviewer},
		{"researcher", roles.AgentTypeResearcher},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockClient := mock.NewClient()
			eventBus := pubsub.NewBroker[any]()
			submitter := &mockCommandSubmitter{}

			spawner := NewUnifiedProcessSpawner(UnifiedSpawnerConfig{
				CoordinatorClient: mockClient,
				WorkerClient:      mockClient,
				WorkDir:           "/test/workdir",
				Port:              8080,
				Submitter:         submitter,
				EventBus:          eventBus,
			})

			opts := SpawnOptions{AgentType: tc.agentType}
			proc, err := spawner.SpawnProcess(context.Background(), "worker-1", repository.RoleWorker, opts)
			require.NoError(t, err)
			require.NotNil(t, proc)
			assert.Equal(t, "worker-1", proc.ID)
			assert.Equal(t, repository.RoleWorker, proc.Role)

			// Cleanup
			proc.Stop()
		})
	}
}

func TestUnifiedProcessSpawner_GenerateMCPConfig_HTTP(t *testing.T) {
	mockClient := mock.NewClient()
	spawner := &UnifiedProcessSpawnerImpl{
		workerClient: mockClient,
		port:         9999,
		workDir:      "/test",
	}

	config, err := spawner.generateWorkerMCPConfig("worker-1")
	require.NoError(t, err)
	assert.Contains(t, config, "9999")
	assert.Contains(t, config, "worker-1")
}

// openCodeMockClient is a mock client that returns ClientOpenCode type.
type openCodeMockClient struct {
	*mock.Client
}

func (c *openCodeMockClient) Type() client.ClientType {
	return client.ClientOpenCode
}

func TestUnifiedProcessSpawner_GenerateMCPConfig_OpenCode(t *testing.T) {
	mockClient := &openCodeMockClient{Client: mock.NewClient()}
	spawner := &UnifiedProcessSpawnerImpl{
		workerClient: mockClient,
		port:         9999,
		workDir:      "/test",
	}

	config, err := spawner.generateWorkerMCPConfig("worker-1")
	require.NoError(t, err)
	// OpenCode format uses {"mcp": {...}} wrapper, not {"mcpServers": {...}}
	assert.Contains(t, config, `"mcp"`)
	assert.Contains(t, config, `"perles-worker"`)
	assert.Contains(t, config, `"type":"remote"`)
	assert.Contains(t, config, "9999")
	assert.Contains(t, config, "worker-1")
	// Should NOT contain mcpServers (that's Claude format)
	assert.NotContains(t, config, "mcpServers")
}

func TestUnifiedProcessSpawner_GenerateCoordinatorMCPConfig_OpenCode(t *testing.T) {
	mockClient := &openCodeMockClient{Client: mock.NewClient()}
	spawner := &UnifiedProcessSpawnerImpl{
		coordinatorClient: mockClient,
		port:              9999,
		workDir:           "/test",
	}

	config, err := spawner.generateCoordinatorMCPConfig()
	require.NoError(t, err)
	// OpenCode format uses {"mcp": {...}} wrapper
	assert.Contains(t, config, `"mcp"`)
	assert.Contains(t, config, `"perles-orchestrator"`)
	assert.Contains(t, config, `"type":"remote"`)
	assert.Contains(t, config, "9999")
	// Should NOT contain mcpServers (that's Claude format)
	assert.NotContains(t, config, "mcpServers")
}

func TestUnifiedProcessSpawner_SpawnCoordinator_UsesSystemPromptOverride(t *testing.T) {
	var capturedConfig client.Config
	mockClient := mock.NewClient()
	mockClient.SpawnFunc = func(ctx context.Context, cfg client.Config) (client.HeadlessProcess, error) {
		capturedConfig = cfg
		return mock.NewProcess(), nil
	}

	eventBus := pubsub.NewBroker[any]()
	submitter := &mockCommandSubmitter{}

	spawner := NewUnifiedProcessSpawner(UnifiedSpawnerConfig{
		CoordinatorClient: mockClient,
		WorkerClient:      mockClient,
		WorkDir:           "/test/workdir",
		Port:              8080,
		Submitter:         submitter,
		EventBus:          eventBus,
	})

	customSystemPrompt := "Custom system prompt for coordinator"
	opts := SpawnOptions{
		SystemPromptOverride: customSystemPrompt,
	}

	proc, err := spawner.SpawnProcess(context.Background(), repository.CoordinatorID, repository.RoleCoordinator, opts)
	require.NoError(t, err)
	require.NotNil(t, proc)

	// Verify the system prompt override was used
	assert.Equal(t, customSystemPrompt, capturedConfig.SystemPrompt)
	// Initial prompt should use default since not overridden
	assert.NotEmpty(t, capturedConfig.Prompt)

	// Cleanup
	proc.Stop()
}

func TestUnifiedProcessSpawner_SpawnCoordinator_UsesInitialPromptOverride(t *testing.T) {
	var capturedConfig client.Config
	mockClient := mock.NewClient()
	mockClient.SpawnFunc = func(ctx context.Context, cfg client.Config) (client.HeadlessProcess, error) {
		capturedConfig = cfg
		return mock.NewProcess(), nil
	}

	eventBus := pubsub.NewBroker[any]()
	submitter := &mockCommandSubmitter{}

	spawner := NewUnifiedProcessSpawner(UnifiedSpawnerConfig{
		CoordinatorClient: mockClient,
		WorkerClient:      mockClient,
		WorkDir:           "/test/workdir",
		Port:              8080,
		Submitter:         submitter,
		EventBus:          eventBus,
	})

	customInitialPrompt := "Custom initial prompt for coordinator"
	opts := SpawnOptions{
		InitialPromptOverride: customInitialPrompt,
	}

	proc, err := spawner.SpawnProcess(context.Background(), repository.CoordinatorID, repository.RoleCoordinator, opts)
	require.NoError(t, err)
	require.NotNil(t, proc)

	// Verify the initial prompt override was used
	assert.Equal(t, customInitialPrompt, capturedConfig.Prompt)
	// System prompt should use default since not overridden
	assert.NotEmpty(t, capturedConfig.SystemPrompt)

	// Cleanup
	proc.Stop()
}

func TestUnifiedProcessSpawner_SpawnCoordinator_UsesDefaultWhenNoOverride(t *testing.T) {
	var capturedConfig client.Config
	mockClient := mock.NewClient()
	mockClient.SpawnFunc = func(ctx context.Context, cfg client.Config) (client.HeadlessProcess, error) {
		capturedConfig = cfg
		return mock.NewProcess(), nil
	}

	eventBus := pubsub.NewBroker[any]()
	submitter := &mockCommandSubmitter{}

	spawner := NewUnifiedProcessSpawner(UnifiedSpawnerConfig{
		CoordinatorClient: mockClient,
		WorkerClient:      mockClient,
		WorkDir:           "/test/workdir",
		Port:              8080,
		Submitter:         submitter,
		EventBus:          eventBus,
	})

	// Empty SpawnOptions - no overrides
	opts := SpawnOptions{}

	proc, err := spawner.SpawnProcess(context.Background(), repository.CoordinatorID, repository.RoleCoordinator, opts)
	require.NoError(t, err)
	require.NotNil(t, proc)

	// Verify default prompts were used (non-empty)
	assert.NotEmpty(t, capturedConfig.SystemPrompt)
	assert.NotEmpty(t, capturedConfig.Prompt)

	// Verify these are the actual default prompts by checking they contain expected content
	// The coordinator system prompt should contain coordinator-specific instructions
	assert.Contains(t, capturedConfig.SystemPrompt, "Coordinator")

	// Cleanup
	proc.Stop()
}

func TestUnifiedProcessSpawner_SpawnCoordinator_PassesBeadsDir(t *testing.T) {
	var capturedConfig client.Config
	mockClient := mock.NewClient()
	mockClient.SpawnFunc = func(ctx context.Context, cfg client.Config) (client.HeadlessProcess, error) {
		capturedConfig = cfg
		return mock.NewProcess(), nil
	}

	eventBus := pubsub.NewBroker[any]()
	submitter := &mockCommandSubmitter{}

	spawner := NewUnifiedProcessSpawner(UnifiedSpawnerConfig{
		CoordinatorClient: mockClient,
		WorkerClient:      mockClient,
		WorkDir:           "/test/workdir",
		Port:              8080,
		Submitter:         submitter,
		EventBus:          eventBus,
		BeadsDir:          "/custom/beads/path",
	})

	proc, err := spawner.SpawnProcess(context.Background(), repository.CoordinatorID, repository.RoleCoordinator, SpawnOptions{})
	require.NoError(t, err)
	require.NotNil(t, proc)

	// Verify BeadsDir was passed to client.Config
	assert.Equal(t, "/custom/beads/path", capturedConfig.BeadsDir)

	// Cleanup
	proc.Stop()
}

func TestUnifiedProcessSpawner_SpawnWorker_PassesBeadsDir(t *testing.T) {
	var capturedConfig client.Config
	mockClient := mock.NewClient()
	mockClient.SpawnFunc = func(ctx context.Context, cfg client.Config) (client.HeadlessProcess, error) {
		capturedConfig = cfg
		return mock.NewProcess(), nil
	}

	eventBus := pubsub.NewBroker[any]()
	submitter := &mockCommandSubmitter{}

	spawner := NewUnifiedProcessSpawner(UnifiedSpawnerConfig{
		CoordinatorClient: mockClient,
		WorkerClient:      mockClient,
		WorkDir:           "/test/workdir",
		Port:              8080,
		Submitter:         submitter,
		EventBus:          eventBus,
		BeadsDir:          "/custom/beads/path",
	})

	proc, err := spawner.SpawnProcess(context.Background(), "worker-1", repository.RoleWorker, SpawnOptions{})
	require.NoError(t, err)
	require.NotNil(t, proc)

	// Verify BeadsDir was passed to client.Config
	assert.Equal(t, "/custom/beads/path", capturedConfig.BeadsDir)

	// Cleanup
	proc.Stop()
}

func TestUnifiedProcessSpawner_SpawnCoordinator_UsesWorkflowConfigSystemPromptOverride(t *testing.T) {
	var capturedConfig client.Config
	mockClient := mock.NewClient()
	mockClient.SpawnFunc = func(ctx context.Context, cfg client.Config) (client.HeadlessProcess, error) {
		capturedConfig = cfg
		return mock.NewProcess(), nil
	}

	eventBus := pubsub.NewBroker[any]()
	submitter := &mockCommandSubmitter{}

	spawner := NewUnifiedProcessSpawner(UnifiedSpawnerConfig{
		CoordinatorClient: mockClient,
		WorkerClient:      mockClient,
		WorkDir:           "/test/workdir",
		Port:              8080,
		Submitter:         submitter,
		EventBus:          eventBus,
	})

	workflowSystemPrompt := "Workflow-level system prompt override"
	opts := SpawnOptions{
		WorkflowConfig: &roles.WorkflowConfig{
			SystemPromptOverride: workflowSystemPrompt,
		},
	}

	proc, err := spawner.SpawnProcess(context.Background(), repository.CoordinatorID, repository.RoleCoordinator, opts)
	require.NoError(t, err)
	require.NotNil(t, proc)

	// Verify the workflow config system prompt override was used
	assert.Equal(t, workflowSystemPrompt, capturedConfig.SystemPrompt)
	// Initial prompt should use default since not overridden
	assert.NotEmpty(t, capturedConfig.Prompt)

	// Cleanup
	proc.Stop()
}

func TestUnifiedProcessSpawner_SpawnCoordinator_UsesWorkflowConfigInitialPromptOverride(t *testing.T) {
	var capturedConfig client.Config
	mockClient := mock.NewClient()
	mockClient.SpawnFunc = func(ctx context.Context, cfg client.Config) (client.HeadlessProcess, error) {
		capturedConfig = cfg
		return mock.NewProcess(), nil
	}

	eventBus := pubsub.NewBroker[any]()
	submitter := &mockCommandSubmitter{}

	spawner := NewUnifiedProcessSpawner(UnifiedSpawnerConfig{
		CoordinatorClient: mockClient,
		WorkerClient:      mockClient,
		WorkDir:           "/test/workdir",
		Port:              8080,
		Submitter:         submitter,
		EventBus:          eventBus,
	})

	workflowInitialPrompt := "Workflow-level initial prompt override"
	opts := SpawnOptions{
		WorkflowConfig: &roles.WorkflowConfig{
			InitialPromptOverride: workflowInitialPrompt,
		},
	}

	proc, err := spawner.SpawnProcess(context.Background(), repository.CoordinatorID, repository.RoleCoordinator, opts)
	require.NoError(t, err)
	require.NotNil(t, proc)

	// Verify the workflow config initial prompt override was used
	assert.Equal(t, workflowInitialPrompt, capturedConfig.Prompt)
	// System prompt should use default since not overridden
	assert.NotEmpty(t, capturedConfig.SystemPrompt)

	// Cleanup
	proc.Stop()
}

func TestUnifiedProcessSpawner_SpawnCoordinator_WorkflowConfigTakesPrecedenceOverDirectOverride(t *testing.T) {
	var capturedConfig client.Config
	mockClient := mock.NewClient()
	mockClient.SpawnFunc = func(ctx context.Context, cfg client.Config) (client.HeadlessProcess, error) {
		capturedConfig = cfg
		return mock.NewProcess(), nil
	}

	eventBus := pubsub.NewBroker[any]()
	submitter := &mockCommandSubmitter{}

	spawner := NewUnifiedProcessSpawner(UnifiedSpawnerConfig{
		CoordinatorClient: mockClient,
		WorkerClient:      mockClient,
		WorkDir:           "/test/workdir",
		Port:              8080,
		Submitter:         submitter,
		EventBus:          eventBus,
	})

	directSystemPrompt := "Direct system prompt override (should be ignored)"
	directInitialPrompt := "Direct initial prompt override (should be ignored)"
	workflowSystemPrompt := "Workflow-level system prompt"
	workflowInitialPrompt := "Workflow-level initial prompt"

	opts := SpawnOptions{
		SystemPromptOverride:  directSystemPrompt,
		InitialPromptOverride: directInitialPrompt,
		WorkflowConfig: &roles.WorkflowConfig{
			SystemPromptOverride:  workflowSystemPrompt,
			InitialPromptOverride: workflowInitialPrompt,
		},
	}

	proc, err := spawner.SpawnProcess(context.Background(), repository.CoordinatorID, repository.RoleCoordinator, opts)
	require.NoError(t, err)
	require.NotNil(t, proc)

	// WorkflowConfig overrides should take precedence over direct overrides
	assert.Equal(t, workflowSystemPrompt, capturedConfig.SystemPrompt)
	assert.Equal(t, workflowInitialPrompt, capturedConfig.Prompt)

	// Cleanup
	proc.Stop()
}

func TestUnifiedProcessSpawner_SpawnCoordinator_WorkflowConfigBothOverrides(t *testing.T) {
	var capturedConfig client.Config
	mockClient := mock.NewClient()
	mockClient.SpawnFunc = func(ctx context.Context, cfg client.Config) (client.HeadlessProcess, error) {
		capturedConfig = cfg
		return mock.NewProcess(), nil
	}

	eventBus := pubsub.NewBroker[any]()
	submitter := &mockCommandSubmitter{}

	spawner := NewUnifiedProcessSpawner(UnifiedSpawnerConfig{
		CoordinatorClient: mockClient,
		WorkerClient:      mockClient,
		WorkDir:           "/test/workdir",
		Port:              8080,
		Submitter:         submitter,
		EventBus:          eventBus,
	})

	workflowSystemPrompt := "Workflow system prompt"
	workflowInitialPrompt := "Workflow initial prompt"

	opts := SpawnOptions{
		WorkflowConfig: &roles.WorkflowConfig{
			SystemPromptOverride:  workflowSystemPrompt,
			InitialPromptOverride: workflowInitialPrompt,
		},
	}

	proc, err := spawner.SpawnProcess(context.Background(), repository.CoordinatorID, repository.RoleCoordinator, opts)
	require.NoError(t, err)
	require.NotNil(t, proc)

	// Both workflow config overrides should be used
	assert.Equal(t, workflowSystemPrompt, capturedConfig.SystemPrompt)
	assert.Equal(t, workflowInitialPrompt, capturedConfig.Prompt)

	// Cleanup
	proc.Stop()
}

func TestUnifiedProcessSpawner_SpawnProcess_EmptyBeadsDir(t *testing.T) {
	var capturedConfig client.Config
	mockClient := mock.NewClient()
	mockClient.SpawnFunc = func(ctx context.Context, cfg client.Config) (client.HeadlessProcess, error) {
		capturedConfig = cfg
		return mock.NewProcess(), nil
	}

	eventBus := pubsub.NewBroker[any]()
	submitter := &mockCommandSubmitter{}

	spawner := NewUnifiedProcessSpawner(UnifiedSpawnerConfig{
		CoordinatorClient: mockClient,
		WorkerClient:      mockClient,
		WorkDir:           "/test/workdir",
		Port:              8080,
		Submitter:         submitter,
		EventBus:          eventBus,
		// BeadsDir not set - should be empty string
	})

	proc, err := spawner.SpawnProcess(context.Background(), "worker-1", repository.RoleWorker, SpawnOptions{})
	require.NoError(t, err)
	require.NotNil(t, proc)

	// Verify BeadsDir is empty (providers will handle this gracefully)
	assert.Empty(t, capturedConfig.BeadsDir)

	// Cleanup
	proc.Stop()
}

func TestUnifiedSpawner_SessionDirReplacement(t *testing.T) {
	var capturedConfig client.Config
	mockClient := mock.NewClient()
	mockClient.SpawnFunc = func(ctx context.Context, cfg client.Config) (client.HeadlessProcess, error) {
		capturedConfig = cfg
		return mock.NewProcess(), nil
	}

	eventBus := pubsub.NewBroker[any]()
	submitter := &mockCommandSubmitter{}

	sessionDir := "/home/user/.perles/sessions/myapp/2026-01-30/abc123"
	spawner := NewUnifiedProcessSpawner(UnifiedSpawnerConfig{
		CoordinatorClient: mockClient,
		WorkerClient:      mockClient,
		ObserverClient:    mockClient,
		WorkDir:           "/test/workdir",
		Port:              8080,
		Submitter:         submitter,
		EventBus:          eventBus,
		SessionDir:        sessionDir,
	})

	proc, err := spawner.SpawnProcess(context.Background(), "observer", repository.RoleObserver, SpawnOptions{})
	require.NoError(t, err)
	require.NotNil(t, proc)

	// The observer's idle prompt should have {{SESSION_DIR}} replaced with actual path
	// Note: The current observer prompt doesn't contain {{SESSION_DIR}} yet,
	// but the replacement logic is in place. When the prompt is updated in a
	// dependent task (perles-c1xmd.3), this test will verify proper replacement.
	// For now, verify that sessionDir is stored and the replacement doesn't panic.
	assert.NotEmpty(t, capturedConfig.Prompt)

	// Cleanup
	proc.Stop()
}

func TestUnifiedSpawner_SessionDirReplacement_WithPlaceholder(t *testing.T) {
	var capturedConfig client.Config
	mockClient := mock.NewClient()
	mockClient.SpawnFunc = func(ctx context.Context, cfg client.Config) (client.HeadlessProcess, error) {
		capturedConfig = cfg
		return mock.NewProcess(), nil
	}

	eventBus := pubsub.NewBroker[any]()
	submitter := &mockCommandSubmitter{}

	sessionDir := "/home/user/.perles/sessions/myapp/2026-01-30/abc123"
	spawner := NewUnifiedProcessSpawner(UnifiedSpawnerConfig{
		CoordinatorClient: mockClient,
		WorkerClient:      mockClient,
		ObserverClient:    mockClient,
		WorkDir:           "/test/workdir",
		Port:              8080,
		Submitter:         submitter,
		EventBus:          eventBus,
		SessionDir:        sessionDir,
	})

	proc, err := spawner.SpawnProcess(context.Background(), "observer", repository.RoleObserver, SpawnOptions{})
	require.NoError(t, err)
	require.NotNil(t, proc)

	// Verify the prompt does NOT contain the unreplaced placeholder
	// (if it existed, it would be replaced; if not present, this still passes)
	assert.NotContains(t, capturedConfig.Prompt, "{{SESSION_DIR}}")

	// Cleanup
	proc.Stop()
}

func TestUnifiedSpawner_SessionDirReplacement_EmptySessionDir(t *testing.T) {
	var capturedConfig client.Config
	mockClient := mock.NewClient()
	mockClient.SpawnFunc = func(ctx context.Context, cfg client.Config) (client.HeadlessProcess, error) {
		capturedConfig = cfg
		return mock.NewProcess(), nil
	}

	eventBus := pubsub.NewBroker[any]()
	submitter := &mockCommandSubmitter{}

	// Empty SessionDir - should not panic
	spawner := NewUnifiedProcessSpawner(UnifiedSpawnerConfig{
		CoordinatorClient: mockClient,
		WorkerClient:      mockClient,
		ObserverClient:    mockClient,
		WorkDir:           "/test/workdir",
		Port:              8080,
		Submitter:         submitter,
		EventBus:          eventBus,
		SessionDir:        "", // Empty
	})

	proc, err := spawner.SpawnProcess(context.Background(), "observer", repository.RoleObserver, SpawnOptions{})
	require.NoError(t, err)
	require.NotNil(t, proc)

	// Verify the prompt is still valid (non-empty)
	assert.NotEmpty(t, capturedConfig.Prompt)
	// The placeholder would be replaced with empty string if present
	assert.NotContains(t, capturedConfig.Prompt, "{{SESSION_DIR}}")

	// Cleanup
	proc.Stop()
}

func TestUnifiedSpawner_SessionDirConfig(t *testing.T) {
	// Verify that SessionDir is correctly stored in the config
	sessionDir := "/custom/session/path"

	spawner := NewUnifiedProcessSpawner(UnifiedSpawnerConfig{
		CoordinatorClient: mock.NewClient(),
		WorkerClient:      mock.NewClient(),
		WorkDir:           "/test/workdir",
		Port:              8080,
		Submitter:         &mockCommandSubmitter{},
		EventBus:          pubsub.NewBroker[any](),
		SessionDir:        sessionDir,
	})

	// Verify the spawner stored the sessionDir
	assert.Equal(t, sessionDir, spawner.sessionDir)
}

func TestUnifiedSpawner_RespectsObserverPromptOverride(t *testing.T) {
	var capturedConfig client.Config
	mockClient := mock.NewClient()
	mockClient.SpawnFunc = func(ctx context.Context, cfg client.Config) (client.HeadlessProcess, error) {
		capturedConfig = cfg
		return mock.NewProcess(), nil
	}

	eventBus := pubsub.NewBroker[any]()
	submitter := &mockCommandSubmitter{}

	spawner := NewUnifiedProcessSpawner(UnifiedSpawnerConfig{
		CoordinatorClient: mockClient,
		WorkerClient:      mockClient,
		ObserverClient:    mockClient,
		WorkDir:           "/test/workdir",
		Port:              8080,
		Submitter:         submitter,
		EventBus:          eventBus,
	})

	customPrompt := "Custom observer resume prompt for context recovery"
	opts := SpawnOptions{
		InitialPromptOverride: customPrompt,
	}

	proc, err := spawner.SpawnProcess(context.Background(), "observer", repository.RoleObserver, opts)
	require.NoError(t, err)
	require.NotNil(t, proc)

	// Verify the prompt override was used
	assert.Equal(t, customPrompt, capturedConfig.Prompt)
	// System prompt should still be the default observer system prompt
	assert.Contains(t, capturedConfig.SystemPrompt, "Observer")

	// Cleanup
	proc.Stop()
}

func TestUnifiedSpawner_ObserverPromptOverride_SessionDirReplaced(t *testing.T) {
	var capturedConfig client.Config
	mockClient := mock.NewClient()
	mockClient.SpawnFunc = func(ctx context.Context, cfg client.Config) (client.HeadlessProcess, error) {
		capturedConfig = cfg
		return mock.NewProcess(), nil
	}

	eventBus := pubsub.NewBroker[any]()
	submitter := &mockCommandSubmitter{}

	sessionDir := "/home/user/.perles/sessions/myapp/2026-01-30/abc123"
	spawner := NewUnifiedProcessSpawner(UnifiedSpawnerConfig{
		CoordinatorClient: mockClient,
		WorkerClient:      mockClient,
		ObserverClient:    mockClient,
		WorkDir:           "/test/workdir",
		Port:              8080,
		Submitter:         submitter,
		EventBus:          eventBus,
		SessionDir:        sessionDir,
	})

	// Use a prompt containing the placeholder to verify replacement occurs
	overridePrompt := "Observer resuming. Read notes from {{SESSION_DIR}}/observer/notes.md"
	opts := SpawnOptions{
		InitialPromptOverride: overridePrompt,
	}

	proc, err := spawner.SpawnProcess(context.Background(), "observer", repository.RoleObserver, opts)
	require.NoError(t, err)
	require.NotNil(t, proc)

	// Verify {{SESSION_DIR}} was replaced in the override prompt
	expectedPrompt := "Observer resuming. Read notes from /home/user/.perles/sessions/myapp/2026-01-30/abc123/observer/notes.md"
	assert.Equal(t, expectedPrompt, capturedConfig.Prompt)
	// Ensure no unreplaced placeholders remain
	assert.NotContains(t, capturedConfig.Prompt, "{{SESSION_DIR}}")

	// Cleanup
	proc.Stop()
}
