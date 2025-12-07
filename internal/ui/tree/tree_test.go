package tree

import (
	"testing"

	"perles/internal/beads"

	"github.com/charmbracelet/x/exp/teatest"
	"github.com/stretchr/testify/require"
)

func makeTestIssueMap() map[string]*beads.Issue {
	return map[string]*beads.Issue{
		"epic-1": {
			ID:        "epic-1",
			TitleText: "Epic One",
			Status:    beads.StatusOpen,
			Type:      beads.TypeEpic,
			Priority:  beads.PriorityHigh,
			Children:  []string{"task-1", "task-2"},
		},
		"task-1": {
			ID:        "task-1",
			TitleText: "Task One",
			Status:    beads.StatusClosed,
			Type:      beads.TypeTask,
			Priority:  beads.PriorityCritical,
			ParentID:  "epic-1",
		},
		"task-2": {
			ID:        "task-2",
			TitleText: "Task Two",
			Status:    beads.StatusOpen,
			Type:      beads.TypeTask,
			Priority:  beads.PriorityMedium,
			ParentID:  "epic-1",
			Children:  []string{"subtask-1"},
		},
		"subtask-1": {
			ID:        "subtask-1",
			TitleText: "Subtask One",
			Status:    beads.StatusInProgress,
			Type:      beads.TypeTask,
			Priority:  beads.PriorityMedium,
			ParentID:  "task-2",
		},
	}
}

func TestNew_Basic(t *testing.T) {
	issueMap := makeTestIssueMap()
	m := New("epic-1", issueMap, DirectionDown, ModeDeps)

	require.NotNil(t, m)
	require.NotNil(t, m.root)
	require.Equal(t, "epic-1", m.root.Issue.ID)
	require.Equal(t, DirectionDown, m.direction)
	require.Equal(t, "epic-1", m.originalID)
	require.Equal(t, 0, m.cursor)
	require.Len(t, m.nodes, 4) // epic-1, task-1, task-2, subtask-1
}

func TestNew_InvalidRoot(t *testing.T) {
	issueMap := makeTestIssueMap()
	m := New("nonexistent", issueMap, DirectionDown, ModeDeps)

	require.NotNil(t, m)
	require.Nil(t, m.root)
	require.Empty(t, m.nodes)
}

func TestSetSize(t *testing.T) {
	issueMap := makeTestIssueMap()
	m := New("epic-1", issueMap, DirectionDown, ModeDeps)

	m.SetSize(80, 24)
	require.Equal(t, 80, m.width)
	require.Equal(t, 24, m.height)
}

func TestMoveCursor_Basic(t *testing.T) {
	issueMap := makeTestIssueMap()
	m := New("epic-1", issueMap, DirectionDown, ModeDeps)

	require.Equal(t, 0, m.cursor)

	m.MoveCursor(1)
	require.Equal(t, 1, m.cursor)

	m.MoveCursor(1)
	require.Equal(t, 2, m.cursor)

	m.MoveCursor(-1)
	require.Equal(t, 1, m.cursor)
}

func TestMoveCursor_Bounds(t *testing.T) {
	issueMap := makeTestIssueMap()
	m := New("epic-1", issueMap, DirectionDown, ModeDeps)

	// Try to go above top
	m.MoveCursor(-10)
	require.Equal(t, 0, m.cursor)

	// Try to go below bottom
	m.MoveCursor(100)
	require.Equal(t, 3, m.cursor) // Last node (4 nodes, index 3)

	// Still at bottom after trying to go further
	m.MoveCursor(1)
	require.Equal(t, 3, m.cursor)
}

func TestSelectedNode(t *testing.T) {
	issueMap := makeTestIssueMap()
	m := New("epic-1", issueMap, DirectionDown, ModeDeps)

	node := m.SelectedNode()
	require.NotNil(t, node)
	require.Equal(t, "epic-1", node.Issue.ID)

	m.MoveCursor(1)
	node = m.SelectedNode()
	require.Equal(t, "task-1", node.Issue.ID)
}

