package orchestration

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// ===========================================================================
// workerConfirmation Unit Tests (Task perles-oph9.10)
// ===========================================================================

func TestNewWorkerConfirmation_CreatesWithCorrectExpectedCount(t *testing.T) {
	// Verify newWorkerConfirmation creates tracker with correct expected count
	wc := newWorkerConfirmation(5)

	require.NotNil(t, wc, "newWorkerConfirmation should return non-nil")
	require.Equal(t, 5, wc.Expected(), "expected count should match input")
	require.Equal(t, 0, wc.Count(), "initial confirmed count should be 0")
	require.False(t, wc.IsComplete(), "should not be complete initially")
}

func TestNewWorkerConfirmation_ZeroExpected(t *testing.T) {
	// Verify behavior with zero expected workers
	wc := newWorkerConfirmation(0)

	require.NotNil(t, wc)
	require.Equal(t, 0, wc.Expected())
	// With 0 expected, should immediately be "complete" (0 >= 0)
	require.True(t, wc.IsComplete())
}

func TestNewWorkerConfirmation_InitializesEmptyMap(t *testing.T) {
	// Verify the confirmed map is initialized
	wc := newWorkerConfirmation(3)

	confirmed := wc.ConfirmedWorkers()
	require.NotNil(t, confirmed)
	require.Empty(t, confirmed)
}

func TestNewWorkerConfirmation_DoneChannelOpen(t *testing.T) {
	// Verify Done channel is open (not closed) initially
	wc := newWorkerConfirmation(2)

	select {
	case <-wc.Done():
		t.Fatal("Done channel should not be closed initially")
	default:
		// Expected - channel is open
	}
}

// ===========================================================================
// Confirm() Method Tests
// ===========================================================================

func TestConfirm_ReturnsFalseUntilAllConfirmed(t *testing.T) {
	// Verify Confirm() returns false until all workers confirmed
	wc := newWorkerConfirmation(3)

	// First worker
	result := wc.Confirm("worker-1")
	require.False(t, result, "should return false with 1/3 confirmed")
	require.Equal(t, 1, wc.Count())

	// Second worker
	result = wc.Confirm("worker-2")
	require.False(t, result, "should return false with 2/3 confirmed")
	require.Equal(t, 2, wc.Count())
}

func TestConfirm_ReturnsTrueWhenLastWorkerConfirms(t *testing.T) {
	// Verify Confirm() returns true when last worker confirms
	wc := newWorkerConfirmation(3)

	wc.Confirm("worker-1")
	wc.Confirm("worker-2")

	// Third worker - should complete
	result := wc.Confirm("worker-3")
	require.True(t, result, "should return true when all confirmed")
	require.Equal(t, 3, wc.Count())
	require.True(t, wc.IsComplete())
}

func TestConfirm_Idempotent_DuplicateIgnored(t *testing.T) {
	// Verify Confirm() is idempotent - duplicate confirmations are ignored
	wc := newWorkerConfirmation(2)

	// First confirmation
	wc.Confirm("worker-1")
	require.Equal(t, 1, wc.Count())

	// Duplicate confirmation - should be ignored
	wc.Confirm("worker-1")
	require.Equal(t, 1, wc.Count(), "duplicate confirmation should not increment count")

	// Another duplicate
	wc.Confirm("worker-1")
	require.Equal(t, 1, wc.Count(), "multiple duplicates should not increment count")
}

func TestConfirm_Idempotent_ReturnsTrueAfterCompletion(t *testing.T) {
	// Verify that Confirm() returns true for duplicates after completion
	wc := newWorkerConfirmation(2)

	wc.Confirm("worker-1")
	result := wc.Confirm("worker-2")
	require.True(t, result, "should be complete")

	// Now confirm again - should still return true (already complete)
	result = wc.Confirm("worker-1")
	require.True(t, result, "duplicate after completion should return true")

	// New duplicate
	result = wc.Confirm("worker-2")
	require.True(t, result, "duplicate after completion should return true")
}

func TestConfirm_MoreThanExpected(t *testing.T) {
	// Verify behavior when more workers confirm than expected
	wc := newWorkerConfirmation(2)

	wc.Confirm("worker-1")
	wc.Confirm("worker-2")
	require.True(t, wc.IsComplete())

	// Confirm additional workers
	result := wc.Confirm("worker-3")
	require.True(t, result, "should still return true after expected reached")
	require.Equal(t, 3, wc.Count(), "count should include extra workers")
}

