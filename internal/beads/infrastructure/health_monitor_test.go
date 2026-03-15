package infrastructure

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	appbeads "github.com/hk9890/perles/internal/beads/application"
	"github.com/stretchr/testify/require"
)

type fakeHealthTicker struct {
	ch        chan time.Time
	mu        sync.Mutex
	stopCalls int
}

func newFakeHealthTicker() *fakeHealthTicker {
	return &fakeHealthTicker{ch: make(chan time.Time, 16)}
}

func (t *fakeHealthTicker) C() <-chan time.Time {
	return t.ch
}

func (t *fakeHealthTicker) Stop() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.stopCalls++
}

func (t *fakeHealthTicker) stopCount() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.stopCalls
}

type fakeHealthMonitorClient struct {
	mu            sync.Mutex
	reconnectErrs []error
	states        []appbeads.ConnectivityState
	reconnectCh   chan struct{}
	stateCh       chan appbeads.ConnectivityState
}

func newFakeHealthMonitorClient() *fakeHealthMonitorClient {
	return &fakeHealthMonitorClient{
		reconnectCh: make(chan struct{}, 16),
		stateCh:     make(chan appbeads.ConnectivityState, 16),
	}
}

func (c *fakeHealthMonitorClient) ReconnectIfRecoverable(err error) (bool, error) {
	c.mu.Lock()
	c.reconnectErrs = append(c.reconnectErrs, err)
	c.mu.Unlock()
	c.reconnectCh <- struct{}{}
	return true, nil
}

func (c *fakeHealthMonitorClient) setConnectivityState(state appbeads.ConnectivityState, _ error) {
	c.mu.Lock()
	c.states = append(c.states, state)
	c.mu.Unlock()
	c.stateCh <- state
}

func (c *fakeHealthMonitorClient) reconnectCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.reconnectErrs)
}

func (c *fakeHealthMonitorClient) stateCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.states)
}

func waitSignal(t *testing.T, ch <-chan struct{}, msg string) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for %s", msg)
	}
}

func waitState(t *testing.T, ch <-chan appbeads.ConnectivityState, msg string) appbeads.ConnectivityState {
	t.Helper()
	select {
	case s := <-ch:
		return s
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for %s", msg)
		return ""
	}
}

func TestHealthMonitor_PingFailureTriggersReconnect(t *testing.T) {
	client := newFakeHealthMonitorClient()
	ticker := newFakeHealthTicker()

	monitor := newHealthMonitor(
		client,
		func(context.Context) error { return errors.New("connection refused") },
		10*time.Second,
		func(time.Duration) healthTicker { return ticker },
	)
	t.Cleanup(monitor.Stop)

	ticker.ch <- time.Now()
	waitSignal(t, client.reconnectCh, "reconnect attempt")
	require.Equal(t, 1, client.reconnectCount())
}

func TestHealthMonitor_RecoveryPublishesHealthyState(t *testing.T) {
	client := newFakeHealthMonitorClient()
	ticker := newFakeHealthTicker()

	call := 0
	monitor := newHealthMonitor(
		client,
		func(context.Context) error {
			call++
			if call == 1 {
				return errors.New("connection refused")
			}
			return nil
		},
		10*time.Second,
		func(time.Duration) healthTicker { return ticker },
	)
	t.Cleanup(monitor.Stop)

	ticker.ch <- time.Now()
	waitSignal(t, client.reconnectCh, "reconnect attempt")

	ticker.ch <- time.Now()
	state := waitState(t, client.stateCh, "healthy state event")
	require.Equal(t, appbeads.ConnectivityStateHealthy, state)
}

func TestHealthMonitor_StopIsIdempotentAndStopsLoop(t *testing.T) {
	client := newFakeHealthMonitorClient()
	ticker := newFakeHealthTicker()

	pingCalls := 0
	pingCalled := make(chan struct{}, 16)
	monitor := newHealthMonitor(
		client,
		func(context.Context) error {
			pingCalls++
			pingCalled <- struct{}{}
			return nil
		},
		10*time.Second,
		func(time.Duration) healthTicker { return ticker },
	)

	ticker.ch <- time.Now()
	waitSignal(t, pingCalled, "initial ping")

	monitor.Stop()
	monitor.Stop()
	require.Equal(t, 1, ticker.stopCount())

	ticker.ch <- time.Now()
	time.Sleep(100 * time.Millisecond)
	require.Equal(t, 1, pingCalls)
}

func TestHealthMonitor_IntervalClamping(t *testing.T) {
	require.Equal(t, defaultHealthMonitorInterval, clampHealthCheckInterval(0))
	require.Equal(t, minHealthMonitorInterval, clampHealthCheckInterval(3*time.Second))
	require.Equal(t, minHealthMonitorInterval, clampHealthCheckInterval(minHealthMonitorInterval))
	require.Equal(t, 12*time.Second, clampHealthCheckInterval(12*time.Second))
}

func TestHealthMonitor_UsesClampedIntervalForTicker(t *testing.T) {
	client := newFakeHealthMonitorClient()
	ticker := newFakeHealthTicker()

	var used time.Duration
	monitor := newHealthMonitor(
		client,
		func(context.Context) error { return nil },
		2*time.Second,
		func(d time.Duration) healthTicker {
			used = d
			return ticker
		},
	)
	monitor.Stop()

	require.Equal(t, minHealthMonitorInterval, used)
}

func TestHealthMonitor_HealthyPingsDoNotTriggerReconnect(t *testing.T) {
	client := newFakeHealthMonitorClient()
	ticker := newFakeHealthTicker()

	monitor := newHealthMonitor(
		client,
		func(context.Context) error { return nil },
		10*time.Second,
		func(time.Duration) healthTicker { return ticker },
	)
	t.Cleanup(monitor.Stop)

	ticker.ch <- time.Now()
	ticker.ch <- time.Now()
	ticker.ch <- time.Now()
	time.Sleep(100 * time.Millisecond)

	require.Equal(t, 0, client.reconnectCount())
	require.Equal(t, 0, client.stateCount())
}
