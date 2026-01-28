// Package adapter provides the MCP tool adapter layer for the v2 orchestration architecture.
// The V2Adapter bridges MCP tool calls to v2 commands by parsing arguments, creating commands,
// and converting results to MCP format.
package adapter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/zjrosen/perles/internal/log"
	mcptypes "github.com/zjrosen/perles/internal/orchestration/mcp/types"
	"github.com/zjrosen/perles/internal/orchestration/v2/command"
	"github.com/zjrosen/perles/internal/orchestration/v2/processor"
	"github.com/zjrosen/perles/internal/orchestration/v2/prompt/roles"
	"github.com/zjrosen/perles/internal/orchestration/v2/repository"
)

// DefaultTimeout is the default timeout for command execution.
const DefaultTimeout = 30 * time.Second

// WorkflowConfigProvider returns workflow-specific prompt configuration for a given agent type.
// This allows the adapter to inject workflow customizations when spawning processes.
type WorkflowConfigProvider interface {
	GetWorkflowConfig(agentType roles.AgentType) *roles.WorkflowConfig
}

// V2Adapter bridges MCP tool calls to v2 commands.
// It parses MCP arguments, creates commands, submits them to the processor,
// and converts results back to MCP format.
//
// For read-only operations (query_worker_state), the adapter
// reads directly from repositories without going through the CommandProcessor,
// since these operations don't mutate state and don't require FIFO ordering.
type V2Adapter struct {
	processor        *processor.CommandProcessor
	processRepo      repository.ProcessRepository
	taskRepo         repository.TaskRepository
	queueRepo        repository.QueueRepository
	workflowProvider WorkflowConfigProvider
	timeout          time.Duration
	sessionID        string // Session ID for accountability summary generation
	workDir          string // Working directory (project root or worktree path)
	sessionDir       string // Session directory for accountability summaries
}

// Option configures the V2Adapter.
type Option func(*V2Adapter)

// WithTimeout sets the default timeout for command execution.
func WithTimeout(timeout time.Duration) Option {
	return func(a *V2Adapter) {
		a.timeout = timeout
	}
}

// WithProcessRepository sets the process repository for read-only operations.
func WithProcessRepository(repo repository.ProcessRepository) Option {
	return func(a *V2Adapter) {
		a.processRepo = repo
	}
}

// WithTaskRepository sets the task repository for read-only operations.
func WithTaskRepository(repo repository.TaskRepository) Option {
	return func(a *V2Adapter) {
		a.taskRepo = repo
	}
}

// WithQueueRepository sets the queue repository for read-only operations.
func WithQueueRepository(repo repository.QueueRepository) Option {
	return func(a *V2Adapter) {
		a.queueRepo = repo
	}
}

// WithSessionID sets the session ID, work directory, and session directory for accountability
// summary generation. The sessionDir is the actual path where session files are stored
// (e.g., ~/.perles/sessions/{app}/{date}/{id}/ for centralized storage).
func WithSessionID(sessionID, workDir, sessionDir string) Option {
	return func(a *V2Adapter) {
		a.sessionID = sessionID
		a.workDir = workDir
		a.sessionDir = sessionDir
	}
}

// NewV2Adapter creates a new V2Adapter with the given processor.
func NewV2Adapter(proc *processor.CommandProcessor, opts ...Option) *V2Adapter {
	a := &V2Adapter{
		processor: proc,
		timeout:   DefaultTimeout,
	}
	for _, opt := range opts {
		opt(a)
	}
	return a
}

// SetWorkflowConfigProvider sets the workflow config provider after construction.
// This is useful when the provider (e.g., the orchestration Model) is created after
// the adapter, but needs to provide workflow configuration for process spawning.
func (a *V2Adapter) SetWorkflowConfigProvider(provider WorkflowConfigProvider) {
	a.workflowProvider = provider
}

// ===========================================================================
// MCP Argument Types
// ===========================================================================

// retireWorkerArgs holds arguments for retire_worker tool.
type retireWorkerArgs struct {
	WorkerID string `json:"worker_id"`
	Reason   string `json:"reason,omitempty"`
}

// replaceWorkerArgs holds arguments for replace_worker tool.
type replaceWorkerArgs struct {
	WorkerID string `json:"worker_id"`
	Reason   string `json:"reason,omitempty"`
}

// sendToWorkerArgs holds arguments for send_to_worker tool.
type sendToWorkerArgs struct {
	WorkerID string `json:"worker_id"`
	Message  string `json:"message"`
}

// assignTaskArgs holds arguments for assign_task tool.
type assignTaskArgs struct {
	WorkerID string `json:"worker_id"`
	TaskID   string `json:"task_id"`
	Summary  string `json:"summary,omitempty"`
	ThreadID string `json:"thread_id,omitempty"`
}

