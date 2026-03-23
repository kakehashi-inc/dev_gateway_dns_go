package models

import "time"

// HostCertificate represents an SSL certificate for a specific hostname.
type HostCertificate struct {
	ID        int64     `json:"id"`
	Hostname  string    `json:"hostname"`
	CertPEM   []byte    `json:"cert_pem"`
	KeyPEM    []byte    `json:"key_pem"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}
