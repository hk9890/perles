package infrastructure

import (
	"context"
	"math/rand"
	"time"

	appbeads "github.com/hk9890/perles/internal/beads/application"
)

// RetryPolicy controls retry behavior for transient connectivity failures.
type RetryPolicy struct {
	MaxAttempts    int
	InitialBackoff time.Duration
	MaxBackoff     time.Duration
	Jitter         float64

	randFloat func() float64
	sleep     func(context.Context, time.Duration) error
}

// DefaultQueryRetryPolicy returns retry defaults for query/read paths.
func DefaultQueryRetryPolicy() RetryPolicy {
	return RetryPolicy{
		MaxAttempts:    3,
		InitialBackoff: 200 * time.Millisecond,
		MaxBackoff:     2 * time.Second,
		Jitter:         0.20,
	}
}

// DefaultStartupRetryPolicy returns retry defaults for startup paths.
func DefaultStartupRetryPolicy() RetryPolicy {
	return RetryPolicy{
		MaxAttempts:    5,
		InitialBackoff: 250 * time.Millisecond,
		MaxBackoff:     5 * time.Second,
		Jitter:         0.10,
	}
}

// Execute runs fn and retries recoverable connectivity failures using
// exponential backoff with jitter.
func (p RetryPolicy) Execute(ctx context.Context, fn func() error) error {
	if ctx == nil {
		ctx = context.Background()
	}

	maxAttempts := p.maxAttempts()
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}

		err := fn()
		if err == nil {
			return nil
		}
		if attempt == maxAttempts {
			return err
		}
		if !appbeads.IsRecoverableConnectivityError(err) {
			return err
		}

		if err := p.sleepWithContext(ctx, p.backoffForAttempt(attempt)); err != nil {
			return err
		}
	}

	return nil
}

func (p RetryPolicy) maxAttempts() int {
	if p.MaxAttempts < 1 {
		return 1
	}
	return p.MaxAttempts
}

func (p RetryPolicy) backoffForAttempt(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}

	backoff := max(time.Duration(0), p.InitialBackoff)

	for i := 1; i < attempt; i++ {
		if p.MaxBackoff > 0 && backoff >= p.MaxBackoff {
			backoff = p.MaxBackoff
			break
		}
		backoff *= 2
	}

	if p.MaxBackoff > 0 && backoff > p.MaxBackoff {
		backoff = p.MaxBackoff
	}

	return p.applyJitter(backoff)
}

func (p RetryPolicy) applyJitter(backoff time.Duration) time.Duration {
	if backoff <= 0 {
		return 0
	}

	jitter := p.Jitter
	if jitter <= 0 {
		return backoff
	}

	randFloat := p.randFloat
	if randFloat == nil {
		randFloat = rand.Float64
	}

	v := randFloat()
	if v < 0 {
		v = 0
	}
	if v > 1 {
		v = 1
	}

	multiplier := 1 + ((v*2)-1)*jitter
	jittered := max(time.Duration(0), time.Duration(float64(backoff)*multiplier))
	if p.MaxBackoff > 0 && jittered > p.MaxBackoff {
		jittered = p.MaxBackoff
	}
	return jittered
}

func (p RetryPolicy) sleepWithContext(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}

	if p.sleep != nil {
		return p.sleep(ctx, d)
	}

	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
