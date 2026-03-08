package watcher_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/hk9890/perles/internal/pubsub"
	"github.com/hk9890/perles/internal/watcher"
)

func TestDefaultConfig(t *testing.T) {
	cfg := watcher.DefaultConfig()
	require.Equal(t, 1*time.Second, cfg.PollInterval)
}

func TestWatcher_PollingPublishesDBChanged(t *testing.T) {
	w, err := watcher.New(watcher.Config{PollInterval: 25 * time.Millisecond})
	require.NoError(t, err)
	defer func() { _ = w.Stop() }()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	sub := w.Broker().Subscribe(ctx)

	require.NoError(t, w.Start())

	select {
	case evt := <-sub:
		require.Equal(t, pubsub.UpdatedEvent, evt.Type)
		require.Equal(t, watcher.DBChanged, evt.Payload.Type)
		require.Nil(t, evt.Payload.Error)
	case <-time.After(200 * time.Millisecond):
		require.Fail(t, "expected DBChanged from polling watcher")
	}
}

func TestWatcher_PollingRepeats(t *testing.T) {
	w, err := watcher.New(watcher.Config{PollInterval: 20 * time.Millisecond})
	require.NoError(t, err)
	defer func() { _ = w.Stop() }()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	sub := w.Broker().Subscribe(ctx)

	require.NoError(t, w.Start())

	deadline := time.After(250 * time.Millisecond)
	count := 0
	for count < 2 {
		select {
		case evt := <-sub:
			require.Equal(t, watcher.DBChanged, evt.Payload.Type)
			count++
		case <-deadline:
			require.Failf(t, "timeout", "expected repeated polling events, got %d", count)
		}
	}
}

func TestWatcher_StopClosesSubscriptions(t *testing.T) {
	w, err := watcher.New(watcher.Config{PollInterval: 20 * time.Millisecond})
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
	w, err := watcher.New(watcher.Config{})
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
