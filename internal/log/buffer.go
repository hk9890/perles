package log

import "sync"

// RingBuffer holds recent log entries for the overlay.
type RingBuffer struct {
	mu       sync.RWMutex
	entries  []string
	capacity int
	head     int
	size     int
}

// NewRingBuffer creates a buffer with given capacity.
// Capacity must be >= 1; values <= 0 are normalized to 1.
func NewRingBuffer(capacity int) *RingBuffer {
	if capacity <= 0 {
		capacity = 1
	}
	return &RingBuffer{
		entries:  make([]string, capacity),
		capacity: capacity,
	}
}

// Add appends an entry, overwriting oldest if full.
func (r *RingBuffer) Add(entry string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.entries[r.head] = entry
	r.head = (r.head + 1) % r.capacity
	if r.size < r.capacity {
		r.size++
	}
}

// GetLast returns the last n entries, oldest first.
func (r *RingBuffer) GetLast(n int) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if n > r.size {
		n = r.size
	}
	if n == 0 {
		return nil
	}

	result := make([]string, n)
	start := (r.head - n + r.capacity) % r.capacity
	for i := 0; i < n; i++ {
		idx := (start + i) % r.capacity
		result[i] = r.entries[idx]
	}
	return result
}

// Clear empties the buffer.
func (r *RingBuffer) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.head = 0
	r.size = 0
}
