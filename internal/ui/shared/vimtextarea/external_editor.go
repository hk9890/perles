package vimtextarea

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/hk9890/perles/internal/ui/shared/editor"
)

// OpenExternalEditorCommand opens the current content in $EDITOR.
// When the editor closes, the content is read back and sent as ExternalEditorFinishedMsg.
type OpenExternalEditorCommand struct {
	MotionBase
}

// Execute returns PassThrough - actual execution happens via tea.ExecProcess.
// The command's work is done in CreateEditorCmd which returns a tea.Cmd.
func (c *OpenExternalEditorCommand) Execute(m *Model) ExecuteResult {
	return Executed
}

// Keys returns the trigger keys for this command.
func (c *OpenExternalEditorCommand) Keys() []string {
	return []string{"<ctrl+g>"}
}

// Mode returns the mode this command operates in.
func (c *OpenExternalEditorCommand) Mode() Mode {
	return ModeNormal
}

// ID returns the hierarchical identifier for this command.
func (c *OpenExternalEditorCommand) ID() string {
	return "external.editor"
}

// CreateEditorCmd creates the tea.Cmd that opens the external editor.
// This must be called separately because it needs the current content.
func CreateEditorCmd(content string) tea.Cmd {
	return editor.OpenCmd(content)
}
