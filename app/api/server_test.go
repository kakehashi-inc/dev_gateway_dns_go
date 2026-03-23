package api

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"dev_gateway_dns/app/cert"
	"dev_gateway_dns/app/dns"
	"dev_gateway_dns/app/models"
	"dev_gateway_dns/app/modules"
	"dev_gateway_dns/app/proxy"

	_ "modernc.org/sqlite"
)

// migrationSQL contains the schema needed for tests.
const migrationSQL = `
CREATE TABLE IF NOT EXISTS proxy_rules (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    hostname TEXT NOT NULL UNIQUE,
    backend_protocol TEXT NOT NULL DEFAULT 'http',
    backend_ip TEXT,
    backend_port INTEGER NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT 1,
    created_at DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS dns_records (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    type TEXT NOT NULL,
    value TEXT NOT NULL,
    ttl INTEGER NOT NULL DEFAULT 300,
    priority INTEGER,
    weight INTEGER,
    port INTEGER,
    created_at DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS ca_certificate (
    id INTEGER PRIMARY KEY,
    cert_pem BLOB NOT NULL,
    key_pem BLOB NOT NULL,
    expires_at DATETIME NOT NULL,
    created_at DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS host_certificates (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    hostname TEXT NOT NULL UNIQUE,
    cert_pem BLOB NOT NULL,
    key_pem BLOB NOT NULL,
    expires_at DATETIME NOT NULL,
    created_at DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS settings (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL,
    updated_at DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS access_logs (
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
);

INSERT OR IGNORE INTO settings (key, value, updated_at) VALUES ('http_port', '80', datetime('now'));
INSERT OR IGNORE INTO settings (key, value, updated_at) VALUES ('https_port', '443', datetime('now'));
INSERT OR IGNORE INTO settings (key, value, updated_at) VALUES ('dns_port', '53', datetime('now'));
INSERT OR IGNORE INTO settings (key, value, updated_at) VALUES ('proxy_port', '8888', datetime('now'));
INSERT OR IGNORE INTO settings (key, value, updated_at) VALUES ('admin_port', '9090', datetime('now'));
INSERT OR IGNORE INTO settings (key, value, updated_at) VALUES ('listen_addresses', '["0.0.0.0"]', datetime('now'));
INSERT OR IGNORE INTO settings (key, value, updated_at) VALUES ('upstream_dns_fallback', '["8.8.8.8","1.1.1.1"]', datetime('now'));
INSERT OR IGNORE INTO settings (key, value, updated_at) VALUES ('dns_query_history_size', '1000', datetime('now'));
INSERT OR IGNORE INTO settings (key, value, updated_at) VALUES ('log_level', '"info"', datetime('now'));
`

// testEnv holds all dependencies needed for the API server tests.
type testEnv struct {
	db       *sql.DB
	server   *Server
	baseURL  string
	client   *http.Client
	adminPort int
}

// newTestEnv creates an in-memory SQLite DB, runs migrations, and starts the API server
// on a random available port.
func newTestEnv(t *testing.T) *testEnv {
	t.Helper()

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory DB: %v", err)
	}

	if _, err := db.Exec(migrationSQL); err != nil {
		t.Fatalf("failed to run migration: %v", err)
	}

	// Find a free port for the admin API.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to find free port: %v", err)
	}
	adminPort := listener.Addr().(*net.TCPAddr).Port
	listener.Close()

	config := &modules.AppConfig{
		HTTPPort:            59201,
		HTTPSPort:           59202,
		DNSPort:             59203,
		ProxyPort:           59204,
		AdminPort:           adminPort,
		ListenAddresses:     []string{"127.0.0.1"},
		UpstreamDNSFallback: []string{"8.8.8.8", "1.1.1.1"},
		DNSQueryHistorySize: 100,
		LogLevel:            "info",
	}

	// Create real but minimal dependencies.
	certMgr := cert.NewManager(db)
	if err := certMgr.Init(); err != nil {
		t.Fatalf("failed to init cert manager: %v", err)
	}

	autoRecords := dns.NewAutoRecordMap()
	queryLog := dns.NewRingBuffer(100)

	nopLogAccess := func(_ models.AccessLog) {}
	nopResolveIP := func() string { return "127.0.0.1" }

	rp := proxy.NewReverseProxy(db, []string{"127.0.0.1"}, config.HTTPPort, config.HTTPSPort,
		certMgr.GetCertificate, nopLogAccess, nopResolveIP)
	fp := proxy.NewForwardProxy([]string{"127.0.0.1"}, config.ProxyPort,
		certMgr.GetCertificate, nopLogAccess, nopResolveIP)

	srv := NewServer(db, config, rp, fp, certMgr, autoRecords, queryLog, "test-0.0.1", []string{"127.0.0.1"}, nil)
	if err := srv.Start(); err != nil {
		t.Fatalf("failed to start API server: %v", err)
	}

	baseURL := fmt.Sprintf("http://127.0.0.1:%d", adminPort)

	// Wait for the server to be ready.
	client := &http.Client{Timeout: 5 * time.Second}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := client.Get(baseURL + "/api/v1/status/health")
		if err == nil {
			resp.Body.Close()
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	t.Cleanup(func() {
		srv.Stop()
		db.Close()
	})

	return &testEnv{
		db:        db,
		server:    srv,
		baseURL:   baseURL,
		client:    client,
		adminPort: adminPort,
	}
}

