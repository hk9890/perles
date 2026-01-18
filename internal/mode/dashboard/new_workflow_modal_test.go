package dashboard

import (
	"errors"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/zjrosen/perles/internal/git"
	"github.com/zjrosen/perles/internal/keys"
	"github.com/zjrosen/perles/internal/mocks"
	"github.com/zjrosen/perles/internal/mode"
	"github.com/zjrosen/perles/internal/orchestration/controlplane"
	"github.com/zjrosen/perles/internal/orchestration/workflow"
)

// === Test Helpers ===

// createTestRegistry creates a workflow registry with test templates.
func createTestRegistry(t *testing.T) *workflow.Registry {
	t.Helper()
	registry := workflow.NewRegistry()

	// Add test workflows
	registry.Add(workflow.Workflow{
		ID:          "quick-plan",
		Name:        "Quick Plan",
		Description: "Fast planning workflow",
		Category:    "Planning",
		TargetMode:  workflow.TargetOrchestration,
	})
	registry.Add(workflow.Workflow{
		ID:          "cook",
		Name:        "Cook",
		Description: "Implementation workflow",
		Category:    "Implementation",
		TargetMode:  workflow.TargetOrchestration,
	})
	registry.Add(workflow.Workflow{
		ID:          "research",
		Name:        "Research",
		Description: "Research to tasks",
		Category:    "Research",
		TargetMode:  workflow.TargetOrchestration,
	})

	return registry
}

// createTestModelWithRegistry creates a dashboard model with a mock ControlPlane and registry.
func createTestModelWithRegistry(t *testing.T, workflows []*controlplane.WorkflowInstance) (Model, *mockControlPlane, *workflow.Registry) {
	t.Helper()

	mockCP := newMockControlPlane()
	mockCP.On("List", mock.Anything, mock.Anything).Return(workflows, nil).Maybe()

	eventCh := make(chan controlplane.ControlPlaneEvent)
	close(eventCh)
	mockCP.On("Subscribe", mock.Anything).Return((<-chan controlplane.ControlPlaneEvent)(eventCh), func() {}).Maybe()

	registry := createTestRegistry(t)

	cfg := Config{
		ControlPlane: mockCP,
		Services:     mode.Services{},
		Registry:     registry,
	}

	m := New(cfg)
	m.workflows = workflows
	m.workflowList = m.workflowList.SetWorkflows(workflows)
	m.resourceSummary = m.resourceSummary.Update(workflows)
	m = m.SetSize(100, 40).(Model)

	return m, mockCP, registry
}

// === Unit Tests: Modal loads templates from registry ===

func TestNewWorkflowModal_LoadsTemplatesFromRegistry(t *testing.T) {
	registry := createTestRegistry(t)
	modal := NewNewWorkflowModal(registry, nil, nil)
	require.NotNil(t, modal)

	// Modal should be created with templates from registry
	// The form should have fields configured
	view := modal.View()
	require.NotEmpty(t, view)
	require.Contains(t, view, "Template")
}

func TestNewWorkflowModal_HandlesNilRegistry(t *testing.T) {
	modal := NewNewWorkflowModal(nil, nil, nil)
	require.NotNil(t, modal)

	// Should still render without crashing
	view := modal.View()
	require.NotEmpty(t, view)
}

// === Unit Tests: Form validation ===

func TestNewWorkflowModal_ValidationRejectsEmptyGoal(t *testing.T) {
	registry := createTestRegistry(t)
	modal := NewNewWorkflowModal(registry, nil, nil)

	// Validation should fail with empty goal
	values := map[string]any{
		"template":     "quick-plan",
		"name":         "",
		"goal":         "",
		"priority":     "normal",
		"max_workers":  "",
		"token_budget": "",
	}

	err := modal.validate(values)
	require.Error(t, err)
	require.Contains(t, err.Error(), "goal is required")
}

