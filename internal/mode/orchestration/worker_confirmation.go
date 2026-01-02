package orchestration

import (
	"maps"
	"sync"
)

// workerConfirmation tracks worker confirmation status during initialization.
// It provides thread-safe tracking of which workers have confirmed their readiness,
// with channel-based completion signaling when all expected workers have confirmed.
//
// This type replaces the fragile phase-based race condition handling in handleMessageEvent
// with a dedicated tracker that properly handles:
//   - Idempotent confirmations (same worker confirming twice doesn't double-count)
//   - Thread-safe concurrent access
//   - Channel-based completion notification
type workerConfirmation struct {
	mu        sync.Mutex
	confirmed map[string]bool // workerID -> confirmed status
	expected  int             // number of workers expected to confirm
	done      chan struct{}   // closed when all workers confirmed
	closed    bool            // tracks if done channel is already closed
}

// newWorkerConfirmation creates a new workerConfirmation tracker.
// The expected parameter specifies how many workers must confirm before
// the Done() channel is closed.
func newWorkerConfirmation(expected int) *workerConfirmation {
	return &workerConfirmation{
		confirmed: make(map[string]bool),
		expected:  expected,
		done:      make(chan struct{}),
	}
}

// Confirm marks a worker as confirmed and returns true when all expected workers
// have confirmed. This method is idempotent - calling Confirm with the same workerID
// multiple times has no additional effect after the first call.
//
// When the last worker confirms (confirmed count reaches expected), the Done() channel
// is closed and this method returns true.
//
// Thread-safe: This method can be called concurrently from multiple goroutines.
func (wc *workerConfirmation) Confirm(workerID string) bool {
	wc.mu.Lock()
	defer wc.mu.Unlock()

	// Idempotent: if already confirmed, just check if all are confirmed
	if wc.confirmed[workerID] {
		return len(wc.confirmed) >= wc.expected
	}

	// Mark this worker as confirmed
	wc.confirmed[workerID] = true

	// Check if all workers are now confirmed
	allConfirmed := len(wc.confirmed) >= wc.expected

	// Close the done channel if all confirmed (only once)
	if allConfirmed && !wc.closed {
		close(wc.done)
		wc.closed = true
	}

	return allConfirmed
}

// Done returns a channel that is closed when all expected workers have confirmed.
// This allows callers to use select to wait for completion:
//
//	select {
//	case <-wc.Done():
//	    // All workers confirmed
//	case <-ctx.Done():
//	    // Context cancelled
//	}
func (wc *workerConfirmation) Done() <-chan struct{} {
	return wc.done
}

// Count returns the current number of confirmed workers.
// Thread-safe: This method can be called concurrently from multiple goroutines.
func (wc *workerConfirmation) Count() int {
	wc.mu.Lock()
	defer wc.mu.Unlock()
	return len(wc.confirmed)
}

// Expected returns the expected number of workers to confirm.
// This is a convenience method for logging and debugging.
func (wc *workerConfirmation) Expected() int {
	return wc.expected
}

// IsComplete returns true if all expected workers have confirmed.
// Thread-safe: This method can be called concurrently from multiple goroutines.
func (wc *workerConfirmation) IsComplete() bool {
	wc.mu.Lock()
	defer wc.mu.Unlock()
	return len(wc.confirmed) >= wc.expected
}

// ConfirmedWorkers returns a copy of the confirmed worker IDs map.
// This is useful for debugging and for the SpinnerData() method.
// Thread-safe: Returns a copy to avoid races.
func (wc *workerConfirmation) ConfirmedWorkers() map[string]bool {
	wc.mu.Lock()
	defer wc.mu.Unlock()

	// Return a copy to avoid external mutation
	result := make(map[string]bool, len(wc.confirmed))
	maps.Copy(result, wc.confirmed)
	return result
}
