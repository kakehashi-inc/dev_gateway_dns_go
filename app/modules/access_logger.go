package modules

import (
	"database/sql"
	"fmt"
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

// Cleanup deletes access logs older than the retention period.
// retentionDays is the number of days to keep (e.g., 7 means keep today and 6 days before).
func (al *AccessLogger) Cleanup(retentionDays int) error {
	cutoff := time.Now().UTC().Truncate(24*time.Hour).AddDate(0, 0, -(retentionDays - 1))
	result, err := al.db.Exec("DELETE FROM access_logs WHERE timestamp < ?", cutoff)
	if err != nil {
		return fmt.Errorf("failed to delete old access logs: %w", err)
	}
	deleted, _ := result.RowsAffected()
	if deleted > 0 {
		log.Printf("Deleted %d old access log entries (before %s)", deleted, cutoff.Format("2006-01-02"))
	}
	return nil
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