// assignTaskReviewArgs holds arguments for assign_task_review tool.
type assignTaskReviewArgs struct {
	ReviewerID    string `json:"reviewer_id"`
	TaskID        string `json:"task_id"`
	ImplementerID string `json:"implementer_id"`
	Summary       string `json:"summary,omitempty"`
	ReviewType    string `json:"review_type,omitempty"`
}

// assignReviewFeedbackArgs holds arguments for assign_review_feedback tool.
type assignReviewFeedbackArgs struct {
	ImplementerID string `json:"implementer_id"`
	TaskID        string `json:"task_id"`
	Feedback      string `json:"feedback"`
}

// approveCommitArgs holds arguments for approve_commit tool.
type approveCommitArgs struct {
	ImplementerID string `json:"implementer_id"`
	TaskID        string `json:"task_id"`
	CommitMessage string `json:"commit_message,omitempty"`
}

// reportImplementationCompleteArgs holds arguments for report_implementation_complete tool.
type reportImplementationCompleteArgs struct {
	Summary string `json:"summary"`
}

// reportReviewVerdictArgs holds arguments for report_review_verdict tool.
type reportReviewVerdictArgs struct {
	Verdict  string `json:"verdict"`
	Comments string `json:"comments,omitempty"`
}

// spawnWorkerArgs holds arguments for spawn_worker tool.
type spawnWorkerArgs struct {
	AgentType string `json:"agent_type,omitempty"`
}

// signalWorkflowCompleteArgs holds arguments for signal_workflow_complete tool.
type signalWorkflowCompleteArgs struct {
	Status      string `json:"status"`                 // Required: "success", "partial", or "aborted"
	Summary     string `json:"summary"`                // Required: summary of what was accomplished
	EpicID      string `json:"epic_id,omitempty"`      // Optional: epic ID that was completed
	TasksClosed int    `json:"tasks_closed,omitempty"` // Optional: number of tasks closed during workflow
}

// ===========================================================================
// Process Lifecycle Handlers
// ===========================================================================

// ErrAgentTypeNotFound is returned when an invalid agent_type is specified.
var ErrAgentTypeNotFound = errors.New("agent_type not found: must be one of 'implementer', 'reviewer', 'researcher', or omitted for generic")

// HandleSpawnProcess handles the spawn_process MCP tool call.
// For workers, it spawns a new idle worker ready for task assignment.
// Supports optional agent_type parameter for specialized workers.
func (a *V2Adapter) HandleSpawnProcess(ctx context.Context, args json.RawMessage) (*mcptypes.ToolCallResult, error) {
	var parsed spawnWorkerArgs
	if len(args) > 0 {
		if err := json.Unmarshal(args, &parsed); err != nil {
			return nil, fmt.Errorf("invalid arguments: %w", err)
		}
	}

	// Parse and validate agent_type
	agentType := roles.AgentTypeGeneric // Default to generic
	if parsed.AgentType != "" {
		agentType = roles.AgentType(parsed.AgentType)
		if !agentType.IsValid() {
			return nil, ErrAgentTypeNotFound
		}
	}

	// Build command options
	opts := []command.SpawnProcessOption{command.WithAgentType(agentType)}

	// Get workflow config if provider is configured
	if a.workflowProvider != nil {
		if wfConfig := a.workflowProvider.GetWorkflowConfig(agentType); wfConfig != nil {
			opts = append(opts, command.WithWorkflowConfig(wfConfig))
		} else {
			log.Debug(log.CatOrch, "HandleSpawnProcess: workflowProvider returned nil config")
		}
	}

	// Create command with options
	cmd := command.NewSpawnProcessCommand(command.SourceMCPTool, repository.RoleWorker, opts...)

	result, err := a.submitWithTimeout(ctx, cmd)
	if err != nil {
		return nil, fmt.Errorf("spawn_process command failed: %w", err)
	}

	if !result.Success {
		return mcptypes.ErrorResult(result.Error.Error()), nil
	}

	// Extract ProcessID from result
	processID := extractProcessID(result.Data)
	return mcptypes.SuccessResult(fmt.Sprintf("Process %s spawned the process will notify you when they are ready. DO NOT assign work until they have sent you a ready signal", processID)), nil
}

