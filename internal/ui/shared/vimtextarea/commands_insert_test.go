package vimtextarea

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// ============================================================================
// InsertTextCommand Tests
// ============================================================================

func TestInsertTextCommand_Execute(t *testing.T) {
	m := newTestModelWithContent("hello")
	m.cursorCol = 5

	cmd := &InsertTextCommand{row: 0, col: 5, text: " world"}
	err := cmd.Execute(m)

	assert.Equal(t, Executed, err)
	assert.Equal(t, "hello world", m.content[0])
	assert.Equal(t, 11, m.cursorCol)
}

// TestInsertTextCommand_ExecuteMiddle verifies inserting text in middle of line
func TestInsertTextCommand_ExecuteMiddle(t *testing.T) {
	m := newTestModelWithContent("helloworld")
	m.cursorCol = 5

	cmd := &InsertTextCommand{row: 0, col: 5, text: " "}
	err := cmd.Execute(m)

	assert.Equal(t, Executed, err)
	assert.Equal(t, "hello world", m.content[0])
	assert.Equal(t, 6, m.cursorCol)
}

// TestInsertTextCommand_ExecuteEmpty verifies inserting empty text is no-op
func TestInsertTextCommand_ExecuteEmpty(t *testing.T) {
	m := newTestModelWithContent("hello")
	m.cursorCol = 0

	cmd := &InsertTextCommand{row: 0, col: 0, text: ""}
	err := cmd.Execute(m)

	assert.Equal(t, Executed, err)
	assert.Equal(t, "hello", m.content[0])
}

// TestInsertTextCommand_ExecuteMultiLine verifies multi-line paste
func TestInsertTextCommand_ExecuteMultiLine(t *testing.T) {
	m := newTestModelWithContent("hello world")
	m.cursorCol = 5

	cmd := &InsertTextCommand{row: 0, col: 5, text: "\nfoo\nbar"}
	err := cmd.Execute(m)

	assert.Equal(t, Executed, err)
	assert.Len(t, m.content, 3)
	assert.Equal(t, "hello", m.content[0])
	assert.Equal(t, "foo", m.content[1])
	assert.Equal(t, "bar world", m.content[2])
	assert.Equal(t, 2, m.cursorRow)
	assert.Equal(t, 3, m.cursorCol)
}

// TestInsertTextCommand_Undo verifies removing inserted text
func TestInsertTextCommand_Undo(t *testing.T) {
	m := newTestModelWithContent("hello")
	m.cursorCol = 5

	cmd := &InsertTextCommand{row: 0, col: 5, text: " world"}
	_ = cmd.Execute(m)
	assert.Equal(t, "hello world", m.content[0])

	err := cmd.Undo(m)
	assert.NoError(t, err)
	assert.Equal(t, "hello", m.content[0])
	assert.Equal(t, 0, m.cursorRow)
	assert.Equal(t, 5, m.cursorCol)
}

// TestInsertTextCommand_UndoMultiLine verifies undoing multi-line paste
func TestInsertTextCommand_UndoMultiLine(t *testing.T) {
	m := newTestModelWithContent("hello world")
	m.cursorCol = 5

	cmd := &InsertTextCommand{row: 0, col: 5, text: "\nfoo\nbar"}
	_ = cmd.Execute(m)
	assert.Len(t, m.content, 3)

	err := cmd.Undo(m)
	assert.NoError(t, err)
	assert.Len(t, m.content, 1)
	assert.Equal(t, "hello world", m.content[0])
	assert.Equal(t, 0, m.cursorRow)
	assert.Equal(t, 5, m.cursorCol)
}

// ============================================================================
// SplitLineCommand Tests
// ============================================================================

// TestSplitLineCommand_Execute verifies splitting line at cursor
func TestSplitLineCommand_Execute(t *testing.T) {
	m := newTestModelWithContent("hello world")
	m.cursorCol = 5

	cmd := &SplitLineCommand{row: 0, col: 5}
	err := cmd.Execute(m)

	assert.Equal(t, Executed, err)
	assert.Len(t, m.content, 2)
	assert.Equal(t, "hello", m.content[0])
	assert.Equal(t, " world", m.content[1])
	assert.Equal(t, 1, m.cursorRow)
	assert.Equal(t, 0, m.cursorCol)
}

