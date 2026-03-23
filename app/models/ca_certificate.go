package models

import "time"

// CACertificate represents the root CA certificate (single row).
type CACertificate struct {
	ID        int64     `json:"id"`
	CertPEM   []byte    `json:"cert_pem"`
	KeyPEM    []byte    `json:"key_pem"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}
