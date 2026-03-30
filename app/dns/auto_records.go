package dns

import (
	"strings"
	"sync"
)

// AutoRecordMap manages in-memory DNS A records generated from proxy rules.
// Thread-safe via sync.RWMutex.
type AutoRecordMap struct {
	mu        sync.RWMutex
	records   map[string][]string // hostname -> []IP (exact match)
	wildcards map[string][]string // base domain -> []IP (wildcard)
}

// NewAutoRecordMap creates a new empty AutoRecordMap.
func NewAutoRecordMap() *AutoRecordMap {
	return &AutoRecordMap{
		records:   make(map[string][]string),
		wildcards: make(map[string][]string),
	}
}

// Set adds or replaces the IP list for a hostname.
// Wildcard hostnames (*.example.com) are stored in a separate map.
func (m *AutoRecordMap) Set(hostname string, ips []string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if strings.HasPrefix(hostname, "*.") {
		m.wildcards[hostname[2:]] = ips
	} else {
		m.records[hostname] = ips
	}
}

// Delete removes the entry for a hostname.
func (m *AutoRecordMap) Delete(hostname string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if strings.HasPrefix(hostname, "*.") {
		delete(m.wildcards, hostname[2:])
	} else {
		delete(m.records, hostname)
	}
}

// Lookup returns the IP list for a hostname and whether it was found.
// Exact match takes priority over wildcard.
func (m *AutoRecordMap) Lookup(hostname string) ([]string, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if ips, ok := m.records[hostname]; ok {
		return ips, true
	}
	idx := strings.Index(hostname, ".")
	if idx >= 0 {
		parent := hostname[idx+1:]
		if ips, ok := m.wildcards[parent]; ok {
			return ips, true
		}
	}
	return nil, false
}

// All returns a copy of all auto records.
// Wildcard entries are returned with their "*.domain" prefix.
func (m *AutoRecordMap) All() map[string][]string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make(map[string][]string, len(m.records)+len(m.wildcards))
	for k, v := range m.records {
		cp := make([]string, len(v))
		copy(cp, v)
		result[k] = cp
	}
	for k, v := range m.wildcards {
		cp := make([]string, len(v))
		copy(cp, v)
		result["*."+k] = cp
	}
	return result
}
