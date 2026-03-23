package cert

import (
	"crypto/x509"
	"database/sql"
	"encoding/pem"
	"testing"

	_ "modernc.org/sqlite"
)

// setupTestDB creates an in-memory SQLite database with the required schema.
func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory database: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	schema := `
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
	`
	if _, err := db.Exec(schema); err != nil {
		t.Fatalf("failed to create schema: %v", err)
	}
	return db
}

// setupManager creates a Manager with an initialized CA for tests that need it.
func setupManager(t *testing.T) *Manager {
	t.Helper()
	db := setupTestDB(t)
	mgr := NewManager(db)
	if err := mgr.Init(); err != nil {
		t.Fatalf("failed to initialize manager: %v", err)
	}
	return mgr
}

func TestInit_CreatesValidCA(t *testing.T) {
	db := setupTestDB(t)
	mgr := NewManager(db)

	if err := mgr.Init(); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	if mgr.caCert == nil {
		t.Fatal("CA certificate is nil after Init")
	}
	if mgr.caKey == nil {
		t.Fatal("CA private key is nil after Init")
	}
	if !mgr.caCert.IsCA {
		t.Error("CA certificate IsCA flag is false")
	}
	if mgr.caCert.BasicConstraintsValid != true {
		t.Error("CA certificate BasicConstraintsValid is false")
	}
	if mgr.caCert.Subject.CommonName != "DevGatewayDNS CA" {
		t.Errorf("unexpected CA common name: got %q, want %q", mgr.caCert.Subject.CommonName, "DevGatewayDNS CA")
	}
	if mgr.caCert.KeyUsage&x509.KeyUsageCertSign == 0 {
		t.Error("CA certificate missing KeyUsageCertSign")
	}
	if mgr.caCert.KeyUsage&x509.KeyUsageCRLSign == 0 {
		t.Error("CA certificate missing KeyUsageCRLSign")
	}

	// Verify CA is persisted in the database
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM ca_certificate").Scan(&count); err != nil {
		t.Fatalf("failed to query ca_certificate: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 CA row, got %d", count)
	}
}

func TestInit_LoadsExistingCA(t *testing.T) {
	db := setupTestDB(t)

	// First manager generates the CA
	mgr1 := NewManager(db)
	if err := mgr1.Init(); err != nil {
		t.Fatalf("first Init failed: %v", err)
	}
	firstSerial := mgr1.caCert.SerialNumber

	// Second manager should load the same CA
	mgr2 := NewManager(db)
	if err := mgr2.Init(); err != nil {
		t.Fatalf("second Init failed: %v", err)
	}

	if mgr2.caCert.SerialNumber.Cmp(firstSerial) != 0 {
		t.Error("second Init generated a new CA instead of loading the existing one")
	}
}

func TestGetCertificate_CreatesValidHostCert(t *testing.T) {
	mgr := setupManager(t)
	hostname := "test.local"

	tlsCert, err := mgr.GetCertificate(hostname)
	if err != nil {
		t.Fatalf("GetCertificate failed: %v", err)
	}
	if tlsCert == nil {
		t.Fatal("returned TLS certificate is nil")
	}

	// Parse the leaf certificate to verify properties
	leaf, err := x509.ParseCertificate(tlsCert.Certificate[0])
	if err != nil {
		t.Fatalf("failed to parse leaf certificate: %v", err)
	}

	if leaf.Subject.CommonName != hostname {
		t.Errorf("unexpected CN: got %q, want %q", leaf.Subject.CommonName, hostname)
	}

	foundDNS := false
	for _, name := range leaf.DNSNames {
		if name == hostname {
			foundDNS = true
			break
		}
	}
	if !foundDNS {
		t.Errorf("hostname %q not found in DNSNames: %v", hostname, leaf.DNSNames)
	}

	if leaf.KeyUsage&x509.KeyUsageDigitalSignature == 0 {
		t.Error("host certificate missing KeyUsageDigitalSignature")
	}

	foundServerAuth := false
	for _, usage := range leaf.ExtKeyUsage {
		if usage == x509.ExtKeyUsageServerAuth {
			foundServerAuth = true
			break
		}
	}
	if !foundServerAuth {
		t.Error("host certificate missing ExtKeyUsageServerAuth")
	}

	// Verify the certificate is signed by our CA
	roots := x509.NewCertPool()
	roots.AddCert(mgr.caCert)
	opts := x509.VerifyOptions{
		Roots:   roots,
		DNSName: hostname,
	}
	if _, err := leaf.Verify(opts); err != nil {
		t.Errorf("host certificate verification failed: %v", err)
	}
}

