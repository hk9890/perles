package infrastructure

import (
	"context"
	"sync"
	"time"

	appbeads "github.com/hk9890/perles/internal/beads/application"
)

const (
	defaultHealthMonitorInterval = 10 * time.Second
	minHealthMonitorInterval     = 5 * time.Second
)

type healthPingFunc func(context.Context) error

type healthMonitorClient interface {
	ReconnectIfRecoverable(err error) (attempted bool, reconnectErr error)
	setConnectivityState(state appbeads.ConnectivityState, err error)
}

type healthTicker interface {
	C() <-chan time.Time
	Stop()
}

type realHealthTicker struct {
	ticker *time.Ticker
}

func (t *realHealthTicker) C() <-chan time.Time {
	return t.ticker.C
}

func (t *realHealthTicker) Stop() {
	t.ticker.Stop()
}

type HealthMonitor struct {
	client     healthMonitorClient
	ping       healthPingFunc
	interval   time.Duration
	newTicker  func(time.Duration) healthTicker
	ctx        context.Context
	cancel     context.CancelFunc
	done       chan struct{}
	stopOnce   sync.Once
	hadFailure bool
}

func NewHealthMonitor(client healthMonitorClient, ping healthPingFunc, interval time.Duration) *HealthMonitor {
	return newHealthMonitor(client, ping, interval, func(d time.Duration) healthTicker {
		return &realHealthTicker{ticker: time.NewTicker(d)}
	})
}

func newHealthMonitor(
	client healthMonitorClient,
	ping healthPingFunc,
	interval time.Duration,
	newTicker func(time.Duration) healthTicker,
) *HealthMonitor {
	if newTicker == nil {
		newTicker = func(d time.Duration) healthTicker {
			return &realHealthTicker{ticker: time.NewTicker(d)}
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	m := &HealthMonitor{
		client:    client,
		ping:      ping,
		interval:  clampHealthCheckInterval(interval),
		newTicker: newTicker,
		ctx:       ctx,
		cancel:    cancel,
		done:      make(chan struct{}),
	}

	go m.run()
	return m
}

func clampHealthCheckInterval(interval time.Duration) time.Duration {
	if interval <= 0 {
		return defaultHealthMonitorInterval
	}
	if interval < minHealthMonitorInterval {
		return minHealthMonitorInterval
	}
	return interval
}

func (m *HealthMonitor) run() {
	defer close(m.done)

	ticker := m.newTicker(m.interval)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C():
			m.checkHealth()
		}
	}
}

func (m *HealthMonitor) checkHealth() {
	if m.client == nil || m.ping == nil {
		return
	}

	err := m.ping(m.ctx)
	if err != nil {
		m.hadFailure = true
		_, _ = m.client.ReconnectIfRecoverable(err)
		return
	}

	if m.hadFailure {
		m.hadFailure = false
		m.client.setConnectivityState(appbeads.ConnectivityStateHealthy, nil)
	}
}

func (m *HealthMonitor) Stop() {
	if m == nil {
		return
	}

	m.stopOnce.Do(func() {
		m.cancel()
		<-m.done
	})
}
