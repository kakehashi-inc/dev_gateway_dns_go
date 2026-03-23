package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"dev_gateway_dns/app/cert"
	"dev_gateway_dns/app/dns"
	"dev_gateway_dns/app/models"
	"dev_gateway_dns/app/modules"
	"dev_gateway_dns/app/proxy"
	"dev_gateway_dns/app/status"

	"github.com/skip2/go-qrcode"
	"nhooyr.io/websocket"
	"nhooyr.io/websocket/wsjson"
)

// Server is the REST API and WebSocket server for the admin UI.
type Server struct {
	db           *sql.DB
	config       *modules.AppConfig
	reverseProxy *proxy.ReverseProxy
	forwardProxy *proxy.ForwardProxy
	certManager  *cert.Manager
	autoRecords  *dns.AutoRecordMap
	queryLog     *dns.RingBuffer
	httpServer   *http.Server
	frontendFS   fs.FS
	startedAt    time.Time
	version      string
	ctx          context.Context
	cancel       context.CancelFunc
}

// NewServer creates a new API server.
func NewServer(
	db *sql.DB,
	config *modules.AppConfig,
	rp *proxy.ReverseProxy,
	fp *proxy.ForwardProxy,
	cm *cert.Manager,
	autoRecords *dns.AutoRecordMap,
	queryLog *dns.RingBuffer,
	version string,
	frontendFS fs.FS,
) *Server {
	ctx, cancel := context.WithCancel(context.Background())
	return &Server{
		db:           db,
		config:       config,
		reverseProxy: rp,
		forwardProxy: fp,
		certManager:  cm,
		autoRecords:  autoRecords,
		queryLog:     queryLog,
		startedAt:    time.Now(),
		version:      version,
		frontendFS:   frontendFS,
		ctx:          ctx,
		cancel:       cancel,
	}
}

// Start begins serving the admin API.
func (s *Server) Start() error {
	mux := http.NewServeMux()

	// Proxy rules
	mux.HandleFunc("/api/v1/proxy/rules", s.handleProxyRules)
	mux.HandleFunc("/api/v1/proxy/rules/", s.handleProxyRule)

	// DNS records
	mux.HandleFunc("/api/v1/dns/records", s.handleDNSRecords)
	mux.HandleFunc("/api/v1/dns/records/", s.handleDNSRecord)
	mux.HandleFunc("/api/v1/dns/upstream", s.handleDNSUpstream)

	// Certificates
	mux.HandleFunc("/api/v1/certs", s.handleCertsList)
	mux.HandleFunc("/api/v1/certs/ca/download", s.handleCADownload)
	mux.HandleFunc("/api/v1/certs/ca/qrcode", s.handleCAQRCode)
	mux.HandleFunc("/api/v1/certs/", s.handleCertAction)

	// CA direct distribution
	mux.HandleFunc("/ca", s.handleCADirect)

	// Status
	mux.HandleFunc("/api/v1/status/overview", s.handleStatusOverview)
	mux.HandleFunc("/api/v1/status/interfaces", s.handleStatusInterfaces)
	mux.HandleFunc("/api/v1/status/health", s.handleStatusHealth)
	mux.HandleFunc("/api/v1/status/live", s.handleStatusLive)
	mux.HandleFunc("/api/v1/dns/queries/live", s.handleDNSQueriesLive)

	// Settings
	mux.HandleFunc("/api/v1/settings", s.handleSettings)

	// Frontend static files
	if s.frontendFS != nil {
		fileServer := http.FileServer(http.FS(s.frontendFS))
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			// Serve API and /ca routes normally (already handled above)
			// For SPA: try serving the file, fall back to index.html
			path := r.URL.Path
			if path == "/" || path == "" {
				r.URL.Path = "/index.html"
				fileServer.ServeHTTP(w, r)
				return
			}
			// Try to open the file
			f, err := s.frontendFS.Open(strings.TrimPrefix(path, "/"))
			if err != nil {
				// File not found: serve index.html for SPA routing
				r.URL.Path = "/index.html"
				fileServer.ServeHTTP(w, r)
				return
			}
			f.Close()
			fileServer.ServeHTTP(w, r)
		})
	}

	addr := fmt.Sprintf("%s:%d", "0.0.0.0", s.config.AdminPort)
	s.httpServer = &http.Server{
		Addr:    addr,
		Handler: corsMiddleware(mux),
	}

	go func() {
		log.Printf("Admin API listening on %s", addr)
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("Admin API error: %v", err)
			s.httpServer = nil
		}
	}()

	return nil
}

