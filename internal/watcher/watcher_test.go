package watcher_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"

	"github.com/hk9890/perles/internal/pubsub"
	"github.com/hk9890/perles/internal/watcher"
)

func TestDefaultConfig(t *testing.T) {
	cfg := watcher.DefaultConfig()
	require.Equal(t, 1*time.Second, cfg.PollInterval)
	require.Equal(t, 5*time.Second, cfg.UnfocusedPollInterval)
}

func TestWatcher_NewRequiresDetector(t *testing.T) {
	_, err := watcher.New(watcher.Config{PollInterval: 10 * time.Millisecond})
	require.Error(t, err)
}

func TestWatcher_StartupBaselineNoInitialEventWhenUnchanged(t *testing.T) {
	detector := &sequenceDetector{sequence: []watcher.Fingerprint{
		{Branch: "main", WorkingHash: "w1", HeadHash: "h1"},
		{Branch: "main", WorkingHash: "w1", HeadHash: "h1"},
		{Branch: "main", WorkingHash: "w1", HeadHash: "h1"},
	}}

	w, err := watcher.New(watcher.Config{PollInterval: 20 * time.Millisecond, Detector: detector})
	require.NoError(t, err)
	defer func() { _ = w.Stop() }()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	sub := w.Broker().Subscribe(ctx)

	require.NoError(t, w.Start())

	select {
	case evt := <-sub:
		require.Failf(t, "unexpected event", "got %v", evt)
	case <-time.After(120 * time.Millisecond):
		// expected: baseline + unchanged polls emit nothing
	}
}

func TestWatcher_ChangedFingerprintPublishesOncePerObservedChange(t *testing.T) {
	detector := &sequenceDetector{sequence: []watcher.Fingerprint{
		{Branch: "main", WorkingHash: "w1", HeadHash: "h1"}, // startup baseline
		{Branch: "main", WorkingHash: "w1", HeadHash: "h1"}, // unchanged
		{Branch: "main", WorkingHash: "w2", HeadHash: "h1"}, // change => event 1
		{Branch: "main", WorkingHash: "w2", HeadHash: "h1"}, // unchanged
		{Branch: "main", WorkingHash: "w3", HeadHash: "h2"}, // change => event 2
		{Branch: "main", WorkingHash: "w3", HeadHash: "h2"}, // unchanged
	}}

	w, err := watcher.New(watcher.Config{PollInterval: 15 * time.Millisecond, Detector: detector})
	require.NoError(t, err)
	defer func() { _ = w.Stop() }()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	sub := w.Broker().Subscribe(ctx)

	require.NoError(t, w.Start())

	deadline := time.After(350 * time.Millisecond)
	count := 0
	for count < 2 {
		select {
		case evt := <-sub:
			require.Equal(t, pubsub.UpdatedEvent, evt.Type)
			require.Equal(t, watcher.DBChanged, evt.Payload.Type)
			count++
		case <-deadline:
			require.Failf(t, "timeout", "expected 2 change events, got %d", count)
		}
	}

	// Ensure no extra events for repeated identical fingerprints.
	select {
	case evt := <-sub:
		require.Failf(t, "unexpected event", "got extra event: %v", evt)
	case <-time.After(100 * time.Millisecond):
	}
}

func TestWatcher_StopClosesSubscriptions(t *testing.T) {
	detector := &sequenceDetector{sequence: []watcher.Fingerprint{{Branch: "main", WorkingHash: "w1", HeadHash: "h1"}}}
	w, err := watcher.New(watcher.Config{PollInterval: 20 * time.Millisecond, Detector: detector})
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	sub := w.Broker().Subscribe(ctx)

	require.NoError(t, w.Start())
	require.NoError(t, w.Stop())
	require.NoError(t, w.Stop(), "Stop should be idempotent")

	select {
	case _, ok := <-sub:
		require.False(t, ok, "subscription should close after Stop")
	case <-time.After(200 * time.Millisecond):
		require.Fail(t, "subscription channel not closed after Stop")
	}
}

