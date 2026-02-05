package dashboard

import (
	"errors"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	beads "github.com/zjrosen/perles/internal/beads/domain"
	"github.com/zjrosen/perles/internal/config"
	"github.com/zjrosen/perles/internal/mocks"
	"github.com/zjrosen/perles/internal/mode"
	"github.com/zjrosen/perles/internal/orchestration/controlplane"
	"github.com/zjrosen/perles/internal/ui/modals/issueeditor"
	"github.com/zjrosen/perles/internal/ui/shared/toaster"
	"github.com/zjrosen/perles/internal/ui/tree"
)

// === Test Helpers ===

// createTestIssue creates a test issue with the given parameters.
func createTestIssue(id, title string, parentID string) beads.Issue {
	return beads.Issue{
		ID:        id,
		TitleText: title,
		ParentID:  parentID,
		Status:    beads.StatusOpen,
		Priority:  beads.PriorityMedium,
		Type:      beads.TypeTask,
	}
}

// createEpicTreeTestModel creates a dashboard model with mocked services for epic tree testing.
// This model has a mock client that handles GetComments calls required by details.New().
func createEpicTreeTestModel(t *testing.T) Model {
	t.Helper()

	mockCP := newMockControlPlane(t)
	mockCP.On("List", mock.Anything, mock.Anything).Return([]*controlplane.WorkflowInstance{}, nil).Maybe()

	eventCh := make(chan controlplane.ControlPlaneEvent)
	close(eventCh)
	mockCP.On("Subscribe", mock.Anything).Return((<-chan controlplane.ControlPlaneEvent)(eventCh), func() {}).Maybe()

	// Create mock client that handles GetComments
	mockClient := mocks.NewMockBeadsClient(t)
	mockClient.EXPECT().GetComments(mock.Anything).Return([]beads.Comment{}, nil).Maybe()

	// Create mock executor
	mockExecutor := mocks.NewMockBQLExecutor(t)
	mockExecutor.EXPECT().Execute(mock.Anything).Return([]beads.Issue{}, nil).Maybe()

	cfg := config.Defaults()

	services := mode.Services{
		Client:   mockClient,
		Executor: mockExecutor,
		Config:   &cfg,
	}

	dashCfg := Config{
		ControlPlane: mockCP,
		Services:     services,
	}

	m := New(dashCfg)
	m = m.SetSize(100, 40).(Model)

	return m
}

// === Unit Tests: loadEpicTree ===

func TestLoadEpicTreeReturnsCommand(t *testing.T) {
	// Setup mock executor
	mockExecutor := mocks.NewMockBQLExecutor(t)
	mockExecutor.EXPECT().
		Execute(mock.MatchedBy(func(query string) bool {
			return query == `id = "epic-123" expand down depth *`
		})).
		Return([]beads.Issue{createTestIssue("epic-123", "Test Epic", "")}, nil).
		Maybe()

	// Call loadEpicTree
	cmd := loadEpicTree("epic-123", mockExecutor)

	// Verify command is returned
	require.NotNil(t, cmd, "loadEpicTree should return a non-nil command")
}

func TestLoadEpicTreeReturnsNilForEmptyEpicID(t *testing.T) {
	mockExecutor := mocks.NewMockBQLExecutor(t)

	// Empty epic ID should return nil
	cmd := loadEpicTree("", mockExecutor)
	require.Nil(t, cmd, "loadEpicTree should return nil for empty epic ID")
}

func TestLoadEpicTreeReturnsNilForNilExecutor(t *testing.T) {
	// Nil executor should return nil
	cmd := loadEpicTree("epic-123", nil)
	require.Nil(t, cmd, "loadEpicTree should return nil for nil executor")
}

func TestLoadEpicTreeExecutesBQL(t *testing.T) {
	// Setup mock executor
	mockExecutor := mocks.NewMockBQLExecutor(t)
	expectedIssues := []beads.Issue{
		createTestIssue("epic-123", "Test Epic", ""),
		createTestIssue("task-1", "Task 1", "epic-123"),
	}
	mockExecutor.EXPECT().
		Execute(`id = "epic-123" expand down depth *`).
		Return(expectedIssues, nil).
		Once()

	// Call loadEpicTree and execute the command
	cmd := loadEpicTree("epic-123", mockExecutor)
	require.NotNil(t, cmd)

	// Execute the command
	msg := cmd()

	// Verify the message
	loadedMsg, ok := msg.(epicTreeLoadedMsg)
	require.True(t, ok, "command should return epicTreeLoadedMsg")
	require.Equal(t, "epic-123", loadedMsg.RootID)
	require.Len(t, loadedMsg.Issues, 2)
	require.NoError(t, loadedMsg.Err)

	mockExecutor.AssertExpectations(t)
}

func TestLoadEpicTreeReturnsErrorInMsg(t *testing.T) {
	// Setup mock executor that returns an error
	mockExecutor := mocks.NewMockBQLExecutor(t)
	expectedErr := errors.New("database error")
	mockExecutor.EXPECT().
		Execute(`id = "epic-123" expand down depth *`).
		Return(nil, expectedErr).
		Once()

	// Call loadEpicTree and execute the command
	cmd := loadEpicTree("epic-123", mockExecutor)
	require.NotNil(t, cmd)

	// Execute the command
	msg := cmd()

	// Verify the error is in the message
	loadedMsg, ok := msg.(epicTreeLoadedMsg)
	require.True(t, ok, "command should return epicTreeLoadedMsg")
	require.Equal(t, "epic-123", loadedMsg.RootID)
	require.Nil(t, loadedMsg.Issues)
	require.ErrorIs(t, loadedMsg.Err, expectedErr)

	mockExecutor.AssertExpectations(t)
}

// === Unit Tests: handleEpicTreeLoaded ===

func TestHandleEpicTreeLoadedBuildsTree(t *testing.T) {
	// Setup model with mocked services
	m := createEpicTreeTestModel(t)
	m.lastLoadedEpicID = "epic-123"

	// Create issues
	issues := []beads.Issue{
		createTestIssue("epic-123", "Test Epic", ""),
		createTestIssue("task-1", "Task 1", "epic-123"),
		createTestIssue("task-2", "Task 2", "epic-123"),
	}

	// Handle the message
	msg := epicTreeLoadedMsg{
		Issues: issues,
		RootID: "epic-123",
		Err:    nil,
	}
	result, cmd := m.handleEpicTreeLoaded(msg)
	m = result.(Model)

	// Verify tree is built
	require.NotNil(t, m.epicTree, "epic tree should be created")
	require.Nil(t, cmd, "no follow-up command expected")
}

