package modules

import (
	"testing"
)

func TestDetectNICs_NoError(t *testing.T) {
	nics, err := DetectNICs()
	if err != nil {
		t.Fatalf("DetectNICs returned error: %v", err)
	}
	// In CI environments the list may be empty, but the call should succeed.
	t.Logf("detected %d NICs", len(nics))
	for _, nic := range nics {
		if nic.Name == "" {
			t.Error("NIC has empty name")
		}
		if len(nic.IPs) == 0 {
			t.Errorf("NIC %q has no IPs", nic.Name)
		}
	}
}

func TestResolveListenIPs_Wildcard(t *testing.T) {
	ips, err := ResolveListenIPs([]string{"0.0.0.0"})
	if err != nil {
		t.Fatalf("ResolveListenIPs(0.0.0.0) returned error: %v", err)
	}
	// When using 0.0.0.0, the result should match GetAllNICIPs.
	allIPs, err := GetAllNICIPs()
	if err != nil {
		t.Fatalf("GetAllNICIPs returned error: %v", err)
	}
	if len(ips) != len(allIPs) {
		t.Errorf("ResolveListenIPs returned %d IPs, GetAllNICIPs returned %d", len(ips), len(allIPs))
	}
}

func TestResolveListenIPs_SpecificIP(t *testing.T) {
	ips, err := ResolveListenIPs([]string{"192.168.1.100"})
	if err != nil {
		t.Fatalf("ResolveListenIPs returned error: %v", err)
	}
	if len(ips) != 1 {
		t.Fatalf("expected 1 IP, got %d", len(ips))
	}
	if ips[0] != "192.168.1.100" {
		t.Errorf("IP = %q, want %q", ips[0], "192.168.1.100")
	}
}
