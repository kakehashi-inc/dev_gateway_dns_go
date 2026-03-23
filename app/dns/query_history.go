package dns

import (
	"sync"
	"time"
)

// QueryLogEntry represents a single DNS query log entry.
type QueryLogEntry struct {
	ClientIP     string        `json:"client_ip"`
	Hostname     string        `json:"hostname"`
	RecordType   string        `json:"record_type"`
	ResponseType string        `json:"response_type"` // "auto", "manual", "upstream"
	ResponseTime time.Duration `json:"response_time_ns"`
	Timestamp    time.Time     `json:"timestamp"`
}

// RingBuffer is a fixed-size circular buffer for DNS query history.
type RingBuffer struct {
	mu      sync.RWMutex
	entries []QueryLogEntry
	size    int
	head    int
	count   int
}

// NewRingBuffer creates a ring buffer with the given capacity.
func NewRingBuffer(size int) *RingBuffer {
	return &RingBuffer{
		entries: make([]QueryLogEntry, size),
		size:    size,
	}
}

// Add inserts a new entry into the ring buffer.
func (rb *RingBuffer) Add(entry QueryLogEntry) {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	rb.entries[rb.head] = entry
	rb.head = (rb.head + 1) % rb.size
	if rb.count < rb.size {
		rb.count++
	}
}

// Entries returns all stored entries in chronological order.
func (rb *RingBuffer) Entries() []QueryLogEntry {
	rb.mu.RLock()
	defer rb.mu.RUnlock()
	result := make([]QueryLogEntry, 0, rb.count)
	start := rb.head - rb.count
	if start < 0 {
		start += rb.size
	}
	for i := 0; i < rb.count; i++ {
		idx := (start + i) % rb.size
		result = append(result, rb.entries[idx])
	}
	return result
}
