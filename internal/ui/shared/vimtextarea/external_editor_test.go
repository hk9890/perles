package vimtextarea

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/require"

	"github.com/hk9890/perles/internal/ui/shared/editor"
)

func TestOpenExternalEditorCommand_Keys(t *testing.T) {
	cmd := &OpenExternalEditorCommand{}
	require.Equal(t, []string{"<ctrl+g>"}, cmd.Keys())
}

func TestOpenExternalEditorCommand_Mode(t *testing.T) {
	cmd := &OpenExternalEditorCommand{}
	require.Equal(t, ModeNormal, cmd.Mode())
}

func TestOpenExternalEditorCommand_ID(t *testing.T) {
	cmd := &OpenExternalEditorCommand{}
	require.Equal(t, "external.editor", cmd.ID())
}

func TestOpenExternalEditorCommand_NotUndoable(t *testing.T) {
	cmd := &OpenExternalEditorCommand{}
	require.False(t, cmd.IsUndoable())
	require.False(t, cmd.ChangesContent())
	require.False(t, cmd.IsModeChange())
}

func TestCtrlG_TriggersExternalEditor(t *testing.T) {
	m := New(Config{VimEnabled: true, DefaultMode: ModeNormal})
	m.SetValue("initial content")

	// Simulate Ctrl+G keypress
	keyMsg := tea.KeyMsg{Type: tea.KeyCtrlG}
	m, cmd := m.Update(keyMsg)

	// Should return a tea.Cmd that will eventually produce externalEditorExecMsg
	require.NotNil(t, cmd)
}

func TestCtrlG_WorksInInsertMode(t *testing.T) {
	m := New(Config{VimEnabled: true, DefaultMode: ModeInsert})
	m.SetValue("some text")

	keyMsg := tea.KeyMsg{Type: tea.KeyCtrlG}
	m, cmd := m.Update(keyMsg)

	require.NotNil(t, cmd)
}

func TestCtrlG_WorksWithVimDisabled(t *testing.T) {
	// When VimEnabled is false, textarea is always in "insert mode" behavior
	m := New(Config{VimEnabled: false})
	m.SetValue("some text")

	keyMsg := tea.KeyMsg{Type: tea.KeyCtrlG}
	m, cmd := m.Update(keyMsg)

	require.NotNil(t, cmd, "Ctrl+G should work even with VimEnabled=false")
}

func TestExternalEditorFinishedMsg_UpdatesContent(t *testing.T) {
	m := New(Config{VimEnabled: true, DefaultMode: ModeNormal})
	m.SetValue("old content")

	msg := editor.FinishedMsg{Content: "new content from editor"}
	m, _ = m.Update(msg)

	require.Equal(t, "new content from editor", m.Value())
}

func TestExternalEditorFinishedMsg_ErrorDoesNotUpdateContent(t *testing.T) {
	m := New(Config{VimEnabled: true, DefaultMode: ModeNormal})
	m.SetValue("original content")

	msg := editor.FinishedMsg{
		Content: "should not appear",
		Err:     tea.ErrInterrupted,
	}
	m, _ = m.Update(msg)

	require.Equal(t, "original content", m.Value())
}

func TestCreateEditorCmd_ReturnsCmd(t *testing.T) {
	cmd := CreateEditorCmd("test content")
	require.NotNil(t, cmd)
}
