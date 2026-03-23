package dns

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestRingBuffer_AddAndEntries(t *testing.T) {
	rb := NewRingBuffer(5)

	entry := QueryLogEntry{
		ClientIP:     "192.168.1.1",
		Hostname:     "example.com",
		RecordType:   "A",
		ResponseType: "auto",
		ResponseTime: 100 * time.Microsecond,
		Timestamp:    time.Now(),
	}
	rb.Add(entry)

	entries := rb.Entries()
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Hostname != "example.com" {
		t.Errorf("expected hostname example.com, got %s", entries[0].Hostname)
	}
	if entries[0].ClientIP != "192.168.1.1" {
		t.Errorf("expected client IP 192.168.1.1, got %s", entries[0].ClientIP)
	}
}

func TestRingBuffer_EntriesEmpty(t *testing.T) {
	rb := NewRingBuffer(10)

	entries := rb.Entries()
	if len(entries) != 0 {
		t.Fatalf("expected 0 entries from empty buffer, got %d", len(entries))
	}
}

func TestRingBuffer_FillExactly(t *testing.T) {
	const size = 3
	rb := NewRingBuffer(size)

	for i := 0; i < size; i++ {
		rb.Add(QueryLogEntry{Hostname: fmt.Sprintf("host-%d.com", i)})
	}

	entries := rb.Entries()
	if len(entries) != size {
		t.Fatalf("expected %d entries, got %d", size, len(entries))
	}
	for i := 0; i < size; i++ {
		expected := fmt.Sprintf("host-%d.com", i)
		if entries[i].Hostname != expected {
			t.Errorf("entry[%d]: expected %s, got %s", i, expected, entries[i].Hostname)
		}
	}
}

func TestRingBuffer_Overflow(t *testing.T) {
	const size = 3
	rb := NewRingBuffer(size)

	// Add 5 entries into a buffer of size 3. The oldest 2 should be evicted.
	for i := 0; i < 5; i++ {
		rb.Add(QueryLogEntry{Hostname: fmt.Sprintf("host-%d.com", i)})
	}

	entries := rb.Entries()
	if len(entries) != size {
		t.Fatalf("expected %d entries after overflow, got %d", size, len(entries))
	}

	// Should contain host-2, host-3, host-4 in chronological order.
	expected := []string{"host-2.com", "host-3.com", "host-4.com"}
	for i, e := range expected {
		if entries[i].Hostname != e {
			t.Errorf("entry[%d]: expected %s, got %s", i, e, entries[i].Hostname)
		}
	}
}

func TestRingBuffer_ChronologicalOrder(t *testing.T) {
	const size = 5
	rb := NewRingBuffer(size)

	// Add entries with timestamps in sequence.
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 8; i++ {
		rb.Add(QueryLogEntry{
			Hostname:  fmt.Sprintf("host-%d.com", i),
			Timestamp: base.Add(time.Duration(i) * time.Second),
		})
	}

	entries := rb.Entries()
	if len(entries) != size {
		t.Fatalf("expected %d entries, got %d", size, len(entries))
	}

	// Verify chronological ordering: timestamps should be non-decreasing.
	for i := 1; i < len(entries); i++ {
		if entries[i].Timestamp.Before(entries[i-1].Timestamp) {
			t.Errorf("entries not in chronological order: entry[%d] (%v) before entry[%d] (%v)",
				i, entries[i].Timestamp, i-1, entries[i-1].Timestamp)
		}
	}

	// After adding 8 entries with size 5, entries 3-7 should remain.
	if entries[0].Hostname != "host-3.com" {
		t.Errorf("expected first entry host-3.com, got %s", entries[0].Hostname)
	}
	if entries[4].Hostname != "host-7.com" {
		t.Errorf("expected last entry host-7.com, got %s", entries[4].Hostname)
	}
}

func TestRingBuffer_SingleCapacity(t *testing.T) {
	rb := NewRingBuffer(1)

	rb.Add(QueryLogEntry{Hostname: "first.com"})
	rb.Add(QueryLogEntry{Hostname: "second.com"})

	entries := rb.Entries()
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Hostname != "second.com" {
		t.Errorf("expected second.com, got %s", entries[0].Hostname)
	}
}

func TestRingBuffer_PreservesAllFields(t *testing.T) {
	rb := NewRingBuffer(5)

	ts := time.Date(2026, 3, 15, 10, 30, 0, 0, time.UTC)
	entry := QueryLogEntry{
		ClientIP:     "10.0.0.1",
		Hostname:     "api.example.com",
		RecordType:   "AAAA",
		ResponseType: "upstream",
		ResponseTime: 5 * time.Millisecond,
		Timestamp:    ts,
	}
	rb.Add(entry)

	entries := rb.Entries()
	got := entries[0]
	if got.ClientIP != entry.ClientIP {
		t.Errorf("ClientIP: expected %s, got %s", entry.ClientIP, got.ClientIP)
	}
	if got.Hostname != entry.Hostname {
		t.Errorf("Hostname: expected %s, got %s", entry.Hostname, got.Hostname)
	}
	if got.RecordType != entry.RecordType {
		t.Errorf("RecordType: expected %s, got %s", entry.RecordType, got.RecordType)
	}
	if got.ResponseType != entry.ResponseType {
		t.Errorf("ResponseType: expected %s, got %s", entry.ResponseType, got.ResponseType)
	}
	if got.ResponseTime != entry.ResponseTime {
		t.Errorf("ResponseTime: expected %v, got %v", entry.ResponseTime, got.ResponseTime)
	}
	if !got.Timestamp.Equal(entry.Timestamp) {
		t.Errorf("Timestamp: expected %v, got %v", entry.Timestamp, got.Timestamp)
	}
}

func TestRingBuffer_ConcurrentAccess(t *testing.T) {
	rb := NewRingBuffer(100)
	const goroutines = 20
	const iterations = 200

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for g := 0; g < goroutines; g++ {
		go func(id int) {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				rb.Add(QueryLogEntry{
					Hostname: fmt.Sprintf("host-%d-%d.com", id, i),
				})
				rb.Entries()
			}
		}(g)
	}

	wg.Wait()

	entries := rb.Entries()
	if len(entries) > 100 {
		t.Errorf("expected at most 100 entries, got %d", len(entries))
	}
}