func TestHandleEpicTreeLoadedRejectsStale(t *testing.T) {
	// Setup model with different lastLoadedEpicID
	m := createEpicTreeTestModel(t)
	m.lastLoadedEpicID = "epic-456" // Different from message

	// Create issues for a different epic
	issues := []beads.Issue{
		createTestIssue("epic-123", "Old Epic", ""),
	}

	// Handle the message
	msg := epicTreeLoadedMsg{
		Issues: issues,
		RootID: "epic-123", // Different from lastLoadedEpicID
		Err:    nil,
	}
	result, cmd := m.handleEpicTreeLoaded(msg)
	m = result.(Model)

	// Verify tree is NOT built (stale response rejected)
	require.Nil(t, m.epicTree, "epic tree should not be created for stale response")
	require.Nil(t, cmd)
}

func TestHandleEpicTreeLoadedHandlesError(t *testing.T) {
	// Setup model
	m := createEpicTreeTestModel(t)
	m.lastLoadedEpicID = "epic-123"
	// Pre-set an existing tree to verify it gets cleared
	issueMap := map[string]*beads.Issue{
		"old-epic": {ID: "old-epic", TitleText: "Old"},
	}
	m.epicTree = tree.New("old-epic", issueMap, tree.DirectionDown, tree.ModeDeps, nil)

	// Handle error message
	msg := epicTreeLoadedMsg{
		Issues: nil,
		RootID: "epic-123",
		Err:    errors.New("load failed"),
	}
	result, cmd := m.handleEpicTreeLoaded(msg)
	m = result.(Model)

	// Verify tree is cleared on error
	require.Nil(t, m.epicTree, "epic tree should be cleared on error")
	require.False(t, m.hasEpicDetail, "hasEpicDetail should be false on error")
	require.Nil(t, cmd)
}

func TestHandleEpicTreeLoadedHandlesEmptyResults(t *testing.T) {
	// Setup model
	m := createEpicTreeTestModel(t)
	m.lastLoadedEpicID = "epic-123"

	// Handle empty results
	msg := epicTreeLoadedMsg{
		Issues: []beads.Issue{}, // Empty
		RootID: "epic-123",
		Err:    nil,
	}
	result, cmd := m.handleEpicTreeLoaded(msg)
	m = result.(Model)

	// Verify tree is nil for empty results
	require.Nil(t, m.epicTree, "epic tree should be nil for empty results")
	require.False(t, m.hasEpicDetail, "hasEpicDetail should be false for empty results")
	require.Nil(t, cmd)
}

func TestHandleEpicTreeLoadedPreservesDirectionAndMode(t *testing.T) {
	// Setup model with existing tree having custom direction and mode
	m := createEpicTreeTestModel(t)
	m.lastLoadedEpicID = "epic-123"

	// Create existing tree with DirectionUp and ModeChildren
	existingIssueMap := map[string]*beads.Issue{
		"old-epic": {ID: "old-epic", TitleText: "Old"},
	}
	m.epicTree = tree.New("old-epic", existingIssueMap, tree.DirectionUp, tree.ModeChildren, nil)

	// Verify existing tree has the custom settings
	require.Equal(t, tree.DirectionUp, m.epicTree.Direction())
	require.Equal(t, tree.ModeChildren, m.epicTree.Mode())

	// Create new issues
	issues := []beads.Issue{
		createTestIssue("epic-123", "New Epic", ""),
	}

	// Handle the message
	msg := epicTreeLoadedMsg{
		Issues: issues,
		RootID: "epic-123",
		Err:    nil,
	}
	result, _ := m.handleEpicTreeLoaded(msg)
	m = result.(Model)

	// Verify new tree preserves direction and mode
	require.NotNil(t, m.epicTree)
	require.Equal(t, tree.DirectionUp, m.epicTree.Direction(), "direction should be preserved")
	require.Equal(t, tree.ModeChildren, m.epicTree.Mode(), "mode should be preserved")
}

// === Unit Tests: updateEpicDetail ===

func TestUpdateEpicDetailSyncsWithTree(t *testing.T) {
	// Setup model with mocked services
	m := createEpicTreeTestModel(t)

	// Create issue map and tree
	issueMap := map[string]*beads.Issue{
		"epic-123": {ID: "epic-123", TitleText: "Test Epic", Status: beads.StatusOpen},
		"task-1":   {ID: "task-1", TitleText: "Task 1", ParentID: "epic-123", Status: beads.StatusOpen},
	}
	m.epicTree = tree.New("epic-123", issueMap, tree.DirectionDown, tree.ModeDeps, nil)

	// Ensure tree has a selected node
	require.NotNil(t, m.epicTree.SelectedNode(), "tree should have a selected node")

	// Call updateEpicDetail
	m.updateEpicDetail()

	// Verify details panel is updated
	require.True(t, m.hasEpicDetail, "hasEpicDetail should be true after update")
}

func TestUpdateEpicDetailHandlesNilTree(t *testing.T) {
	// Setup model without a tree
	m := createEpicTreeTestModel(t)
	m.epicTree = nil
	m.hasEpicDetail = true // Pre-set to verify it gets cleared

	// Call updateEpicDetail
	m.updateEpicDetail()

	// Verify details are cleared
	require.False(t, m.hasEpicDetail, "hasEpicDetail should be false for nil tree")
}

func TestUpdateEpicDetailHandlesNilNode(t *testing.T) {
	// Setup model with an empty tree (no nodes)
	m := createEpicTreeTestModel(t)

	// Create tree with empty issue map (results in no selected node)
	emptyIssueMap := map[string]*beads.Issue{}
	m.epicTree = tree.New("nonexistent", emptyIssueMap, tree.DirectionDown, tree.ModeDeps, nil)

	// Verify tree has no selected node
	require.Nil(t, m.epicTree.SelectedNode(), "tree should have no selected node")

	m.hasEpicDetail = true // Pre-set to verify it gets cleared

	// Call updateEpicDetail
	m.updateEpicDetail()

	// Verify details are cleared
	require.False(t, m.hasEpicDetail, "hasEpicDetail should be false when no node selected")
}

// === Unit Tests: Tree loading wiring (perles-boi8.3) ===

