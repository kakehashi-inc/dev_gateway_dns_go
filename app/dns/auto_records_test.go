package dns

import (
	"fmt"
	"sync"
	"testing"
)

func TestAutoRecordMap_SetAndLookup(t *testing.T) {
	m := NewAutoRecordMap()

	ips := []string{"192.168.1.1", "10.0.0.1"}
	m.Set("example.com", ips)

	got, ok := m.Lookup("example.com")
	if !ok {
		t.Fatal("expected Lookup to return true for existing hostname")
	}
	if len(got) != len(ips) {
		t.Fatalf("expected %d IPs, got %d", len(ips), len(got))
	}
	for i, ip := range ips {
		if got[i] != ip {
			t.Errorf("IP[%d]: expected %s, got %s", i, ip, got[i])
		}
	}
}

func TestAutoRecordMap_LookupMissing(t *testing.T) {
	m := NewAutoRecordMap()

	_, ok := m.Lookup("nonexistent.com")
	if ok {
		t.Error("expected Lookup to return false for missing hostname")
	}
}

func TestAutoRecordMap_SetOverwrite(t *testing.T) {
	m := NewAutoRecordMap()

	m.Set("example.com", []string{"1.1.1.1"})
	m.Set("example.com", []string{"2.2.2.2", "3.3.3.3"})

	got, ok := m.Lookup("example.com")
	if !ok {
		t.Fatal("expected Lookup to return true")
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 IPs after overwrite, got %d", len(got))
	}
	if got[0] != "2.2.2.2" || got[1] != "3.3.3.3" {
		t.Errorf("unexpected IPs after overwrite: %v", got)
	}
}

func TestAutoRecordMap_Delete(t *testing.T) {
	m := NewAutoRecordMap()

	m.Set("example.com", []string{"1.1.1.1"})
	m.Delete("example.com")

	_, ok := m.Lookup("example.com")
	if ok {
		t.Error("expected Lookup to return false after Delete")
	}
}

func TestAutoRecordMap_DeleteNonexistent(t *testing.T) {
	m := NewAutoRecordMap()

	// Deleting a key that does not exist should not panic.
	m.Delete("nonexistent.com")
}

func TestAutoRecordMap_All(t *testing.T) {
	m := NewAutoRecordMap()

	m.Set("a.com", []string{"1.1.1.1"})
	m.Set("b.com", []string{"2.2.2.2", "3.3.3.3"})

	all := m.All()
	if len(all) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(all))
	}

	aIPs, ok := all["a.com"]
	if !ok {
		t.Fatal("expected a.com in All result")
	}
	if len(aIPs) != 1 || aIPs[0] != "1.1.1.1" {
		t.Errorf("unexpected IPs for a.com: %v", aIPs)
	}

	bIPs, ok := all["b.com"]
	if !ok {
		t.Fatal("expected b.com in All result")
	}
	if len(bIPs) != 2 {
		t.Errorf("expected 2 IPs for b.com, got %d", len(bIPs))
	}
}

func TestAutoRecordMap_AllReturnsCopy(t *testing.T) {
	m := NewAutoRecordMap()

	m.Set("example.com", []string{"1.1.1.1"})
	all := m.All()

	// Mutate the returned map and verify the original is unaffected.
	all["example.com"][0] = "9.9.9.9"
	all["new.com"] = []string{"8.8.8.8"}

	got, ok := m.Lookup("example.com")
	if !ok {
		t.Fatal("expected Lookup to return true")
	}
	if got[0] != "1.1.1.1" {
		t.Error("mutation of All() result should not affect internal state")
	}

	_, ok = m.Lookup("new.com")
	if ok {
		t.Error("adding to All() result should not affect internal state")
	}
}

func TestAutoRecordMap_AllEmpty(t *testing.T) {
	m := NewAutoRecordMap()

	all := m.All()
	if len(all) != 0 {
		t.Fatalf("expected empty map, got %d entries", len(all))
	}
}

func TestAutoRecordMap_ConcurrentAccess(t *testing.T) {
	m := NewAutoRecordMap()
	const goroutines = 50
	const iterations = 100

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for g := 0; g < goroutines; g++ {
		go func(id int) {
			defer wg.Done()
			host := fmt.Sprintf("host-%d.com", id)
			for i := 0; i < iterations; i++ {
				m.Set(host, []string{fmt.Sprintf("10.0.%d.%d", id%256, i%256)})
				m.Lookup(host)
				m.All()
				if i%10 == 0 {
					m.Delete(host)
				}
			}
		}(g)
	}

	wg.Wait()
}
