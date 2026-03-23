package models

import "time"

// DNSRecord represents a manually configured DNS record stored in the database.
type DNSRecord struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Type      string    `json:"type"`
	Value     string    `json:"value"`
	TTL       int       `json:"ttl"`
	Priority  *int      `json:"priority,omitempty"`
	Weight    *int      `json:"weight,omitempty"`
	Port      *int      `json:"port,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// DNSRecordView represents a DNS record for API responses, merging auto and manual records.
type DNSRecordView struct {
	ID       *int64 `json:"id,omitempty"`
	Name     string `json:"name"`
	Type     string `json:"type"`
	Value    string `json:"value"`
	TTL      int    `json:"ttl"`
	Priority *int   `json:"priority,omitempty"`
	Weight   *int   `json:"weight,omitempty"`
	Port     *int   `json:"port,omitempty"`
	Locked   bool   `json:"locked"`
	Source   string `json:"source"`
}