func TestGetCertificate_CachesResult(t *testing.T) {
	mgr := setupManager(t)
	hostname := "cached.local"

	cert1, err := mgr.GetCertificate(hostname)
	if err != nil {
		t.Fatalf("first GetCertificate failed: %v", err)
	}

	cert2, err := mgr.GetCertificate(hostname)
	if err != nil {
		t.Fatalf("second GetCertificate failed: %v", err)
	}

	// Same pointer means it came from cache
	if cert1 != cert2 {
		t.Error("second call did not return cached certificate (different pointer)")
	}

	// Verify it is in the cache
	mgr.mu.RLock()
	_, inCache := mgr.cache[hostname]
	mgr.mu.RUnlock()
	if !inCache {
		t.Error("certificate not found in cache after GetCertificate")
	}
}

func TestRegenerateHostCert_CreatesNewCert(t *testing.T) {
	mgr := setupManager(t)
	hostname := "regen.local"

	original, err := mgr.GetCertificate(hostname)
	if err != nil {
		t.Fatalf("initial GetCertificate failed: %v", err)
	}

	regenerated, err := mgr.RegenerateHostCert(hostname)
	if err != nil {
		t.Fatalf("RegenerateHostCert failed: %v", err)
	}

	if regenerated == nil {
		t.Fatal("regenerated certificate is nil")
	}

	// Parse both to compare serial numbers
	origLeaf, err := x509.ParseCertificate(original.Certificate[0])
	if err != nil {
		t.Fatalf("failed to parse original leaf: %v", err)
	}
	regenLeaf, err := x509.ParseCertificate(regenerated.Certificate[0])
	if err != nil {
		t.Fatalf("failed to parse regenerated leaf: %v", err)
	}

	if origLeaf.SerialNumber.Cmp(regenLeaf.SerialNumber) == 0 {
		t.Error("regenerated certificate has the same serial number as the original")
	}

	// Verify the new cert is valid
	roots := x509.NewCertPool()
	roots.AddCert(mgr.caCert)
	opts := x509.VerifyOptions{
		Roots:   roots,
		DNSName: hostname,
	}
	if _, err := regenLeaf.Verify(opts); err != nil {
		t.Errorf("regenerated certificate verification failed: %v", err)
	}
}

func TestGetCACertPEM_ReturnsValidPEM(t *testing.T) {
	mgr := setupManager(t)

	pemData, err := mgr.GetCACertPEM()
	if err != nil {
		t.Fatalf("GetCACertPEM failed: %v", err)
	}
	if len(pemData) == 0 {
		t.Fatal("PEM data is empty")
	}

	block, _ := pem.Decode(pemData)
	if block == nil {
		t.Fatal("failed to decode PEM block")
	}
	if block.Type != "CERTIFICATE" {
		t.Errorf("unexpected PEM block type: got %q, want %q", block.Type, "CERTIFICATE")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("failed to parse certificate from PEM: %v", err)
	}
	if !cert.IsCA {
		t.Error("parsed certificate is not a CA")
	}
}

func TestGetCACertPEM_ErrorWhenNotInitialized(t *testing.T) {
	db := setupTestDB(t)
	mgr := NewManager(db)

	_, err := mgr.GetCACertPEM()
	if err == nil {
		t.Fatal("expected error when CA not initialized, got nil")
	}
}

func TestGetCACertDER_ReturnsValidDER(t *testing.T) {
	mgr := setupManager(t)

	derData, err := mgr.GetCACertDER()
	if err != nil {
		t.Fatalf("GetCACertDER failed: %v", err)
	}
	if len(derData) == 0 {
		t.Fatal("DER data is empty")
	}

	cert, err := x509.ParseCertificate(derData)
	if err != nil {
		t.Fatalf("failed to parse DER certificate: %v", err)
	}
	if !cert.IsCA {
		t.Error("parsed DER certificate is not a CA")
	}
	if cert.Subject.CommonName != "DevGatewayDNS CA" {
		t.Errorf("unexpected CN from DER: got %q", cert.Subject.CommonName)
	}
}