// HandleRetireProcess handles the retire_process MCP tool call.
func (a *V2Adapter) HandleRetireProcess(ctx context.Context, args json.RawMessage) (*mcptypes.ToolCallResult, error) {
	var parsed retireWorkerArgs
	if err := json.Unmarshal(args, &parsed); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	cmd := command.NewRetireProcessCommand(command.SourceMCPTool, parsed.WorkerID, parsed.Reason)
	if err := cmd.Validate(); err != nil {
		return nil, fmt.Errorf("retire_process command validation failed: %w", err)
	}

	result, err := a.submitWithTimeout(ctx, cmd)
	if err != nil {
		return nil, fmt.Errorf("retire_process command failed: %w", err)
	}

	if !result.Success {
		return mcptypes.ErrorResult(result.Error.Error()), nil
	}

	return mcptypes.SuccessResult(fmt.Sprintf("Process %s retired successfully", parsed.WorkerID)), nil
}

// HandleReplaceProcess handles the replace_process MCP tool call.
func (a *V2Adapter) HandleReplaceProcess(ctx context.Context, args json.RawMessage) (*mcptypes.ToolCallResult, error) {
	var parsed replaceWorkerArgs
	if err := json.Unmarshal(args, &parsed); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	cmd := command.NewReplaceProcessCommand(command.SourceMCPTool, parsed.WorkerID, parsed.Reason)
	if err := cmd.Validate(); err != nil {
		return nil, fmt.Errorf("replace_process command validation failed: %w", err)
	}

	result, err := a.submitWithTimeout(ctx, cmd)
	if err != nil {
		return nil, fmt.Errorf("replace_process command failed: %w", err)
	}

	if !result.Success {
		return mcptypes.ErrorResult(result.Error.Error()), nil
	}

	return mcptypes.SuccessResult(fmt.Sprintf("Process %s replaced successfully", parsed.WorkerID)), nil
}

// processStatusToWorkerStatus converts ProcessStatus to the string format expected by the API.
// This maintains backward compatibility with the old WorkerStatus string representation.
func processStatusToWorkerStatus(status repository.ProcessStatus) string {
	switch status {
	case repository.StatusPending, repository.StatusStarting:
		return "starting"
	case repository.StatusReady:
		return "ready"
	case repository.StatusWorking:
		return "working"
	case repository.StatusRetired, repository.StatusFailed:
		return "retired"
	default:
		return "unknown"
	}
}

// formatContextUsage returns a human-readable string for context usage.
func formatContextUsage(contextTokens, contextWindow int) string {
	tokensK := contextTokens / 1000
	windowK := contextWindow / 1000
	percentage := (contextTokens * 100) / contextWindow
	return fmt.Sprintf("%dk/%dk (%d%%)", tokensK, windowK, percentage)
}

// queryWorkerStateArgs holds arguments for query_worker_state tool.
type queryWorkerStateArgs struct {
	WorkerID string `json:"worker_id,omitempty"`
	TaskID   string `json:"task_id,omitempty"`
}

// workerStateInfo represents worker state in query_worker_state response.
// Field names match coordinator.go for backward compatibility.
type workerStateInfo struct {
	WorkerID     string `json:"worker_id"`
	Status       string `json:"status"`
	Phase        string `json:"phase"`
	AgentType    string `json:"agent_type,omitempty"`
	TaskID       string `json:"task_id,omitempty"`
	SessionID    string `json:"session_id,omitempty"`
	QueueSize    int    `json:"queue_size,omitempty"`
	ContextUsage string `json:"context_usage,omitempty"`
	StartedAt    string `json:"started_at"`
	CreatedAt    string `json:"created_at,omitempty"`
	RetiredAt    string `json:"retired_at,omitempty"`
	// Task details if assigned
	TaskStatus  string `json:"task_status,omitempty"`
	TaskStarted string `json:"task_started,omitempty"`
	ReviewerID  string `json:"reviewer_id,omitempty"`
}

// taskAssignmentInfo represents a task assignment in the query_worker_state response.
type taskAssignmentInfo struct {
	TaskID          string `json:"task_id"`
	Implementer     string `json:"implementer"`
	Reviewer        string `json:"reviewer,omitempty"`
	Status          string `json:"status"`
	StartedAt       string `json:"started_at,omitempty"`
	ReviewStartedAt string `json:"review_started_at,omitempty"`
}

// workerStateResponse is the response format for query_worker_state tool.
type workerStateResponse struct {
	Workers        []workerStateInfo             `json:"workers"`
	ReadyWorkers   []string                      `json:"ready_workers"`
	RetiredWorkers []string                      `json:"retired_workers"`
	FailedWorkers  []string                      `json:"failed_workers"`
	Tasks          map[string]taskAssignmentInfo `json:"tasks"`
}