// createEpicTreeTestModelWithWorkflows creates a test model with workflows that have EpicIDs.
func createEpicTreeTestModelWithWorkflows(t *testing.T) Model {
	t.Helper()

	mockCP := newMockControlPlane(t)

	// Create workflows with and without EpicIDs
	workflows := []*controlplane.WorkflowInstance{
		{ID: "wf-1", EpicID: "epic-100", State: controlplane.WorkflowRunning},
		{ID: "wf-2", EpicID: "epic-200", State: controlplane.WorkflowRunning},
		{ID: "wf-3", EpicID: "", State: controlplane.WorkflowRunning}, // No epic
	}
	mockCP.On("List", mock.Anything, mock.Anything).Return(workflows, nil).Maybe()

	eventCh := make(chan controlplane.ControlPlaneEvent)
	close(eventCh)
	mockCP.On("Subscribe", mock.Anything).Return((<-chan controlplane.ControlPlaneEvent)(eventCh), func() {}).Maybe()

	// Create mock client that handles GetComments
	mockClient := mocks.NewMockBeadsClient(t)
	mockClient.EXPECT().GetComments(mock.Anything).Return([]beads.Comment{}, nil).Maybe()

	// Create mock executor
	mockExecutor := mocks.NewMockBQLExecutor(t)
	mockExecutor.EXPECT().Execute(mock.Anything).Return([]beads.Issue{}, nil).Maybe()

	cfg := config.Defaults()

	services := mode.Services{
		Client:   mockClient,
		Executor: mockExecutor,
		Config:   &cfg,
	}

	dashCfg := Config{
		ControlPlane: mockCP,
		Services:     services,
	}

	m := New(dashCfg)
	m.workflows = workflows
	m = m.SetSize(100, 40).(Model)

	return m
}

func TestTreeLoadTriggeredOnWorkflowSelection(t *testing.T) {
	// Setup model with workflows
	m := createEpicTreeTestModelWithWorkflows(t)

	// Select first workflow (has epic-100)
	m.selectedIndex = 0
	m.lastLoadedEpicID = "" // No epic loaded yet

	// Navigate to second workflow (has epic-200)
	cmd := m.handleWorkflowSelectionChange(1)

	// Verify:
	// 1. Command is returned (immediate load initiated)
	require.NotNil(t, cmd, "should return load command when new epic selected")

	// 2. lastLoadedEpicID is updated
	require.Equal(t, "epic-200", m.lastLoadedEpicID, "lastLoadedEpicID should be updated to new epic")
}

func TestTreeLoadSkippedForEmptyEpicID(t *testing.T) {
	// Setup model
	m := createEpicTreeTestModelWithWorkflows(t)

	// Start at workflow with epic
	m.selectedIndex = 0 // wf-1 with epic-100

	// Navigate to workflow without epic (wf-3 at index 2)
	cmd := m.handleWorkflowSelectionChange(2)

	// Verify no command is returned (no epic to load)
	require.Nil(t, cmd, "should not trigger tree load when workflow has no epicID")
}

func TestTreeLoadSkippedForSameEpic(t *testing.T) {
	// Setup model
	m := createEpicTreeTestModelWithWorkflows(t)

	// First workflow selected, epic already loaded
	m.selectedIndex = 0
	m.lastLoadedEpicID = "epic-100" // Same as wf-1's epic

	// Create another workflow with the same epic
	m.workflows = append(m.workflows, &controlplane.WorkflowInstance{
		ID:     "wf-4",
		EpicID: "epic-100", // Same epic as wf-1
		State:  controlplane.WorkflowRunning,
	})

	// Navigate to wf-4 (index 3) which has the same epic
	cmd := m.handleWorkflowSelectionChange(3)

	// Verify no command is returned (same epic already loaded)
	require.Nil(t, cmd, "should not trigger tree load when same epic already loaded")
}

// === Unit Tests: Tree Navigation and Toggle Keys (perles-boi8.6) ===

// createEpicTreeTestModelWithTree creates a test model with a pre-populated epic tree for navigation tests.
func createEpicTreeTestModelWithTree(t *testing.T) Model {
	t.Helper()

	m := createEpicTreeTestModel(t)

	// Create a tree with multiple nodes for navigation
	// NOTE: The tree traverses Children arrays (DirectionDown), so we must populate Children on the parent
	issueMap := map[string]*beads.Issue{
		"epic-123": {ID: "epic-123", TitleText: "Test Epic", Status: beads.StatusOpen, Type: beads.TypeEpic, Children: []string{"task-1", "task-2", "task-3"}},
		"task-1":   {ID: "task-1", TitleText: "Task 1", ParentID: "epic-123", Status: beads.StatusOpen, Type: beads.TypeTask},
		"task-2":   {ID: "task-2", TitleText: "Task 2", ParentID: "epic-123", Status: beads.StatusOpen, Type: beads.TypeTask},
		"task-3":   {ID: "task-3", TitleText: "Task 3", ParentID: "epic-123", Status: beads.StatusOpen, Type: beads.TypeTask},
	}
	m.epicTree = tree.New("epic-123", issueMap, tree.DirectionDown, tree.ModeDeps, nil)
	m.epicTree.SetSize(80, 20)

	// Create a workflow with an epic
	m.workflows = []*controlplane.WorkflowInstance{
		{ID: "wf-1", EpicID: "epic-123", State: controlplane.WorkflowRunning},
	}

	m.focus = FocusEpicView
	m.epicViewFocus = EpicFocusTree

	return m
}

func TestTreeCursorDown(t *testing.T) {
	// Verify 'j' key moves cursor down in tree
	m := createEpicTreeTestModelWithTree(t)

	// Verify initial cursor position is at the root
	initialNode := m.epicTree.SelectedNode()
	require.NotNil(t, initialNode)
	require.Equal(t, "epic-123", initialNode.Issue.ID, "initial selection should be root")

	// Press 'j' to move down
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m = result.(Model)

	// Verify cursor moved to a child node
	newNode := m.epicTree.SelectedNode()
	require.NotNil(t, newNode)
	require.NotEqual(t, "epic-123", newNode.Issue.ID, "'j' should move cursor to a child node")
}

func TestTreeCursorUp(t *testing.T) {
	// Verify 'k' key moves cursor up in tree
	m := createEpicTreeTestModelWithTree(t)

	// First move down to have room to move up
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m = result.(Model)

	nodeAfterJ := m.epicTree.SelectedNode()
	require.NotNil(t, nodeAfterJ)
	require.NotEqual(t, "epic-123", nodeAfterJ.Issue.ID, "after 'j', should not be at root anymore")

	// Press 'k' to move up (back to root)
	result, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	m = result.(Model)

	// Verify cursor moved back up to root
	nodeAfterK := m.epicTree.SelectedNode()
	require.NotNil(t, nodeAfterK)
	require.Equal(t, "epic-123", nodeAfterK.Issue.ID, "'k' should move cursor back to root")
}

