// Package watcher provides polling-based refresh events for beads data.
package watcher

import (
	"sync"
	"time"

	"github.com/hk9890/perles/internal/log"
	"github.com/hk9890/perles/internal/pubsub"
)

// WatcherEventType identifies the kind of watcher event.
type WatcherEventType string

const (
	// DBChanged is emitted on each polling tick to refresh UI state.
	DBChanged WatcherEventType = "db_changed"
	// WatcherError is emitted when the watcher encounters an internal error.
	// Kept for API compatibility.
	WatcherError WatcherEventType = "error"
)

// WatcherEvent represents an event from the database watcher.
type WatcherEvent struct {
	Type  WatcherEventType
	Error error // Non-nil for WatcherError events
}

// Watcher emits periodic refresh events and publishes them via broker.
//
// Dolt refresh contract:
//   - Refresh is polling-based (not SQLite file-event based)
//   - Default polling interval is 1s
//   - Target visible refresh latency is under 2s from backend change
type Watcher struct {
	pollInterval time.Duration
	done         chan struct{}
	stopOnce     sync.Once
	broker       *pubsub.Broker[WatcherEvent]
}

// Config holds watcher configuration options.
type Config struct {
	// PollInterval controls how often DBChanged is emitted.
	PollInterval time.Duration
}

// DefaultConfig returns sensible defaults for the watcher.
func DefaultConfig() Config {
	return Config{
		PollInterval: 1 * time.Second,
	}
}

// New creates a new polling watcher.
func New(cfg Config) (*Watcher, error) {
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = DefaultConfig().PollInterval
	}
	log.Debug(log.CatWatcher, "Creating polling watcher", "interval", cfg.PollInterval)

	return &Watcher{
		pollInterval: cfg.PollInterval,
		done:         make(chan struct{}),
		broker:       pubsub.NewBroker[WatcherEvent](),
	}, nil
}

// Start begins polling for refresh updates.
// Subscribe to watcher events using Broker().Subscribe(ctx) instead of the old channel return.
func (w *Watcher) Start() error {
	log.Info(log.CatWatcher, "Started polling refresh watcher", "interval", w.pollInterval)
	go w.loop()

	return nil
}

// Stop terminates the watcher and releases resources.
// broker.Close() is called to notify subscribers and close channels.
func (w *Watcher) Stop() error {
	w.stopOnce.Do(func() {
		log.Debug(log.CatWatcher, "Stopping watcher")
		close(w.done)
		w.broker.Close()
	})
	return nil
}

// Broker returns the pub/sub broker for subscribing to watcher events.
// The broker is created in New(), so it is always valid even before Start() is called.
func (w *Watcher) Broker() *pubsub.Broker[WatcherEvent] {
	return w.broker
}

// loop emits periodic refresh events.
func (w *Watcher) loop() {
	ticker := time.NewTicker(w.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			log.Debug(log.CatWatcher, "Polling tick, triggering refresh")
			w.broker.Publish(pubsub.UpdatedEvent, WatcherEvent{
				Type: DBChanged,
			})

		case <-w.done:
			return
		}
	}
}