// Stop shuts down the admin API server.
func (s *Server) Stop() {
	s.cancel()
	if s.httpServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		s.httpServer.Shutdown(ctx)
	}
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// --- Proxy Rules ---

func (s *Server) handleProxyRules(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.listProxyRules(w, r)
	case http.MethodPost:
		s.createProxyRule(w, r)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) listProxyRules(w http.ResponseWriter, _ *http.Request) {
	rows, err := s.db.Query(
		"SELECT id, hostname, backend_protocol, backend_ip, backend_port, enabled, created_at, updated_at FROM proxy_rules ORDER BY id",
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	var rules []models.ProxyRule
	for rows.Next() {
		var rule models.ProxyRule
		if err := rows.Scan(&rule.ID, &rule.Hostname, &rule.BackendProtocol, &rule.BackendIP,
			&rule.BackendPort, &rule.Enabled, &rule.CreatedAt, &rule.UpdatedAt); err != nil {
			continue
		}
		rules = append(rules, rule)
	}
	if rules == nil {
		rules = []models.ProxyRule{}
	}
	writeJSON(w, http.StatusOK, rules)
}

func (s *Server) createProxyRule(w http.ResponseWriter, r *http.Request) {
	var rule models.ProxyRule
	if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	now := time.Now().UTC()
	result, err := s.db.Exec(
		`INSERT INTO proxy_rules (hostname, backend_protocol, backend_ip, backend_port, enabled, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		rule.Hostname, rule.BackendProtocol, rule.BackendIP, rule.BackendPort, rule.Enabled, now, now,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	id, _ := result.LastInsertId()
	rule.ID = id
	rule.CreatedAt = now
	rule.UpdatedAt = now

	s.reverseProxy.UpdateRule(&rule)
	s.forwardProxy.UpdateRule(&rule)
	s.syncAutoRecords()

	// Generate certificate for the hostname
	if rule.Enabled {
		go s.certManager.GetCertificate(rule.Hostname)
	}

	writeJSON(w, http.StatusCreated, rule)
}

func (s *Server) handleProxyRule(w http.ResponseWriter, r *http.Request) {
	idStr := strings.TrimPrefix(r.URL.Path, "/api/v1/proxy/rules/")

	// Check for toggle action
	if strings.HasSuffix(idStr, "/toggle") {
		idStr = strings.TrimSuffix(idStr, "/toggle")
		if r.Method == http.MethodPatch {
			s.toggleProxyRule(w, idStr)
			return
		}
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid ID")
		return
	}

	switch r.Method {
	case http.MethodPut:
		s.updateProxyRule(w, r, id)
	case http.MethodDelete:
		s.deleteProxyRule(w, id)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) updateProxyRule(w http.ResponseWriter, r *http.Request, id int64) {
	var rule models.ProxyRule
	if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	// Get old hostname for auto record cleanup
	var oldHostname string
	s.db.QueryRow("SELECT hostname FROM proxy_rules WHERE id = ?", id).Scan(&oldHostname)

	now := time.Now().UTC()
	_, err := s.db.Exec(
		`UPDATE proxy_rules SET hostname=?, backend_protocol=?, backend_ip=?, backend_port=?, enabled=?, updated_at=?
		 WHERE id=?`,
		rule.Hostname, rule.BackendProtocol, rule.BackendIP, rule.BackendPort, rule.Enabled, now, id,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	rule.ID = id
	rule.UpdatedAt = now

	if oldHostname != rule.Hostname {
		s.reverseProxy.RemoveRule(oldHostname)
		s.forwardProxy.RemoveRule(oldHostname)
		s.autoRecords.Delete(oldHostname)
	}
	s.reverseProxy.UpdateRule(&rule)
	s.forwardProxy.UpdateRule(&rule)
	s.syncAutoRecords()

	writeJSON(w, http.StatusOK, rule)
}

func (s *Server) deleteProxyRule(w http.ResponseWriter, id int64) {
	var hostname string
	s.db.QueryRow("SELECT hostname FROM proxy_rules WHERE id = ?", id).Scan(&hostname)

	_, err := s.db.Exec("DELETE FROM proxy_rules WHERE id = ?", id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.reverseProxy.RemoveRule(hostname)
	s.forwardProxy.RemoveRule(hostname)
	s.autoRecords.Delete(hostname)

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (s *Server) toggleProxyRule(w http.ResponseWriter, idStr string) {
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid ID")
		return
	}

	var enabled bool
	if err := s.db.QueryRow("SELECT enabled FROM proxy_rules WHERE id = ?", id).Scan(&enabled); err != nil {
		writeError(w, http.StatusNotFound, "rule not found")
		return
	}

	newEnabled := !enabled
	now := time.Now().UTC()
	s.db.Exec("UPDATE proxy_rules SET enabled=?, updated_at=? WHERE id=?", newEnabled, now, id)

	var rule models.ProxyRule
	s.db.QueryRow(
		"SELECT id, hostname, backend_protocol, backend_ip, backend_port, enabled, created_at, updated_at FROM proxy_rules WHERE id = ?", id,
	).Scan(&rule.ID, &rule.Hostname, &rule.BackendProtocol, &rule.BackendIP, &rule.BackendPort, &rule.Enabled, &rule.CreatedAt, &rule.UpdatedAt)

	s.reverseProxy.UpdateRule(&rule)
	s.forwardProxy.UpdateRule(&rule)
	s.syncAutoRecords()

	writeJSON(w, http.StatusOK, rule)
}

// --- DNS Records ---

func (s *Server) handleDNSRecords(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.listDNSRecords(w)
	case http.MethodPost:
		s.createDNSRecord(w, r)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) listDNSRecords(w http.ResponseWriter) {
	var records []models.DNSRecordView

	// Manual records from DB
	rows, err := s.db.Query("SELECT id, name, type, value, ttl, priority, weight, port FROM dns_records ORDER BY name")
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	for rows.Next() {
		var rec models.DNSRecordView
		var id int64
		if err := rows.Scan(&id, &rec.Name, &rec.Type, &rec.Value, &rec.TTL, &rec.Priority, &rec.Weight, &rec.Port); err != nil {
			continue
		}
		rec.ID = &id
		rec.Locked = false
		rec.Source = "manual"
		records = append(records, rec)
	}

	// Auto records from memory
	for hostname, ips := range s.autoRecords.All() {
		for _, ip := range ips {
			records = append(records, models.DNSRecordView{
				Name:   hostname,
				Type:   "A",
				Value:  ip,
				TTL:    60,
				Locked: true,
				Source: "proxy",
			})
		}
	}

	if records == nil {
		records = []models.DNSRecordView{}
	}
	writeJSON(w, http.StatusOK, records)
}

// validRecordTypes defines the DNS record types supported by the DNS server.
var validRecordTypes = map[string]bool{
	"A": true, "AAAA": true, "CNAME": true, "MX": true, "TXT": true,
	"SRV": true, "NS": true, "PTR": true, "CAA": true, "SOA": true,
	"NAPTR": true, "SSHFP": true, "TLSA": true, "DS": true, "DNSKEY": true,
}

func (s *Server) createDNSRecord(w http.ResponseWriter, r *http.Request) {
	var rec models.DNSRecord
	if err := json.NewDecoder(r.Body).Decode(&rec); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if !validRecordTypes[rec.Type] {
		writeError(w, http.StatusBadRequest, "unsupported record type")
		return
	}
	if rec.TTL == 0 {
		rec.TTL = 300
	}

	now := time.Now().UTC()
	result, err := s.db.Exec(
		`INSERT INTO dns_records (name, type, value, ttl, priority, weight, port, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		rec.Name, rec.Type, rec.Value, rec.TTL, rec.Priority, rec.Weight, rec.Port, now, now,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	rec.ID, _ = result.LastInsertId()
	rec.CreatedAt = now
	rec.UpdatedAt = now
	writeJSON(w, http.StatusCreated, rec)
}

func (s *Server) handleDNSRecord(w http.ResponseWriter, r *http.Request) {
	idStr := strings.TrimPrefix(r.URL.Path, "/api/v1/dns/records/")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid ID")
		return
	}

	switch r.Method {
	case http.MethodPut:
		s.updateDNSRecord(w, r, id)
	case http.MethodDelete:
		s.deleteDNSRecord(w, id)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) updateDNSRecord(w http.ResponseWriter, r *http.Request, id int64) {
	var rec models.DNSRecord
	if err := json.NewDecoder(r.Body).Decode(&rec); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if !validRecordTypes[rec.Type] {
		writeError(w, http.StatusBadRequest, "unsupported record type")
		return
	}

	now := time.Now().UTC()
	_, err := s.db.Exec(
		`UPDATE dns_records SET name=?, type=?, value=?, ttl=?, priority=?, weight=?, port=?, updated_at=? WHERE id=?`,
		rec.Name, rec.Type, rec.Value, rec.TTL, rec.Priority, rec.Weight, rec.Port, now, id,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	rec.ID = id
	rec.UpdatedAt = now
	writeJSON(w, http.StatusOK, rec)
}

func (s *Server) deleteDNSRecord(w http.ResponseWriter, id int64) {
	_, err := s.db.Exec("DELETE FROM dns_records WHERE id = ?", id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// --- DNS Upstream ---

func (s *Server) handleDNSUpstream(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, map[string][]string{"upstream_dns_fallback": s.config.UpstreamDNSFallback})
	case http.MethodPut:
		var body struct {
			UpstreamDNSFallback []string `json:"upstream_dns_fallback"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON")
			return
		}
		s.config.UpstreamDNSFallback = body.UpstreamDNSFallback
		data, _ := json.Marshal(body.UpstreamDNSFallback)
		modules.SaveSettingToDB(s.db, "upstream_dns_fallback", string(data))
		writeJSON(w, http.StatusOK, body)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// --- Certificates ---

func (s *Server) handleCertsList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	certs, err := s.certManager.ListCertificates()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if certs == nil {
		certs = []cert.CertInfo{}
	}
	writeJSON(w, http.StatusOK, certs)
}

func (s *Server) handleCADownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	format := r.URL.Query().Get("format")
	if format == "" {
		format = "pem"
	}

	switch format {
	case "pem":
		data, err := s.certManager.GetCACertPEM()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		w.Header().Set("Content-Type", "application/x-pem-file")
		w.Header().Set("Content-Disposition", "attachment; filename=DevGatewayDNS_CA.pem")
		w.Write(data)
	case "der":
		data, err := s.certManager.GetCACertDER()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		w.Header().Set("Content-Type", "application/x-x509-ca-cert")
		w.Header().Set("Content-Disposition", "attachment; filename=DevGatewayDNS_CA.crt")
		w.Write(data)
	case "p12", "pkcs12":
		data, err := s.certManager.GetCACertP12()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		w.Header().Set("Content-Type", "application/x-pkcs12")
		w.Header().Set("Content-Disposition", "attachment; filename=DevGatewayDNS_CA.p12")
		w.Write(data)
	default:
		writeError(w, http.StatusBadRequest, "unsupported format (use pem, der, or p12)")
	}
}

func (s *Server) handleCAQRCode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	host := r.Host
	caURL := fmt.Sprintf("http://%s/ca", host)
	png, err := qrcode.Encode(caURL, qrcode.Medium, 256)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate QR code")
		return
	}

	w.Header().Set("Content-Type", "image/png")
	w.Write(png)
}

func (s *Server) handleCertAction(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/certs/")
	if strings.HasSuffix(path, "/regenerate") && r.Method == http.MethodPost {
		hostname := strings.TrimSuffix(path, "/regenerate")
		_, err := s.certManager.RegenerateHostCert(hostname)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "regenerated", "hostname": hostname})
		return
	}
	writeError(w, http.StatusNotFound, "not found")
}

func (s *Server) handleCADirect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	data, err := s.certManager.GetCACertDER()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/x-x509-ca-cert")
	w.Header().Set("Content-Disposition", "attachment; filename=DevGatewayDNS_CA.crt")
	w.Write(data)
}

// --- Status ---

func (s *Server) handleStatusOverview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var activeRules int
	s.db.QueryRow("SELECT COUNT(*) FROM proxy_rules WHERE enabled = 1").Scan(&activeRules)

	overview := status.OverviewStatus{
		Version:     s.version,
		Uptime:      time.Since(s.startedAt).Truncate(time.Second).String(),
		StartedAt:   s.startedAt,
		ActiveRules: activeRules,
	}
	writeJSON(w, http.StatusOK, overview)
}

func (s *Server) handleStatusInterfaces(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	nics, err := modules.DetectNICs()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, nics)
}

func (s *Server) handleStatusHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	results := status.RunHealthChecks(
		s.config.HTTPPort, s.config.HTTPSPort, s.config.DNSPort,
		s.config.ProxyPort, s.config.AdminPort,
	)
	writeJSON(w, http.StatusOK, results)
}

func (s *Server) handleStatusLive(w http.ResponseWriter, r *http.Request) {
	c, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: []string{"*"},
	})
	if err != nil {
		return
	}
	defer c.Close(websocket.StatusNormalClosure, "")

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-r.Context().Done():
			return
		case <-ticker.C:
			rows, err := s.db.Query(
				"SELECT id, timestamp, source, client_ip, hostname, method, path, status_code, response_time_ms, backend FROM access_logs ORDER BY id DESC LIMIT 50",
			)
			if err != nil {
				continue
			}
			var logs []models.AccessLog
			for rows.Next() {
				var l models.AccessLog
				if err := rows.Scan(&l.ID, &l.Timestamp, &l.Source, &l.ClientIP, &l.Hostname, &l.Method, &l.Path, &l.StatusCode, &l.ResponseTimeMs, &l.Backend); err != nil {
					continue
				}
				logs = append(logs, l)
			}
			rows.Close()
			wsjson.Write(s.ctx, c, logs)
		}
	}
}

func (s *Server) handleDNSQueriesLive(w http.ResponseWriter, r *http.Request) {
	c, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: []string{"*"},
	})
	if err != nil {
		return
	}
	defer c.Close(websocket.StatusNormalClosure, "")

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-r.Context().Done():
			return
		case <-ticker.C:
			entries := s.queryLog.Entries()
			wsjson.Write(s.ctx, c, entries)
		}
	}
}

// --- Settings ---

func (s *Server) handleSettings(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		cfg, err := modules.LoadConfigFromDB(s.db)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, cfg)
	case http.MethodPut:
		var cfg modules.AppConfig
		if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON")
			return
		}
		if err := modules.SaveConfigToDB(s.db, &cfg); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "updated", "note": "restart required for port changes"})
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// syncAutoRecords rebuilds auto records from DB.
func (s *Server) syncAutoRecords() {
	ips, err := modules.ResolveListenIPs(s.config.ListenAddresses)
	if err != nil {
		log.Printf("Failed to resolve listen IPs: %v", err)
		return
	}

	rows, err := s.db.Query("SELECT hostname FROM proxy_rules WHERE enabled = 1")
	if err != nil {
		return
	}
	defer rows.Close()

	// Build new set of hostnames
	active := make(map[string]bool)
	for rows.Next() {
		var hostname string
		if err := rows.Scan(&hostname); err != nil {
			continue
		}
		active[hostname] = true
		s.autoRecords.Set(hostname, ips)
	}

	// Remove hostnames no longer active
	for hostname := range s.autoRecords.All() {
		if !active[hostname] {
			s.autoRecords.Delete(hostname)
		}
	}
}