// ===========================================================================
// Done() Channel Tests
// ===========================================================================

func TestDone_ClosesWhenAllConfirmed(t *testing.T) {
	// Verify Done() channel closes when all confirmed
	wc := newWorkerConfirmation(2)

	// Channel should be open initially
	select {
	case <-wc.Done():
		t.Fatal("Done channel should be open initially")
	default:
	}

	// Confirm first worker
	wc.Confirm("worker-1")
	select {
	case <-wc.Done():
		t.Fatal("Done channel should still be open with 1/2 confirmed")
	default:
	}

	// Confirm second worker - should close channel
	wc.Confirm("worker-2")

	select {
	case <-wc.Done():
		// Expected - channel is closed
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Done channel should be closed after all confirmed")
	}
}

func TestDone_CanBeSelectedOn(t *testing.T) {
	// Verify Done() works correctly in a select statement
	wc := newWorkerConfirmation(1)

	done := make(chan bool)
	go func() {
		select {
		case <-wc.Done():
			done <- true
		case <-time.After(time.Second):
			done <- false
		}
	}()

	// Confirm the worker
	wc.Confirm("worker-1")

	// Should complete quickly
	select {
	case result := <-done:
		require.True(t, result, "select should receive from Done()")
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for Done()")
	}
}

func TestDone_MultipleReceiversAllUnblock(t *testing.T) {
	// Verify multiple goroutines waiting on Done() all unblock
	wc := newWorkerConfirmation(1)

	const numReceivers = 5
	received := make(chan int, numReceivers)

	// Start multiple receivers
	for i := 0; i < numReceivers; i++ {
		i := i
		go func() {
			<-wc.Done()
			received <- i
		}()
	}

	// Small delay to ensure goroutines are waiting
	time.Sleep(10 * time.Millisecond)

	// Confirm the worker
	wc.Confirm("worker-1")

	// All receivers should unblock
	receivedCount := 0
	timeout := time.After(time.Second)
	for receivedCount < numReceivers {
		select {
		case <-received:
			receivedCount++
		case <-timeout:
			t.Fatalf("timeout: only %d/%d receivers unblocked", receivedCount, numReceivers)
		}
	}
}

// ===========================================================================
// Count() Method Tests
// ===========================================================================

func TestCount_ReturnsAccurateCount(t *testing.T) {
	// Verify Count() returns accurate confirmed count
	wc := newWorkerConfirmation(5)

	require.Equal(t, 0, wc.Count(), "initial count should be 0")

	wc.Confirm("worker-1")
	require.Equal(t, 1, wc.Count())

	wc.Confirm("worker-2")
	require.Equal(t, 2, wc.Count())

	wc.Confirm("worker-3")
	require.Equal(t, 3, wc.Count())

	// Duplicate should not change count
	wc.Confirm("worker-1")
	require.Equal(t, 3, wc.Count())

	wc.Confirm("worker-4")
	require.Equal(t, 4, wc.Count())

	wc.Confirm("worker-5")
	require.Equal(t, 5, wc.Count())
}

// ===========================================================================
// Thread Safety Tests
// ===========================================================================

func TestConfirm_ThreadSafe_ConcurrentConfirmations(t *testing.T) {
	// Verify Confirm() is thread-safe with concurrent calls
	wc := newWorkerConfirmation(100)

	var wg sync.WaitGroup
	wg.Add(100)

	// Confirm 100 workers concurrently
	for i := 0; i < 100; i++ {
		i := i
		go func() {
			defer wg.Done()
			wc.Confirm(workerID(i))
		}()
	}

	wg.Wait()

	// Should have exactly 100 confirmed
	require.Equal(t, 100, wc.Count(), "all workers should be confirmed")
	require.True(t, wc.IsComplete())
}

func TestConfirm_ThreadSafe_ConcurrentDuplicates(t *testing.T) {
	// Verify thread safety with concurrent duplicate confirmations
	wc := newWorkerConfirmation(3)

	var wg sync.WaitGroup
	wg.Add(30) // 10 goroutines per worker, 3 workers

	// Each worker confirmed 10 times concurrently
	for i := 0; i < 3; i++ {
		workerName := workerID(i)
		for j := 0; j < 10; j++ {
			go func() {
				defer wg.Done()
				wc.Confirm(workerName)
			}()
		}
	}

	wg.Wait()

	// Should have exactly 3 confirmed (duplicates ignored)
	require.Equal(t, 3, wc.Count(), "should only count unique workers")
	require.True(t, wc.IsComplete())
}