// HandleQueryWorkerState handles the query_worker_state MCP tool call.
// This is a read-only operation that reads directly from repositories
// without going through the CommandProcessor, since it doesn't mutate state.
//
// Supports optional filters:
//   - worker_id: filter to specific worker
//   - task_id: filter to worker assigned to specific task
//
// Returns a response with:
//   - workers: array of worker state info
//   - ready_workers: array of worker IDs that are Ready status with no assigned task
func (a *V2Adapter) HandleQueryWorkerState(_ context.Context, args json.RawMessage) (*mcptypes.ToolCallResult, error) {
	if a.processRepo == nil {
		return nil, fmt.Errorf("process repository not configured for read-only operations")
	}

	var parsed queryWorkerStateArgs
	if len(args) > 0 {
		if err := json.Unmarshal(args, &parsed); err != nil {
			return nil, fmt.Errorf("invalid arguments: %w", err)
		}
	}

	// Get all active workers from repository
	workers := a.processRepo.ActiveWorkers()

	// Build response
	response := workerStateResponse{
		Workers:        make([]workerStateInfo, 0),
		ReadyWorkers:   make([]string, 0),
		RetiredWorkers: make([]string, 0),
		FailedWorkers:  make([]string, 0),
		Tasks:          make(map[string]taskAssignmentInfo),
	}

	// Populate retired workers (gracefully retired)
	retiredWorkers := a.processRepo.RetiredWorkers()
	for _, p := range retiredWorkers {
		response.RetiredWorkers = append(response.RetiredWorkers, p.ID)
	}

	// Populate failed workers (session expired, crashed, etc.)
	failedWorkers := a.processRepo.FailedWorkers()
	for _, p := range failedWorkers {
		response.FailedWorkers = append(response.FailedWorkers, p.ID)
	}

	// Populate all tasks
	if a.taskRepo != nil {
		allTasks := a.taskRepo.All()
		for _, task := range allTasks {
			info := taskAssignmentInfo{
				TaskID:      task.TaskID,
				Implementer: task.Implementer,
				Reviewer:    task.Reviewer,
				Status:      string(task.Status),
			}
			if !task.StartedAt.IsZero() {
				info.StartedAt = task.StartedAt.Format("2006-01-02T15:04:05Z07:00")
			}
			if !task.ReviewStartedAt.IsZero() {
				info.ReviewStartedAt = task.ReviewStartedAt.Format("2006-01-02T15:04:05Z07:00")
			}
			response.Tasks[task.TaskID] = info
		}
	}

	for _, p := range workers {
		// Filter by worker_id if specified
		if parsed.WorkerID != "" && p.ID != parsed.WorkerID {
			continue
		}

		// Filter by task_id if specified
		if parsed.TaskID != "" && p.TaskID != parsed.TaskID {
			continue
		}

		// Get queue size if queue repository is available
		queueSize := 0
		if a.queueRepo != nil {
			queueSize = a.queueRepo.Size(p.ID)
		}

		// Build worker info matching coordinator format
		phase := ""
		if p.Phase != nil {
			phase = string(*p.Phase)
		}
		info := workerStateInfo{
			WorkerID:  p.ID,
			Status:    processStatusToWorkerStatus(p.Status),
			Phase:     phase,
			AgentType: p.AgentType.String(),
			TaskID:    p.TaskID,
			SessionID: p.SessionID,
			QueueSize: queueSize,
			StartedAt: p.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			CreatedAt: p.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		}

		// Add retired_at if worker is retired
		if !p.RetiredAt.IsZero() {
			info.RetiredAt = p.RetiredAt.Format("2006-01-02T15:04:05Z07:00")
		}

		// Add context usage if metrics available
		if p.Metrics != nil && p.Metrics.TokensUsed > 0 && p.Metrics.TotalTokens > 0 {
			info.ContextUsage = formatContextUsage(p.Metrics.TokensUsed, p.Metrics.TotalTokens)
		}

		// Get current task assignment if task repository is available
		if a.taskRepo != nil && p.TaskID != "" {
			if task, err := a.taskRepo.Get(p.TaskID); err == nil {
				info.TaskStatus = string(task.Status)
				if !task.StartedAt.IsZero() {
					info.TaskStarted = task.StartedAt.Format("2006-01-02T15:04:05Z07:00")
				}
				info.ReviewerID = task.Reviewer
			}
		}

		response.Workers = append(response.Workers, info)

		// Track ready workers (Ready status with no task)
		if p.Status == repository.StatusReady && p.TaskID == "" {
			response.ReadyWorkers = append(response.ReadyWorkers, p.ID)
		}
	}

	jsonBytes, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal worker state: %w", err)
	}

	return mcptypes.StructuredResult(string(jsonBytes), response), nil
}

// ===========================================================================
// Messaging Handlers (Batch 2)
// ===========================================================================