// TestSplitLineCommand_ExecuteAtStart verifies splitting at line start
func TestSplitLineCommand_ExecuteAtStart(t *testing.T) {
	m := newTestModelWithContent("hello")
	m.cursorCol = 0

	cmd := &SplitLineCommand{row: 0, col: 0}
	err := cmd.Execute(m)

	assert.Equal(t, Executed, err)
	assert.Len(t, m.content, 2)
	assert.Equal(t, "", m.content[0])
	assert.Equal(t, "hello", m.content[1])
}

// TestSplitLineCommand_ExecuteAtEnd verifies splitting at line end
func TestSplitLineCommand_ExecuteAtEnd(t *testing.T) {
	m := newTestModelWithContent("hello")
	m.cursorCol = 5

	cmd := &SplitLineCommand{row: 0, col: 5}
	err := cmd.Execute(m)

	assert.Equal(t, Executed, err)
	assert.Len(t, m.content, 2)
	assert.Equal(t, "hello", m.content[0])
	assert.Equal(t, "", m.content[1])
}

// TestSplitLineCommand_Undo verifies rejoining split lines
func TestSplitLineCommand_Undo(t *testing.T) {
	m := newTestModelWithContent("hello world")
	m.cursorCol = 5

	cmd := &SplitLineCommand{row: 0, col: 5}
	_ = cmd.Execute(m)
	assert.Len(t, m.content, 2)

	err := cmd.Undo(m)
	assert.NoError(t, err)
	assert.Len(t, m.content, 1)
	assert.Equal(t, "hello world", m.content[0])
	assert.Equal(t, 0, m.cursorRow)
	assert.Equal(t, 5, m.cursorCol)
}

// ============================================================================
// InsertLineCommand Tests
// ============================================================================

func TestInsertLineCommand_ExecuteBelow(t *testing.T) {
	m := newTestModelWithContent("line1", "line2")
	m.cursorRow = 0

	cmd := &InsertLineCommand{above: false}
	err := cmd.Execute(m)

	assert.Equal(t, Executed, err)
	assert.Len(t, m.content, 3)
	assert.Equal(t, "line1", m.content[0])
	assert.Equal(t, "", m.content[1])
	assert.Equal(t, "line2", m.content[2])
	assert.Equal(t, 1, m.cursorRow)
	assert.Equal(t, 0, m.cursorCol)
}

// TestInsertLineCommand_ExecuteAbove verifies 'O' inserts line above
func TestInsertLineCommand_ExecuteAbove(t *testing.T) {
	m := newTestModelWithContent("line1", "line2")
	m.cursorRow = 1

	cmd := &InsertLineCommand{above: true}
	err := cmd.Execute(m)

	assert.Equal(t, Executed, err)
	assert.Len(t, m.content, 3)
	assert.Equal(t, "line1", m.content[0])
	assert.Equal(t, "", m.content[1])
	assert.Equal(t, "line2", m.content[2])
	assert.Equal(t, 1, m.cursorRow)
	assert.Equal(t, 0, m.cursorCol)
}

// TestInsertLineCommand_UndoBelow verifies undoing 'o'
func TestInsertLineCommand_UndoBelow(t *testing.T) {
	m := newTestModelWithContent("line1", "line2")
	m.cursorRow = 0

	cmd := &InsertLineCommand{above: false}
	_ = cmd.Execute(m)
	assert.Len(t, m.content, 3)

	err := cmd.Undo(m)
	assert.NoError(t, err)
	assert.Len(t, m.content, 2)
	assert.Equal(t, "line1", m.content[0])
	assert.Equal(t, "line2", m.content[1])
	assert.Equal(t, 0, m.cursorRow)
}

// TestInsertLineCommand_UndoAbove verifies undoing 'O'
func TestInsertLineCommand_UndoAbove(t *testing.T) {
	m := newTestModelWithContent("line1", "line2")
	m.cursorRow = 1

	cmd := &InsertLineCommand{above: true}
	_ = cmd.Execute(m)
	assert.Len(t, m.content, 3)

	err := cmd.Undo(m)
	assert.NoError(t, err)
	assert.Len(t, m.content, 2)
	assert.Equal(t, "line1", m.content[0])
	assert.Equal(t, "line2", m.content[1])
	assert.Equal(t, 1, m.cursorRow)
}

// ============================================================================
// Metadata Tests for Insert Commands
// ============================================================================

// TestInsertTextCommand_Metadata verifies command metadata
func TestInsertTextCommand_Metadata(t *testing.T) {
	cmd := &InsertTextCommand{}
	assert.Equal(t, []string{"<char>"}, cmd.Keys())
	assert.Equal(t, ModeInsert, cmd.Mode())
	assert.Equal(t, "insert.text", cmd.ID())
	assert.True(t, cmd.IsUndoable())
	assert.True(t, cmd.ChangesContent())
	assert.False(t, cmd.IsModeChange())
}

