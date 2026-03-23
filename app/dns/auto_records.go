package dns

import (
	"sync"
)

// AutoRecordMap manages in-memory DNS A records generated from proxy rules.
// Thread-safe via sync.RWMutex.
type AutoRecordMap struct {
	mu      sync.RWMutex
	records map[string][]string // hostname -> []IP
}

// NewAutoRecordMap creates a new empty AutoRecordMap.
func NewAutoRecordMap() *AutoRecordMap {
	return &AutoRecordMap{
		records: make(map[string][]string),
	}
}

// Set adds or replaces the IP list for a hostname.
func (m *AutoRecordMap) Set(hostname string, ips []string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.records[hostname] = ips
}

// Delete removes the entry for a hostname.
func (m *AutoRecordMap) Delete(hostname string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.records, hostname)
}

// Lookup returns the IP list for a hostname and whether it was found.
func (m *AutoRecordMap) Lookup(hostname string) ([]string, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	ips, ok := m.records[hostname]
	return ips, ok
}

// All returns a copy of all auto records.
func (m *AutoRecordMap) All() map[string][]string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make(map[string][]string, len(m.records))
	for k, v := range m.records {
		cp := make([]string, len(v))
		copy(cp, v)
		result[k] = cp
	}
	return result
}
