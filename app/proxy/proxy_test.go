package proxy

import (
	"database/sql"
	"net/http"
	"testing"

	"dev_gateway_dns/app/models"

	_ "modernc.org/sqlite"
)

// --- extractHostname tests ---

func TestExtractHostname_WithPort(t *testing.T) {
	got := extractHostname("example.com:8080")
	if got != "example.com" {
		t.Errorf("extractHostname(%q) = %q, want %q", "example.com:8080", got, "example.com")
	}
}

func TestExtractHostname_WithoutPort(t *testing.T) {
	got := extractHostname("example.com")
	if got != "example.com" {
		t.Errorf("extractHostname(%q) = %q, want %q", "example.com", got, "example.com")
	}
}

func TestExtractHostname_IPv6WithPort(t *testing.T) {
	got := extractHostname("[::1]:8080")
	if got != "::1" {
		t.Errorf("extractHostname(%q) = %q, want %q", "[::1]:8080", got, "::1")
	}
}

func TestExtractHostname_IPv6WithoutPort(t *testing.T) {
	// net.SplitHostPort fails for bare IPv6 without port, so it returns the input as-is
	input := "::1"
	got := extractHostname(input)
	if got != input {
		t.Errorf("extractHostname(%q) = %q, want %q", input, got, input)
	}
}

func TestExtractHostname_Empty(t *testing.T) {
	got := extractHostname("")
	if got != "" {
		t.Errorf("extractHostname(%q) = %q, want %q", "", got, "")
	}
}

// --- extractIP tests ---

func TestExtractIP_WithPort(t *testing.T) {
	got := extractIP("192.168.1.1:12345")
	if got != "192.168.1.1" {
		t.Errorf("extractIP(%q) = %q, want %q", "192.168.1.1:12345", got, "192.168.1.1")
	}
}

func TestExtractIP_WithoutPort(t *testing.T) {
	got := extractIP("192.168.1.1")
	if got != "192.168.1.1" {
		t.Errorf("extractIP(%q) = %q, want %q", "192.168.1.1", got, "192.168.1.1")
	}
}

func TestExtractIP_IPv6WithPort(t *testing.T) {
	got := extractIP("[::1]:9090")
	if got != "::1" {
		t.Errorf("extractIP(%q) = %q, want %q", "[::1]:9090", got, "::1")
	}
}

func TestExtractIP_Empty(t *testing.T) {
	got := extractIP("")
	if got != "" {
		t.Errorf("extractIP(%q) = %q, want %q", "", got, "")
	}
}

// --- rewriteLocationHeader tests ---

func TestRewriteLocationHeader_RewritesHost(t *testing.T) {
	resp := &http.Response{
		Header: http.Header{},
	}
	resp.Header.Set("Location", "http://backend.local:3000/dashboard")
	rewriteLocationHeader(resp, "myapp.test")

	got := resp.Header.Get("Location")
	want := "http://myapp.test/dashboard"
	if got != want {
		t.Errorf("Location = %q, want %q", got, want)
	}
}

func TestRewriteLocationHeader_EmptyLocation(t *testing.T) {
	resp := &http.Response{
		Header: http.Header{},
	}
	rewriteLocationHeader(resp, "myapp.test")

	got := resp.Header.Get("Location")
	if got != "" {
		t.Errorf("Location = %q, want empty", got)
	}
}

func TestRewriteLocationHeader_RelativePath(t *testing.T) {
	resp := &http.Response{
		Header: http.Header{},
	}
	resp.Header.Set("Location", "/redirect")
	rewriteLocationHeader(resp, "myapp.test")

	got := resp.Header.Get("Location")
	// Relative URL has no host, so u.Host = "myapp.test" sets it
	want := "//myapp.test/redirect"
	if got != want {
		t.Errorf("Location = %q, want %q", got, want)
	}
}

// --- rewriteCookies tests ---

func TestRewriteCookies_RewritesDomain(t *testing.T) {
	resp := &http.Response{
		Header: http.Header{},
	}
	resp.Header.Add("Set-Cookie", "session=abc; Domain=backend.local; Path=/app")

	rewriteCookies(resp, "myapp.test")

	cookies := resp.Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected 1 cookie, got %d", len(cookies))
	}
	if cookies[0].Domain != "myapp.test" {
		t.Errorf("cookie Domain = %q, want %q", cookies[0].Domain, "myapp.test")
	}
	if cookies[0].Path != "/app" {
		t.Errorf("cookie Path = %q, want %q", cookies[0].Path, "/app")
	}
}

func TestRewriteCookies_StripsDomainPort(t *testing.T) {
	resp := &http.Response{
		Header: http.Header{},
	}
	resp.Header.Add("Set-Cookie", "token=xyz; Domain=backend.local:3000; Path=/")

	rewriteCookies(resp, "myapp.test:8080")

	cookies := resp.Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected 1 cookie, got %d", len(cookies))
	}
	// Domain should have port stripped
	if cookies[0].Domain != "myapp.test" {
		t.Errorf("cookie Domain = %q, want %q", cookies[0].Domain, "myapp.test")
	}
}

