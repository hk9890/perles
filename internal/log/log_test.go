package log

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

// resetLogger resets the global logger state for testing.
// IMPORTANT: Tests that use this must not run in parallel.
func resetLogger() {
	defaultLogger = nil
	once = sync.Once{}
}

// captureWriter is an io.Writer that captures writes for testing.
type captureWriter struct {
	buf bytes.Buffer
	mu  sync.Mutex
}

func (w *captureWriter) Write(p []byte) (n int, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buf.Write(p)
}

func (w *captureWriter) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buf.String()
}

func TestLogger_NilSafety_Debug(t *testing.T) {
	resetLogger()
	// Should not panic when logger is nil
	Debug(CatBQL, "test message", "key", "value")
}

func TestLogger_NilSafety_Info(t *testing.T) {
	resetLogger()
	Info(CatDB, "test message", "key", "value")
}

func TestLogger_NilSafety_Warn(t *testing.T) {
	resetLogger()
	Warn(CatConfig, "test message", "key", "value")
}

func TestLogger_NilSafety_Error(t *testing.T) {
	resetLogger()
	Error(CatUI, "test message", "key", "value")
}

func TestLogger_NilSafety_ErrorErr(t *testing.T) {
	resetLogger()
	ErrorErr(CatMode, "test message", nil, "key", "value")
}

func TestLogger_NilSafety_GetRecentLogs(t *testing.T) {
	resetLogger()
	logs := GetRecentLogs(10)
	require.Nil(t, logs)
}

func TestLogger_NilSafety_SetEnabled(t *testing.T) {
	resetLogger()
	// Should not panic
	SetEnabled(true)
	SetEnabled(false)
}

func TestLogger_NilSafety_SetMinLevel(t *testing.T) {
	resetLogger()
	// Should not panic
	SetMinLevel(LevelInfo)
}

func TestLogger_Init(t *testing.T) {
	resetLogger()
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	cleanup, err := Init(logPath, 10)
	require.NoError(t, err)
	require.NotNil(t, cleanup)
	defer cleanup()

	require.NotNil(t, defaultLogger)
	require.True(t, defaultLogger.enabled)
}

func TestLogger_Init_InvalidPath(t *testing.T) {
	resetLogger()
	// Try to create log in non-existent directory
	_, err := Init("/nonexistent/path/test.log", 10)
	require.Error(t, err)
}

func TestLogger_LevelFiltering(t *testing.T) {
	resetLogger()
	writer := &captureWriter{}
	defaultLogger = &Logger{
		writer:   writer,
		buffer:   NewRingBuffer(10),
		enabled:  true,
		minLevel: LevelInfo, // DEBUG should be filtered
	}

	Debug(CatBQL, "debug message")
	Info(CatBQL, "info message")
	Warn(CatBQL, "warn message")
	Error(CatBQL, "error message")

	output := writer.String()
	require.NotContains(t, output, "debug message")
	require.Contains(t, output, "info message")
	require.Contains(t, output, "warn message")
	require.Contains(t, output, "error message")
}

func TestLogger_LevelFiltering_WarnOnly(t *testing.T) {
	resetLogger()
	writer := &captureWriter{}
	defaultLogger = &Logger{
		writer:   writer,
		buffer:   NewRingBuffer(10),
		enabled:  true,
		minLevel: LevelWarn,
	}

	Debug(CatDB, "debug")
	Info(CatDB, "info")
	Warn(CatDB, "warn")
	Error(CatDB, "error")

	output := writer.String()
	require.NotContains(t, output, "debug")
	require.NotContains(t, output, "info")
	require.Contains(t, output, "warn")
	require.Contains(t, output, "error")
}

func TestLogger_LevelFiltering_ErrorOnly(t *testing.T) {
	resetLogger()
	writer := &captureWriter{}
	defaultLogger = &Logger{
		writer:   writer,
		buffer:   NewRingBuffer(10),
		enabled:  true,
		minLevel: LevelError,
	}

	Debug(CatConfig, "debug")
	Info(CatConfig, "info")
	Warn(CatConfig, "warn")
	Error(CatConfig, "error")

	output := writer.String()
	require.NotContains(t, output, "debug")
	require.NotContains(t, output, "info")
	require.NotContains(t, output, "warn")
	require.Contains(t, output, "error")
}

