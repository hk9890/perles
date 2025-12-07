package log

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewRingBuffer_ValidCapacity(t *testing.T) {
	buf := NewRingBuffer(5)
	require.NotNil(t, buf)
	require.Equal(t, 5, buf.capacity)
	require.Equal(t, 0, buf.size)
}

func TestNewRingBuffer_ZeroCapacity(t *testing.T) {
	// Zero capacity should be normalized to 1
	buf := NewRingBuffer(0)
	require.NotNil(t, buf)
	require.Equal(t, 1, buf.capacity)
}

func TestNewRingBuffer_NegativeCapacity(t *testing.T) {
	// Negative capacity should be normalized to 1
	buf := NewRingBuffer(-5)
	require.NotNil(t, buf)
	require.Equal(t, 1, buf.capacity)
}

func TestRingBuffer_BasicAddGet(t *testing.T) {
	buf := NewRingBuffer(5)
	buf.Add("a")
	buf.Add("b")

	entries := buf.GetLast(2)
	require.Equal(t, []string{"a", "b"}, entries)
}

func TestRingBuffer_Wraparound(t *testing.T) {
	buf := NewRingBuffer(3)
	buf.Add("a")
	buf.Add("b")
	buf.Add("c")
	buf.Add("d") // Should overwrite "a"

	entries := buf.GetLast(3)
	require.Equal(t, []string{"b", "c", "d"}, entries)
}

func TestRingBuffer_MultipleWraparounds(t *testing.T) {
	buf := NewRingBuffer(2)
	buf.Add("a")
	buf.Add("b")
	buf.Add("c") // overwrites "a"
	buf.Add("d") // overwrites "b"
	buf.Add("e") // overwrites "c"

	entries := buf.GetLast(2)
	require.Equal(t, []string{"d", "e"}, entries)
}

func TestRingBuffer_GetLast_PartialBuffer(t *testing.T) {
	buf := NewRingBuffer(10)
	buf.Add("a")
	buf.Add("b")

	// Request more than available
	entries := buf.GetLast(5)
	require.Equal(t, []string{"a", "b"}, entries)
}

func TestRingBuffer_GetLast_ExactMatch(t *testing.T) {
	buf := NewRingBuffer(3)
	buf.Add("a")
	buf.Add("b")
	buf.Add("c")

	entries := buf.GetLast(3)
	require.Equal(t, []string{"a", "b", "c"}, entries)
}

func TestRingBuffer_GetLast_Subset(t *testing.T) {
	buf := NewRingBuffer(5)
	buf.Add("a")
	buf.Add("b")
	buf.Add("c")
	buf.Add("d")
	buf.Add("e")

	// Get only last 2
	entries := buf.GetLast(2)
	require.Equal(t, []string{"d", "e"}, entries)
}

func TestRingBuffer_EmptyBuffer(t *testing.T) {
	buf := NewRingBuffer(5)

	entries := buf.GetLast(3)
	require.Nil(t, entries)
}

func TestRingBuffer_GetLast_ZeroCount(t *testing.T) {
	buf := NewRingBuffer(5)
	buf.Add("a")

	entries := buf.GetLast(0)
	require.Nil(t, entries)
}

func TestRingBuffer_Clear(t *testing.T) {
	buf := NewRingBuffer(5)
	buf.Add("a")
	buf.Add("b")
	buf.Add("c")

	buf.Clear()

	entries := buf.GetLast(3)
	require.Nil(t, entries)
	require.Equal(t, 0, buf.size)
}

func TestRingBuffer_ClearThenAdd(t *testing.T) {
	buf := NewRingBuffer(3)
	buf.Add("a")
	buf.Add("b")
	buf.Clear()
	buf.Add("x")
	buf.Add("y")

	entries := buf.GetLast(2)
	require.Equal(t, []string{"x", "y"}, entries)
}

func TestRingBuffer_ChronologicalOrder(t *testing.T) {
	buf := NewRingBuffer(5)
	buf.Add("first")
	buf.Add("second")
	buf.Add("third")
	buf.Add("fourth")
	buf.Add("fifth")

	entries := buf.GetLast(5)
	// Should be oldest first
	require.Equal(t, []string{"first", "second", "third", "fourth", "fifth"}, entries)
}

func TestRingBuffer_ChronologicalOrderAfterWraparound(t *testing.T) {
	buf := NewRingBuffer(3)
	buf.Add("a") // index 0
	buf.Add("b") // index 1
	buf.Add("c") // index 2
	buf.Add("d") // index 0 (overwrites "a")
	buf.Add("e") // index 1 (overwrites "b")

	entries := buf.GetLast(3)
	// Should be c, d, e (oldest to newest)
	require.Equal(t, []string{"c", "d", "e"}, entries)
}

func TestRingBuffer_SingleCapacity(t *testing.T) {
	buf := NewRingBuffer(1)
	buf.Add("a")
	buf.Add("b")
	buf.Add("c")

	entries := buf.GetLast(1)
	require.Equal(t, []string{"c"}, entries)
}

func TestRingBuffer_Concurrent(t *testing.T) {
	buf := NewRingBuffer(100)
	var wg sync.WaitGroup

	// Concurrent writes
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				buf.Add("entry")
			}
		}(i)
	}

	// Concurrent reads
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				_ = buf.GetLast(10)
			}
		}()
	}

	wg.Wait()

	// Should not panic and should have entries
	entries := buf.GetLast(100)
	require.NotNil(t, entries)
	require.Equal(t, 100, len(entries))
}