// HandleSendToWorker handles the send_to_worker MCP tool call.
func (a *V2Adapter) HandleSendToWorker(ctx context.Context, args json.RawMessage) (*mcptypes.ToolCallResult, error) {
	var parsed sendToWorkerArgs
	if err := json.Unmarshal(args, &parsed); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	cmd := command.NewSendToProcessCommand(command.SourceMCPTool, parsed.WorkerID, parsed.Message)
	if err := cmd.Validate(); err != nil {
		return nil, fmt.Errorf("send_to_worker command validation failed: %w", err)
	}

	result, err := a.submitWithTimeout(ctx, cmd)
	if err != nil {
		return nil, fmt.Errorf("send_to_worker command failed: %w", err)
	}

	if !result.Success {
		return mcptypes.ErrorResult(result.Error.Error()), nil
	}

	return mcptypes.SuccessResult(fmt.Sprintf("Message sent to worker %s", parsed.WorkerID)), nil
}

// ===========================================================================
// Task Assignment Handlers (Batch 3-4)
// ===========================================================================

// HandleAssignTask handles the assign_task MCP tool call.
func (a *V2Adapter) HandleAssignTask(ctx context.Context, args json.RawMessage) (*mcptypes.ToolCallResult, error) {
	var parsed assignTaskArgs
	if err := json.Unmarshal(args, &parsed); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	cmd := command.NewAssignTaskCommand(command.SourceMCPTool, parsed.WorkerID, parsed.TaskID, parsed.Summary, parsed.ThreadID)
	err := cmd.Validate()
	if err != nil {
		return nil, fmt.Errorf("assign_task command validation failed: %w", err)
	}

	result, err := a.submitWithTimeout(ctx, cmd)
	if err != nil {
		return nil, fmt.Errorf("assign_task command failed: %w", err)
	}

	if !result.Success {
		return mcptypes.ErrorResult(result.Error.Error()), nil
	}

	return mcptypes.SuccessResult(fmt.Sprintf("Task %s assigned to worker %s", parsed.TaskID, parsed.WorkerID)), nil
}

// HandleAssignTaskReview handles the assign_task_review MCP tool call.
func (a *V2Adapter) HandleAssignTaskReview(ctx context.Context, args json.RawMessage) (*mcptypes.ToolCallResult, error) {
	var parsed assignTaskReviewArgs
	if err := json.Unmarshal(args, &parsed); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	// Parse review type with default to complex
	reviewType := command.ReviewTypeComplex
	if parsed.ReviewType == string(command.ReviewTypeSimple) {
		reviewType = command.ReviewTypeSimple
	}

	cmd := command.NewAssignReviewCommand(command.SourceMCPTool, parsed.ReviewerID, parsed.TaskID, parsed.ImplementerID, reviewType)
	if err := cmd.Validate(); err != nil {
		return nil, fmt.Errorf("assign_task_review command validation failed: %w", err)
	}

	result, err := a.submitWithTimeout(ctx, cmd)
	if err != nil {
		return nil, fmt.Errorf("assign_task_review command failed: %w", err)
	}

	if !result.Success {
		return mcptypes.ErrorResult(result.Error.Error()), nil
	}

	return mcptypes.SuccessResult(fmt.Sprintf("Review of task %s assigned to worker %s", parsed.TaskID, parsed.ReviewerID)), nil
}

// HandleAssignReviewFeedback handles the assign_review_feedback MCP tool call.
// This transitions an implementer to the AddressingFeedback phase with a message.
func (a *V2Adapter) HandleAssignReviewFeedback(ctx context.Context, args json.RawMessage) (*mcptypes.ToolCallResult, error) {
	var parsed assignReviewFeedbackArgs
	if err := json.Unmarshal(args, &parsed); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	cmd := command.NewAssignReviewFeedbackCommand(command.SourceMCPTool, parsed.ImplementerID, parsed.TaskID, parsed.Feedback)
	if err := cmd.Validate(); err != nil {
		return nil, fmt.Errorf("assign_review_feedback command validation failed: %w", err)
	}

	result, err := a.submitWithTimeout(ctx, cmd)
	if err != nil {
		return nil, fmt.Errorf("assign_review_feedback command failed: %w", err)
	}

	if !result.Success {
		return mcptypes.ErrorResult(result.Error.Error()), nil
	}

	return mcptypes.SuccessResult(fmt.Sprintf("Review feedback sent to worker %s for task %s", parsed.ImplementerID, parsed.TaskID)), nil
}

// HandleApproveCommit handles the approve_commit MCP tool call.
func (a *V2Adapter) HandleApproveCommit(ctx context.Context, args json.RawMessage) (*mcptypes.ToolCallResult, error) {
	var parsed approveCommitArgs
	if err := json.Unmarshal(args, &parsed); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	cmd := command.NewApproveCommitCommand(command.SourceMCPTool, parsed.ImplementerID, parsed.TaskID)
	if err := cmd.Validate(); err != nil {
		return nil, fmt.Errorf("approve_commit command validation failed: %w", err)
	}

	result, err := a.submitWithTimeout(ctx, cmd)
	if err != nil {
		return nil, fmt.Errorf("approve_commit command failed: %w", err)
	}

	if !result.Success {
		return mcptypes.ErrorResult(result.Error.Error()), nil
	}

	return mcptypes.SuccessResult(fmt.Sprintf("Commit approved for worker %s on task %s", parsed.ImplementerID, parsed.TaskID)), nil
}

