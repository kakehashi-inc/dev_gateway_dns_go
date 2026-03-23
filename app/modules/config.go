package modules

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// DefaultSettings defines the default configuration values.
var DefaultSettings = map[string]string{
	"http_port":              "80",
	"https_port":             "443",
	"dns_port":               "53",
	"proxy_port":             "8888",
	"admin_port":             "9090",
	"listen_addresses":       `["0.0.0.0"]`,
	"upstream_dns_fallback":  `["8.8.8.8","1.1.1.1"]`,
	"dns_query_history_size": "1000",
	"log_level":              `"info"`,
}

// AppConfig holds the runtime configuration loaded from the database.
type AppConfig struct {
	HTTPPort            int      `json:"http_port"`
	HTTPSPort           int      `json:"https_port"`
	DNSPort             int      `json:"dns_port"`
	ProxyPort           int      `json:"proxy_port"`
	AdminPort           int      `json:"admin_port"`
	ListenAddresses     []string `json:"listen_addresses"`
	UpstreamDNSFallback []string `json:"upstream_dns_fallback"`
	DNSQueryHistorySize int      `json:"dns_query_history_size"`
	LogLevel            string   `json:"log_level"`
	DBPath              string   `json:"-"`
}

// NewDefaultConfig returns a config with default values.
func NewDefaultConfig() *AppConfig {
	return &AppConfig{
		HTTPPort:            80,
		HTTPSPort:           443,
		DNSPort:             53,
		ProxyPort:           8888,
		AdminPort:           9090,
		ListenAddresses:     []string{"0.0.0.0"},
		UpstreamDNSFallback: []string{"8.8.8.8", "1.1.1.1"},
		DNSQueryHistorySize: 1000,
		LogLevel:            "info",
	}
}

// LoadConfigFromDB loads settings from the database and returns an AppConfig.
func LoadConfigFromDB(db *sql.DB) (*AppConfig, error) {
	cfg := NewDefaultConfig()

	rows, err := db.Query("SELECT key, value FROM settings")
	if err != nil {
		return cfg, fmt.Errorf("failed to query settings: %w", err)
	}
	defer rows.Close()

	settings := make(map[string]string)
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			continue
		}
		settings[key] = value
	}

	if v, ok := settings["http_port"]; ok {
		fmt.Sscanf(v, "%d", &cfg.HTTPPort)
	}
	if v, ok := settings["https_port"]; ok {
		fmt.Sscanf(v, "%d", &cfg.HTTPSPort)
	}
	if v, ok := settings["dns_port"]; ok {
		fmt.Sscanf(v, "%d", &cfg.DNSPort)
	}
	if v, ok := settings["proxy_port"]; ok {
		fmt.Sscanf(v, "%d", &cfg.ProxyPort)
	}
	if v, ok := settings["admin_port"]; ok {
		fmt.Sscanf(v, "%d", &cfg.AdminPort)
	}
	if v, ok := settings["listen_addresses"]; ok {
		json.Unmarshal([]byte(v), &cfg.ListenAddresses)
	}
	if v, ok := settings["upstream_dns_fallback"]; ok {
		json.Unmarshal([]byte(v), &cfg.UpstreamDNSFallback)
	}
	if v, ok := settings["dns_query_history_size"]; ok {
		fmt.Sscanf(v, "%d", &cfg.DNSQueryHistorySize)
	}
	if v, ok := settings["log_level"]; ok {
		json.Unmarshal([]byte(v), &cfg.LogLevel)
	}

	return cfg, nil
}

// SaveSettingToDB saves a single setting key-value pair to the database.
func SaveSettingToDB(db *sql.DB, key, value string) error {
	_, err := db.Exec(
		`INSERT INTO settings (key, value, updated_at) VALUES (?, ?, ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`,
		key, value, time.Now().UTC(),
	)
	return err
}

// SaveConfigToDB persists the full AppConfig to the database.
func SaveConfigToDB(db *sql.DB, cfg *AppConfig) error {
	listenJSON, _ := json.Marshal(cfg.ListenAddresses)
	upstreamJSON, _ := json.Marshal(cfg.UpstreamDNSFallback)
	logLevelJSON, _ := json.Marshal(cfg.LogLevel)

	settings := map[string]string{
		"http_port":              fmt.Sprintf("%d", cfg.HTTPPort),
		"https_port":             fmt.Sprintf("%d", cfg.HTTPSPort),
		"dns_port":               fmt.Sprintf("%d", cfg.DNSPort),
		"proxy_port":             fmt.Sprintf("%d", cfg.ProxyPort),
		"admin_port":             fmt.Sprintf("%d", cfg.AdminPort),
		"listen_addresses":       string(listenJSON),
		"upstream_dns_fallback":  string(upstreamJSON),
		"dns_query_history_size": fmt.Sprintf("%d", cfg.DNSQueryHistorySize),
		"log_level":              string(logLevelJSON),
	}

	for k, v := range settings {
		if err := SaveSettingToDB(db, k, v); err != nil {
			return fmt.Errorf("failed to save setting %s: %w", k, err)
		}
	}
	return nil
}
