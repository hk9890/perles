package threadpicker

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hk9890/perles/internal/orchestration/fabric/domain"
)

func makeThread(id, createdBy, content string) domain.Thread {
	return domain.Thread{
		ID:        id,
		CreatedBy: createdBy,
		Content:   content,
		CreatedAt: time.Now(),
	}
}

func TestNew(t *testing.T) {
	m := New()
	assert.False(t, m.IsActive())
	assert.Equal(t, 0, m.ThreadCount())
	assert.Nil(t, m.Selected())
}

func TestActivate(t *testing.T) {
	m := New()
	threads := []domain.Thread{
		makeThread("t1", "coordinator", "First thread"),
		makeThread("t2", "worker-1", "Second thread"),
	}

	m = m.Activate(threads)

	assert.True(t, m.IsActive())
	assert.Equal(t, 2, m.ThreadCount())
	require.NotNil(t, m.Selected())
	assert.Equal(t, "t1", m.Selected().ID)
}

func TestDeactivate(t *testing.T) {
	m := New()
	threads := []domain.Thread{
		makeThread("t1", "coordinator", "First thread"),
	}

	m = m.Activate(threads)
	assert.True(t, m.IsActive())

	m = m.Deactivate()
	assert.False(t, m.IsActive())
	assert.Nil(t, m.Selected())
}

func TestNavigation(t *testing.T) {
	m := New()
	threads := []domain.Thread{
		makeThread("t1", "coordinator", "First thread"),
		makeThread("t2", "worker-1", "Second thread"),
		makeThread("t3", "worker-2", "Third thread"),
	}
	m = m.Activate(threads)

	// Start at first item
	require.Equal(t, "t1", m.Selected().ID)

	// Next
	m = m.Next()
	require.Equal(t, "t2", m.Selected().ID)

	// Next again
	m = m.Next()
	require.Equal(t, "t3", m.Selected().ID)

	// Next wraps around
	m = m.Next()
	require.Equal(t, "t1", m.Selected().ID)

	// Prev wraps around from first
	m = m.Prev()
	require.Equal(t, "t3", m.Selected().ID)
}

func TestFilter(t *testing.T) {
	m := New()
	threads := []domain.Thread{
		makeThread("t1", "coordinator", "Auth implementation"),
		makeThread("t2", "worker-1", "Database setup"),
		makeThread("t3", "worker-2", "Auth refactor"),
	}
	m = m.Activate(threads)

	// Filter by content
	m, hasMatches := m.UpdateQuery("auth")
	require.True(t, hasMatches)
	require.NotNil(t, m.Selected())
	// Should filter to auth threads only
	assert.Equal(t, 2, len(m.filtered))

	// Filter by author
	m, hasMatches = m.UpdateQuery("worker-1")
	require.True(t, hasMatches)
	assert.Equal(t, 1, len(m.filtered))
	assert.Equal(t, "t2", m.Selected().ID)

	// No matches
	m, hasMatches = m.UpdateQuery("nonexistent")
	require.False(t, hasMatches)
}

func TestHandleKey_Navigation(t *testing.T) {
	m := New()
	threads := []domain.Thread{
		makeThread("t1", "coordinator", "First thread"),
		makeThread("t2", "worker-1", "Second thread"),
	}
	m = m.Activate(threads)

	// Down key
	m, consumed, selected := m.HandleKey(tea.KeyMsg{Type: tea.KeyDown})
	assert.True(t, consumed)
	assert.Nil(t, selected)
	assert.Equal(t, "t2", m.Selected().ID)

	// j key
	m, consumed, selected = m.HandleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	assert.True(t, consumed)
	assert.Nil(t, selected)
	assert.Equal(t, "t1", m.Selected().ID) // Wrapped

	// Up key
	m, consumed, selected = m.HandleKey(tea.KeyMsg{Type: tea.KeyUp})
	assert.True(t, consumed)
	assert.Nil(t, selected)
	assert.Equal(t, "t2", m.Selected().ID) // Wrapped back

	// k key
	m, consumed, selected = m.HandleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	assert.True(t, consumed)
	assert.Nil(t, selected)
	assert.Equal(t, "t1", m.Selected().ID)
}

func TestHandleKey_Enter(t *testing.T) {
	m := New()
	threads := []domain.Thread{
		makeThread("t1", "coordinator", "First thread"),
		makeThread("t2", "worker-1", "Second thread"),
	}
	m = m.Activate(threads)

	// Move to second item
	m = m.Next()

	// Enter selects and deactivates
	m, consumed, selected := m.HandleKey(tea.KeyMsg{Type: tea.KeyEnter})
	assert.True(t, consumed)
	require.NotNil(t, selected)
	assert.Equal(t, "t2", selected.ID)
	assert.False(t, m.IsActive())
}

func TestHandleKey_Escape(t *testing.T) {
	m := New()
	threads := []domain.Thread{
		makeThread("t1", "coordinator", "First thread"),
	}
	m = m.Activate(threads)

	// Esc deactivates without selecting
	m, consumed, selected := m.HandleKey(tea.KeyMsg{Type: tea.KeyEscape})
	assert.True(t, consumed)
	assert.Nil(t, selected)
	assert.False(t, m.IsActive())
}

func TestHandleKey_WhenInactive(t *testing.T) {
	m := New()

	// Keys are not consumed when inactive
	m, consumed, selected := m.HandleKey(tea.KeyMsg{Type: tea.KeyEnter})
	assert.False(t, consumed)
	assert.Nil(t, selected)
}

func TestView_Empty(t *testing.T) {
	m := New()

	// Inactive - no view
	view := m.View(80)
	assert.Empty(t, view)

	// Active but empty threads - no view
	m = m.Activate([]domain.Thread{})
	view = m.View(80)
	assert.Empty(t, view)
}

func TestView_WithThreads(t *testing.T) {
	m := New()
	threads := []domain.Thread{
		makeThread("abc12345", "coordinator", "First thread content here"),
		makeThread("def67890", "worker-1", "Second thread content"),
	}
	m = m.Activate(threads)

	view := m.View(80)
	require.NotEmpty(t, view)

	// Should contain thread IDs (truncated to 8 chars)
	assert.Contains(t, view, "abc12345")
	assert.Contains(t, view, "def67890")

	// Should contain author (coordinator abbreviated to "coord")
	assert.Contains(t, view, "coord")
	assert.Contains(t, view, "worker-1")
}