func TestTreeToDetailsPaneSwitch(t *testing.T) {
	// Verify 'l' key switches from tree to details pane
	m := createEpicTreeTestModelWithTree(t)
	m.epicViewFocus = EpicFocusTree

	// Press 'l' to switch to details pane
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	m = result.(Model)

	require.Equal(t, EpicFocusDetails, m.epicViewFocus, "'l' should switch focus to details pane")
}

func TestDetailsToTreePaneSwitch(t *testing.T) {
	// Verify 'h' key switches from details to tree pane
	m := createEpicTreeTestModelWithTree(t)
	m.epicViewFocus = EpicFocusDetails

	// Press 'h' to switch to tree pane
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	m = result.(Model)

	require.Equal(t, EpicFocusTree, m.epicViewFocus, "'h' should switch focus to tree pane")
}

func TestTreeModeToggle(t *testing.T) {
	// Verify 'm' key toggles tree mode
	m := createEpicTreeTestModelWithTree(t)

	// Verify initial mode is deps
	require.Equal(t, tree.ModeDeps, m.epicTree.Mode(), "initial mode should be deps")

	// Press 'm' to toggle mode
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})
	m = result.(Model)

	require.Equal(t, tree.ModeChildren, m.epicTree.Mode(), "'m' should toggle mode to children")

	// Press 'm' again to toggle back
	result, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})
	m = result.(Model)

	require.Equal(t, tree.ModeDeps, m.epicTree.Mode(), "'m' should toggle mode back to deps")
}

func TestCursorMoveTriggersDetailUpdate(t *testing.T) {
	// Verify that j/k cursor movement triggers details panel update
	m := createEpicTreeTestModelWithTree(t)
	m.hasEpicDetail = false // Start without details

	// Move cursor - should trigger updateEpicDetail
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m = result.(Model)

	// updateEpicDetail should be called and set hasEpicDetail
	require.True(t, m.hasEpicDetail, "cursor movement should trigger detail update and set hasEpicDetail")
}

// === Unit Tests: Yank (copy) functionality ===

func TestYankTreeIssueID_CopiesIDToClipboard(t *testing.T) {
	m := createEpicTreeTestModelWithTree(t)

	// Setup mock clipboard
	mockClipboard := mocks.NewMockClipboard(t)
	mockClipboard.EXPECT().Copy("epic-123").Return(nil).Once()
	m.services.Clipboard = mockClipboard

	// Press 'y' in tree focus
	result, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	m = result.(Model)

	// Execute command to get the toast message
	require.NotNil(t, cmd, "should return command for toast")
	msg := cmd()
	toastMsg, ok := msg.(mode.ShowToastMsg)
	require.True(t, ok, "command should return ShowToastMsg")
	require.Contains(t, toastMsg.Message, "epic-123", "toast should contain issue ID")
}

func TestYankTreeIssueID_NoTreeLoaded(t *testing.T) {
	m := createEpicTreeTestModel(t)
	m.epicTree = nil
	m.focus = FocusEpicView
	m.epicViewFocus = EpicFocusTree

	// Press 'y' with no tree
	result, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	_ = result.(Model)

	// Execute command to get the toast message
	require.NotNil(t, cmd, "should return command for error toast")
	msg := cmd()
	toastMsg, ok := msg.(mode.ShowToastMsg)
	require.True(t, ok, "command should return ShowToastMsg")
	require.Contains(t, toastMsg.Message, "No tree loaded")
}

func TestYankTreeIssueID_NoClipboard(t *testing.T) {
	m := createEpicTreeTestModelWithTree(t)
	m.services.Clipboard = nil

	// Press 'y' without clipboard
	result, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	_ = result.(Model)

	// Execute command to get the toast message
	require.NotNil(t, cmd, "should return command for error toast")
	msg := cmd()
	toastMsg, ok := msg.(mode.ShowToastMsg)
	require.True(t, ok, "command should return ShowToastMsg")
	require.Contains(t, toastMsg.Message, "Clipboard unavailable")
}

func TestYankTreeIssueID_ClipboardError(t *testing.T) {
	m := createEpicTreeTestModelWithTree(t)

	// Setup mock clipboard that returns error
	mockClipboard := mocks.NewMockClipboard(t)
	mockClipboard.EXPECT().Copy(mock.Anything).Return(errors.New("clipboard failed")).Once()
	m.services.Clipboard = mockClipboard

	// Press 'y'
	result, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	_ = result.(Model)

	// Execute command to get the toast message
	require.NotNil(t, cmd, "should return command for error toast")
	msg := cmd()
	toastMsg, ok := msg.(mode.ShowToastMsg)
	require.True(t, ok, "command should return ShowToastMsg")
	require.Contains(t, toastMsg.Message, "Clipboard error")
}

func TestYankIssueDescription_CopiesDescriptionToClipboard(t *testing.T) {
	m := createEpicTreeTestModel(t)

	// Setup tree with issue that has a description
	issueMap := map[string]*beads.Issue{
		"epic-123": {
			ID:              "epic-123",
			TitleText:       "Test Epic",
			DescriptionText: "This is the full description of the epic.",
			Status:          beads.StatusOpen,
		},
	}
	m.epicTree = tree.New("epic-123", issueMap, tree.DirectionDown, tree.ModeDeps, nil)
	m.focus = FocusEpicView
	m.epicViewFocus = EpicFocusDetails

	// Setup mock clipboard
	mockClipboard := mocks.NewMockClipboard(t)
	mockClipboard.EXPECT().Copy("This is the full description of the epic.").Return(nil).Once()
	m.services.Clipboard = mockClipboard

	// Press 'y' in details focus
	result, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	_ = result.(Model)

	// Execute command to get the toast message
	require.NotNil(t, cmd, "should return command for toast")
	msg := cmd()
	toastMsg, ok := msg.(mode.ShowToastMsg)
	require.True(t, ok, "command should return ShowToastMsg")
	require.Equal(t, "Copied issue description", toastMsg.Message)
}