// ===========================================================================
// State Transition Handlers (Batch 5)
// ===========================================================================

// ReportImplementationCompleteResult contains the result of report_implementation_complete.
// This allows the MCP layer to access the task's ThreadID for Fabric replies.
type ReportImplementationCompleteResult struct {
	Success  bool
	ThreadID string // Fabric thread ID for the task conversation
	Message  string
}

// HandleReportImplementationComplete handles the report_implementation_complete MCP tool call.
// Returns ReportImplementationCompleteResult with ThreadID for Fabric integration.
func (a *V2Adapter) HandleReportImplementationComplete(ctx context.Context, args json.RawMessage, workerID string) (*ReportImplementationCompleteResult, error) {
	var parsed reportImplementationCompleteArgs
	if err := json.Unmarshal(args, &parsed); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	cmd := command.NewReportCompleteCommand(command.SourceMCPTool, workerID, parsed.Summary)
	if err := cmd.Validate(); err != nil {
		return nil, fmt.Errorf("report_implementation_complete command validation failed: %w", err)
	}

	result, err := a.submitWithTimeout(ctx, cmd)
	if err != nil {
		return nil, fmt.Errorf("report_implementation_complete command failed: %w", err)
	}

	if !result.Success {
		return &ReportImplementationCompleteResult{
			Success: false,
			Message: result.Error.Error(),
		}, nil
	}

	// Look up the task's ThreadID for Fabric reply
	var threadID string
	if a.taskRepo != nil {
		task, err := a.taskRepo.GetByWorker(workerID)
		if err == nil && task != nil {
			threadID = task.ThreadID
		}
	}

	return &ReportImplementationCompleteResult{
		Success:  true,
		ThreadID: threadID,
		Message:  "Implementation complete signal sent",
	}, nil
}

// ReportReviewVerdictResult contains the result of report_review_verdict.
// This allows the MCP layer to access the task's ThreadID for Fabric replies.
type ReportReviewVerdictResult struct {
	Success  bool
	ThreadID string // Fabric thread ID for the task conversation
	Verdict  string // "APPROVED" or "DENIED"
	Comments string // Review comments
	Message  string
}

// HandleReportReviewVerdict handles the report_review_verdict MCP tool call.
// Returns ReportReviewVerdictResult with ThreadID for Fabric integration.
func (a *V2Adapter) HandleReportReviewVerdict(ctx context.Context, args json.RawMessage, workerID string) (*ReportReviewVerdictResult, error) {
	var parsed reportReviewVerdictArgs
	if err := json.Unmarshal(args, &parsed); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	if parsed.Verdict == "" {
		return nil, fmt.Errorf("verdict is required")
	}

	// Convert string verdict to command.Verdict
	var verdict command.Verdict
	switch parsed.Verdict {
	case "APPROVED":
		verdict = command.VerdictApproved
	case "DENIED":
		verdict = command.VerdictDenied
	default:
		return nil, fmt.Errorf("invalid verdict: must be APPROVED or DENIED")
	}

	cmd := command.NewReportVerdictCommand(command.SourceMCPTool, workerID, verdict, parsed.Comments)
	if err := cmd.Validate(); err != nil {
		return nil, fmt.Errorf("report_review_verdict command validation failed: %w", err)
	}

	result, err := a.submitWithTimeout(ctx, cmd)
	if err != nil {
		return nil, fmt.Errorf("report_review_verdict command failed: %w", err)
	}

	if !result.Success {
		return &ReportReviewVerdictResult{
			Success: false,
			Message: result.Error.Error(),
		}, nil
	}

	// Look up the task's ThreadID for Fabric reply
	var threadID string
	if a.taskRepo != nil {
		task, err := a.taskRepo.GetByWorker(workerID)
		if err == nil && task != nil {
			threadID = task.ThreadID
		}
	}

	return &ReportReviewVerdictResult{
		Success:  true,
		ThreadID: threadID,
		Verdict:  parsed.Verdict,
		Comments: parsed.Comments,
		Message:  fmt.Sprintf("Review verdict %s submitted", parsed.Verdict),
	}, nil
}

// ===========================================================================
// BD Integration Handlers (Batch 6)
// ===========================================================================

// markTaskCompleteArgs holds arguments for mark_task_complete tool.
type markTaskCompleteArgs struct {
	TaskID string `json:"task_id"`
}