func TestGetProxyRules_Empty(t *testing.T) {
	env := newTestEnv(t)

	resp, err := env.client.Get(env.baseURL + "/api/v1/proxy/rules")
	if err != nil {
		t.Fatalf("GET /api/v1/proxy/rules failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var rules []models.ProxyRule
	if err := json.NewDecoder(resp.Body).Decode(&rules); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(rules) != 0 {
		t.Errorf("expected empty rules array, got %d entries", len(rules))
	}
}

func TestPostProxyRule_Create(t *testing.T) {
	env := newTestEnv(t)

	body := `{"hostname":"test.local","backend_protocol":"http","backend_port":3000,"enabled":true}`
	resp, err := env.client.Post(
		env.baseURL+"/api/v1/proxy/rules",
		"application/json",
		strings.NewReader(body),
	)
	if err != nil {
		t.Fatalf("POST /api/v1/proxy/rules failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d, want %d; body = %s", resp.StatusCode, http.StatusCreated, respBody)
	}

	var rule models.ProxyRule
	if err := json.NewDecoder(resp.Body).Decode(&rule); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if rule.Hostname != "test.local" {
		t.Errorf("hostname = %q, want %q", rule.Hostname, "test.local")
	}
	if rule.ID == 0 {
		t.Error("expected non-zero ID")
	}
}

func TestGetDNSRecords(t *testing.T) {
	env := newTestEnv(t)

	resp, err := env.client.Get(env.baseURL + "/api/v1/dns/records")
	if err != nil {
		t.Fatalf("GET /api/v1/dns/records failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
}

func TestGetSettings(t *testing.T) {
	env := newTestEnv(t)

	resp, err := env.client.Get(env.baseURL + "/api/v1/settings")
	if err != nil {
		t.Fatalf("GET /api/v1/settings failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var cfg modules.AppConfig
	if err := json.NewDecoder(resp.Body).Decode(&cfg); err != nil {
		t.Fatalf("failed to decode settings: %v", err)
	}
	if cfg.AdminPort == 0 {
		t.Error("expected non-zero admin_port in settings")
	}
}

func TestGetStatusOverview(t *testing.T) {
	env := newTestEnv(t)

	resp, err := env.client.Get(env.baseURL + "/api/v1/status/overview")
	if err != nil {
		t.Fatalf("GET /api/v1/status/overview failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var overview map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&overview); err != nil {
		t.Fatalf("failed to decode overview: %v", err)
	}
	version, ok := overview["version"]
	if !ok {
		t.Fatal("response missing 'version' field")
	}
	if version != "test-0.0.1" {
		t.Errorf("version = %v, want %q", version, "test-0.0.1")
	}
}

func TestGetStatusHealth(t *testing.T) {
	env := newTestEnv(t)

	resp, err := env.client.Get(env.baseURL + "/api/v1/status/health")
	if err != nil {
		t.Fatalf("GET /api/v1/status/health failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
}

func TestGetCerts(t *testing.T) {
	env := newTestEnv(t)

	resp, err := env.client.Get(env.baseURL + "/api/v1/certs")
	if err != nil {
		t.Fatalf("GET /api/v1/certs failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
}
