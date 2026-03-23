package modules

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func TestNewDefaultConfig(t *testing.T) {
	cfg := NewDefaultConfig()

	if cfg.HTTPPort != 80 {
		t.Errorf("HTTPPort = %d, want 80", cfg.HTTPPort)
	}
	if cfg.HTTPSPort != 443 {
		t.Errorf("HTTPSPort = %d, want 443", cfg.HTTPSPort)
	}
	if cfg.DNSPort != 53 {
		t.Errorf("DNSPort = %d, want 53", cfg.DNSPort)
	}
	if cfg.ProxyPort != 8888 {
		t.Errorf("ProxyPort = %d, want 8888", cfg.ProxyPort)
	}
	if cfg.AdminPort != 9090 {
		t.Errorf("AdminPort = %d, want 9090", cfg.AdminPort)
	}
	if len(cfg.ListenAddresses) != 1 || cfg.ListenAddresses[0] != "0.0.0.0" {
		t.Errorf("ListenAddresses = %v, want [0.0.0.0]", cfg.ListenAddresses)
	}
	if len(cfg.UpstreamDNSFallback) != 2 || cfg.UpstreamDNSFallback[0] != "8.8.8.8" || cfg.UpstreamDNSFallback[1] != "1.1.1.1" {
		t.Errorf("UpstreamDNSFallback = %v, want [8.8.8.8 1.1.1.1]", cfg.UpstreamDNSFallback)
	}
	if cfg.DNSQueryHistorySize != 1000 {
		t.Errorf("DNSQueryHistorySize = %d, want 1000", cfg.DNSQueryHistorySize)
	}
	if cfg.LogLevel != "info" {
		t.Errorf("LogLevel = %q, want \"info\"", cfg.LogLevel)
	}
}

func openTestDBWithSettings(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory db: %v", err)
	}
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS settings (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL,
		updated_at DATETIME NOT NULL DEFAULT (datetime('now'))
	)`)
	if err != nil {
		t.Fatalf("failed to create settings table: %v", err)
	}
	return db
}

func TestSaveAndLoadConfigRoundTrip(t *testing.T) {
	db := openTestDBWithSettings(t)
	defer db.Close()

	original := &AppConfig{
		HTTPPort:            8080,
		HTTPSPort:           8443,
		DNSPort:             5353,
		ProxyPort:           9999,
		AdminPort:           7070,
		ListenAddresses:     []string{"192.168.1.1", "10.0.0.1"},
		UpstreamDNSFallback: []string{"9.9.9.9"},
		DNSQueryHistorySize: 500,
		LogLevel:            "debug",
	}

	if err := SaveConfigToDB(db, original); err != nil {
		t.Fatalf("SaveConfigToDB failed: %v", err)
	}

	loaded, err := LoadConfigFromDB(db)
	if err != nil {
		t.Fatalf("LoadConfigFromDB failed: %v", err)
	}

	if loaded.HTTPPort != original.HTTPPort {
		t.Errorf("HTTPPort = %d, want %d", loaded.HTTPPort, original.HTTPPort)
	}
	if loaded.HTTPSPort != original.HTTPSPort {
		t.Errorf("HTTPSPort = %d, want %d", loaded.HTTPSPort, original.HTTPSPort)
	}
	if loaded.DNSPort != original.DNSPort {
		t.Errorf("DNSPort = %d, want %d", loaded.DNSPort, original.DNSPort)
	}
	if loaded.ProxyPort != original.ProxyPort {
		t.Errorf("ProxyPort = %d, want %d", loaded.ProxyPort, original.ProxyPort)
	}
	if loaded.AdminPort != original.AdminPort {
		t.Errorf("AdminPort = %d, want %d", loaded.AdminPort, original.AdminPort)
	}
	if len(loaded.ListenAddresses) != len(original.ListenAddresses) {
		t.Errorf("ListenAddresses length = %d, want %d", len(loaded.ListenAddresses), len(original.ListenAddresses))
	} else {
		for i, addr := range loaded.ListenAddresses {
			if addr != original.ListenAddresses[i] {
				t.Errorf("ListenAddresses[%d] = %q, want %q", i, addr, original.ListenAddresses[i])
			}
		}
	}
	if len(loaded.UpstreamDNSFallback) != 1 || loaded.UpstreamDNSFallback[0] != "9.9.9.9" {
		t.Errorf("UpstreamDNSFallback = %v, want [9.9.9.9]", loaded.UpstreamDNSFallback)
	}
	if loaded.DNSQueryHistorySize != original.DNSQueryHistorySize {
		t.Errorf("DNSQueryHistorySize = %d, want %d", loaded.DNSQueryHistorySize, original.DNSQueryHistorySize)
	}
	if loaded.LogLevel != original.LogLevel {
		t.Errorf("LogLevel = %q, want %q", loaded.LogLevel, original.LogLevel)
	}
}

func TestSaveSettingToDB_Upsert(t *testing.T) {
	db := openTestDBWithSettings(t)
	defer db.Close()

	// Insert initial value
	if err := SaveSettingToDB(db, "test_key", "value1"); err != nil {
		t.Fatalf("first SaveSettingToDB failed: %v", err)
	}

	// Verify initial value
	var val string
	if err := db.QueryRow("SELECT value FROM settings WHERE key = ?", "test_key").Scan(&val); err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if val != "value1" {
		t.Errorf("initial value = %q, want %q", val, "value1")
	}

	// Upsert with new value
	if err := SaveSettingToDB(db, "test_key", "value2"); err != nil {
		t.Fatalf("second SaveSettingToDB failed: %v", err)
	}

	// Verify updated value
	if err := db.QueryRow("SELECT value FROM settings WHERE key = ?", "test_key").Scan(&val); err != nil {
		t.Fatalf("query after upsert failed: %v", err)
	}
	if val != "value2" {
		t.Errorf("upserted value = %q, want %q", val, "value2")
	}

	// Verify only one row exists
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM settings WHERE key = ?", "test_key").Scan(&count); err != nil {
		t.Fatalf("count query failed: %v", err)
	}
	if count != 1 {
		t.Errorf("row count = %d, want 1", count)
	}
}
