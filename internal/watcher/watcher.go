// Package watcher provides polling-based refresh events for beads data.
package watcher

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/hk9890/perles/internal/log"
	"github.com/hk9890/perles/internal/pubsub"
)

// WatcherEventType identifies the kind of watcher event.
type WatcherEventType string

const (
	// DBChanged is emitted when polling detects a changed DB fingerprint.
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
	focusedPollInterval   time.Duration
	unfocusedPollInterval time.Duration
	detector              FingerprintDetector
	hasBaseline           bool
	baseline              Fingerprint
	focused               bool
	control               chan focusStateChange
	done                  chan struct{}
	stopOnce              sync.Once
	broker                *pubsub.Broker[WatcherEvent]
	mu                    sync.RWMutex
}

// Config holds watcher configuration options.
type Config struct {
	// PollInterval controls how often the change detector is polled.
	// This is the focused/foreground cadence.
	PollInterval time.Duration

	// UnfocusedPollInterval controls polling cadence when terminal focus is lost.
	UnfocusedPollInterval time.Duration

	// Detector computes the current DB fingerprint used for change detection.
	Detector FingerprintDetector
}

type focusStateChange struct {
	focused   bool
	immediate bool
}

// Fingerprint identifies a specific observed Dolt state.
type Fingerprint struct {
	Branch      string
	WorkingHash string
	HeadHash    string
}

// FingerprintDetector computes the current DB fingerprint token.
//
// For Dolt, the initial implementation uses this tuple:
//   - active_branch()
//   - dolt_hashof_db('WORKING')
//   - dolt_hashof_db('HEAD')
type FingerprintDetector interface {
	Fingerprint(ctx context.Context) (Fingerprint, error)
}

// FingerprintDetectorFunc adapts a function to FingerprintDetector.
type FingerprintDetectorFunc func(ctx context.Context) (Fingerprint, error)

// Fingerprint implements FingerprintDetector.
func (f FingerprintDetectorFunc) Fingerprint(ctx context.Context) (Fingerprint, error) {
	return f(ctx)
}

type doltFingerprintDetector struct {
	db          *sql.DB
	useFallback bool
}

const (
	doltFingerprintQuery = `SELECT
	active_branch() AS branch,
	dolt_hashof_db('WORKING') AS working_hash,
	dolt_hashof_db('HEAD') AS head_hash`

	doltFallbackFingerprintQuery = `SELECT
	active_branch() AS branch,
	(SELECT commit_hash FROM dolt_log LIMIT 1) AS head_commit,
	(SELECT COUNT(*) FROM dolt_status) AS status_count`
)

// NewDoltFingerprintDetector creates a Dolt-backed fingerprint detector.
func NewDoltFingerprintDetector(db *sql.DB) FingerprintDetector {
	if db == nil {
		return nil
	}
	return &doltFingerprintDetector{db: db}
}

func (d *doltFingerprintDetector) Fingerprint(ctx context.Context) (Fingerprint, error) {
	if d.useFallback {
		return d.fallbackFingerprint(ctx)
	}

	fp, err := d.primaryFingerprint(ctx)
	if err == nil {
		return fp, nil
	}

	fallbackFP, fallbackErr := d.fallbackFingerprint(ctx)
	if fallbackErr != nil {
		return Fingerprint{}, fmt.Errorf(
			"querying dolt fingerprint: %w",
			errors.Join(
				fmt.Errorf("primary failed: %w", err),
				fmt.Errorf("fallback failed: %w", fallbackErr),
			),
		)
	}

	d.useFallback = true
	log.Warn(log.CatWatcher, "Primary Dolt fingerprint query failed; falling back to dolt_log/dolt_status fingerprint", "error", err)
	return fallbackFP, nil
}

func (d *doltFingerprintDetector) primaryFingerprint(ctx context.Context) (Fingerprint, error) {
	row := d.db.QueryRowContext(ctx, doltFingerprintQuery)
	var fp Fingerprint
	if err := row.Scan(&fp.Branch, &fp.WorkingHash, &fp.HeadHash); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Fingerprint{}, errors.New("dolt fingerprint query returned no rows")
		}
		return Fingerprint{}, err
	}
	return fp, nil
}

func (d *doltFingerprintDetector) fallbackFingerprint(ctx context.Context) (Fingerprint, error) {
	row := d.db.QueryRowContext(ctx, doltFallbackFingerprintQuery)
	var (
		fp          Fingerprint
		headCommit  string
		statusCount int
	)
	if err := row.Scan(&fp.Branch, &headCommit, &statusCount); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Fingerprint{}, errors.New("fallback dolt fingerprint query returned no rows")
		}
		return Fingerprint{}, err
	}
	fp.HeadHash = headCommit
	fp.WorkingHash = fmt.Sprintf("status_count:%d", statusCount)
	return fp, nil
}