func TestGetCACertDER_ErrorWhenNotInitialized(t *testing.T) {
	db := setupTestDB(t)
	mgr := NewManager(db)

	_, err := mgr.GetCACertDER()
	if err == nil {
		t.Fatal("expected error when CA not initialized, got nil")
	}
}

func TestGetCACertP12_ReturnsValidP12(t *testing.T) {
	mgr := setupManager(t)

	p12Data, err := mgr.GetCACertP12()
	if err != nil {
		t.Fatalf("GetCACertP12 failed: %v", err)
	}
	if len(p12Data) == 0 {
		t.Fatal("P12 data is empty")
	}

	// P12 data should start with a recognizable ASN.1 SEQUENCE tag
	if p12Data[0] != 0x30 {
		t.Errorf("P12 data does not start with ASN.1 SEQUENCE tag (0x30), got 0x%02x", p12Data[0])
	}
}

func TestGetCACertP12_ErrorWhenNotInitialized(t *testing.T) {
	db := setupTestDB(t)
	mgr := NewManager(db)

	_, err := mgr.GetCACertP12()
	if err == nil {
		t.Fatal("expected error when CA not initialized, got nil")
	}
}

func TestListCertificates_ReturnsCorrectList(t *testing.T) {
	mgr := setupManager(t)

	// Initially empty
	certs, err := mgr.ListCertificates()
	if err != nil {
		t.Fatalf("ListCertificates failed: %v", err)
	}
	if len(certs) != 0 {
		t.Errorf("expected 0 certificates initially, got %d", len(certs))
	}

	// Generate certificates for multiple hosts
	hostnames := []string{"alpha.local", "beta.local", "gamma.local"}
	for _, h := range hostnames {
		if _, err := mgr.GetCertificate(h); err != nil {
			t.Fatalf("GetCertificate(%q) failed: %v", h, err)
		}
	}

	certs, err = mgr.ListCertificates()
	if err != nil {
		t.Fatalf("ListCertificates failed after generating certs: %v", err)
	}
	if len(certs) != len(hostnames) {
		t.Fatalf("expected %d certificates, got %d", len(hostnames), len(certs))
	}

	// Results should be sorted by hostname
	for i, ci := range certs {
		if ci.Hostname != hostnames[i] {
			t.Errorf("certificate[%d]: got hostname %q, want %q", i, ci.Hostname, hostnames[i])
		}
		if ci.ExpiresAt.IsZero() {
			t.Errorf("certificate[%d]: ExpiresAt is zero", i)
		}
		if ci.CreatedAt.IsZero() {
			t.Errorf("certificate[%d]: CreatedAt is zero", i)
		}
	}
}

func TestListCertificates_AfterRegenerate(t *testing.T) {
	mgr := setupManager(t)

	hostname := "regen-list.local"
	if _, err := mgr.GetCertificate(hostname); err != nil {
		t.Fatalf("GetCertificate failed: %v", err)
	}

	if _, err := mgr.RegenerateHostCert(hostname); err != nil {
		t.Fatalf("RegenerateHostCert failed: %v", err)
	}

	certs, err := mgr.ListCertificates()
	if err != nil {
		t.Fatalf("ListCertificates failed: %v", err)
	}

	// Should still have exactly 1 entry (regenerate replaces, not duplicates)
	if len(certs) != 1 {
		t.Errorf("expected 1 certificate after regeneration, got %d", len(certs))
	}
}

func TestGetCertificate_MultipleHostsIndependent(t *testing.T) {
	mgr := setupManager(t)

	cert1, err := mgr.GetCertificate("host1.local")
	if err != nil {
		t.Fatalf("GetCertificate(host1) failed: %v", err)
	}
	cert2, err := mgr.GetCertificate("host2.local")
	if err != nil {
		t.Fatalf("GetCertificate(host2) failed: %v", err)
	}

	leaf1, _ := x509.ParseCertificate(cert1.Certificate[0])
	leaf2, _ := x509.ParseCertificate(cert2.Certificate[0])

	if leaf1.SerialNumber.Cmp(leaf2.SerialNumber) == 0 {
		t.Error("different hosts got the same serial number")
	}
	if leaf1.Subject.CommonName == leaf2.Subject.CommonName {
		t.Error("different hosts got the same common name")
	}
}