func TestRewriteCookies_NormalizesPath(t *testing.T) {
	resp := &http.Response{
		Header: http.Header{},
	}
	resp.Header.Add("Set-Cookie", "id=1; Path=subpath")

	rewriteCookies(resp, "myapp.test")

	cookies := resp.Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected 1 cookie, got %d", len(cookies))
	}
	if cookies[0].Path != "/subpath" {
		t.Errorf("cookie Path = %q, want %q", cookies[0].Path, "/subpath")
	}
}

func TestRewriteCookies_NoCookies(t *testing.T) {
	resp := &http.Response{
		Header: http.Header{},
	}
	// Should not panic
	rewriteCookies(resp, "myapp.test")

	cookies := resp.Cookies()
	if len(cookies) != 0 {
		t.Errorf("expected 0 cookies, got %d", len(cookies))
	}
}

// --- helper to create in-memory SQLite DB with proxy_rules table ---

func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory db: %v", err)
	}
	_, err = db.Exec(`CREATE TABLE proxy_rules (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		hostname TEXT NOT NULL UNIQUE,
		backend_protocol TEXT NOT NULL DEFAULT 'http',
		backend_ip TEXT,
		backend_port INTEGER NOT NULL,
		enabled BOOLEAN NOT NULL DEFAULT 1,
		created_at DATETIME NOT NULL DEFAULT (datetime('now')),
		updated_at DATETIME NOT NULL DEFAULT (datetime('now'))
	)`)
	if err != nil {
		t.Fatalf("failed to create proxy_rules table: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func newTestReverseProxy(db *sql.DB) *ReverseProxy {
	return NewReverseProxy(db, []string{"127.0.0.1"}, 80, 443,
		nil,
		nil,
		func() string { return "127.0.0.1" },
	)
}

// --- ReverseProxy.LoadRules tests ---

func TestReverseProxy_LoadRules_Empty(t *testing.T) {
	db := setupTestDB(t)
	rp := newTestReverseProxy(db)

	if err := rp.LoadRules(); err != nil {
		t.Fatalf("LoadRules() error: %v", err)
	}
	if len(rp.rules) != 0 {
		t.Errorf("expected 0 rules, got %d", len(rp.rules))
	}
}

func TestReverseProxy_LoadRules_WithData(t *testing.T) {
	db := setupTestDB(t)

	_, err := db.Exec(
		`INSERT INTO proxy_rules (hostname, backend_protocol, backend_ip, backend_port, enabled) VALUES (?, ?, ?, ?, ?)`,
		"app.test", "http", "10.0.0.1", 3000, true,
	)
	if err != nil {
		t.Fatalf("failed to insert test rule: %v", err)
	}
	_, err = db.Exec(
		`INSERT INTO proxy_rules (hostname, backend_protocol, backend_port, enabled) VALUES (?, ?, ?, ?)`,
		"api.test", "https", 8443, false,
	)
	if err != nil {
		t.Fatalf("failed to insert test rule: %v", err)
	}

	rp := newTestReverseProxy(db)
	if err := rp.LoadRules(); err != nil {
		t.Fatalf("LoadRules() error: %v", err)
	}

	if len(rp.rules) != 2 {
		t.Fatalf("expected 2 rules, got %d", len(rp.rules))
	}

	rule, ok := rp.rules["app.test"]
	if !ok {
		t.Fatal("expected rule for app.test")
	}
	if rule.BackendProtocol != "http" {
		t.Errorf("BackendProtocol = %q, want %q", rule.BackendProtocol, "http")
	}
	if rule.BackendPort != 3000 {
		t.Errorf("BackendPort = %d, want %d", rule.BackendPort, 3000)
	}
	if rule.BackendIP == nil || *rule.BackendIP != "10.0.0.1" {
		t.Errorf("BackendIP = %v, want %q", rule.BackendIP, "10.0.0.1")
	}

	rule2, ok := rp.rules["api.test"]
	if !ok {
		t.Fatal("expected rule for api.test")
	}
	if rule2.Enabled {
		t.Error("expected api.test rule to be disabled")
	}
}

// --- ReverseProxy.UpdateRule / RemoveRule tests ---

func TestReverseProxy_UpdateRule_Enabled(t *testing.T) {
	db := setupTestDB(t)
	rp := newTestReverseProxy(db)

	rule := &models.ProxyRule{
		ID:              1,
		Hostname:        "new.test",
		BackendProtocol: "http",
		BackendPort:     4000,
		Enabled:         true,
	}
	rp.UpdateRule(rule)

	if _, ok := rp.rules["new.test"]; !ok {
		t.Error("expected rule for new.test after UpdateRule")
	}
}

func TestReverseProxy_UpdateRule_Disabled(t *testing.T) {
	db := setupTestDB(t)
	rp := newTestReverseProxy(db)

	// First add a rule
	rp.UpdateRule(&models.ProxyRule{
		Hostname: "remove.test", BackendProtocol: "http", BackendPort: 3000, Enabled: true,
	})
	if _, ok := rp.rules["remove.test"]; !ok {
		t.Fatal("expected rule to exist before disabling")
	}

	// Now update with Enabled=false, which should remove it
	rp.UpdateRule(&models.ProxyRule{
		Hostname: "remove.test", BackendProtocol: "http", BackendPort: 3000, Enabled: false,
	})
	if _, ok := rp.rules["remove.test"]; ok {
		t.Error("expected rule to be removed when disabled")
	}
}

func TestReverseProxy_RemoveRule(t *testing.T) {
	db := setupTestDB(t)
	rp := newTestReverseProxy(db)

	rp.UpdateRule(&models.ProxyRule{
		Hostname: "gone.test", BackendProtocol: "http", BackendPort: 5000, Enabled: true,
	})
	rp.RemoveRule("gone.test")

	if _, ok := rp.rules["gone.test"]; ok {
		t.Error("expected rule to be removed after RemoveRule")
	}
}

func TestReverseProxy_RemoveRule_NonExistent(t *testing.T) {
	db := setupTestDB(t)
	rp := newTestReverseProxy(db)

	// Should not panic
	rp.RemoveRule("nonexistent.test")
}

// --- ForwardProxy.SetRules / UpdateRule / RemoveRule tests ---

func newTestForwardProxy() *ForwardProxy {
	return NewForwardProxy([]string{"127.0.0.1"}, 8888,
		nil,
		nil,
		func() string { return "127.0.0.1" },
	)
}

func TestForwardProxy_SetRules(t *testing.T) {
	fp := newTestForwardProxy()

	rules := map[string]*models.ProxyRule{
		"a.test": {Hostname: "a.test", BackendProtocol: "http", BackendPort: 3000, Enabled: true},
		"b.test": {Hostname: "b.test", BackendProtocol: "https", BackendPort: 8443, Enabled: true},
	}
	fp.SetRules(rules)

	if len(fp.rules) != 2 {
		t.Errorf("expected 2 rules, got %d", len(fp.rules))
	}
	if _, ok := fp.rules["a.test"]; !ok {
		t.Error("expected rule for a.test")
	}
	if _, ok := fp.rules["b.test"]; !ok {
		t.Error("expected rule for b.test")
	}
}

func TestForwardProxy_SetRules_Replaces(t *testing.T) {
	fp := newTestForwardProxy()

	fp.SetRules(map[string]*models.ProxyRule{
		"old.test": {Hostname: "old.test", BackendProtocol: "http", BackendPort: 3000, Enabled: true},
	})
	fp.SetRules(map[string]*models.ProxyRule{
		"new.test": {Hostname: "new.test", BackendProtocol: "http", BackendPort: 4000, Enabled: true},
	})

	if _, ok := fp.rules["old.test"]; ok {
		t.Error("old.test should have been replaced")
	}
	if _, ok := fp.rules["new.test"]; !ok {
		t.Error("expected new.test after SetRules replacement")
	}
}

func TestForwardProxy_UpdateRule_Enabled(t *testing.T) {
	fp := newTestForwardProxy()

	fp.UpdateRule(&models.ProxyRule{
		Hostname: "up.test", BackendProtocol: "http", BackendPort: 3000, Enabled: true,
	})
	if _, ok := fp.rules["up.test"]; !ok {
		t.Error("expected rule for up.test")
	}
}

func TestForwardProxy_UpdateRule_Disabled(t *testing.T) {
	fp := newTestForwardProxy()

	fp.UpdateRule(&models.ProxyRule{
		Hostname: "dis.test", BackendProtocol: "http", BackendPort: 3000, Enabled: true,
	})
	fp.UpdateRule(&models.ProxyRule{
		Hostname: "dis.test", BackendProtocol: "http", BackendPort: 3000, Enabled: false,
	})

	if _, ok := fp.rules["dis.test"]; ok {
		t.Error("expected rule to be removed when disabled")
	}
}

func TestForwardProxy_RemoveRule(t *testing.T) {
	fp := newTestForwardProxy()

	fp.UpdateRule(&models.ProxyRule{
		Hostname: "rm.test", BackendProtocol: "http", BackendPort: 3000, Enabled: true,
	})
	fp.RemoveRule("rm.test")

	if _, ok := fp.rules["rm.test"]; ok {
		t.Error("expected rule to be removed")
	}
}

func TestForwardProxy_RemoveRule_NonExistent(t *testing.T) {
	fp := newTestForwardProxy()
	// Should not panic
	fp.RemoveRule("nonexistent.test")
}