func TestNewWorkflowModal_ValidationRejectsEmptyTemplate(t *testing.T) {
	registry := createTestRegistry(t)
	modal := NewNewWorkflowModal(registry, nil, nil)

	values := map[string]any{
		"template":     "",
		"name":         "",
		"goal":         "Test goal",
		"priority":     "normal",
		"max_workers":  "",
		"token_budget": "",
	}

	err := modal.validate(values)
	require.Error(t, err)
	require.Contains(t, err.Error(), "template is required")
}

func TestNewWorkflowModal_ValidationRejectsInvalidMaxWorkers(t *testing.T) {
	registry := createTestRegistry(t)
	modal := NewNewWorkflowModal(registry, nil, nil)

	values := map[string]any{
		"template":     "quick-plan",
		"name":         "",
		"goal":         "Test goal",
		"priority":     "normal",
		"max_workers":  "invalid",
		"token_budget": "",
	}

	err := modal.validate(values)
	require.Error(t, err)
	require.Contains(t, err.Error(), "max workers must be a positive number")
}

func TestNewWorkflowModal_ValidationRejectsInvalidTokenBudget(t *testing.T) {
	registry := createTestRegistry(t)
	modal := NewNewWorkflowModal(registry, nil, nil)

	values := map[string]any{
		"template":     "quick-plan",
		"name":         "",
		"goal":         "Test goal",
		"priority":     "normal",
		"max_workers":  "",
		"token_budget": "-100",
	}

	err := modal.validate(values)
	require.Error(t, err)
	require.Contains(t, err.Error(), "token budget must be a positive number")
}

func TestNewWorkflowModal_ValidationAcceptsValidInput(t *testing.T) {
	registry := createTestRegistry(t)
	modal := NewNewWorkflowModal(registry, nil, nil)

	values := map[string]any{
		"template":     "quick-plan",
		"name":         "My Workflow",
		"goal":         "Test goal",
		"priority":     "normal",
		"max_workers":  "4",
		"token_budget": "10000",
	}

	err := modal.validate(values)
	require.NoError(t, err)
}

// === Unit Tests: Cancel closes modal without action ===

func TestNewWorkflowModal_CancelClosesModal(t *testing.T) {
	m, _, _ := createTestModelWithRegistry(t, []*controlplane.WorkflowInstance{})

	// Open the modal
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	m = result.(Model)
	require.True(t, m.InNewWorkflowModal())

	// Press Escape to cancel
	result, _ = m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	m = result.(Model)

	// Modal should now receive CancelNewWorkflowMsg
	result, _ = m.Update(CancelNewWorkflowMsg{})
	m = result.(Model)
	require.False(t, m.InNewWorkflowModal())
}

// === Unit Tests: Create calls ControlPlane.Create ===

func TestNewWorkflowModal_CreateCallsControlPlane(t *testing.T) {
	mockCP := newMockControlPlane()
	mockCP.On("List", mock.Anything, mock.Anything).Return([]*controlplane.WorkflowInstance{}, nil).Maybe()
	mockCP.On("Create", mock.Anything, mock.MatchedBy(func(spec controlplane.WorkflowSpec) bool {
		return spec.TemplateID == "quick-plan" && spec.InitialGoal == "Test goal"
	})).Return(controlplane.WorkflowID("new-workflow-id"), nil).Once()

	eventCh := make(chan controlplane.ControlPlaneEvent)
	close(eventCh)
	mockCP.On("Subscribe", mock.Anything).Return((<-chan controlplane.ControlPlaneEvent)(eventCh), func() {}).Maybe()

	registry := createTestRegistry(t)
	modal := NewNewWorkflowModal(registry, mockCP, nil)

	// Simulate form submission
	values := map[string]any{
		"template":     "quick-plan",
		"name":         "",
		"goal":         "Test goal",
		"priority":     "normal",
		"max_workers":  "",
		"token_budget": "",
	}

	msg := modal.onSubmit(values)
	createMsg, ok := msg.(CreateWorkflowMsg)
	require.True(t, ok)
	require.Equal(t, controlplane.WorkflowID("new-workflow-id"), createMsg.WorkflowID)

	mockCP.AssertExpectations(t)
}

