package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const metadataFilename = "metadata.json"

// Metadata is the JSON-serializable session information.
type Metadata struct {
	// SessionID is the unique session identifier (UUID).
	SessionID string `json:"session_id"`

	// StartTime is when the session was created.
	StartTime time.Time `json:"start_time"`

	// EndTime is when the session ended (zero if still running).
	EndTime time.Time `json:"end_time,omitzero"`

	// Status is the current session state.
	Status Status `json:"status"`

	// SessionDir is the session storage directory path.
	SessionDir string `json:"session_dir"`

	// EpicID is the bd epic ID associated with this session (if any).
	EpicID string `json:"epic_id,omitempty"`

	// AccountabilitySummaryPath is the path to the aggregated accountability summary.
	AccountabilitySummaryPath string `json:"accountability_summary_path,omitempty"`

	// CoordinatorID is the coordinator's process identifier.
	CoordinatorID string `json:"coordinator_id,omitempty"`

	// CoordinatorSessionRef is the headless client session reference
	// (e.g., Claude Code session ID) for resuming the coordinator.
	CoordinatorSessionRef string `json:"coordinator_session_ref,omitempty"`

	// Resumable indicates this session can be resumed.
	// Set to true after first successful coordinator turn when session ref is captured.
	Resumable bool `json:"resumable,omitempty"`

	// Workers contains metadata for each spawned worker.
	Workers []WorkerMetadata `json:"workers"`

	// ClientType is the AI client type (e.g., "claude").
	ClientType string `json:"client_type"`

	// Model is the AI model used (e.g., "sonnet").
	Model string `json:"model,omitempty"`

	// TokenUsage aggregates token usage across the session (sum of all processes).
	TokenUsage TokenUsageSummary `json:"token_usage,omitzero"`

	// CoordinatorTokenUsage tracks the coordinator's cumulative token usage.
	CoordinatorTokenUsage TokenUsageSummary `json:"coordinator_token_usage,omitzero"`

	// ApplicationName is the derived or configured name for the application.
	// Used for organizing sessions in centralized storage.
	ApplicationName string `json:"application_name,omitempty"`

	// WorkDir is the project working directory where the session was initiated.
	// This preserves the actual project location even when using git worktrees.
	WorkDir string `json:"work_dir,omitempty"`

	// DatePartition is the date-based partition (YYYY-MM-DD format) for organizing sessions.
	DatePartition string `json:"date_partition,omitempty"`

	// WorkflowCompletionStatus indicates the workflow outcome.
	// Values: "success", "partial", "aborted", or empty (not yet completed).
	WorkflowCompletionStatus string `json:"workflow_completion_status,omitempty"`

	// WorkflowCompletedAt is when signal_workflow_complete was called (zero if not completed).
	WorkflowCompletedAt time.Time `json:"workflow_completed_at,omitzero"`

	// WorkflowSummary is the completion summary provided by the coordinator.
	WorkflowSummary string `json:"workflow_summary,omitempty"`
}

// WorkerMetadata tracks individual worker lifecycle.
type WorkerMetadata struct {
	// ID is the worker identifier (e.g., "worker-1").
	ID string `json:"id"`

	// SpawnedAt is when the worker was created.
	SpawnedAt time.Time `json:"spawned_at"`

	// RetiredAt is when the worker was shut down (zero if still active).
	RetiredAt time.Time `json:"retired_at,omitzero"`

	// FinalPhase is the worker's last workflow phase before retirement.
	FinalPhase string `json:"final_phase,omitempty"`

	// HeadlessSessionRef is the AI client session reference for resuming this worker.
	HeadlessSessionRef string `json:"headless_session_ref,omitempty"`

	// WorkDir is this worker's working directory.
	// Currently same as session WorkDir, but supports future per-worker worktrees.
	WorkDir string `json:"work_dir,omitempty"`

	// TokenUsage tracks cumulative token usage for this worker.
	TokenUsage TokenUsageSummary `json:"token_usage,omitzero"`
}

// TokenUsageSummary aggregates token usage across the session.
type TokenUsageSummary struct {
	// ContextTokens is the current context window usage (input + cache).
	// This is the latest value, not cumulative (context fluctuates per turn).
	ContextTokens int `json:"context_tokens"`

	// TotalOutputTokens is the cumulative number of output tokens generated.
	TotalOutputTokens int `json:"total_output_tokens"`

	// TotalCostUSD is the cumulative total cost in USD.
	TotalCostUSD float64 `json:"total_cost_usd"`
}

// Save writes metadata to metadata.json in the given directory.
// It creates the directory if it doesn't exist.
func (m *Metadata) Save(dir string) error {
	// Ensure directory exists
	if err := os.MkdirAll(dir, 0750); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}

	// Marshal to JSON with indentation for readability
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling metadata: %w", err)
	}

	// Write to file
	path := filepath.Join(dir, metadataFilename)
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("writing metadata file: %w", err)
	}

	return nil
}

// Load reads metadata from metadata.json in the given directory.
func Load(dir string) (*Metadata, error) {
	path := filepath.Join(dir, metadataFilename)

	data, err := os.ReadFile(path) //nolint:gosec // G304: path is constructed from trusted dir parameter
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("metadata file not found: %w", err)
		}
		return nil, fmt.Errorf("reading metadata file: %w", err)
	}

	var m Metadata
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("unmarshaling metadata: %w", err)
	}

	return &m, nil
}