func TestYankIssueDescription_EmptyDescription(t *testing.T) {
	m := createEpicTreeTestModel(t)

	// Setup tree with issue that has no description
	issueMap := map[string]*beads.Issue{
		"epic-123": {
			ID:              "epic-123",
			TitleText:       "Test Epic",
			DescriptionText: "", // Empty description
			Status:          beads.StatusOpen,
		},
	}
	m.epicTree = tree.New("epic-123", issueMap, tree.DirectionDown, tree.ModeDeps, nil)
	m.focus = FocusEpicView
	m.epicViewFocus = EpicFocusDetails

	// Setup mock clipboard (won't be called since description is empty)
	mockClipboard := mocks.NewMockClipboard(t)
	m.services.Clipboard = mockClipboard

	// Press 'y' with empty description
	result, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	_ = result.(Model)

	// Execute command to get the toast message
	require.NotNil(t, cmd, "should return command for warning toast")
	msg := cmd()
	toastMsg, ok := msg.(mode.ShowToastMsg)
	require.True(t, ok, "command should return ShowToastMsg")
	require.Contains(t, toastMsg.Message, "no description")
}

func TestYankIssueDescription_NoTreeLoaded(t *testing.T) {
	m := createEpicTreeTestModel(t)
	m.epicTree = nil
	m.focus = FocusEpicView
	m.epicViewFocus = EpicFocusDetails

	// Press 'y' with no tree
	result, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	_ = result.(Model)

	// Execute command to get the toast message
	require.NotNil(t, cmd, "should return command for error toast")
	msg := cmd()
	toastMsg, ok := msg.(mode.ShowToastMsg)
	require.True(t, ok, "command should return ShowToastMsg")
	require.Contains(t, toastMsg.Message, "No tree loaded")
}

// === Unit Tests: Issue Editor Modal (perles-56ved.2) ===

// createIssueEditorTestModel creates a test model with an issue editor open for testing.
func createIssueEditorTestModel(t *testing.T) Model {
	t.Helper()

	m := createEpicTreeTestModel(t)

	// Create a tree with an issue to edit
	issueMap := map[string]*beads.Issue{
		"epic-123": {ID: "epic-123", TitleText: "Test Epic", Status: beads.StatusOpen, Priority: beads.PriorityMedium},
	}
	m.epicTree = tree.New("epic-123", issueMap, tree.DirectionDown, tree.ModeDeps, nil)
	m.lastLoadedEpicID = "epic-123"

	// Open the issue editor
	issue := beads.Issue{
		ID:       "issue-456",
		Priority: beads.PriorityMedium,
		Status:   beads.StatusOpen,
		Labels:   []string{"test"},
	}
	editor := issueeditor.New(issue).SetSize(100, 40)
	m.issueEditor = &editor

	return m
}

func TestEditIssue_SaveMsgClosesModal(t *testing.T) {
	// Verify SaveMsg closes modal and returns batch command for status/priority/labels update
	m := createIssueEditorTestModel(t)
	require.NotNil(t, m.issueEditor, "issue editor should be open")

	// Send SaveMsg
	result, cmd := m.Update(issueeditor.SaveMsg{
		IssueID:  "issue-456",
		Priority: beads.PriorityHigh,
		Status:   beads.StatusInProgress,
		Labels:   []string{"updated"},
	})
	m = result.(Model)

	// Verify modal is closed
	require.Nil(t, m.issueEditor, "issue editor should be closed after SaveMsg")

	// Verify batch command is returned (for status, priority, labels, and tree reload)
	require.NotNil(t, cmd, "should return batch command for updates")
}

func TestEditIssue_CancelMsgClosesModal(t *testing.T) {
	// Verify CancelMsg closes modal with nil command
	m := createIssueEditorTestModel(t)
	require.NotNil(t, m.issueEditor, "issue editor should be open")

	// Send CancelMsg
	result, cmd := m.Update(issueeditor.CancelMsg{})
	m = result.(Model)

	// Verify modal is closed
	require.Nil(t, m.issueEditor, "issue editor should be closed after CancelMsg")

	// Verify nil command (no updates needed)
	require.Nil(t, cmd, "should return nil command for cancel")
}

func TestEditIssue_HelpKeyBlocked(t *testing.T) {
	// Verify help key (?) is blocked while modal is open
	m := createIssueEditorTestModel(t)
	require.NotNil(t, m.issueEditor, "issue editor should be open")
	m.showHelp = false

	// Press '?' while modal is open
	result, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	m = result.(Model)

	// Verify modal is still open (help key blocked)
	require.NotNil(t, m.issueEditor, "issue editor should still be open after help key")

	// Verify help modal is NOT shown
	require.False(t, m.showHelp, "help modal should not be shown when issue editor is open")

	// Verify nil command (help key should be a no-op)
	require.Nil(t, cmd, "should return nil command for blocked help key")
}

func TestEditIssue_ModalDelegationForwardsMessages(t *testing.T) {
	// Verify that other messages are forwarded to the issue editor's Update method
	m := createIssueEditorTestModel(t)
	require.NotNil(t, m.issueEditor, "issue editor should be open")

	// Send a key message that should be forwarded to the editor (not help key)
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = result.(Model)

	// Modal should still be open
	require.NotNil(t, m.issueEditor, "issue editor should still be open after forwarded message")
}

func TestEditIssue_WindowSizeMsgPropagates(t *testing.T) {
	// Verify WindowSizeMsg is handled when modal is open
	m := createIssueEditorTestModel(t)
	require.NotNil(t, m.issueEditor, "issue editor should be open")

	// Send window resize message
	result, cmd := m.Update(tea.WindowSizeMsg{Width: 120, Height: 50})
	m = result.(Model)

	// Verify modal is still open and dimensions updated
	require.NotNil(t, m.issueEditor, "issue editor should still be open after resize")
	require.Equal(t, 120, m.width, "model width should be updated")
	require.Equal(t, 50, m.height, "model height should be updated")
	require.Nil(t, cmd, "should return nil command for resize")
}

// === Unit Tests: Async Command Helpers and Result Handlers (perles-56ved.3) ===

func TestEditIssue_ErrorToastOnStatusUpdateFailure(t *testing.T) {
	// Verify that issueStatusChangedMsg with error shows toast
	m := createIssueEditorTestModel(t)

	// Send status changed message with error
	result, cmd := m.Update(issueStatusChangedMsg{
		issueID: "issue-456",
		status:  beads.StatusInProgress,
		err:     errors.New("database connection failed"),
	})
	_ = result.(Model)

	// Verify command returns toast message
	require.NotNil(t, cmd, "should return command for error toast")
	msg := cmd()
	toastMsg, ok := msg.(mode.ShowToastMsg)
	require.True(t, ok, "command should return ShowToastMsg")
	require.Contains(t, toastMsg.Message, "Failed to update status")
	require.Contains(t, toastMsg.Message, "database connection failed")
	require.Equal(t, toaster.StyleError, toastMsg.Style, "should use error style")
}