// markTaskFailedArgs holds arguments for mark_task_failed tool.
type markTaskFailedArgs struct {
	TaskID string `json:"task_id"`
	Reason string `json:"reason"`
}

// HandleMarkTaskComplete handles the mark_task_complete MCP tool call.
// Routes through the v2 command processor using CmdMarkTaskComplete.
func (a *V2Adapter) HandleMarkTaskComplete(ctx context.Context, args json.RawMessage) (*mcptypes.ToolCallResult, error) {
	var parsed markTaskCompleteArgs
	if err := json.Unmarshal(args, &parsed); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	cmd := command.NewMarkTaskCompleteCommand(command.SourceMCPTool, parsed.TaskID)
	if err := cmd.Validate(); err != nil {
		return nil, fmt.Errorf("mark_task_complete command validation failed: %w", err)
	}

	result, err := a.submitWithTimeout(ctx, cmd)
	if err != nil {
		return nil, fmt.Errorf("mark_task_complete command failed: %w", err)
	}

	if !result.Success {
		return mcptypes.ErrorResult(result.Error.Error()), nil
	}

	// Return structured response for consistency with existing behavior
	response := map[string]any{
		"status":  "success",
		"message": fmt.Sprintf("Task %s marked as completed", parsed.TaskID),
	}
	data, _ := json.MarshalIndent(response, "", "  ")
	return mcptypes.StructuredResult(string(data), response), nil
}

// HandleMarkTaskFailed handles the mark_task_failed MCP tool call.
// Routes through the v2 command processor using CmdMarkTaskFailed.
func (a *V2Adapter) HandleMarkTaskFailed(ctx context.Context, args json.RawMessage) (*mcptypes.ToolCallResult, error) {
	var parsed markTaskFailedArgs
	if err := json.Unmarshal(args, &parsed); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	cmd := command.NewMarkTaskFailedCommand(command.SourceMCPTool, parsed.TaskID, parsed.Reason)
	if err := cmd.Validate(); err != nil {
		return nil, fmt.Errorf("mark_task_failed command validation failed: %w", err)
	}

	result, err := a.submitWithTimeout(ctx, cmd)
	if err != nil {
		return nil, fmt.Errorf("mark_task_failed command failed: %w", err)
	}

	if !result.Success {
		return mcptypes.ErrorResult(result.Error.Error()), nil
	}

	return mcptypes.SuccessResult(fmt.Sprintf("Task %s marked as failed with comment: %s", parsed.TaskID, parsed.Reason)), nil
}

// ===========================================================================
// Worker Control Handlers
// ===========================================================================

// HandleStopProcess stops a process (worker or coordinator) programmatically.
// Used by MCP tools to enable coordinator-initiated stops.
func (a *V2Adapter) HandleStopProcess(processID string, force bool, reason string) error {
	cmd := command.NewStopProcessCommand(command.SourceMCPTool, processID, force, reason)

	if err := cmd.Validate(); err != nil {
		return fmt.Errorf("invalid stop process command: %w", err)
	}

	return a.processor.Submit(cmd)
}

// ===========================================================================
// Aggregation Handlers
// ===========================================================================

// generateAccountabilitySummaryArgs holds arguments for generate_accountability_summary tool.
type generateAccountabilitySummaryArgs struct {
	WorkerID string `json:"worker_id"`
}

// HandleGenerateAccountabilitySummary handles the generate_accountability_summary MCP tool call.
// It sends the AggregationWorkerPrompt to the specified worker.
// Uses the adapter's stored sessionDir for centralized session storage support.
func (a *V2Adapter) HandleGenerateAccountabilitySummary(ctx context.Context, args json.RawMessage) (*mcptypes.ToolCallResult, error) {
	var parsed generateAccountabilitySummaryArgs
	if err := json.Unmarshal(args, &parsed); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	if a.sessionDir == "" {
		return nil, fmt.Errorf("session directory not configured on adapter")
	}

	cmd := command.NewGenerateAccountabilitySummaryCommand(command.SourceMCPTool, parsed.WorkerID, a.sessionDir)
	if err := cmd.Validate(); err != nil {
		return nil, fmt.Errorf("generate_accountability_summary command validation failed: %w", err)
	}

	result, err := a.submitWithTimeout(ctx, cmd)
	if err != nil {
		return nil, fmt.Errorf("generate_accountability_summary command failed: %w", err)
	}

	if !result.Success {
		return mcptypes.ErrorResult(result.Error.Error()), nil
	}

	return mcptypes.SuccessResult(fmt.Sprintf("Accountability summary task assigned to worker %s", parsed.WorkerID)), nil
}

// ===========================================================================
// Workflow Lifecycle Handlers
// ===========================================================================

