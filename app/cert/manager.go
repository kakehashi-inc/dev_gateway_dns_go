package cert

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"database/sql"
	"encoding/pem"
	"fmt"
	"log"
	"math/big"
	"sync"
	"time"
)

// Manager handles CA and host certificate lifecycle.
type Manager struct {
	db     *sql.DB
	mu     sync.RWMutex
	caCert *x509.Certificate
	caKey  *ecdsa.PrivateKey
	cache  map[string]*tls.Certificate
}

// NewManager creates a new certificate manager.
func NewManager(db *sql.DB) *Manager {
	return &Manager{
		db:    db,
		cache: make(map[string]*tls.Certificate),
	}
}

// Init loads or generates the root CA certificate.
func (m *Manager) Init() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var certPEM, keyPEM []byte
	var expiresAt time.Time
	err := m.db.QueryRow("SELECT cert_pem, key_pem, expires_at FROM ca_certificate WHERE id = 1").
		Scan(&certPEM, &keyPEM, &expiresAt)

	if err == nil && time.Now().Before(expiresAt) {
		return m.loadCA(certPEM, keyPEM)
	}

	return m.generateAndSaveCA()
}

// GetCertificate returns a TLS certificate for the given hostname.
// It loads from cache, then DB, then generates a new one.
func (m *Manager) GetCertificate(hostname string) (*tls.Certificate, error) {
	m.mu.RLock()
	if cert, ok := m.cache[hostname]; ok {
		m.mu.RUnlock()
		return cert, nil
	}
	m.mu.RUnlock()

	return m.loadOrGenerateHostCert(hostname)
}

// GetCACertPEM returns the CA certificate in PEM format.
func (m *Manager) GetCACertPEM() ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.caCert == nil {
		return nil, fmt.Errorf("CA certificate not initialized")
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: m.caCert.Raw}), nil
}

// GetCACertDER returns the CA certificate in DER format.
func (m *Manager) GetCACertDER() ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.caCert == nil {
		return nil, fmt.Errorf("CA certificate not initialized")
	}
	return m.caCert.Raw, nil
}

// GetCACertP12 returns the CA certificate in PKCS#12 format with an empty password.
func (m *Manager) GetCACertP12() ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.caCert == nil || m.caKey == nil {
		return nil, fmt.Errorf("CA certificate not initialized")
	}

	return EncodePKCS12(m.caKey, m.caCert, "")
}

// RegenerateHostCert generates a new certificate for a hostname, replacing any existing one.
func (m *Manager) RegenerateHostCert(hostname string) (*tls.Certificate, error) {
	m.mu.Lock()
	delete(m.cache, hostname)
	m.mu.Unlock()

	_, err := m.db.Exec("DELETE FROM host_certificates WHERE hostname = ?", hostname)
	if err != nil {
		return nil, fmt.Errorf("failed to delete old certificate: %w", err)
	}

	return m.loadOrGenerateHostCert(hostname)
}

// ListCertificates returns all host certificates from the database.
func (m *Manager) ListCertificates() ([]CertInfo, error) {
	rows, err := m.db.Query("SELECT hostname, expires_at, created_at FROM host_certificates ORDER BY hostname")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var certs []CertInfo
	for rows.Next() {
		var ci CertInfo
		if err := rows.Scan(&ci.Hostname, &ci.ExpiresAt, &ci.CreatedAt); err != nil {
			continue
		}
		certs = append(certs, ci)
	}
	return certs, nil
}

