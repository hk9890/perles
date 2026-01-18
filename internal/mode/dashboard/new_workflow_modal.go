// Package dashboard implements the multi-workflow dashboard TUI mode.
package dashboard

import (
	"context"
	"errors"
	"strconv"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/zjrosen/perles/internal/git"
	"github.com/zjrosen/perles/internal/orchestration/controlplane"
	"github.com/zjrosen/perles/internal/orchestration/workflow"
	"github.com/zjrosen/perles/internal/ui/shared/formmodal"
)

// NewWorkflowModal holds the state for the new workflow creation modal.
type NewWorkflowModal struct {
	form            formmodal.Model
	registry        *workflow.Registry
	controlPlane    controlplane.ControlPlane
	gitExecutor     git.GitExecutor
	worktreeEnabled bool // track if worktree options are available
}

// CreateWorkflowMsg is sent when a workflow is created successfully.
type CreateWorkflowMsg struct {
	WorkflowID controlplane.WorkflowID
	Name       string
}

// CancelNewWorkflowMsg is sent when the modal is cancelled.
type CancelNewWorkflowMsg struct{}

// NewNewWorkflowModal creates a new workflow creation modal.
// gitExecutor is optional - if nil or if ListBranches() fails, worktree options are disabled.
func NewNewWorkflowModal(registry *workflow.Registry, cp controlplane.ControlPlane, gitExecutor git.GitExecutor) *NewWorkflowModal {
	m := &NewWorkflowModal{
		registry:     registry,
		controlPlane: cp,
		gitExecutor:  gitExecutor,
	}

	// Build template options from registry
	templateOptions := buildTemplateOptions(registry)

	// Build branch options from git executor (if available)
	branchOptions, worktreeAvailable := buildBranchOptions(gitExecutor)
	m.worktreeEnabled = worktreeAvailable

	// Build form fields
	fields := []formmodal.FieldConfig{
		{
			Key:               "template",
			Type:              formmodal.FieldTypeSearchSelect,
			Label:             "Template",
			Hint:              "required",
			Options:           templateOptions,
			SearchPlaceholder: "Search templates...",
			MaxVisibleItems:   5,
		},
		{
			Key:         "name",
			Type:        formmodal.FieldTypeText,
			Label:       "Name",
			Hint:        "optional",
			Placeholder: "Workflow name (defaults to template name)",
		},
		{
			Key:         "goal",
			Type:        formmodal.FieldTypeTextArea,
			Label:       "Goal",
			Hint:        "required",
			Placeholder: "What should this workflow accomplish?",
			MaxHeight:   5,
			VimEnabled:  true,
		},
	}

	// Add worktree fields if git support is available
	if worktreeAvailable {
		// Helper to check if worktree is enabled
		worktreeEnabled := func(values map[string]any) bool {
			v, _ := values["use_worktree"].(string)
			return v == "true"
		}

		worktreeFields := []formmodal.FieldConfig{
			{
				Key:   "use_worktree",
				Type:  formmodal.FieldTypeToggle,
				Label: "Git Worktree",
				Hint:  "optional",
				Options: []formmodal.ListOption{
					{Label: "No", Value: "false", Selected: true},
					{Label: "Yes", Value: "true"},
				},
			},
			{
				Key:               "base_branch",
				Type:              formmodal.FieldTypeSearchSelect,
				Label:             "Base Branch",
				Hint:              "required",
				Options:           branchOptions,
				SearchPlaceholder: "Search branches...",
				MaxVisibleItems:   5,
				VisibleWhen:       worktreeEnabled,
			},
			{
				Key:         "custom_branch",
				Type:        formmodal.FieldTypeText,
				Label:       "Branch Name",
				Hint:        "optional - auto-generated if empty",
				Placeholder: "perles-workflow-abc123",
				VisibleWhen: worktreeEnabled,
			},
		}
		fields = append(fields, worktreeFields...)
	}

	cfg := formmodal.FormConfig{
		Title:       "New Workflow",
		Fields:      fields,
		SubmitLabel: "Create",
		MinWidth:    65,
		Validate:    m.validate,
		OnSubmit:    m.onSubmit,
		OnCancel:    func() tea.Msg { return CancelNewWorkflowMsg{} },
	}

	m.form = formmodal.New(cfg)
	return m
}

// buildBranchOptions converts git branches to list options.
// Returns the options and a boolean indicating if worktree support is available.
func buildBranchOptions(gitExecutor git.GitExecutor) ([]formmodal.ListOption, bool) {
	if gitExecutor == nil {
		return nil, false
	}

	branches, err := gitExecutor.ListBranches()
	if err != nil {
		return nil, false
	}

	if len(branches) == 0 {
		return nil, false
	}

	options := make([]formmodal.ListOption, len(branches))
	for i, branch := range branches {
		options[i] = formmodal.ListOption{
			Label:    branch.Name,
			Value:    branch.Name,
			Selected: branch.IsCurrent, // Select current branch by default
		}
	}

	return options, true
}

