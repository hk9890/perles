// Package persistence provides JSONL event logging and restoration for Fabric.
// It enables session persistence by capturing all Fabric events and replaying them
// to rebuild in-memory repository state.
package persistence

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/zjrosen/perles/internal/orchestration/fabric"
)

// FabricEventsFile is the filename for the Fabric events JSONL log.
const FabricEventsFile = "fabric.jsonl"

// maxLineSize is the buffer size for reading JSONL lines (1MB to handle large artifacts).
const maxLineSize = 1024 * 1024

// PersistedEvent wraps a Fabric event with persistence-specific metadata.
// This is the structure written to the JSONL file.
type PersistedEvent struct {
	// Version is the schema version for forward compatibility.
	Version int `json:"version"`

	// Timestamp is when the event was persisted (may differ from event timestamp).
	Timestamp time.Time `json:"timestamp"`

	// Event is the Fabric event being persisted.
	Event fabric.Event `json:"event"`
}

// currentVersion is the current schema version for persisted events.
const currentVersion = 1

// EventLogger captures Fabric events and persists them to a JSONL file.
// It implements the event handler interface expected by FabricService.SetEventHandler.
//
// Design notes:
// - Events are written synchronously for durability (no buffering)
// - Malformed events are logged but don't stop the system
// - Thread-safe via mutex
type EventLogger struct {
	mu       sync.Mutex
	file     *os.File
	encoder  *json.Encoder
	filePath string

	// Stats
	eventsWritten int64
	errors        int64
	lastError     error
}

// NewEventLogger creates a new EventLogger that writes to the specified session directory.
// The fabric.jsonl file is created or appended to if it already exists.
func NewEventLogger(sessionDir string) (*EventLogger, error) {
	filePath := filepath.Join(sessionDir, FabricEventsFile)

	// Open file in append mode, create if not exists
	file, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644) //nolint:gosec // internal path
	if err != nil {
		return nil, fmt.Errorf("opening fabric events file: %w", err)
	}

	return &EventLogger{
		file:     file,
		encoder:  json.NewEncoder(file),
		filePath: filePath,
	}, nil
}

// HandleEvent is the callback to be registered with FabricService.SetEventHandler.
// It persists each event to the JSONL file.
// Note: Artifact content is not stored here - artifacts reference files by path.
func (l *EventLogger) HandleEvent(event fabric.Event) {
	l.mu.Lock()
	defer l.mu.Unlock()

	persisted := PersistedEvent{
		Version:   currentVersion,
		Timestamp: time.Now(),
		Event:     event,
	}

	if err := l.encoder.Encode(persisted); err != nil {
		l.errors++
		l.lastError = err
		return
	}

	l.eventsWritten++
}

// Close flushes and closes the underlying file.
func (l *EventLogger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.file != nil {
		if err := l.file.Sync(); err != nil {
			return fmt.Errorf("syncing fabric events file: %w", err)
		}
		if err := l.file.Close(); err != nil {
			return fmt.Errorf("closing fabric events file: %w", err)
		}
		l.file = nil
	}
	return nil
}

// Stats returns persistence statistics.
func (l *EventLogger) Stats() (eventsWritten, errors int64, lastError error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.eventsWritten, l.errors, l.lastError
}

// FilePath returns the path to the JSONL file.
func (l *EventLogger) FilePath() string {
	return l.filePath
}

// ChainHandler creates an event handler that calls multiple handlers in sequence.
// This allows both the Broker and EventLogger to receive events from FabricService.
func ChainHandler(handlers ...func(fabric.Event)) func(fabric.Event) {
	return func(event fabric.Event) {
		for _, h := range handlers {
			if h != nil {
				h(event)
			}
		}
	}
}