// === Unit Tests: Create workflow always starts immediately ===

func TestDashboard_CreateWorkflowStartsImmediately(t *testing.T) {
	mockCP := newMockControlPlane()
	mockCP.On("List", mock.Anything, mock.Anything).Return([]*controlplane.WorkflowInstance{}, nil).Maybe()
	mockCP.On("Start", mock.Anything, controlplane.WorkflowID("new-wf")).Return(nil).Once()

	eventCh := make(chan controlplane.ControlPlaneEvent)
	close(eventCh)
	mockCP.On("Subscribe", mock.Anything).Return((<-chan controlplane.ControlPlaneEvent)(eventCh), func() {}).Maybe()

	registry := createTestRegistry(t)

	cfg := Config{
		ControlPlane: mockCP,
		Services:     mode.Services{},
		Registry:     registry,
	}

	m := New(cfg)
	m.workflows = []*controlplane.WorkflowInstance{}
	m = m.SetSize(100, 40).(Model)

	// Open modal
	result, _ := m.openNewWorkflowModal()
	m = result.(Model)

	// Simulate successful creation
	result, cmd := m.Update(CreateWorkflowMsg{
		WorkflowID: "new-wf",
		Name:       "Test",
	})
	m = result.(Model)

	// Modal should be closed
	require.False(t, m.InNewWorkflowModal())

	// Command should be returned (includes start workflow)
	require.NotNil(t, cmd)
}

// === Unit Tests: Resource limits default to empty ===

func TestNewWorkflowModal_ResourceLimitsOptional(t *testing.T) {
	registry := createTestRegistry(t)
	modal := NewNewWorkflowModal(registry, nil, nil)

	values := map[string]any{
		"template":     "quick-plan",
		"name":         "",
		"goal":         "Test goal",
		"priority":     "normal",
		"max_workers":  "",
		"token_budget": "",
	}

	// Should pass validation with empty resource limits
	err := modal.validate(values)
	require.NoError(t, err)
}

// === Unit Tests: Tab navigates between fields ===

func TestNewWorkflowModal_TabNavigates(t *testing.T) {
	registry := createTestRegistry(t)
	modal := NewNewWorkflowModal(registry, nil, nil).SetSize(100, 40)

	// Press Tab - should navigate to next field
	modal, _ = modal.Update(tea.KeyMsg{Type: tea.KeyTab})

	// Modal should still be functional
	require.NotNil(t, modal)
	view := modal.View()
	require.NotEmpty(t, view)
}

// === Unit Tests: N key opens modal ===

func TestDashboard_NKeyOpensModal(t *testing.T) {
	m, _, _ := createTestModelWithRegistry(t, []*controlplane.WorkflowInstance{})
	require.False(t, m.InNewWorkflowModal())

	// Press n to open modal
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	m = result.(Model)

	require.True(t, m.InNewWorkflowModal())
	// Note: Init command may be nil if no text inputs need blink
}

func TestDashboard_ShiftNKeyOpensModal(t *testing.T) {
	m, _, _ := createTestModelWithRegistry(t, []*controlplane.WorkflowInstance{})

	// Press N (shift+n) to open modal
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'N'}})
	m = result.(Model)

	require.True(t, m.InNewWorkflowModal())
}

// === Unit Tests: Escape key in dashboard doesn't interfere ===

func TestDashboard_EscapeKeyWithoutModal(t *testing.T) {
	m, _, _ := createTestModelWithRegistry(t, []*controlplane.WorkflowInstance{})

	// Press Escape without modal open - should not crash
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	m = result.(Model)

	// Dashboard should still be functional
	view := m.View()
	require.NotEmpty(t, view)
}

// === Unit Tests: Modal overlay rendering ===

