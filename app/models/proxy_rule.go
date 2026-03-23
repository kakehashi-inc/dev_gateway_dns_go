package models

import "time"

// ProxyRule represents a reverse proxy routing rule.
type ProxyRule struct {
	ID              int64     `json:"id"`
	Hostname        string    `json:"hostname"`
	BackendProtocol string    `json:"backend_protocol"`
	BackendIP       *string   `json:"backend_ip"`
	BackendPort     int       `json:"backend_port"`
	Enabled         bool      `json:"enabled"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}
