package cli

import (
	"os"
	"testing"
)

// --- Parse tests ---

func TestParseServe_Defaults(t *testing.T) {
	os.Args = []string{"devgatewaydns", "serve"}
	cmd, err := Parse("1.0.0")
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	if cmd.Name != "serve" {
		t.Errorf("Name = %q, want %q", cmd.Name, "serve")
	}
	if cmd.Config == nil {
		t.Fatal("Config is nil")
	}
	if cmd.Config.HTTPPort != 80 {
		t.Errorf("HTTPPort = %d, want %d", cmd.Config.HTTPPort, 80)
	}
	if cmd.Config.HTTPSPort != 443 {
		t.Errorf("HTTPSPort = %d, want %d", cmd.Config.HTTPSPort, 443)
	}
	if cmd.Config.DNSPort != 53 {
		t.Errorf("DNSPort = %d, want %d", cmd.Config.DNSPort, 53)
	}
	if cmd.Config.ProxyPort != 8888 {
		t.Errorf("ProxyPort = %d, want %d", cmd.Config.ProxyPort, 8888)
	}
	if cmd.Config.AdminPort != 9090 {
		t.Errorf("AdminPort = %d, want %d", cmd.Config.AdminPort, 9090)
	}
	if cmd.DBPath != "" {
		t.Errorf("DBPath = %q, want empty", cmd.DBPath)
	}
}

func TestParseServe_WithOverrides(t *testing.T) {
	os.Args = []string{"devgatewaydns", "serve", "--http-port", "8080", "--dns-port", "5353"}
	cmd, err := Parse("1.0.0")
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	if cmd.Name != "serve" {
		t.Errorf("Name = %q, want %q", cmd.Name, "serve")
	}
	if cmd.Config.HTTPPort != 8080 {
		t.Errorf("HTTPPort = %d, want %d", cmd.Config.HTTPPort, 8080)
	}
	if cmd.Config.DNSPort != 5353 {
		t.Errorf("DNSPort = %d, want %d", cmd.Config.DNSPort, 5353)
	}
	// Verify HasFlag for set flags
	if !cmd.HasFlag("http-port") {
		t.Error("HasFlag(http-port) should be true")
	}
	if !cmd.HasFlag("dns-port") {
		t.Error("HasFlag(dns-port) should be true")
	}
	// Unset flags should report false
	if cmd.HasFlag("https-port") {
		t.Error("HasFlag(https-port) should be false")
	}
}

func TestParseServe_MultipleListenAddresses(t *testing.T) {
	os.Args = []string{"devgatewaydns", "serve", "--listen", "192.168.1.1", "--listen", "10.0.0.1"}
	cmd, err := Parse("1.0.0")
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	if len(cmd.Listens) != 2 {
		t.Fatalf("Listens len = %d, want 2", len(cmd.Listens))
	}
	if cmd.Listens[0] != "192.168.1.1" {
		t.Errorf("Listens[0] = %q, want %q", cmd.Listens[0], "192.168.1.1")
	}
	if cmd.Listens[1] != "10.0.0.1" {
		t.Errorf("Listens[1] = %q, want %q", cmd.Listens[1], "10.0.0.1")
	}
	if len(cmd.Config.ListenAddresses) != 2 {
		t.Fatalf("Config.ListenAddresses len = %d, want 2", len(cmd.Config.ListenAddresses))
	}
	if !cmd.HasFlag("listen") {
		t.Error("HasFlag(listen) should be true")
	}
}

func TestParseInstall_WithDBPath(t *testing.T) {
	os.Args = []string{"devgatewaydns", "install", "--db", "/tmp/test.db"}
	cmd, err := Parse("1.0.0")
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	if cmd.Name != "install" {
		t.Errorf("Name = %q, want %q", cmd.Name, "install")
	}
	if cmd.DBPath != "/tmp/test.db" {
		t.Errorf("DBPath = %q, want %q", cmd.DBPath, "/tmp/test.db")
	}
	if !cmd.HasFlag("db") {
		t.Error("HasFlag(db) should be true")
	}
}