func TestDashboard_ModalRendersAsOverlay(t *testing.T) {
	m, _, _ := createTestModelWithRegistry(t, []*controlplane.WorkflowInstance{})

	// Open modal
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	m = result.(Model)

	// View should contain modal content
	view := m.View()
	require.Contains(t, view, "New Workflow")
	require.Contains(t, view, "Template")
	require.Contains(t, view, "Goal")
}

// === Unit Tests: Window resize updates modal ===

func TestDashboard_WindowResizeUpdatesModal(t *testing.T) {
	m, _, _ := createTestModelWithRegistry(t, []*controlplane.WorkflowInstance{})

	// Open modal
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	m = result.(Model)

	// Resize window
	result, _ = m.Update(tea.WindowSizeMsg{Width: 150, Height: 50})
	m = result.(Model)

	require.Equal(t, 150, m.width)
	require.Equal(t, 50, m.height)
	require.True(t, m.InNewWorkflowModal())
}

// === Unit Tests: Modal handles Ctrl+S ===

func TestNewWorkflowModal_CtrlSSavesForm(t *testing.T) {
	registry := createTestRegistry(t)
	modal := NewNewWorkflowModal(registry, nil, nil).SetSize(100, 40)

	// Press Ctrl+S - should trigger save/validation
	// Since form is empty, it should show validation error
	modal, _ = modal.Update(tea.KeyMsg{Type: tea.KeyCtrlS})

	// Modal should still be functional (validation error shown)
	require.NotNil(t, modal)
}

// === Integration Tests: Full workflow creation flow ===

func TestDashboard_FullWorkflowCreationFlow(t *testing.T) {
	mockCP := newMockControlPlane()
	mockCP.On("List", mock.Anything, mock.Anything).Return([]*controlplane.WorkflowInstance{}, nil).Maybe()
	mockCP.On("Create", mock.Anything, mock.Anything).Return(controlplane.WorkflowID("created-wf"), nil).Once()

	eventCh := make(chan controlplane.ControlPlaneEvent)
	close(eventCh)
	mockCP.On("Subscribe", mock.Anything).Return((<-chan controlplane.ControlPlaneEvent)(eventCh), func() {}).Maybe()

	registry := createTestRegistry(t)

	cfg := Config{
		ControlPlane: mockCP,
		Services:     mode.Services{},
		Registry:     registry,
	}

	m := New(cfg)
	m = m.SetSize(100, 40).(Model)

	// 1. Open modal
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	m = result.(Model)
	require.True(t, m.InNewWorkflowModal())

	// 2. Simulate receiving CreateWorkflowMsg (as if form was filled and submitted)
	result, _ = m.Update(CreateWorkflowMsg{
		WorkflowID: "created-wf",
		Name:       "Test Workflow",
	})
	m = result.(Model)

	// 3. Modal should be closed
	require.False(t, m.InNewWorkflowModal())
}

// Test that buildTemplateOptions handles empty registry
func TestBuildTemplateOptions_EmptyRegistry(t *testing.T) {
	registry := workflow.NewRegistry()
	options := buildTemplateOptions(registry)
	require.Empty(t, options)
}

// Test that buildTemplateOptions creates correct options
func TestBuildTemplateOptions_CreatesCorrectOptions(t *testing.T) {
	registry := createTestRegistry(t)
	options := buildTemplateOptions(registry)

	require.Len(t, options, 3)

	// Options should include template info
	hasQuickPlan := false
	for _, opt := range options {
		if opt.Value == "quick-plan" {
			hasQuickPlan = true
			require.Contains(t, opt.Label, "Quick Plan")
		}
	}
	require.True(t, hasQuickPlan)
}

// Test that buildTemplateOptions handles nil registry
func TestBuildTemplateOptions_NilRegistry(t *testing.T) {
	options := buildTemplateOptions(nil)
	require.Empty(t, options)
}

