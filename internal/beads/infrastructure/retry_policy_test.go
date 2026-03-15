package infrastructure

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRetryPolicy_Defaults(t *testing.T) {
	query := DefaultQueryRetryPolicy()
	require.Equal(t, 3, query.MaxAttempts)
	require.Equal(t, 200*time.Millisecond, query.InitialBackoff)
	require.Equal(t, 2*time.Second, query.MaxBackoff)
	require.Equal(t, 0.20, query.Jitter)

	startup := DefaultStartupRetryPolicy()
	require.Equal(t, 5, startup.MaxAttempts)
	require.Equal(t, 250*time.Millisecond, startup.InitialBackoff)
	require.Equal(t, 5*time.Second, startup.MaxBackoff)
	require.Equal(t, 0.10, startup.Jitter)
}

func TestRetryPolicy_BackoffForAttempt(t *testing.T) {
	policy := RetryPolicy{
		MaxAttempts:    5,
		InitialBackoff: 200 * time.Millisecond,
		MaxBackoff:     2 * time.Second,
		Jitter:         0,
	}

	tests := []struct {
		attempt int
		want    time.Duration
	}{
		{attempt: 1, want: 200 * time.Millisecond},
		{attempt: 2, want: 400 * time.Millisecond},
		{attempt: 3, want: 800 * time.Millisecond},
		{attempt: 4, want: 1600 * time.Millisecond},
		{attempt: 5, want: 2 * time.Second},
		{attempt: 6, want: 2 * time.Second},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("attempt_%d", tt.attempt), func(t *testing.T) {
			require.Equal(t, tt.want, policy.backoffForAttempt(tt.attempt))
		})
	}
}

func TestRetryPolicy_JitterWithinBounds(t *testing.T) {
	base := 500 * time.Millisecond

	minPolicy := RetryPolicy{Jitter: 0.20, randFloat: func() float64 { return 0 }}
	maxPolicy := RetryPolicy{Jitter: 0.20, randFloat: func() float64 { return 1 }}
	midPolicy := RetryPolicy{Jitter: 0.20, randFloat: func() float64 { return 0.5 }}

	require.Equal(t, 400*time.Millisecond, minPolicy.applyJitter(base))
	require.Equal(t, 600*time.Millisecond, maxPolicy.applyJitter(base))
	require.Equal(t, base, midPolicy.applyJitter(base))
}

func TestRetryPolicy_ExecuteMaxAttemptsRespected(t *testing.T) {
	policy := RetryPolicy{
		MaxAttempts:    3,
		InitialBackoff: time.Millisecond,
		MaxBackoff:     10 * time.Millisecond,
		Jitter:         0,
		sleep: func(context.Context, time.Duration) error {
			return nil
		},
	}

	attempts := 0
	expectedErr := errors.New("connection refused")
	err := policy.Execute(context.Background(), func() error {
		attempts++
		return expectedErr
	})

	require.ErrorIs(t, err, expectedErr)
	require.Equal(t, 3, attempts)
}

func TestRetryPolicy_ExecuteEarlyExitNonRecoverable(t *testing.T) {
	policy := RetryPolicy{
		MaxAttempts:    5,
		InitialBackoff: time.Millisecond,
		MaxBackoff:     10 * time.Millisecond,
		Jitter:         0,
		sleep: func(context.Context, time.Duration) error {
			return nil
		},
	}

	attempts := 0
	expectedErr := errors.New("validation failed")
	err := policy.Execute(context.Background(), func() error {
		attempts++
		return expectedErr
	})

	require.ErrorIs(t, err, expectedErr)
	require.Equal(t, 1, attempts)
}

func TestRetryPolicy_ExecuteSuccess(t *testing.T) {
	policy := RetryPolicy{
		MaxAttempts:    3,
		InitialBackoff: time.Millisecond,
		MaxBackoff:     10 * time.Millisecond,
		Jitter:         0,
		sleep: func(context.Context, time.Duration) error {
			return nil
		},
	}

	attempts := 0
	err := policy.Execute(context.Background(), func() error {
		attempts++
		if attempts == 1 {
			return errors.New("connection refused")
		}
		return nil
	})

	require.NoError(t, err)
	require.Equal(t, 2, attempts)
}

func TestRetryPolicy_ExecuteContextCancellationStopsRetries(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	policy := RetryPolicy{
		MaxAttempts:    3,
		InitialBackoff: time.Millisecond,
		MaxBackoff:     10 * time.Millisecond,
		Jitter:         0,
		sleep: func(_ context.Context, _ time.Duration) error {
			cancel()
			return context.Canceled
		},
	}

	attempts := 0
	err := policy.Execute(ctx, func() error {
		attempts++
		return errors.New("connection refused")
	})

	require.ErrorIs(t, err, context.Canceled)
	require.Equal(t, 1, attempts)
}