func TestConfirm_ThreadSafe_ConcurrentWithDone(t *testing.T) {
	// Verify no race between Confirm() and Done() channel read
	wc := newWorkerConfirmation(10)

	var wg sync.WaitGroup
	wg.Add(20) // 10 confirmers + 10 Done() waiters

	// Start goroutines waiting on Done()
	doneReceived := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			defer wg.Done()
			<-wc.Done()
			doneReceived <- true
		}()
	}

	// Start goroutines confirming workers
	for i := 0; i < 10; i++ {
		i := i
		go func() {
			defer wg.Done()
			wc.Confirm(workerID(i))
		}()
	}

	// All should complete without deadlock
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(5 * time.Second):
		t.Fatal("timeout - possible deadlock")
	}

	// All Done() waiters should have received
	require.Len(t, doneReceived, 10)
}

func TestCount_ThreadSafe_ConcurrentReads(t *testing.T) {
	// Verify Count() is thread-safe with concurrent reads and writes
	wc := newWorkerConfirmation(50)

	var wg sync.WaitGroup
	wg.Add(100) // 50 confirmers + 50 readers

	// Start readers
	for i := 0; i < 50; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				_ = wc.Count()
				time.Sleep(time.Microsecond)
			}
		}()
	}

	// Start writers (confirmers)
	for i := 0; i < 50; i++ {
		i := i
		go func() {
			defer wg.Done()
			wc.Confirm(workerID(i))
		}()
	}

	wg.Wait()
	require.Equal(t, 50, wc.Count())
}

// ===========================================================================
// Additional Edge Case Tests
// ===========================================================================

func TestConfirmedWorkers_ReturnsCopy(t *testing.T) {
	// Verify ConfirmedWorkers() returns a copy, not the internal map
	wc := newWorkerConfirmation(3)

	wc.Confirm("worker-1")
	wc.Confirm("worker-2")

	// Get the copy
	confirmed := wc.ConfirmedWorkers()
	require.Len(t, confirmed, 2)

	// Mutate the copy
	confirmed["fake-worker"] = true
	delete(confirmed, "worker-1")

	// Original should be unchanged
	original := wc.ConfirmedWorkers()
	require.Len(t, original, 2)
	require.True(t, original["worker-1"])
	require.True(t, original["worker-2"])
	require.False(t, original["fake-worker"])
}

func TestIsComplete_CorrectState(t *testing.T) {
	// Verify IsComplete() correctly reports completion state
	wc := newWorkerConfirmation(2)

	require.False(t, wc.IsComplete())

	wc.Confirm("worker-1")
	require.False(t, wc.IsComplete())

	wc.Confirm("worker-2")
	require.True(t, wc.IsComplete())

	// Adding more workers doesn't change completion
	wc.Confirm("worker-3")
	require.True(t, wc.IsComplete())
}

func TestExpected_ReturnsExpectedCount(t *testing.T) {
	// Verify Expected() returns the configured expected count
	testCases := []int{0, 1, 5, 10, 100}

	for _, expected := range testCases {
		wc := newWorkerConfirmation(expected)
		require.Equal(t, expected, wc.Expected(), "Expected() should return %d", expected)
	}
}

func TestDone_ChannelNeverClosedTwice(t *testing.T) {
	// Verify Done channel is only closed once, even with many confirmations
	wc := newWorkerConfirmation(1)

	// Confirm multiple times (should only close once)
	for i := 0; i < 10; i++ {
		wc.Confirm("worker-1")
	}

	// Should not panic and channel should be closed
	select {
	case <-wc.Done():
		// Success
	default:
		t.Fatal("Done channel should be closed")
	}
}

func TestConfirm_EmptyWorkerID(t *testing.T) {
	// Verify empty worker ID is handled correctly
	wc := newWorkerConfirmation(1)

	result := wc.Confirm("")
	require.True(t, result, "empty worker ID should still count as confirmation")
	require.Equal(t, 1, wc.Count())
	require.True(t, wc.IsComplete())
}

// ===========================================================================
// Helper Functions
// ===========================================================================

func workerID(i int) string {
	return "worker-" + string(rune('0'+i%10)) + string(rune('0'+i/10))
}