// Test escape key handler checks for common escape binding
func TestNewWorkflowModal_EscapeClearsModal(t *testing.T) {
	registry := createTestRegistry(t)
	modal := NewNewWorkflowModal(registry, nil, nil).SetSize(100, 40)

	// Press escape
	modal, cmd := modal.Update(keys.Common.Escape.Keys()[0])
	require.NotNil(t, modal)

	// Should produce a cancel message command
	if cmd != nil {
		msg := cmd()
		_, isCancel := msg.(CancelNewWorkflowMsg)
		require.True(t, isCancel)
	}
}

// === Worktree UI Tests ===

// createMockGitExecutorWithBranches creates a mock GitExecutor with test branches.
func createMockGitExecutorWithBranches(t *testing.T) *mocks.MockGitExecutor {
	t.Helper()
	mockGit := mocks.NewMockGitExecutor(t)
	mockGit.EXPECT().ListBranches().Return([]git.BranchInfo{
		{Name: "main", IsCurrent: false},
		{Name: "develop", IsCurrent: true},
		{Name: "feature/auth", IsCurrent: false},
	}, nil).Maybe()
	return mockGit
}

func TestNewWorkflowModal_PopulatesBranchOptionsFromListBranches(t *testing.T) {
	registry := createTestRegistry(t)
	mockGit := createMockGitExecutorWithBranches(t)

	modal := NewNewWorkflowModal(registry, nil, mockGit)
	require.NotNil(t, modal)
	require.True(t, modal.worktreeEnabled)

	// Modal should contain Git Worktree toggle (always visible)
	view := modal.SetSize(100, 40).View()
	require.Contains(t, view, "Git Worktree")

	// Branch fields should be hidden initially (worktree toggle defaults to No)
	require.NotContains(t, view, "Base Branch")
	require.NotContains(t, view, "Branch Name")

	// Navigate to the worktree toggle and switch to Yes
	// Tab through: Template -> Name -> Goal -> Git Worktree
	modal, _ = modal.Update(tea.KeyMsg{Type: tea.KeyTab})
	modal, _ = modal.Update(tea.KeyMsg{Type: tea.KeyTab})
	modal, _ = modal.Update(tea.KeyMsg{Type: tea.KeyTab})
	// Switch toggle to Yes (right arrow)
	modal, _ = modal.Update(tea.KeyMsg{Type: tea.KeyRight})

	// Now branch fields should be visible
	view = modal.View()
	require.Contains(t, view, "Base Branch")
	require.Contains(t, view, "Branch Name")
}

func TestNewWorkflowModal_DisablesWorktreeFieldsWhenListBranchesFails(t *testing.T) {
	registry := createTestRegistry(t)
	mockGit := mocks.NewMockGitExecutor(t)
	mockGit.EXPECT().ListBranches().Return(nil, errors.New("not a git repo"))

	modal := NewNewWorkflowModal(registry, nil, mockGit)
	require.NotNil(t, modal)
	require.False(t, modal.worktreeEnabled)

	// Modal should NOT contain worktree fields when git fails
	view := modal.SetSize(100, 40).View()
	require.NotContains(t, view, "Git Worktree")
	require.NotContains(t, view, "Base Branch")
}

func TestNewWorkflowModal_DisablesWorktreeFieldsWhenGitExecutorNil(t *testing.T) {
	registry := createTestRegistry(t)

	modal := NewNewWorkflowModal(registry, nil, nil)
	require.NotNil(t, modal)
	require.False(t, modal.worktreeEnabled)

	// Modal should NOT contain worktree fields when no git executor
	view := modal.SetSize(100, 40).View()
	require.NotContains(t, view, "Git Worktree")
	require.NotContains(t, view, "Base Branch")
}

