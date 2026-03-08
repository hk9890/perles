// Package editor provides external editor functionality for editing text content.
package editor

import (
	"errors"
	"os"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// FinishedMsg is sent when the external editor closes.
// Contains the edited content from the temp file.
type FinishedMsg struct {
	Content string
	Err     error
}

// ExecMsg is sent when the external editor command is ready to execute.
// The parent component handles this via tea.ExecProcess by calling ExecCmd().
type ExecMsg struct {
	cmd     *exec.Cmd
	tmpPath string
}

// OpenCmd creates the tea.Cmd that opens the external editor with the given content.
// The editor is determined by $VISUAL, $EDITOR, or falls back to "vi".
//
// Usage:
//  1. Call OpenCmd(content) to get a tea.Cmd
//  2. Handle ExecMsg in Update by calling msg.ExecCmd()
//  3. Handle FinishedMsg to get the edited content
func OpenCmd(content string) tea.Cmd {
	return func() tea.Msg {
		// Get editor from environment (VISUAL > EDITOR > vi)
		editor := os.Getenv("VISUAL")
		if editor == "" {
			editor = os.Getenv("EDITOR")
		}
		if editor == "" {
			editor = "vi"
		}

		editorArgs := strings.Fields(editor)
		if len(editorArgs) == 0 {
			return FinishedMsg{Err: errors.New("editor command is empty")}
		}

		// Create temp file with current content
		tmpFile, err := os.CreateTemp("", "perles-edit-*.md")
		if err != nil {
			return FinishedMsg{Err: err}
		}
		tmpPath := tmpFile.Name()

		// Write content and close
		if _, err := tmpFile.WriteString(content); err != nil {
			_ = os.Remove(tmpPath)
			return FinishedMsg{Err: err}
		}
		if err := tmpFile.Close(); err != nil {
			_ = os.Remove(tmpPath)
			return FinishedMsg{Err: err}
		}

		// Create the editor command.
		//nolint:gosec // G204,G702: editor selection is an intentional local-user choice via VISUAL/EDITOR.
		cmd := exec.Command(editorArgs[0], append(editorArgs[1:], tmpPath)...)

		// Return an ExecMsg that will be handled by the parent
		return ExecMsg{
			cmd:     cmd,
			tmpPath: tmpPath,
		}
	}
}

// ExecCmd returns the tea.ExecProcess command that runs the editor.
// Call this from the parent Update when receiving ExecMsg.
func (msg ExecMsg) ExecCmd() tea.Cmd {
	return tea.ExecProcess(msg.cmd, func(err error) tea.Msg {
		// Cleanup: always remove temp file
		defer func() { _ = os.Remove(msg.tmpPath) }()

		if err != nil {
			return FinishedMsg{Err: err}
		}

		// Read back content
		content, readErr := os.ReadFile(msg.tmpPath)
		if readErr != nil {
			return FinishedMsg{Err: readErr}
		}

		// Trim all trailing newlines that editors like vim add on save
		text := strings.TrimRight(string(content), "\n")

		return FinishedMsg{Content: text}
	})
}
