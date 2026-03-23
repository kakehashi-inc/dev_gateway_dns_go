-- +goose Up

INSERT OR IGNORE INTO settings (key, value, updated_at) VALUES ('access_log_retention_days', '7', datetime('now'));

-- +goose Down

DELETE FROM settings WHERE key = 'access_log_retention_days';