// TestSplitLineCommand_Metadata verifies command metadata
func TestSplitLineCommand_Metadata(t *testing.T) {
	cmd := &SplitLineCommand{}
	assert.Equal(t, []string{"<alt+enter>"}, cmd.Keys())
	assert.Equal(t, ModeInsert, cmd.Mode())
	assert.Equal(t, "insert.split_line", cmd.ID())
	assert.True(t, cmd.IsUndoable())
	assert.True(t, cmd.ChangesContent())
	assert.False(t, cmd.IsModeChange())
}

// TestInsertLineCommand_Metadata_Below verifies 'o' metadata
func TestInsertLineCommand_Metadata_Below(t *testing.T) {
	cmd := &InsertLineCommand{above: false}
	assert.Equal(t, []string{"o"}, cmd.Keys())
	assert.Equal(t, ModeNormal, cmd.Mode())
	assert.Equal(t, "insert.line_below", cmd.ID())
	assert.True(t, cmd.IsUndoable())
	assert.True(t, cmd.ChangesContent())
	assert.False(t, cmd.IsModeChange())
}

// TestInsertLineCommand_Metadata_Above verifies 'O' metadata
func TestInsertLineCommand_Metadata_Above(t *testing.T) {
	cmd := &InsertLineCommand{above: true}
	assert.Equal(t, []string{"O"}, cmd.Keys())
	assert.Equal(t, ModeNormal, cmd.Mode())
	assert.Equal(t, "insert.line_above", cmd.ID())
	assert.True(t, cmd.IsUndoable())
	assert.True(t, cmd.ChangesContent())
	assert.False(t, cmd.IsModeChange())
}

// TestInsertLineCommand_UndoInvalidRow verifies undo with invalid row
func TestInsertLineCommand_UndoInvalidRow(t *testing.T) {
	m := newTestModelWithContent("line1")
	cmd := &InsertLineCommand{row: 99} // out of range

	err := cmd.Undo(m)
	assert.NoError(t, err)
	assert.Len(t, m.content, 1) // unchanged
}

// ============================================================================
// SpaceCommand Tests
// ============================================================================

// TestSpaceCommand_Execute verifies inserting a space
func TestSpaceCommand_Execute(t *testing.T) {
	m := newTestModelWithContent("hello")
	m.cursorCol = 5

	cmd := &SpaceCommand{}
	result := cmd.Execute(m)

	assert.Equal(t, Executed, result)
	assert.Equal(t, "hello ", m.content[0])
	assert.Equal(t, 6, m.cursorCol)
}

// TestSpaceCommand_ExecuteAtCharLimit verifies space is rejected at limit
func TestSpaceCommand_ExecuteAtCharLimit(t *testing.T) {
	m := newTestModelWithContent("hello")
	m.cursorCol = 5
	m.config.CharLimit = 5 // Already at limit

	cmd := &SpaceCommand{}
	result := cmd.Execute(m)

	assert.Equal(t, Skipped, result)
	assert.Equal(t, "hello", m.content[0])
}

// TestSpaceCommand_Undo verifies removing inserted space
func TestSpaceCommand_Undo(t *testing.T) {
	m := newTestModelWithContent("hello")
	m.cursorCol = 5

	cmd := &SpaceCommand{}
	_ = cmd.Execute(m)
	assert.Equal(t, "hello ", m.content[0])

	err := cmd.Undo(m)
	assert.NoError(t, err)
	assert.Equal(t, "hello", m.content[0])
	assert.Equal(t, 5, m.cursorCol)
}

// TestSpaceCommand_UndoNil verifies undo is safe when not executed
func TestSpaceCommand_UndoNil(t *testing.T) {
	m := newTestModelWithContent("hello")
	cmd := &SpaceCommand{} // Not executed

	err := cmd.Undo(m)
	assert.NoError(t, err)
	assert.Equal(t, "hello", m.content[0])
}

// TestSpaceCommand_Metadata verifies command metadata
func TestSpaceCommand_Metadata(t *testing.T) {
	cmd := &SpaceCommand{}
	assert.Equal(t, []string{"<space>"}, cmd.Keys())
	assert.Equal(t, ModeInsert, cmd.Mode())
	assert.Equal(t, "insert.space", cmd.ID())
	assert.True(t, cmd.IsUndoable())
	assert.True(t, cmd.ChangesContent())
	assert.False(t, cmd.IsModeChange())
}