func TestLogger_CategoryOutput(t *testing.T) {
	resetLogger()
	writer := &captureWriter{}
	defaultLogger = &Logger{
		writer:   writer,
		buffer:   NewRingBuffer(10),
		enabled:  true,
		minLevel: LevelDebug,
	}

	Info(CatBQL, "test message")
	require.Contains(t, writer.String(), "[bql]")

	writer.buf.Reset()
	Info(CatDB, "test message")
	require.Contains(t, writer.String(), "[db]")

	writer.buf.Reset()
	Info(CatConfig, "test message")
	require.Contains(t, writer.String(), "[config]")

	writer.buf.Reset()
	Info(CatWatcher, "test message")
	require.Contains(t, writer.String(), "[watcher]")

	writer.buf.Reset()
	Info(CatUI, "test message")
	require.Contains(t, writer.String(), "[ui]")

	writer.buf.Reset()
	Info(CatMode, "test message")
	require.Contains(t, writer.String(), "[mode]")

	writer.buf.Reset()
	Info(CatBeads, "test message")
	require.Contains(t, writer.String(), "[beads]")
}

func TestLogger_FieldFormatting(t *testing.T) {
	resetLogger()
	writer := &captureWriter{}
	defaultLogger = &Logger{
		writer:   writer,
		buffer:   NewRingBuffer(10),
		enabled:  true,
		minLevel: LevelDebug,
	}

	Info(CatBQL, "test", "key", "value")
	require.Contains(t, writer.String(), "key=value")
}

func TestLogger_FieldFormatting_MultipleFields(t *testing.T) {
	resetLogger()
	writer := &captureWriter{}
	defaultLogger = &Logger{
		writer:   writer,
		buffer:   NewRingBuffer(10),
		enabled:  true,
		minLevel: LevelDebug,
	}

	Info(CatBQL, "test", "name", "alice", "age", 30, "active", true)

	output := writer.String()
	require.Contains(t, output, "name=alice")
	require.Contains(t, output, "age=30")
	require.Contains(t, output, "active=true")
}

func TestLogger_FieldFormatting_OddFieldCount(t *testing.T) {
	resetLogger()
	writer := &captureWriter{}
	defaultLogger = &Logger{
		writer:   writer,
		buffer:   NewRingBuffer(10),
		enabled:  true,
		minLevel: LevelDebug,
	}

	// Odd number of fields - orphan key should get <missing>
	Info(CatBQL, "test", "key1", "value1", "orphan")

	output := writer.String()
	require.Contains(t, output, "key1=value1")
	require.Contains(t, output, "orphan=<missing>")
}

func TestLogger_FieldFormatting_NoFields(t *testing.T) {
	resetLogger()
	writer := &captureWriter{}
	defaultLogger = &Logger{
		writer:   writer,
		buffer:   NewRingBuffer(10),
		enabled:  true,
		minLevel: LevelDebug,
	}

	Info(CatBQL, "message only")

	output := writer.String()
	require.Contains(t, output, "message only")
	require.True(t, strings.HasSuffix(output, "message only\n"))
}

func TestLogger_SetEnabled_Toggle(t *testing.T) {
	resetLogger()
	writer := &captureWriter{}
	defaultLogger = &Logger{
		writer:   writer,
		buffer:   NewRingBuffer(10),
		enabled:  true,
		minLevel: LevelDebug,
	}

	Info(CatBQL, "enabled1")
	require.Contains(t, writer.String(), "enabled1")

	SetEnabled(false)
	Info(CatBQL, "disabled")
	require.NotContains(t, writer.String(), "disabled")

	SetEnabled(true)
	Info(CatBQL, "enabled2")
	require.Contains(t, writer.String(), "enabled2")
}

func TestLogger_SetMinLevel_Dynamic(t *testing.T) {
	resetLogger()
	writer := &captureWriter{}
	defaultLogger = &Logger{
		writer:   writer,
		buffer:   NewRingBuffer(10),
		enabled:  true,
		minLevel: LevelDebug,
	}

	Debug(CatBQL, "debug1")
	require.Contains(t, writer.String(), "debug1")

	SetMinLevel(LevelError)
	Debug(CatBQL, "debug2")
	Info(CatBQL, "info2")
	Warn(CatBQL, "warn2")
	Error(CatBQL, "error2")

	output := writer.String()
	require.NotContains(t, output, "debug2")
	require.NotContains(t, output, "info2")
	require.NotContains(t, output, "warn2")
	require.Contains(t, output, "error2")
}

func TestLogger_ErrorErr_WithError(t *testing.T) {
	resetLogger()
	writer := &captureWriter{}
	defaultLogger = &Logger{
		writer:   writer,
		buffer:   NewRingBuffer(10),
		enabled:  true,
		minLevel: LevelDebug,
	}

	testErr := os.ErrNotExist
	ErrorErr(CatDB, "file not found", testErr, "path", "/test")

	output := writer.String()
	require.Contains(t, output, "file not found")
	require.Contains(t, output, "error=file does not exist")
	require.Contains(t, output, "path=/test")
}