func TestSelectedNode_Empty(t *testing.T) {
	issueMap := makeTestIssueMap()
	m := New("nonexistent", issueMap, DirectionDown, ModeDeps)

	node := m.SelectedNode()
	require.Nil(t, node)
}

func TestRoot(t *testing.T) {
	issueMap := makeTestIssueMap()
	m := New("epic-1", issueMap, DirectionDown, ModeDeps)

	root := m.Root()
	require.NotNil(t, root)
	require.Equal(t, "epic-1", root.Issue.ID)
}

func TestDirection(t *testing.T) {
	issueMap := makeTestIssueMap()
	m := New("epic-1", issueMap, DirectionDown, ModeDeps)

	require.Equal(t, DirectionDown, m.Direction())

	m.SetDirection(DirectionUp)
	require.Equal(t, DirectionUp, m.Direction())
}

func TestRefocus_AndGoBack(t *testing.T) {
	issueMap := makeTestIssueMap()
	m := New("epic-1", issueMap, DirectionDown, ModeDeps)

	// Refocus on task-2
	err := m.Refocus("task-2")
	require.NoError(t, err)
	require.Equal(t, "task-2", m.root.Issue.ID)
	require.Len(t, m.rootStack, 1)
	require.Equal(t, "epic-1", m.rootStack[0])

	// Go back
	needsRequery, _ := m.GoBack()
	require.False(t, needsRequery)
	require.Equal(t, "epic-1", m.root.Issue.ID)
	require.Empty(t, m.rootStack)
}

func TestRefocus_MultipleAndGoToOriginal(t *testing.T) {
	issueMap := makeTestIssueMap()
	m := New("epic-1", issueMap, DirectionDown, ModeDeps)

	// Refocus twice
	_ = m.Refocus("task-2")
	_ = m.Refocus("subtask-1")
	require.Len(t, m.rootStack, 2)
	require.Equal(t, "subtask-1", m.root.Issue.ID)

	// Go to original
	err := m.GoToOriginal()
	require.NoError(t, err)
	require.Equal(t, "epic-1", m.root.Issue.ID)
	require.Empty(t, m.rootStack)
}

func TestGoBack_EmptyStack_NoParent(t *testing.T) {
	issueMap := makeTestIssueMap()
	m := New("epic-1", issueMap, DirectionDown, ModeDeps)

	// GoBack on empty stack with no parent should do nothing
	needsRequery, parentID := m.GoBack()
	require.False(t, needsRequery)
	require.Empty(t, parentID)
	require.Equal(t, "epic-1", m.root.Issue.ID)
}

func TestGoBack_EmptyStack_WithParentInMap(t *testing.T) {
	issueMap := makeTestIssueMap()
	// Start directly on task-1 which has parent epic-1
	m := New("task-1", issueMap, DirectionDown, ModeDeps)

	require.Equal(t, "task-1", m.root.Issue.ID)
	require.Empty(t, m.rootStack)

	// GoBack should navigate to parent (epic-1 is in the map)
	needsRequery, _ := m.GoBack()
	require.False(t, needsRequery)
	require.Equal(t, "epic-1", m.root.Issue.ID)
}

func TestGoBack_EmptyStack_ParentNotInMap(t *testing.T) {
	// Create a minimal issue map without the parent
	issueMap := map[string]*beads.Issue{
		"task-1": {
			ID:        "task-1",
			TitleText: "Task One",
			Status:    beads.StatusOpen,
			ParentID:  "missing-parent",
		},
	}
	m := New("task-1", issueMap, DirectionDown, ModeDeps)

	// GoBack should signal re-query needed when parent not in map
	needsRequery, parentID := m.GoBack()
	require.True(t, needsRequery)
	require.Equal(t, "missing-parent", parentID)
	// Root should be unchanged
	require.Equal(t, "task-1", m.root.Issue.ID)
}