func TestParseUninstall(t *testing.T) {
	os.Args = []string{"devgatewaydns", "uninstall"}
	cmd, err := Parse("1.0.0")
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	if cmd.Name != "uninstall" {
		t.Errorf("Name = %q, want %q", cmd.Name, "uninstall")
	}
}

func TestParseStart(t *testing.T) {
	os.Args = []string{"devgatewaydns", "start"}
	cmd, err := Parse("1.0.0")
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	if cmd.Name != "start" {
		t.Errorf("Name = %q, want %q", cmd.Name, "start")
	}
}

func TestParseStop(t *testing.T) {
	os.Args = []string{"devgatewaydns", "stop"}
	cmd, err := Parse("1.0.0")
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	if cmd.Name != "stop" {
		t.Errorf("Name = %q, want %q", cmd.Name, "stop")
	}
}

func TestParseStatus(t *testing.T) {
	os.Args = []string{"devgatewaydns", "status"}
	cmd, err := Parse("1.0.0")
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	if cmd.Name != "status" {
		t.Errorf("Name = %q, want %q", cmd.Name, "status")
	}
}

func TestParseUnknownCommand(t *testing.T) {
	os.Args = []string{"devgatewaydns", "bogus"}
	_, err := Parse("1.0.0")
	if err == nil {
		t.Fatal("expected error for unknown command")
	}
}

// --- HasFlag tests ---

func TestHasFlag_ReturnsFalseForUnsetFlags(t *testing.T) {
	cmd := &Command{
		Name:     "serve",
		SetFlags: map[string]bool{"http-port": true},
	}
	if cmd.HasFlag("dns-port") {
		t.Error("HasFlag(dns-port) should be false when not set")
	}
	if !cmd.HasFlag("http-port") {
		t.Error("HasFlag(http-port) should be true when set")
	}
}

func TestHasFlag_EmptySetFlags(t *testing.T) {
	cmd := &Command{
		Name:     "uninstall",
		SetFlags: make(map[string]bool),
	}
	if cmd.HasFlag("anything") {
		t.Error("HasFlag should be false with empty SetFlags")
	}
}

// --- multiFlag tests ---

func TestMultiFlag_Set(t *testing.T) {
	var mf multiFlag
	if err := mf.Set("192.168.1.1"); err != nil {
		t.Fatalf("Set(192.168.1.1) error: %v", err)
	}
	if err := mf.Set("10.0.0.1"); err != nil {
		t.Fatalf("Set(10.0.0.1) error: %v", err)
	}
	if len(mf) != 2 {
		t.Fatalf("len = %d, want 2", len(mf))
	}
	if mf[0] != "192.168.1.1" {
		t.Errorf("mf[0] = %q, want %q", mf[0], "192.168.1.1")
	}
	if mf[1] != "10.0.0.1" {
		t.Errorf("mf[1] = %q, want %q", mf[1], "10.0.0.1")
	}
}

func TestMultiFlag_Set_RejectsEmpty(t *testing.T) {
	var mf multiFlag
	if err := mf.Set(""); err == nil {
		t.Fatal("Set('') should return error")
	}
}

func TestMultiFlag_Set_RejectsInvalid(t *testing.T) {
	var mf multiFlag
	if err := mf.Set("not-an-ip"); err == nil {
		t.Fatal("Set(not-an-ip) should return error")
	}
}

func TestMultiFlag_Set_RejectsIPv6(t *testing.T) {
	var mf multiFlag
	if err := mf.Set("::1"); err == nil {
		t.Fatal("Set(::1) should return error for IPv6")
	}
}

func TestMultiFlag_String_Empty(t *testing.T) {
	var mf multiFlag
	got := mf.String()
	if got != "" {
		t.Errorf("String() = %q, want empty", got)
	}
}

func TestMultiFlag_String_WithValues(t *testing.T) {
	mf := multiFlag{"a", "b", "c"}
	got := mf.String()
	want := "a, b, c"
	if got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}
