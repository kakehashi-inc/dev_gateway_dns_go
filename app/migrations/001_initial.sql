-- +goose Up

CREATE TABLE IF NOT EXISTS proxy_rules (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    hostname TEXT NOT NULL UNIQUE,
    backend_protocol TEXT NOT NULL DEFAULT 'http',
    backend_ip TEXT,
    backend_port INTEGER NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT 1,
    created_at DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS dns_records (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    type TEXT NOT NULL,
    value TEXT NOT NULL,
    ttl INTEGER NOT NULL DEFAULT 300,
    priority INTEGER,
    weight INTEGER,
    port INTEGER,
    created_at DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at DATETIME NOT NULL DEFAULT (datetime('now'))
);

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

CREATE TABLE IF NOT EXISTS settings (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL,
    updated_at DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS access_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    timestamp DATETIME NOT NULL,
    source TEXT NOT NULL,
    client_ip TEXT NOT NULL,
    hostname TEXT NOT NULL,
    method TEXT NOT NULL,
    path TEXT NOT NULL,
    status_code INTEGER NOT NULL,
    response_time_ms INTEGER NOT NULL,
    backend TEXT NOT NULL
);

-- Insert default settings
INSERT OR IGNORE INTO settings (key, value, updated_at) VALUES ('http_port', '80', datetime('now'));
INSERT OR IGNORE INTO settings (key, value, updated_at) VALUES ('https_port', '443', datetime('now'));
INSERT OR IGNORE INTO settings (key, value, updated_at) VALUES ('dns_port', '53', datetime('now'));
INSERT OR IGNORE INTO settings (key, value, updated_at) VALUES ('proxy_port', '8888', datetime('now'));
INSERT OR IGNORE INTO settings (key, value, updated_at) VALUES ('admin_port', '9090', datetime('now'));
INSERT OR IGNORE INTO settings (key, value, updated_at) VALUES ('listen_addresses', '["0.0.0.0"]', datetime('now'));
INSERT OR IGNORE INTO settings (key, value, updated_at) VALUES ('upstream_dns_fallback', '["8.8.8.8","1.1.1.1"]', datetime('now'));
INSERT OR IGNORE INTO settings (key, value, updated_at) VALUES ('dns_query_history_size', '1000', datetime('now'));
INSERT OR IGNORE INTO settings (key, value, updated_at) VALUES ('log_level', '"info"', datetime('now'));

-- +goose Down

DROP TABLE IF EXISTS access_logs;
DROP TABLE IF EXISTS settings;
DROP TABLE IF EXISTS host_certificates;
DROP TABLE IF EXISTS ca_certificate;
DROP TABLE IF EXISTS dns_records;
DROP TABLE IF EXISTS proxy_rules;