func TestWatcher_DefaultPollIntervalWhenUnset(t *testing.T) {
	detector := &sequenceDetector{sequence: []watcher.Fingerprint{
		{Branch: "main", WorkingHash: "w1", HeadHash: "h1"},
		{Branch: "main", WorkingHash: "w2", HeadHash: "h1"},
	}}
	w, err := watcher.New(watcher.Config{Detector: detector})
	require.NoError(t, err)
	defer func() { _ = w.Stop() }()

	ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
	defer cancel()
	sub := w.Broker().Subscribe(ctx)

	require.NoError(t, w.Start())

	select {
	case evt := <-sub:
		require.Equal(t, watcher.DBChanged, evt.Payload.Type)
	case <-time.After(1200 * time.Millisecond):
		require.Fail(t, "expected event near default 1s poll interval")
	}
}

func TestWatcher_SetFocusedSwitchesCadence(t *testing.T) {
	detector := &countingDetector{}
	w, err := watcher.New(watcher.Config{
		PollInterval:          20 * time.Millisecond,
		UnfocusedPollInterval: 120 * time.Millisecond,
		Detector:              detector,
	})
	require.NoError(t, err)
	defer func() { _ = w.Stop() }()

	require.NoError(t, w.Start())

	// Focused mode should poll frequently.
	time.Sleep(70 * time.Millisecond)
	require.GreaterOrEqual(t, detector.Count(), 3, "focused cadence should poll frequently")

	beforeBlur := detector.Count()
	w.SetFocused(false)

	// In the next 70ms, blurred cadence (120ms) should not add many polls.
	time.Sleep(70 * time.Millisecond)
	afterShortBlur := detector.Count()
	require.LessOrEqual(t, afterShortBlur-beforeBlur, 1, "blurred cadence should slow poll frequency")

	// Given more time, at least one blurred poll should happen.
	require.Eventually(t, func() bool {
		return detector.Count() >= beforeBlur+1
	}, time.Second, 10*time.Millisecond)
}

func TestWatcher_FocusRegainTriggersImmediateCatchUpWithoutDuplicates(t *testing.T) {
	detector := &sequenceDetector{sequence: []watcher.Fingerprint{
		{Branch: "main", WorkingHash: "w1", HeadHash: "h1"}, // baseline
		{Branch: "main", WorkingHash: "w2", HeadHash: "h1"}, // changed while blurred
		{Branch: "main", WorkingHash: "w2", HeadHash: "h1"}, // unchanged after immediate catch-up
	}}

	w, err := watcher.New(watcher.Config{
		PollInterval:          20 * time.Millisecond,
		UnfocusedPollInterval: 250 * time.Millisecond,
		Detector:              detector,
	})
	require.NoError(t, err)
	defer func() { _ = w.Stop() }()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	sub := w.Broker().Subscribe(ctx)

	require.NoError(t, w.Start())
	w.SetFocused(false)

	// No event while blurred before slow poll interval elapses.
	select {
	case evt := <-sub:
		require.Failf(t, "unexpected event", "got %v", evt)
	case <-time.After(80 * time.Millisecond):
	}

	// Regaining focus should trigger immediate catch-up poll.
	w.SetFocused(true)

	select {
	case evt := <-sub:
		require.Equal(t, pubsub.UpdatedEvent, evt.Type)
		require.Equal(t, watcher.DBChanged, evt.Payload.Type)
	case <-time.After(120 * time.Millisecond):
		require.Fail(t, "expected immediate DBChanged on focus regain")
	}

	// No duplicate refresh when fingerprint remains unchanged.
	select {
	case evt := <-sub:
		require.Failf(t, "unexpected duplicate event", "got %v", evt)
	case <-time.After(80 * time.Millisecond):
	}
}