// CertInfo holds summary information about a certificate.
type CertInfo struct {
	Hostname  string    `json:"hostname"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

func (m *Manager) loadCA(certPEM, keyPEM []byte) error {
	certBlock, _ := pem.Decode(certPEM)
	if certBlock == nil {
		return fmt.Errorf("failed to decode CA cert PEM")
	}
	cert, err := x509.ParseCertificate(certBlock.Bytes)
	if err != nil {
		return fmt.Errorf("failed to parse CA cert: %w", err)
	}

	keyBlock, _ := pem.Decode(keyPEM)
	if keyBlock == nil {
		return fmt.Errorf("failed to decode CA key PEM")
	}
	key, err := x509.ParseECPrivateKey(keyBlock.Bytes)
	if err != nil {
		return fmt.Errorf("failed to parse CA key: %w", err)
	}

	m.caCert = cert
	m.caKey = key
	log.Println("CA certificate loaded from database")
	return nil
}

func (m *Manager) generateAndSaveCA() error {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return fmt.Errorf("failed to generate CA key: %w", err)
	}

	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return fmt.Errorf("failed to generate serial number: %w", err)
	}

	now := time.Now()
	expiresAt := now.Add(10 * 365 * 24 * time.Hour)

	template := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName:   "DevGatewayDNS CA",
			Organization: []string{"DevGatewayDNS"},
		},
		NotBefore:             now,
		NotAfter:              expiresAt,
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		MaxPathLen:            0,
		MaxPathLenZero:        true,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		return fmt.Errorf("failed to create CA certificate: %w", err)
	}

	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		return fmt.Errorf("failed to parse generated CA cert: %w", err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return fmt.Errorf("failed to marshal CA key: %w", err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})

	_, err = m.db.Exec(
		`INSERT INTO ca_certificate (id, cert_pem, key_pem, expires_at, created_at)
		 VALUES (1, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET cert_pem=excluded.cert_pem, key_pem=excluded.key_pem,
		 expires_at=excluded.expires_at, created_at=excluded.created_at`,
		certPEM, keyPEM, expiresAt, now,
	)
	if err != nil {
		return fmt.Errorf("failed to save CA certificate: %w", err)
	}

	m.caCert = cert
	m.caKey = key
	log.Println("New CA certificate generated and saved")
	return nil
}

func (m *Manager) loadOrGenerateHostCert(hostname string) (*tls.Certificate, error) {
	var certPEM, keyPEM []byte
	var expiresAt time.Time
	err := m.db.QueryRow(
		"SELECT cert_pem, key_pem, expires_at FROM host_certificates WHERE hostname = ?",
		hostname,
	).Scan(&certPEM, &keyPEM, &expiresAt)

	if err == nil && time.Now().Before(expiresAt) {
		tlsCert, err := tls.X509KeyPair(certPEM, keyPEM)
		if err == nil {
			m.mu.Lock()
			m.cache[hostname] = &tlsCert
			m.mu.Unlock()
			return &tlsCert, nil
		}
	}

	return m.generateHostCert(hostname)
}

func (m *Manager) generateHostCert(hostname string) (*tls.Certificate, error) {
	m.mu.RLock()
	caCert := m.caCert
	caKey := m.caKey
	m.mu.RUnlock()

	if caCert == nil || caKey == nil {
		return nil, fmt.Errorf("CA certificate not initialized")
	}

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate host key: %w", err)
	}

	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, fmt.Errorf("failed to generate serial number: %w", err)
	}

	now := time.Now()
	expiresAt := now.Add(365 * 24 * time.Hour)

	template := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName: hostname,
		},
		DNSNames:  []string{hostname},
		NotBefore: now,
		NotAfter:  expiresAt,
		KeyUsage:  x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{
			x509.ExtKeyUsageServerAuth,
		},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, caCert, &key.PublicKey, caKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create host certificate: %w", err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal host key: %w", err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})

	_, err = m.db.Exec(
		`INSERT INTO host_certificates (hostname, cert_pem, key_pem, expires_at, created_at)
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(hostname) DO UPDATE SET cert_pem=excluded.cert_pem, key_pem=excluded.key_pem,
		 expires_at=excluded.expires_at, created_at=excluded.created_at`,
		hostname, certPEM, keyPEM, expiresAt, now,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to save host certificate: %w", err)
	}

	tlsCert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, fmt.Errorf("failed to create TLS certificate: %w", err)
	}

	m.mu.Lock()
	m.cache[hostname] = &tlsCert
	m.mu.Unlock()

	log.Printf("Generated certificate for %s", hostname)
	return &tlsCert, nil
}
