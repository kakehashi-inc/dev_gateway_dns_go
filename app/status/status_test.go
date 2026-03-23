package status

import (
	"testing"
)

func TestCheckPortBind_UnboundPort(t *testing.T) {
	// Port 59123 should not be bound on any test environment.
	got := CheckPortBind("127.0.0.1", 59123, "tcp")
	if got {
		t.Errorf("CheckPortBind on unbound port: got true, want false")
	}
}

func TestCheckLoopback_UnboundPort(t *testing.T) {
	got := CheckLoopback(59124, "tcp")
	if got {
		t.Errorf("CheckLoopback on unbound port: got true, want false")
	}
}

func TestRunHealthChecks_EntryCount(t *testing.T) {
	results := RunHealthChecks(59001, 59002, 59003, 59004, 59005)
	if len(results) != 6 {
		t.Errorf("RunHealthChecks returned %d entries, want 6", len(results))
	}

	// Verify service names are set correctly.
	expectedServices := []string{
		"HTTP Proxy", "HTTPS Proxy", "DNS (TCP)", "DNS (UDP)", "Forward Proxy", "Admin UI",
	}
	for i, expected := range expectedServices {
		if results[i].Service != expected {
			t.Errorf("results[%d].Service = %q, want %q", i, results[i].Service, expected)
		}
	}

	// TCP ports should be unbound since we used high random ports.
	// UDP dial always succeeds (connectionless), so skip UDP checks.
	for i, r := range results {
		if r.Protocol == "tcp" && r.Bound {
			t.Errorf("results[%d].Bound = true for TCP port %d, expected false", i, r.Port)
		}
	}
}