// HandleSignalWorkflowComplete handles the signal_workflow_complete MCP tool call.
// This signals that the workflow has completed with a given status and summary.
// Routes through the v2 command processor using CmdSignalWorkflowComplete.
func (a *V2Adapter) HandleSignalWorkflowComplete(ctx context.Context, args json.RawMessage) (*mcptypes.ToolCallResult, error) {
	var parsed signalWorkflowCompleteArgs
	if err := json.Unmarshal(args, &parsed); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	// Validate required fields
	if parsed.Status == "" {
		return nil, fmt.Errorf("status is required")
	}
	if parsed.Summary == "" {
		return nil, fmt.Errorf("summary is required")
	}

	// Convert string status to command.WorkflowStatus
	status := command.WorkflowStatus(parsed.Status)

	cmd := command.NewSignalWorkflowCompleteCommand(command.SourceMCPTool, status, parsed.Summary, parsed.EpicID, parsed.TasksClosed)
	if err := cmd.Validate(); err != nil {
		return nil, fmt.Errorf("signal_workflow_complete command validation failed: %w", err)
	}

	result, err := a.submitWithTimeout(ctx, cmd)
	if err != nil {
		return nil, fmt.Errorf("signal_workflow_complete command failed: %w", err)
	}

	if !result.Success {
		return mcptypes.ErrorResult(result.Error.Error()), nil
	}

	// Build response message based on optional fields
	msg := fmt.Sprintf("Workflow marked as %s", parsed.Status)
	if parsed.EpicID != "" {
		msg += fmt.Sprintf(" (epic: %s)", parsed.EpicID)
	}
	if parsed.TasksClosed > 0 {
		msg += fmt.Sprintf(" - %d tasks closed", parsed.TasksClosed)
	}

	return mcptypes.SuccessResult(msg), nil
}

// ===========================================================================
// User Interaction Handlers
// ===========================================================================

// HandleNotifyUser handles the notify_user MCP tool call.
// This requests user attention for a human checkpoint in DAG workflows.
// Routes through the v2 command processor using CmdNotifyUser.
func (a *V2Adapter) HandleNotifyUser(ctx context.Context, args json.RawMessage) (*mcptypes.ToolCallResult, error) {
	var parsed notifyUserArgs
	if err := json.Unmarshal(args, &parsed); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	// Validate required fields
	if parsed.Message == "" {
		return nil, fmt.Errorf("message is required")
	}

	cmd := command.NewNotifyUserCommand(command.SourceMCPTool, parsed.Message, parsed.Phase, parsed.TaskID)
	if err := cmd.Validate(); err != nil {
		return nil, fmt.Errorf("notify_user command validation failed: %w", err)
	}

	result, err := a.submitWithTimeout(ctx, cmd)
	if err != nil {
		return nil, fmt.Errorf("notify_user command failed: %w", err)
	}

	if !result.Success {
		return mcptypes.ErrorResult(result.Error.Error()), nil
	}

	// Build response message
	msg := "User has been notified"
	if parsed.Phase != "" {
		msg = fmt.Sprintf("User notified for phase: %s", parsed.Phase)
	}

	return mcptypes.SuccessResult(msg), nil
}

// notifyUserArgs represents arguments for the notify_user MCP tool.
type notifyUserArgs struct {
	Message string `json:"message"`
	Phase   string `json:"phase,omitempty"`
	TaskID  string `json:"task_id,omitempty"`
}

// ===========================================================================
// Helper Methods
// ===========================================================================

// submitWithTimeout submits a command to the processor with the adapter's timeout.
func (a *V2Adapter) submitWithTimeout(ctx context.Context, cmd command.Command) (*command.CommandResult, error) {
	// Create a context with timeout
	timeoutCtx, cancel := context.WithTimeout(ctx, a.timeout)
	defer cancel()

	// Submit and wait for result
	result, err := a.processor.SubmitAndWait(timeoutCtx, cmd)
	if err != nil {
		// Check if it's a queue full error
		if errors.Is(err, command.ErrQueueFull) {
			return nil, fmt.Errorf("command queue is full, try again later")
		}
		return nil, err
	}

	return result, nil
}

// processIDExtractor is an interface for types that can provide a process ID.
type processIDExtractor interface {
	GetProcessID() string
}

// extractProcessID extracts a process ID from command result data.
// Supports SpawnProcessResult structs and raw string values.
func extractProcessID(data any) string {
	// Try string directly
	if s, ok := data.(string); ok {
		return s
	}

	// Try interface with GetProcessID method
	if v, ok := data.(processIDExtractor); ok {
		return v.GetProcessID()
	}

	// Fallback: return string representation
	return fmt.Sprintf("%v", data)
}
