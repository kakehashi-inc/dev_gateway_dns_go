package modules

import (
	"database/sql"
	"log"
	"time"

	"dev_gateway_dns/app/models"
)

// AccessLogger writes access log entries to the database.
type AccessLogger struct {
	db *sql.DB
}

// NewAccessLogger creates a new access logger.
func NewAccessLogger(db *sql.DB) *AccessLogger {
	return &AccessLogger{db: db}
}

// Log inserts an access log entry into the database.
func (al *AccessLogger) Log(entry models.AccessLog) {
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now().UTC()
	}
	_, err := al.db.Exec(
		`INSERT INTO access_logs (timestamp, source, client_ip, hostname, method, path, status_code, response_time_ms, backend)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		entry.Timestamp, entry.Source, entry.ClientIP, entry.Hostname,
		entry.Method, entry.Path, entry.StatusCode, entry.ResponseTimeMs, entry.Backend,
	)
	if err != nil {
		log.Printf("Failed to log access: %v", err)
	}
}