func TestWatcher_DetectorErrorsAreSkippedAndPollingContinues(t *testing.T) {
	detector := &sequenceDetector{
		sequence: []watcher.Fingerprint{
			{Branch: "main", WorkingHash: "w1", HeadHash: "h1"}, // startup baseline
			{Branch: "main", WorkingHash: "w1", HeadHash: "h1"}, // unchanged after transient errors
			{Branch: "main", WorkingHash: "w2", HeadHash: "h1"}, // first successful poll after error
		},
		errors: map[int]error{
			1: errors.New("temporary detector failure"),
			2: errors.New("temporary detector failure"),
		},
	}

	w, err := watcher.New(watcher.Config{PollInterval: 20 * time.Millisecond, Detector: detector})
	require.NoError(t, err)
	defer func() { _ = w.Stop() }()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	sub := w.Broker().Subscribe(ctx)

	require.NoError(t, w.Start())

	// First tick errors: should not emit.
	select {
	case evt := <-sub:
		require.Failf(t, "unexpected event", "got event on error tick: %v", evt)
	case <-time.After(60 * time.Millisecond):
	}

	// Next successful changed fingerprint should emit once.
	select {
	case evt := <-sub:
		require.Equal(t, watcher.DBChanged, evt.Payload.Type)
	case <-time.After(200 * time.Millisecond):
		require.Fail(t, "expected DBChanged after error recovery")
	}
}

func TestDoltFingerprintDetector_UsesPrimaryQuery(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	rows := sqlmock.NewRows([]string{"branch", "working_hash", "head_hash"}).
		AddRow("main", "working123", "head456")
	mock.ExpectQuery("SELECT\\s+active_branch\\(\\)").WillReturnRows(rows)

	detector := watcher.NewDoltFingerprintDetector(db)
	fp, err := detector.Fingerprint(context.Background())
	require.NoError(t, err)
	require.Equal(t, watcher.Fingerprint{Branch: "main", WorkingHash: "working123", HeadHash: "head456"}, fp)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestDoltFingerprintDetector_FallsBackWhenPrimaryFails(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectQuery("SELECT\\s+active_branch\\(\\)").WillReturnError(errors.New("FUNCTION dolt_hashof_db does not exist"))
	fallbackRows := sqlmock.NewRows([]string{"branch", "head_commit", "status_count"}).
		AddRow("main", "abc123", 2)
	mock.ExpectQuery("SELECT\\s+active_branch\\(\\)").WillReturnRows(fallbackRows)

	detector := watcher.NewDoltFingerprintDetector(db)
	fp, err := detector.Fingerprint(context.Background())
	require.NoError(t, err)
	require.Equal(t, watcher.Fingerprint{Branch: "main", HeadHash: "abc123", WorkingHash: "status_count:2"}, fp)

	// Subsequent calls should continue using fallback path.
	fallbackRows2 := sqlmock.NewRows([]string{"branch", "head_commit", "status_count"}).
		AddRow("main", "abc124", 0)
	mock.ExpectQuery("SELECT\\s+active_branch\\(\\)").WillReturnRows(fallbackRows2)
	fp, err = detector.Fingerprint(context.Background())
	require.NoError(t, err)
	require.Equal(t, watcher.Fingerprint{Branch: "main", HeadHash: "abc124", WorkingHash: "status_count:0"}, fp)

	require.NoError(t, mock.ExpectationsWereMet())
}

type sequenceDetector struct {
	mu       sync.Mutex
	sequence []watcher.Fingerprint
	errors   map[int]error
	index    int
}

type countingDetector struct {
	mu     sync.Mutex
	count  int
	stamps []time.Time
}

func (c *countingDetector) Fingerprint(context.Context) (watcher.Fingerprint, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.count++
	c.stamps = append(c.stamps, time.Now())
	// Always unchanged fingerprint; we only care about poll timing.
	return watcher.Fingerprint{Branch: "main", WorkingHash: "w1", HeadHash: "h1"}, nil
}

func (c *countingDetector) Count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.count
}

func (c *countingDetector) Timestamps() []time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]time.Time, len(c.stamps))
	copy(out, c.stamps)
	return out
}

func (s *sequenceDetector) Fingerprint(context.Context) (watcher.Fingerprint, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	current := s.index
	s.index++

	if err, ok := s.errors[current]; ok {
		return watcher.Fingerprint{}, err
	}

	if len(s.sequence) == 0 {
		return watcher.Fingerprint{}, errors.New("empty detector sequence")
	}
	if current >= len(s.sequence) {
		return s.sequence[len(s.sequence)-1], nil
	}
	return s.sequence[current], nil
}