func TestView_Basic(t *testing.T) {
	issueMap := makeTestIssueMap()
	m := New("epic-1", issueMap, DirectionDown, ModeDeps)
	m.SetSize(80, 24)

	view := m.View()

	// Should contain issue IDs
	require.Contains(t, view, "epic-1")
	require.Contains(t, view, "task-1")
	require.Contains(t, view, "task-2")
	require.Contains(t, view, "subtask-1")

	// Should contain selection indicator
	require.Contains(t, view, ">")
}

func TestView_UpDirection(t *testing.T) {
	issueMap := makeTestIssueMap()
	m := New("task-1", issueMap, DirectionUp, ModeDeps)
	m.SetSize(80, 24)

	// Direction should be up (parent container uses this for border title)
	require.Equal(t, DirectionUp, m.Direction())

	// View should still render the tree nodes
	view := m.View()
	require.Contains(t, view, "task-1")
}

func TestView_TreeBranches(t *testing.T) {
	issueMap := makeTestIssueMap()
	m := New("epic-1", issueMap, DirectionDown, ModeDeps)
	m.SetSize(80, 24)

	view := m.View()

	// Should contain tree branch characters
	require.Contains(t, view, "├─")
	require.Contains(t, view, "└─")
}

func TestView_StatusIndicators(t *testing.T) {
	issueMap := makeTestIssueMap()
	m := New("epic-1", issueMap, DirectionDown, ModeDeps)
	m.SetSize(80, 24)

	view := m.View()

	// Should have status indicators
	require.Contains(t, view, "✓") // closed
	require.Contains(t, view, "○") // open
	require.Contains(t, view, "●") // in_progress
}

func TestView_Empty(t *testing.T) {
	issueMap := makeTestIssueMap()
	m := New("nonexistent", issueMap, DirectionDown, ModeDeps)

	view := m.View()
	require.Contains(t, view, "No tree data")
}

// Golden tests for tree UI rendering
// Run with -update flag to update golden files: go test -update ./internal/ui/tree/...

// TestView_Golden_Basic tests the basic tree view rendering with multiple nodes.
func TestView_Golden_Basic(t *testing.T) {
	issueMap := makeTestIssueMap()
	m := New("epic-1", issueMap, DirectionDown, ModeDeps)
	m.SetSize(100, 30)

	view := m.View()
	teatest.RequireEqualOutput(t, []byte(view))
}

// TestView_Golden_UpDirection tests tree view with up direction.
func TestView_Golden_UpDirection(t *testing.T) {
	issueMap := makeTestIssueMap()
	m := New("subtask-1", issueMap, DirectionUp, ModeDeps)
	m.SetSize(100, 30)

	view := m.View()
	teatest.RequireEqualOutput(t, []byte(view))
}

// TestView_Golden_CursorMoved tests tree view with cursor on different node.
func TestView_Golden_CursorMoved(t *testing.T) {
	issueMap := makeTestIssueMap()
	m := New("epic-1", issueMap, DirectionDown, ModeDeps)
	m.SetSize(100, 30)

	// Move cursor to task-2 (index 2)
	m.MoveCursor(2)

	view := m.View()
	teatest.RequireEqualOutput(t, []byte(view))
}

// TestView_Golden_Empty tests tree view with no data.
func TestView_Golden_Empty(t *testing.T) {
	issueMap := makeTestIssueMap()
	m := New("nonexistent", issueMap, DirectionDown, ModeDeps)
	m.SetSize(100, 30)

	view := m.View()
	teatest.RequireEqualOutput(t, []byte(view))
}

// TestView_Golden_LeafNode tests tree view when root has no children.
func TestView_Golden_LeafNode(t *testing.T) {
	issueMap := makeTestIssueMap()
	m := New("task-1", issueMap, DirectionDown, ModeDeps)
	m.SetSize(100, 30)

	view := m.View()
	teatest.RequireEqualOutput(t, []byte(view))
}
