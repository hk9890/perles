package chatpanel

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/require"

	"github.com/hk9890/perles/internal/ui/shared/editor"
)

func TestModel_ForwardsExternalEditorMessages_WhenNotVisible(t *testing.T) {
	// Create a minimal model
	m := New(Config{})
	m.visible = false
	m.focused = false
	m.input.SetValue("original content")

	// Send editor.FinishedMsg - should be forwarded even when not visible
	editorMsg := editor.FinishedMsg{Content: "edited in vim"}
	m, _ = m.Update(editorMsg)

	// The input should now have the edited content
	require.Equal(t, "edited in vim", m.input.Value())
}

func TestModel_ForwardsExternalEditorMessages_WhenVisible(t *testing.T) {
	m := New(Config{})
	m.visible = true
	m.focused = true
	m.input.SetValue("original content")

	editorMsg := editor.FinishedMsg{Content: "new content"}
	m, _ = m.Update(editorMsg)

	require.Equal(t, "new content", m.input.Value())
}

func TestModel_BlocksOtherMessages_WhenNotVisible(t *testing.T) {
	m := New(Config{})
	m.visible = false
	m.focused = false
	m.input.SetValue("original")

	// A regular message that would normally be forwarded
	// WindowSizeMsg is handled specially, so use a custom type
	type customMsg struct{}
	m, _ = m.Update(customMsg{})

	// Content should be unchanged (message was blocked)
	require.Equal(t, "original", m.input.Value())
}

func TestModel_CtrlG_ReturnsEditorCmd(t *testing.T) {
	m := New(Config{})
	m.visible = true
	m.focused = true
	m.activeTab = TabChat
	m.input.SetValue("some text to edit")

	// Simulate Ctrl+G keypress
	keyMsg := tea.KeyMsg{Type: tea.KeyCtrlG}
	_, cmd := m.Update(keyMsg)

	// Should return a command (CreateEditorCmd)
	require.NotNil(t, cmd)
}
