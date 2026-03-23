package models

import "time"

// AccessLog represents an HTTP access log entry.
type AccessLog struct {
	ID             int64     `json:"id"`
	Timestamp      time.Time `json:"timestamp"`
	Source         string    `json:"source"`
	ClientIP       string    `json:"client_ip"`
	Hostname       string    `json:"hostname"`
	Method         string    `json:"method"`
	Path           string    `json:"path"`
	StatusCode     int       `json:"status_code"`
	ResponseTimeMs int       `json:"response_time_ms"`
	Backend        string    `json:"backend"`
}
