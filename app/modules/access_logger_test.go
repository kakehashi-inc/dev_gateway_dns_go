package modules

import (
	"database/sql"
	"testing"
	"time"

	"dev_gateway_dns/app/models"

	_ "modernc.org/sqlite"
)

func openTestDBWithAccessLogs(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory db: %v", err)
	}
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS access_logs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		timestamp DATETIME NOT NULL,
		source TEXT NOT NULL,
		client_ip TEXT NOT NULL,
		hostname TEXT NOT NULL,
		method TEXT NOT NULL,
		path TEXT NOT NULL,
		status_code INTEGER NOT NULL,
		response_time_ms INTEGER NOT NULL,
		backend TEXT NOT NULL
	)`)
	if err != nil {
		t.Fatalf("failed to create access_logs table: %v", err)
	}
	return db
}

func TestAccessLogger_Log_InsertsRow(t *testing.T) {
	db := openTestDBWithAccessLogs(t)
	defer db.Close()

	logger := NewAccessLogger(db)

	entry := models.AccessLog{
		Timestamp:      time.Date(2026, 1, 15, 10, 30, 0, 0, time.UTC),
		Source:         "http",
		ClientIP:       "192.168.1.10",
		Hostname:       "example.local",
		Method:         "GET",
		Path:           "/api/health",
		StatusCode:     200,
		ResponseTimeMs: 42,
		Backend:        "localhost:3000",
	}

	logger.Log(entry)

	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM access_logs").Scan(&count); err != nil {
		t.Fatalf("count query failed: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 row, got %d", count)
	}

	var source, clientIP, hostname, method, path, backend string
	var statusCode, responseTimeMs int
	err := db.QueryRow("SELECT source, client_ip, hostname, method, path, status_code, response_time_ms, backend FROM access_logs WHERE id = 1").
		Scan(&source, &clientIP, &hostname, &method, &path, &statusCode, &responseTimeMs, &backend)
	if err != nil {
		t.Fatalf("select query failed: %v", err)
	}

	if source != "http" {
		t.Errorf("source = %q, want %q", source, "http")
	}
	if clientIP != "192.168.1.10" {
		t.Errorf("client_ip = %q, want %q", clientIP, "192.168.1.10")
	}
	if hostname != "example.local" {
		t.Errorf("hostname = %q, want %q", hostname, "example.local")
	}
	if method != "GET" {
		t.Errorf("method = %q, want %q", method, "GET")
	}
	if path != "/api/health" {
		t.Errorf("path = %q, want %q", path, "/api/health")
	}
	if statusCode != 200 {
		t.Errorf("status_code = %d, want 200", statusCode)
	}
	if responseTimeMs != 42 {
		t.Errorf("response_time_ms = %d, want 42", responseTimeMs)
	}
	if backend != "localhost:3000" {
		t.Errorf("backend = %q, want %q", backend, "localhost:3000")
	}
}

func TestAccessLogger_Log_DefaultTimestamp(t *testing.T) {
	db := openTestDBWithAccessLogs(t)
	defer db.Close()

	logger := NewAccessLogger(db)

	// Log with zero timestamp to test default behavior.
	entry := models.AccessLog{
		Source:         "dns",
		ClientIP:       "10.0.0.1",
		Hostname:       "test.local",
		Method:         "QUERY",
		Path:           "/",
		StatusCode:     0,
		ResponseTimeMs: 1,
		Backend:        "upstream",
	}

	before := time.Now().UTC()
	logger.Log(entry)

	var ts string
	if err := db.QueryRow("SELECT timestamp FROM access_logs WHERE id = 1").Scan(&ts); err != nil {
		t.Fatalf("query failed: %v", err)
	}

	// The timestamp should have been set automatically and be recent.
	parsed, err := time.Parse("2006-01-02T15:04:05Z", ts)
	if err != nil {
		// Try alternate format used by SQLite.
		parsed, err = time.Parse("2006-01-02 15:04:05-07:00", ts)
		if err != nil {
			parsed, err = time.Parse("2006-01-02 15:04:05+00:00", ts)
			if err != nil {
				t.Fatalf("failed to parse timestamp %q: %v", ts, err)
			}
		}
	}

	if parsed.Before(before.Add(-time.Second)) {
		t.Errorf("timestamp %v is before test start %v", parsed, before)
	}
}