func TestNewWorkflowModal_OnSubmitSetsWorktreeEnabledCorrectly(t *testing.T) {
	registry := createTestRegistry(t)
	mockGit := createMockGitExecutorWithBranches(t)

	mockCP := newMockControlPlane()
	mockCP.On("Create", mock.Anything, mock.MatchedBy(func(spec controlplane.WorkflowSpec) bool {
		return spec.WorktreeEnabled == true &&
			spec.WorktreeBaseBranch == "main" &&
			spec.WorktreeBranchName == "my-feature"
	})).Return(controlplane.WorkflowID("new-workflow-id"), nil).Once()

	modal := NewNewWorkflowModal(registry, mockCP, mockGit)

	values := map[string]any{
		"template":      "quick-plan",
		"name":          "",
		"goal":          "Test goal",
		"use_worktree":  "true",
		"base_branch":   "main",
		"custom_branch": "my-feature",
	}

	msg := modal.onSubmit(values)
	createMsg, ok := msg.(CreateWorkflowMsg)
	require.True(t, ok)
	require.Equal(t, controlplane.WorkflowID("new-workflow-id"), createMsg.WorkflowID)

	mockCP.AssertExpectations(t)
}

func TestNewWorkflowModal_OnSubmitSetsWorktreeBaseBranchFromSearchSelect(t *testing.T) {
	registry := createTestRegistry(t)
	mockGit := createMockGitExecutorWithBranches(t)

	mockCP := newMockControlPlane()
	mockCP.On("Create", mock.Anything, mock.MatchedBy(func(spec controlplane.WorkflowSpec) bool {
		return spec.WorktreeEnabled == true && spec.WorktreeBaseBranch == "develop"
	})).Return(controlplane.WorkflowID("new-workflow-id"), nil).Once()

	modal := NewNewWorkflowModal(registry, mockCP, mockGit)

	values := map[string]any{
		"template":      "quick-plan",
		"name":          "",
		"goal":          "Test goal",
		"use_worktree":  "true",
		"base_branch":   "develop",
		"custom_branch": "",
	}

	msg := modal.onSubmit(values)
	createMsg, ok := msg.(CreateWorkflowMsg)
	require.True(t, ok)
	require.Equal(t, controlplane.WorkflowID("new-workflow-id"), createMsg.WorkflowID)

	mockCP.AssertExpectations(t)
}

func TestNewWorkflowModal_OnSubmitSetsWorktreeBranchNameFromTextField(t *testing.T) {
	registry := createTestRegistry(t)
	mockGit := createMockGitExecutorWithBranches(t)

	mockCP := newMockControlPlane()
	mockCP.On("Create", mock.Anything, mock.MatchedBy(func(spec controlplane.WorkflowSpec) bool {
		return spec.WorktreeEnabled == true && spec.WorktreeBranchName == "perles-custom-branch"
	})).Return(controlplane.WorkflowID("new-workflow-id"), nil).Once()

	modal := NewNewWorkflowModal(registry, mockCP, mockGit)

	values := map[string]any{
		"template":      "quick-plan",
		"name":          "",
		"goal":          "Test goal",
		"use_worktree":  "true",
		"base_branch":   "main",
		"custom_branch": "perles-custom-branch",
	}

	msg := modal.onSubmit(values)
	createMsg, ok := msg.(CreateWorkflowMsg)
	require.True(t, ok)
	require.Equal(t, controlplane.WorkflowID("new-workflow-id"), createMsg.WorkflowID)

	mockCP.AssertExpectations(t)
}

func TestNewWorkflowModal_ValidationRequiresBaseBranchWhenWorktreeEnabled(t *testing.T) {
	registry := createTestRegistry(t)
	mockGit := createMockGitExecutorWithBranches(t)

	modal := NewNewWorkflowModal(registry, nil, mockGit)

	values := map[string]any{
		"template":      "quick-plan",
		"name":          "",
		"goal":          "Test goal",
		"use_worktree":  "true",
		"base_branch":   "", // Missing base branch
		"custom_branch": "",
	}

	err := modal.validate(values)
	require.Error(t, err)
	require.Contains(t, err.Error(), "base branch is required when worktree is enabled")
}