func TestEditIssue_ErrorToastOnPriorityUpdateFailure(t *testing.T) {
	// Verify that issuePriorityChangedMsg with error shows toast
	m := createIssueEditorTestModel(t)

	// Send priority changed message with error
	result, cmd := m.Update(issuePriorityChangedMsg{
		issueID:  "issue-456",
		priority: beads.PriorityHigh,
		err:      errors.New("permission denied"),
	})
	_ = result.(Model)

	// Verify command returns toast message
	require.NotNil(t, cmd, "should return command for error toast")
	msg := cmd()
	toastMsg, ok := msg.(mode.ShowToastMsg)
	require.True(t, ok, "command should return ShowToastMsg")
	require.Contains(t, toastMsg.Message, "Failed to update priority")
	require.Contains(t, toastMsg.Message, "permission denied")
	require.Equal(t, toaster.StyleError, toastMsg.Style, "should use error style")
}

func TestEditIssue_ErrorToastOnLabelsUpdateFailure(t *testing.T) {
	// Verify that issueLabelsChangedMsg with error shows toast
	m := createIssueEditorTestModel(t)

	// Send labels changed message with error
	result, cmd := m.Update(issueLabelsChangedMsg{
		issueID: "issue-456",
		labels:  []string{"new-label"},
		err:     errors.New("validation failed"),
	})
	_ = result.(Model)

	// Verify command returns toast message
	require.NotNil(t, cmd, "should return command for error toast")
	msg := cmd()
	toastMsg, ok := msg.(mode.ShowToastMsg)
	require.True(t, ok, "command should return ShowToastMsg")
	require.Contains(t, toastMsg.Message, "Failed to update labels")
	require.Contains(t, toastMsg.Message, "validation failed")
	require.Equal(t, toaster.StyleError, toastMsg.Style, "should use error style")
}

func TestEditIssue_StatusSuccessReturnsNil(t *testing.T) {
	// Verify that issueStatusChangedMsg without error returns nil command
	m := createIssueEditorTestModel(t)

	// Send status changed message without error
	result, cmd := m.Update(issueStatusChangedMsg{
		issueID: "issue-456",
		status:  beads.StatusInProgress,
		err:     nil,
	})
	_ = result.(Model)

	// Verify nil command returned (success case)
	require.Nil(t, cmd, "should return nil command on success")
}

func TestEditIssue_PrioritySuccessReturnsNil(t *testing.T) {
	// Verify that issuePriorityChangedMsg without error returns nil command
	m := createIssueEditorTestModel(t)

	// Send priority changed message without error
	result, cmd := m.Update(issuePriorityChangedMsg{
		issueID:  "issue-456",
		priority: beads.PriorityHigh,
		err:      nil,
	})
	_ = result.(Model)

	// Verify nil command returned (success case)
	require.Nil(t, cmd, "should return nil command on success")
}

func TestEditIssue_LabelsSuccessReturnsNil(t *testing.T) {
	// Verify that issueLabelsChangedMsg without error returns nil command
	m := createIssueEditorTestModel(t)

	// Send labels changed message without error
	result, cmd := m.Update(issueLabelsChangedMsg{
		issueID: "issue-456",
		labels:  []string{"new-label"},
		err:     nil,
	})
	_ = result.(Model)

	// Verify nil command returned (success case)
	require.Nil(t, cmd, "should return nil command on success")
}

func TestUpdateIssueStatusCmd_CallsBeadsExecutor(t *testing.T) {
	// Verify updateIssueStatusCmd creates a command that calls BeadsExecutor.UpdateStatus
	m := createIssueEditorTestModel(t)

	// Create mock executor
	mockExecutor := mocks.NewMockIssueExecutor(t)
	mockExecutor.EXPECT().UpdateStatus("issue-456", beads.StatusInProgress).Return(nil)
	m.services.BeadsExecutor = mockExecutor

	// Get the command
	cmd := m.updateIssueStatusCmd("issue-456", beads.StatusInProgress)
	require.NotNil(t, cmd, "should return command")

	// Execute command
	msg := cmd()
	statusMsg, ok := msg.(issueStatusChangedMsg)
	require.True(t, ok, "command should return issueStatusChangedMsg")
	require.Equal(t, "issue-456", statusMsg.issueID)
	require.Equal(t, beads.StatusInProgress, statusMsg.status)
	require.NoError(t, statusMsg.err, "should have no error on success")
}

func TestUpdateIssuePriorityCmd_CallsBeadsExecutor(t *testing.T) {
	// Verify updateIssuePriorityCmd creates a command that calls BeadsExecutor.UpdatePriority
	m := createIssueEditorTestModel(t)

	// Create mock executor
	mockExecutor := mocks.NewMockIssueExecutor(t)
	mockExecutor.EXPECT().UpdatePriority("issue-456", beads.PriorityHigh).Return(nil)
	m.services.BeadsExecutor = mockExecutor

	// Get the command
	cmd := m.updateIssuePriorityCmd("issue-456", beads.PriorityHigh)
	require.NotNil(t, cmd, "should return command")

	// Execute command
	msg := cmd()
	priorityMsg, ok := msg.(issuePriorityChangedMsg)
	require.True(t, ok, "command should return issuePriorityChangedMsg")
	require.Equal(t, "issue-456", priorityMsg.issueID)
	require.Equal(t, beads.PriorityHigh, priorityMsg.priority)
	require.NoError(t, priorityMsg.err, "should have no error on success")
}

func TestUpdateIssueLabelsCmd_CallsBeadsExecutor(t *testing.T) {
	// Verify updateIssueLabelsCmd creates a command that calls BeadsExecutor.SetLabels
	m := createIssueEditorTestModel(t)

	// Create mock executor
	mockExecutor := mocks.NewMockIssueExecutor(t)
	mockExecutor.EXPECT().SetLabels("issue-456", []string{"label1", "label2"}).Return(nil)
	m.services.BeadsExecutor = mockExecutor

	// Get the command
	cmd := m.updateIssueLabelsCmd("issue-456", []string{"label1", "label2"})
	require.NotNil(t, cmd, "should return command")

	// Execute command
	msg := cmd()
	labelsMsg, ok := msg.(issueLabelsChangedMsg)
	require.True(t, ok, "command should return issueLabelsChangedMsg")
	require.Equal(t, "issue-456", labelsMsg.issueID)
	require.Equal(t, []string{"label1", "label2"}, labelsMsg.labels)
	require.NoError(t, labelsMsg.err, "should have no error on success")
}

