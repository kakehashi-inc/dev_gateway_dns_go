package dns

import (
	"testing"
)

func TestUpstreamMap_ResolveKnownIP(t *testing.T) {
	u := NewUpstreamMap([]string{"8.8.8.8", "8.8.4.4"})

	// Manually inject a mapping to avoid depending on real NIC detection.
	u.mu.Lock()
	u.mapping["192.168.1.100"] = []string{"1.1.1.1", "1.0.0.1"}
	u.mu.Unlock()

	got := u.Resolve("192.168.1.100")
	expected := []string{"1.1.1.1", "1.0.0.1"}
	if len(got) != len(expected) {
		t.Fatalf("expected %d servers, got %d", len(expected), len(got))
	}
	for i, s := range expected {
		if got[i] != s {
			t.Errorf("server[%d]: expected %s, got %s", i, s, got[i])
		}
	}
}

func TestUpstreamMap_ResolveUnknownIPFallback(t *testing.T) {
	fallback := []string{"8.8.8.8", "8.8.4.4"}
	u := NewUpstreamMap(fallback)

	got := u.Resolve("10.99.99.99")
	if len(got) != len(fallback) {
		t.Fatalf("expected %d fallback servers, got %d", len(fallback), len(got))
	}
	for i, s := range fallback {
		if got[i] != s {
			t.Errorf("fallback[%d]: expected %s, got %s", i, s, got[i])
		}
	}
}

func TestUpstreamMap_ResolveEmptyMappingFallback(t *testing.T) {
	fallback := []string{"9.9.9.9"}
	u := NewUpstreamMap(fallback)

	// Mapping exists but has empty server list; should fall back.
	u.mu.Lock()
	u.mapping["192.168.1.1"] = []string{}
	u.mu.Unlock()

	got := u.Resolve("192.168.1.1")
	if len(got) != 1 || got[0] != "9.9.9.9" {
		t.Errorf("expected fallback [9.9.9.9], got %v", got)
	}
}

func TestUpstreamMap_ResolveNoFallback(t *testing.T) {
	u := NewUpstreamMap(nil)

	got := u.Resolve("10.0.0.1")
	if got != nil {
		t.Errorf("expected nil when no fallback configured, got %v", got)
	}
}

func TestUpstreamMap_MultipleNICs(t *testing.T) {
	u := NewUpstreamMap([]string{"8.8.8.8"})

	u.mu.Lock()
	u.mapping["192.168.1.1"] = []string{"1.1.1.1"}
	u.mapping["10.0.0.1"] = []string{"9.9.9.9"}
	u.mu.Unlock()

	got1 := u.Resolve("192.168.1.1")
	if len(got1) != 1 || got1[0] != "1.1.1.1" {
		t.Errorf("NIC 192.168.1.1: expected [1.1.1.1], got %v", got1)
	}

	got2 := u.Resolve("10.0.0.1")
	if len(got2) != 1 || got2[0] != "9.9.9.9" {
		t.Errorf("NIC 10.0.0.1: expected [9.9.9.9], got %v", got2)
	}

	// Unknown NIC should fall back.
	got3 := u.Resolve("172.16.0.1")
	if len(got3) != 1 || got3[0] != "8.8.8.8" {
		t.Errorf("unknown NIC: expected fallback [8.8.8.8], got %v", got3)
	}
}

func TestNewUpstreamMap(t *testing.T) {
	fallback := []string{"8.8.8.8"}
	u := NewUpstreamMap(fallback)

	if u.mapping == nil {
		t.Error("expected mapping to be initialized")
	}
	if len(u.fallback) != 1 || u.fallback[0] != "8.8.8.8" {
		t.Errorf("expected fallback [8.8.8.8], got %v", u.fallback)
	}
}