func TestNewWorkflowModal_ValidationRejectsInvalidBranchNames(t *testing.T) {
	registry := createTestRegistry(t)
	mockGit := mocks.NewMockGitExecutor(t)
	mockGit.EXPECT().ListBranches().Return([]git.BranchInfo{
		{Name: "main", IsCurrent: true},
	}, nil)
	mockGit.EXPECT().ValidateBranchName("invalid..branch").Return(errors.New("invalid ref format"))

	modal := NewNewWorkflowModal(registry, nil, mockGit)

	values := map[string]any{
		"template":      "quick-plan",
		"name":          "",
		"goal":          "Test goal",
		"use_worktree":  "true",
		"base_branch":   "main",
		"custom_branch": "invalid..branch", // Invalid branch name
	}

	err := modal.validate(values)
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid branch name")
}

func TestNewWorkflowModal_ValidationAcceptsValidBranchName(t *testing.T) {
	registry := createTestRegistry(t)
	mockGit := mocks.NewMockGitExecutor(t)
	mockGit.EXPECT().ListBranches().Return([]git.BranchInfo{
		{Name: "main", IsCurrent: true},
	}, nil)
	mockGit.EXPECT().ValidateBranchName("feature/valid-branch").Return(nil)

	modal := NewNewWorkflowModal(registry, nil, mockGit)

	values := map[string]any{
		"template":      "quick-plan",
		"name":          "",
		"goal":          "Test goal",
		"use_worktree":  "true",
		"base_branch":   "main",
		"custom_branch": "feature/valid-branch",
	}

	err := modal.validate(values)
	require.NoError(t, err)
}

func TestNewWorkflowModal_ValidationPassesWhenWorktreeDisabled(t *testing.T) {
	registry := createTestRegistry(t)
	mockGit := createMockGitExecutorWithBranches(t)

	modal := NewNewWorkflowModal(registry, nil, mockGit)

	values := map[string]any{
		"template":      "quick-plan",
		"name":          "",
		"goal":          "Test goal",
		"use_worktree":  "false", // Worktree disabled
		"base_branch":   "",      // Empty but should be OK
		"custom_branch": "",
	}

	err := modal.validate(values)
	require.NoError(t, err)
}

func TestBuildBranchOptions_NilGitExecutor(t *testing.T) {
	options, available := buildBranchOptions(nil)
	require.Nil(t, options)
	require.False(t, available)
}

func TestBuildBranchOptions_ListBranchesError(t *testing.T) {
	mockGit := mocks.NewMockGitExecutor(t)
	mockGit.EXPECT().ListBranches().Return(nil, errors.New("git error"))

	options, available := buildBranchOptions(mockGit)
	require.Nil(t, options)
	require.False(t, available)
}

func TestBuildBranchOptions_EmptyBranchList(t *testing.T) {
	mockGit := mocks.NewMockGitExecutor(t)
	mockGit.EXPECT().ListBranches().Return([]git.BranchInfo{}, nil)

	options, available := buildBranchOptions(mockGit)
	require.Nil(t, options)
	require.False(t, available)
}

func TestBuildBranchOptions_ConvertsCorrectly(t *testing.T) {
	mockGit := mocks.NewMockGitExecutor(t)
	mockGit.EXPECT().ListBranches().Return([]git.BranchInfo{
		{Name: "main", IsCurrent: false},
		{Name: "develop", IsCurrent: true},
		{Name: "feature/test", IsCurrent: false},
	}, nil)

	options, available := buildBranchOptions(mockGit)
	require.True(t, available)
	require.Len(t, options, 3)

	// Check first branch
	require.Equal(t, "main", options[0].Label)
	require.Equal(t, "main", options[0].Value)
	require.False(t, options[0].Selected)

	// Check current branch is selected
	require.Equal(t, "develop", options[1].Label)
	require.Equal(t, "develop", options[1].Value)
	require.True(t, options[1].Selected)

	// Check third branch
	require.Equal(t, "feature/test", options[2].Label)
	require.Equal(t, "feature/test", options[2].Value)
	require.False(t, options[2].Selected)
}