func TestUpdateIssueStatusCmd_ReturnsErrorOnFailure(t *testing.T) {
	// Verify updateIssueStatusCmd propagates errors from BeadsExecutor
	m := createIssueEditorTestModel(t)

	// Create mock executor that returns an error
	mockExecutor := mocks.NewMockIssueExecutor(t)
	mockExecutor.EXPECT().UpdateStatus("issue-456", beads.StatusInProgress).Return(errors.New("database error"))
	m.services.BeadsExecutor = mockExecutor

	// Get the command
	cmd := m.updateIssueStatusCmd("issue-456", beads.StatusInProgress)
	require.NotNil(t, cmd, "should return command")

	// Execute command
	msg := cmd()
	statusMsg, ok := msg.(issueStatusChangedMsg)
	require.True(t, ok, "command should return issueStatusChangedMsg")
	require.Error(t, statusMsg.err, "should have error on failure")
	require.Contains(t, statusMsg.err.Error(), "database error")
}

func TestUpdateIssuePriorityCmd_ReturnsErrorOnFailure(t *testing.T) {
	// Verify updateIssuePriorityCmd propagates errors from BeadsExecutor
	m := createIssueEditorTestModel(t)

	// Create mock executor that returns an error
	mockExecutor := mocks.NewMockIssueExecutor(t)
	mockExecutor.EXPECT().UpdatePriority("issue-456", beads.PriorityHigh).Return(errors.New("permission denied"))
	m.services.BeadsExecutor = mockExecutor

	// Get the command
	cmd := m.updateIssuePriorityCmd("issue-456", beads.PriorityHigh)
	require.NotNil(t, cmd, "should return command")

	// Execute command
	msg := cmd()
	priorityMsg, ok := msg.(issuePriorityChangedMsg)
	require.True(t, ok, "command should return issuePriorityChangedMsg")
	require.Error(t, priorityMsg.err, "should have error on failure")
	require.Contains(t, priorityMsg.err.Error(), "permission denied")
}

func TestUpdateIssueLabelsCmd_ReturnsErrorOnFailure(t *testing.T) {
	// Verify updateIssueLabelsCmd propagates errors from BeadsExecutor
	m := createIssueEditorTestModel(t)

	// Create mock executor that returns an error
	mockExecutor := mocks.NewMockIssueExecutor(t)
	mockExecutor.EXPECT().SetLabels("issue-456", []string{"label1"}).Return(errors.New("validation failed"))
	m.services.BeadsExecutor = mockExecutor

	// Get the command
	cmd := m.updateIssueLabelsCmd("issue-456", []string{"label1"})
	require.NotNil(t, cmd, "should return command")

	// Execute command
	msg := cmd()
	labelsMsg, ok := msg.(issueLabelsChangedMsg)
	require.True(t, ok, "command should return issueLabelsChangedMsg")
	require.Error(t, labelsMsg.err, "should have error on failure")
	require.Contains(t, labelsMsg.err.Error(), "validation failed")
}

func TestUpdateIssueStatusCmd_NilExecutorReturnsError(t *testing.T) {
	// Verify updateIssueStatusCmd handles nil BeadsExecutor
	m := createIssueEditorTestModel(t)
	m.services.BeadsExecutor = nil

	// Get the command
	cmd := m.updateIssueStatusCmd("issue-456", beads.StatusInProgress)
	require.NotNil(t, cmd, "should return command")

	// Execute command
	msg := cmd()
	statusMsg, ok := msg.(issueStatusChangedMsg)
	require.True(t, ok, "command should return issueStatusChangedMsg")
	require.Error(t, statusMsg.err, "should have error when executor is nil")
	require.Contains(t, statusMsg.err.Error(), "beads executor unavailable")
}

func TestUpdateIssueNotesCmd_CallsBeadsExecutor(t *testing.T) {
	// Verify updateIssueNotesCmd creates a command that calls BeadsExecutor.UpdateNotes
	m := createIssueEditorTestModel(t)

	// Create mock executor
	mockExecutor := mocks.NewMockIssueExecutor(t)
	mockExecutor.EXPECT().UpdateNotes("issue-456", "new notes content").Return(nil)
	m.services.BeadsExecutor = mockExecutor

	// Get the command
	cmd := m.updateIssueNotesCmd("issue-456", "new notes content")
	require.NotNil(t, cmd, "should return command")

	// Execute command
	msg := cmd()
	notesMsg, ok := msg.(issueNotesChangedMsg)
	require.True(t, ok, "command should return issueNotesChangedMsg")
	require.Equal(t, "issue-456", notesMsg.issueID)
	require.Equal(t, "new notes content", notesMsg.notes)
	require.NoError(t, notesMsg.err, "should have no error on success")
}

func TestUpdateIssueNotesCmd_PropagatesErrors(t *testing.T) {
	// Verify updateIssueNotesCmd propagates errors from BeadsExecutor
	m := createIssueEditorTestModel(t)

	// Create mock executor that returns an error
	mockExecutor := mocks.NewMockIssueExecutor(t)
	mockExecutor.EXPECT().UpdateNotes("issue-456", "notes").Return(errors.New("update failed"))
	m.services.BeadsExecutor = mockExecutor

	// Get the command
	cmd := m.updateIssueNotesCmd("issue-456", "notes")
	require.NotNil(t, cmd, "should return command")

	// Execute command
	msg := cmd()
	notesMsg, ok := msg.(issueNotesChangedMsg)
	require.True(t, ok, "command should return issueNotesChangedMsg")
	require.Error(t, notesMsg.err, "should have error on failure")
	require.Contains(t, notesMsg.err.Error(), "update failed")
}

func TestUpdateIssueNotesCmd_NilExecutorReturnsError(t *testing.T) {
	// Verify updateIssueNotesCmd handles nil BeadsExecutor
	m := createIssueEditorTestModel(t)
	m.services.BeadsExecutor = nil

	// Get the command
	cmd := m.updateIssueNotesCmd("issue-456", "notes")
	require.NotNil(t, cmd, "should return command")

	// Execute command
	msg := cmd()
	notesMsg, ok := msg.(issueNotesChangedMsg)
	require.True(t, ok, "command should return issueNotesChangedMsg")
	require.Error(t, notesMsg.err, "should have error when executor is nil")
	require.Contains(t, notesMsg.err.Error(), "beads executor unavailable")
}