func TestLogger_ErrorErr_NilError(t *testing.T) {
	resetLogger()
	writer := &captureWriter{}
	defaultLogger = &Logger{
		writer:   writer,
		buffer:   NewRingBuffer(10),
		enabled:  true,
		minLevel: LevelDebug,
	}

	ErrorErr(CatDB, "operation failed", nil, "op", "save")

	output := writer.String()
	require.Contains(t, output, "operation failed")
	require.Contains(t, output, "error=<nil>")
	require.Contains(t, output, "op=save")
}

func TestLogger_BufferIntegration(t *testing.T) {
	resetLogger()
	writer := &captureWriter{}
	defaultLogger = &Logger{
		writer:   writer,
		buffer:   NewRingBuffer(5),
		enabled:  true,
		minLevel: LevelDebug,
	}

	Info(CatBQL, "msg1")
	Info(CatBQL, "msg2")
	Info(CatBQL, "msg3")

	logs := GetRecentLogs(3)
	require.Len(t, logs, 3)
	require.Contains(t, logs[0], "msg1")
	require.Contains(t, logs[1], "msg2")
	require.Contains(t, logs[2], "msg3")
}

func TestLogger_BufferIntegration_Overflow(t *testing.T) {
	resetLogger()
	writer := &captureWriter{}
	defaultLogger = &Logger{
		writer:   writer,
		buffer:   NewRingBuffer(3),
		enabled:  true,
		minLevel: LevelDebug,
	}

	Info(CatBQL, "msg1")
	Info(CatBQL, "msg2")
	Info(CatBQL, "msg3")
	Info(CatBQL, "msg4") // overwrites msg1

	logs := GetRecentLogs(3)
	require.Len(t, logs, 3)
	require.NotContains(t, logs[0], "msg1") // msg1 overwritten
	require.Contains(t, logs[0], "msg2")
	require.Contains(t, logs[1], "msg3")
	require.Contains(t, logs[2], "msg4")
}

func TestLogger_OutputFormat(t *testing.T) {
	resetLogger()
	writer := &captureWriter{}
	defaultLogger = &Logger{
		writer:   writer,
		buffer:   NewRingBuffer(10),
		enabled:  true,
		minLevel: LevelDebug,
	}

	Info(CatBQL, "test message", "key", "value")

	output := writer.String()
	// Format: 2025-12-06T10:45:00 [INFO] [bql] test message key=value
	require.Contains(t, output, "[INFO]")
	require.Contains(t, output, "[bql]")
	require.Contains(t, output, "test message")
	require.Contains(t, output, "key=value")
	require.True(t, strings.HasSuffix(output, "\n"))
}

func TestLevel_String(t *testing.T) {
	tests := []struct {
		level    Level
		expected string
	}{
		{LevelDebug, "DEBUG"},
		{LevelInfo, "INFO"},
		{LevelWarn, "WARN"},
		{LevelError, "ERROR"},
		{Level(99), "UNKNOWN"},
	}

	for _, tt := range tests {
		require.Equal(t, tt.expected, tt.level.String())
	}
}

func TestLogger_InitWithTeaLog_Integration(t *testing.T) {
	resetLogger()
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "tea.log")

	cleanup, err := InitWithTeaLog(logPath, "test", 10)
	require.NoError(t, err)
	require.NotNil(t, cleanup)
	defer cleanup()

	Info(CatConfig, "integration test", "key", "value")

	content, err := os.ReadFile(logPath)
	require.NoError(t, err)
	require.Contains(t, string(content), "[INFO]")
	require.Contains(t, string(content), "[config]")
	require.Contains(t, string(content), "integration test")
	require.Contains(t, string(content), "key=value")
}

func TestLogger_NilWriter(t *testing.T) {
	resetLogger()
	defaultLogger = &Logger{
		writer:   nil, // nil writer
		buffer:   NewRingBuffer(10),
		enabled:  true,
		minLevel: LevelDebug,
	}

	// Should not panic with nil writer
	Info(CatBQL, "test", "key", "value")

	// Buffer should still have the entry
	logs := GetRecentLogs(1)
	require.Len(t, logs, 1)
	require.Contains(t, logs[0], "test")
}

func TestLogger_NilBuffer(t *testing.T) {
	resetLogger()
	writer := &captureWriter{}
	defaultLogger = &Logger{
		writer:   writer,
		buffer:   nil, // nil buffer
		enabled:  true,
		minLevel: LevelDebug,
	}

	// Should not panic with nil buffer
	Info(CatBQL, "test", "key", "value")
	require.Contains(t, writer.String(), "test")

	// GetRecentLogs should return nil
	logs := GetRecentLogs(1)
	require.Nil(t, logs)
}
