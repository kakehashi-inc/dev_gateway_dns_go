package status

import (
	"testing"
)

func TestRunHealthChecks_EntryCount(t *testing.T) {
	results := RunHealthChecks([]string{"127.0.0.1"}, 59001, 59002, 59003, 59004, 59005)
	if len(results) != 6 {
		t.Errorf("RunHealthChecks returned %d entries, want 6", len(results))
	}

	expectedServices := []string{
		"HTTP Proxy", "HTTPS Proxy", "DNS (TCP)", "DNS (UDP)", "Forward Proxy", "Admin UI",
	}
	for i, expected := range expectedServices {
		if results[i].Service != expected {
			t.Errorf("results[%d].Service = %q, want %q", i, results[i].Service, expected)
		}
	}

	// All services should be unreachable on these random high ports.
	for i, r := range results {
		if r.Bound {
			t.Errorf("results[%d].Bound = true for %s, expected false", i, r.Service)
		}
	}
}

func TestRunHealthChecks_MultipleAddrs(t *testing.T) {
	results := RunHealthChecks([]string{"127.0.0.1", "127.0.0.2"}, 59001, 59002, 59003, 59004, 59005)
	if len(results) != 12 {
		t.Errorf("RunHealthChecks returned %d entries, want 12", len(results))
	}
}