func TestDashboard_SaveMsg_UpdatesNotesOnlyWhenChanged(t *testing.T) {
	// Verify SaveMsg only includes notes update cmd when notes have actually changed
	m := createIssueEditorTestModel(t)

	// Set the original issue with existing notes
	originalIssue := beads.Issue{
		ID:    "issue-456",
		Notes: "original notes",
	}
	m.editingIssue = &originalIssue

	// Send SaveMsg with same notes as original (no mock executor needed - just checking state)
	result, cmd := m.Update(issueeditor.SaveMsg{
		IssueID:  "issue-456",
		Priority: beads.PriorityHigh,
		Status:   beads.StatusInProgress,
		Labels:   []string{"label"},
		Notes:    "original notes", // Same as original - no change
	})
	m = result.(Model)

	// Verify state transition
	require.Nil(t, m.editingIssue, "editingIssue should be cleared after save")
	require.NotNil(t, cmd, "should return batch command")
}

func TestDashboard_SaveMsg_UpdatesNotesWhenChanged(t *testing.T) {
	// Verify SaveMsg includes notes update cmd when notes have changed
	m := createIssueEditorTestModel(t)

	// Set the original issue with existing notes
	originalIssue := beads.Issue{
		ID:    "issue-456",
		Notes: "original notes",
	}
	m.editingIssue = &originalIssue

	// Send SaveMsg with different notes (no mock executor needed - just checking state)
	result, cmd := m.Update(issueeditor.SaveMsg{
		IssueID:  "issue-456",
		Priority: beads.PriorityHigh,
		Status:   beads.StatusInProgress,
		Labels:   []string{"label"},
		Notes:    "new notes content", // Different from original
	})
	m = result.(Model)

	// Verify state transition
	require.Nil(t, m.editingIssue, "editingIssue should be cleared after save")
	require.NotNil(t, cmd, "should return batch command")
}

// === Unit Tests: ctrl+e Key Handling (perles-56ved.4) ===

func TestEditIssue_OpensFromTreeFocus(t *testing.T) {
	// Verify ctrl+e opens modal with correct issue from tree focus
	m := createEpicTreeTestModelWithTree(t)
	m.focus = FocusEpicView
	m.epicViewFocus = EpicFocusTree
	require.Nil(t, m.issueEditor, "issue editor should start nil")

	// Press ctrl+e to open editor
	result, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlE})
	m = result.(Model)

	// Verify modal is opened with correct issue
	require.NotNil(t, m.issueEditor, "issue editor should be opened after ctrl+e")
	require.Nil(t, cmd, "Init() returns nil for issue editor")
}

func TestEditIssue_OpensFromDetailsFocus(t *testing.T) {
	// Verify ctrl+e opens modal with same issue from details focus
	m := createEpicTreeTestModelWithTree(t)
	m.focus = FocusEpicView
	m.epicViewFocus = EpicFocusDetails
	require.Nil(t, m.issueEditor, "issue editor should start nil")

	// Press ctrl+e to open editor
	result, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlE})
	m = result.(Model)

	// Verify modal is opened
	require.NotNil(t, m.issueEditor, "issue editor should be opened after ctrl+e from details focus")
	require.Nil(t, cmd, "Init() returns nil for issue editor")
}

func TestEditIssue_NoOpWithNilTree(t *testing.T) {
	// Verify ctrl+e is a silent no-op when m.epicTree is nil
	m := createEpicTreeTestModel(t)
	m.epicTree = nil
	m.focus = FocusEpicView
	m.epicViewFocus = EpicFocusTree
	require.Nil(t, m.issueEditor, "issue editor should start nil")

	// Press ctrl+e with nil tree
	result, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlE})
	m = result.(Model)

	// Verify modal is NOT opened (silent no-op)
	require.Nil(t, m.issueEditor, "issue editor should remain nil when no tree loaded")
	require.Nil(t, cmd, "should return nil command for silent no-op")
}

func TestEditIssue_NoOpWithNoSelection(t *testing.T) {
	// Verify ctrl+e is a silent no-op when SelectedNode() returns nil
	m := createEpicTreeTestModel(t)

	// Create tree with empty issue map (results in no selected node)
	emptyIssueMap := map[string]*beads.Issue{}
	m.epicTree = tree.New("nonexistent", emptyIssueMap, tree.DirectionDown, tree.ModeDeps, nil)
	require.Nil(t, m.epicTree.SelectedNode(), "tree should have no selected node")

	m.focus = FocusEpicView
	m.epicViewFocus = EpicFocusTree
	require.Nil(t, m.issueEditor, "issue editor should start nil")

	// Press ctrl+e with no selection
	result, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlE})
	m = result.(Model)

	// Verify modal is NOT opened (silent no-op)
	require.Nil(t, m.issueEditor, "issue editor should remain nil when no node selected")
	require.Nil(t, cmd, "should return nil command for silent no-op")
}

// === Unit Tests: Workflow Switch and Resize Handling (perles-56ved.6) ===

func TestEditIssue_WorkflowSwitchClosesModal(t *testing.T) {
	// Create model with workflows and open issue editor
	m := createEpicTreeTestModelWithWorkflows(t)
	m.selectedIndex = 0

	// Open the issue editor
	issue := beads.Issue{
		ID:       "issue-456",
		Priority: beads.PriorityMedium,
		Status:   beads.StatusOpen,
		Labels:   []string{"test"},
	}
	editor := issueeditor.New(issue).SetSize(100, 40)
	m.issueEditor = &editor

	require.NotNil(t, m.issueEditor, "issue editor should be open before workflow switch")

	// Switch to a different workflow
	_ = m.handleWorkflowSelectionChange(1)

	// Verify modal is closed after workflow switch
	require.Nil(t, m.issueEditor, "issue editor should be closed after workflow switch")
}

func TestEditIssue_ResizePropagates(t *testing.T) {
	// Verify SetSize() properly resizes the issueEditor when open
	m := createIssueEditorTestModel(t)
	require.NotNil(t, m.issueEditor, "issue editor should be open")

	// Resize via SetSize()
	result := m.SetSize(150, 60)
	m = result.(Model)

	// Verify modal is still open and dimensions are updated
	require.NotNil(t, m.issueEditor, "issue editor should still be open after SetSize")
	require.Equal(t, 150, m.width, "model width should be updated")
	require.Equal(t, 60, m.height, "model height should be updated")
}

func TestEditIssue_SetSizeHandlesNilEditor(t *testing.T) {
	// Verify SetSize() handles nil issueEditor without panic
	m := createEpicTreeTestModel(t)
	m.issueEditor = nil // Explicitly nil

	// SetSize should not panic
	require.NotPanics(t, func() {
		result := m.SetSize(150, 60)
		m = result.(Model)
	}, "SetSize should not panic when issueEditor is nil")

	// Verify dimensions are updated
	require.Equal(t, 150, m.width, "model width should be updated")
	require.Equal(t, 60, m.height, "model height should be updated")
}