// DefaultConfig returns sensible defaults for the watcher.
func DefaultConfig() Config {
	return Config{
		PollInterval:          1 * time.Second,
		UnfocusedPollInterval: 5 * time.Second,
	}
}

// New creates a new polling watcher.
func New(cfg Config) (*Watcher, error) {
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = DefaultConfig().PollInterval
	}
	if cfg.UnfocusedPollInterval <= 0 {
		cfg.UnfocusedPollInterval = DefaultConfig().UnfocusedPollInterval
	}
	if cfg.Detector == nil {
		return nil, errors.New("watcher requires a fingerprint detector")
	}
	log.Debug(log.CatWatcher,
		"Creating polling watcher",
		"focusedInterval", cfg.PollInterval,
		"unfocusedInterval", cfg.UnfocusedPollInterval,
	)

	return &Watcher{
		focusedPollInterval:   cfg.PollInterval,
		unfocusedPollInterval: cfg.UnfocusedPollInterval,
		detector:              cfg.Detector,
		focused:               true,
		control:               make(chan focusStateChange, 1),
		done:                  make(chan struct{}),
		broker:                pubsub.NewBroker[WatcherEvent](),
	}, nil
}

// Start begins polling for refresh updates.
// Subscribe to watcher events using Broker().Subscribe(ctx) instead of the old channel return.
func (w *Watcher) Start() error {
	if err := w.initializeBaseline(); err != nil {
		log.Warn(log.CatWatcher, "Failed to establish watcher baseline fingerprint; polling will retry", "error", err)
	}

	log.Info(log.CatWatcher,
		"Started polling refresh watcher",
		"focusedInterval", w.focusedPollInterval,
		"unfocusedInterval", w.unfocusedPollInterval,
	)
	go w.loop()

	return nil
}

func (w *Watcher) initializeBaseline() error {
	ctx, cancel := context.WithTimeout(context.Background(), w.currentPollInterval())
	defer cancel()

	fp, err := w.detector.Fingerprint(ctx)
	if err != nil {
		return err
	}

	w.baseline = fp
	w.hasBaseline = true
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

// SetFocused updates terminal focus state and adjusts polling cadence safely.
// When focus is regained, watcher performs an immediate check to catch up.
func (w *Watcher) SetFocused(focused bool) {
	w.mu.Lock()
	previous := w.focused
	w.focused = focused
	w.mu.Unlock()

	change := focusStateChange{
		focused:   focused,
		immediate: focused && !previous,
	}

	select {
	case w.control <- change:
	default:
		select {
		case <-w.control:
		default:
		}
		select {
		case w.control <- change:
		default:
		}
	}
}

// IsFocused returns true when watcher is polling with foreground cadence.
func (w *Watcher) IsFocused() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.focused
}

func (w *Watcher) currentPollInterval() time.Duration {
	w.mu.RLock()
	defer w.mu.RUnlock()
	if w.focused {
		return w.focusedPollInterval
	}
	return w.unfocusedPollInterval
}

func (w *Watcher) pollOnce(timeout time.Duration) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	fp, err := w.detector.Fingerprint(ctx)
	cancel()
	if err != nil {
		log.Warn(log.CatWatcher, "Watcher fingerprint poll failed; skipping refresh tick", "error", err)
		return
	}

	if !w.hasBaseline {
		w.baseline = fp
		w.hasBaseline = true
		return
	}

	if fp == w.baseline {
		return
	}

	w.baseline = fp
	log.Debug(log.CatWatcher, "Polling detected DB fingerprint change, triggering refresh", "branch", fp.Branch)
	w.broker.Publish(pubsub.UpdatedEvent, WatcherEvent{Type: DBChanged})
}

// loop polls DB fingerprints and emits DBChanged only when they change.
func (w *Watcher) loop() {
	ticker := time.NewTicker(w.currentPollInterval())
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			w.pollOnce(w.currentPollInterval())

		case change := <-w.control:
			interval := w.currentPollInterval()
			ticker.Reset(interval)
			log.Debug(log.CatWatcher, "Watcher focus changed", "focused", change.focused, "interval", interval)
			if change.immediate {
				w.pollOnce(interval)
			}

		case <-w.done:
			return
		}
	}
}