// buildTemplateOptions converts workflow templates to list options.
func buildTemplateOptions(registry *workflow.Registry) []formmodal.ListOption {
	if registry == nil {
		return []formmodal.ListOption{}
	}

	// Get all orchestration-compatible templates
	templates := registry.ListByTargetMode(workflow.TargetOrchestration)

	// If no orchestration-specific templates, fall back to all templates
	if len(templates) == 0 {
		templates = registry.List()
	}

	options := make([]formmodal.ListOption, len(templates))
	for i, tmpl := range templates {
		options[i] = formmodal.ListOption{
			Label:    tmpl.Name,
			Subtext:  tmpl.Description,
			Value:    tmpl.ID,
			Selected: i == 0, // Select first template by default
		}
	}

	return options
}

// validate checks form values before submission.
func (m *NewWorkflowModal) validate(values map[string]any) error {
	// Template is required
	templateID, ok := values["template"].(string)
	if !ok || templateID == "" {
		return errors.New("template is required")
	}

	// Verify template exists
	if m.registry != nil {
		if _, found := m.registry.Get(templateID); !found {
			return errors.New("selected template not found")
		}
	}

	// Goal is required
	goal, ok := values["goal"].(string)
	if !ok || goal == "" {
		return errors.New("goal is required")
	}

	// Validate max_workers if provided (must be positive integer)
	if maxWorkersStr, ok := values["max_workers"].(string); ok && maxWorkersStr != "" {
		maxWorkers, err := strconv.Atoi(maxWorkersStr)
		if err != nil || maxWorkers < 0 {
			return errors.New("max workers must be a positive number")
		}
	}

	// Validate token_budget if provided (must be positive integer)
	if tokenBudgetStr, ok := values["token_budget"].(string); ok && tokenBudgetStr != "" {
		tokenBudget, err := strconv.ParseInt(tokenBudgetStr, 10, 64)
		if err != nil || tokenBudget < 0 {
			return errors.New("token budget must be a positive number")
		}
	}

	// Validate worktree fields if worktree is enabled
	if m.worktreeEnabled {
		useWorktree, _ := values["use_worktree"].(string)
		if useWorktree == "true" {
			// Base branch is required when worktree is enabled
			baseBranch, _ := values["base_branch"].(string)
			if baseBranch == "" {
				return errors.New("base branch is required when worktree is enabled")
			}

			// Validate custom branch name if provided
			customBranch, _ := values["custom_branch"].(string)
			if customBranch != "" && m.gitExecutor != nil {
				if err := m.gitExecutor.ValidateBranchName(customBranch); err != nil {
					return errors.New("invalid branch name: " + err.Error())
				}
			}
		}
	}

	return nil
}

// onSubmit creates the workflow from form values.
func (m *NewWorkflowModal) onSubmit(values map[string]any) tea.Msg {
	templateID := values["template"].(string)
	name := values["name"].(string)
	goal := values["goal"].(string)

	// Build WorkflowSpec
	spec := controlplane.WorkflowSpec{
		TemplateID:  templateID,
		InitialGoal: goal,
		Name:        name,
	}

	// Set worktree fields if worktree options are available
	if m.worktreeEnabled {
		useWorktree, _ := values["use_worktree"].(string)
		if useWorktree == "true" {
			spec.WorktreeEnabled = true
			spec.WorktreeBaseBranch, _ = values["base_branch"].(string)
			spec.WorktreeBranchName, _ = values["custom_branch"].(string)
		}
	}

	// Create the workflow
	if m.controlPlane == nil {
		return CreateWorkflowMsg{Name: spec.Name}
	}

	workflowID, err := m.controlPlane.Create(context.Background(), spec)
	if err != nil {
		// Return error as message (modal will display validation error)
		return CreateWorkflowMsg{Name: spec.Name}
	}

	return CreateWorkflowMsg{
		WorkflowID: workflowID,
		Name:       spec.Name,
	}
}

// SetSize sets the modal dimensions.
func (m *NewWorkflowModal) SetSize(width, height int) *NewWorkflowModal {
	m.form = m.form.SetSize(width, height)
	return m
}

// Init initializes the modal.
func (m *NewWorkflowModal) Init() tea.Cmd {
	return m.form.Init()
}

// Update handles messages for the modal.
func (m *NewWorkflowModal) Update(msg tea.Msg) (*NewWorkflowModal, tea.Cmd) {
	var cmd tea.Cmd
	m.form, cmd = m.form.Update(msg)
	return m, cmd
}

// View renders the modal.
func (m *NewWorkflowModal) View() string {
	return m.form.View()
}

// Overlay renders the modal on top of a background view.
func (m *NewWorkflowModal) Overlay(background string) string {
	return m.form.Overlay(background)
}
