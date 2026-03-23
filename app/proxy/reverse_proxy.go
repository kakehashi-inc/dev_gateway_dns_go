package proxy

import (
	"crypto/tls"
	"database/sql"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"time"

	"dev_gateway_dns/app/models"
)

// ReverseProxy manages HTTP/HTTPS reverse proxy routing.
type ReverseProxy struct {
	db            *sql.DB
	mu            sync.RWMutex
	rules         map[string]*models.ProxyRule // hostname -> rule
	httpServer    *http.Server
	httpsServer   *http.Server
	listenAddr    string
	httpPort      int
	httpsPort     int
	getCert       func(hostname string) (*tls.Certificate, error)
	logAccess     func(entry models.AccessLog)
	resolveAutoIP func() string
}

// NewReverseProxy creates a new ReverseProxy.
func NewReverseProxy(db *sql.DB, listenAddr string, httpPort, httpsPort int,
	getCert func(string) (*tls.Certificate, error),
	logAccess func(models.AccessLog),
	resolveAutoIP func() string,
) *ReverseProxy {
	return &ReverseProxy{
		db:            db,
		rules:         make(map[string]*models.ProxyRule),
		listenAddr:    listenAddr,
		httpPort:      httpPort,
		httpsPort:     httpsPort,
		getCert:       getCert,
		logAccess:     logAccess,
		resolveAutoIP: resolveAutoIP,
	}
}

// LoadRules loads enabled proxy rules from the database.
func (rp *ReverseProxy) LoadRules() error {
	rows, err := rp.db.Query(
		"SELECT id, hostname, backend_protocol, backend_ip, backend_port, enabled, created_at, updated_at FROM proxy_rules",
	)
	if err != nil {
		return fmt.Errorf("failed to load proxy rules: %w", err)
	}
	defer rows.Close()

	rp.mu.Lock()
	defer rp.mu.Unlock()
	rp.rules = make(map[string]*models.ProxyRule)

	for rows.Next() {
		var rule models.ProxyRule
		if err := rows.Scan(&rule.ID, &rule.Hostname, &rule.BackendProtocol, &rule.BackendIP,
			&rule.BackendPort, &rule.Enabled, &rule.CreatedAt, &rule.UpdatedAt); err != nil {
			continue
		}
		rp.rules[rule.Hostname] = &rule
	}
	return nil
}

// UpdateRule updates a rule in the in-memory map.
func (rp *ReverseProxy) UpdateRule(rule *models.ProxyRule) {
	rp.mu.Lock()
	defer rp.mu.Unlock()
	if rule.Enabled {
		rp.rules[rule.Hostname] = rule
	} else {
		delete(rp.rules, rule.Hostname)
	}
}

// RemoveRule removes a rule from the in-memory map.
func (rp *ReverseProxy) RemoveRule(hostname string) {
	rp.mu.Lock()
	defer rp.mu.Unlock()
	delete(rp.rules, hostname)
}

// Start starts the HTTP and HTTPS proxy servers.
func (rp *ReverseProxy) Start() error {
	httpAddr := fmt.Sprintf("%s:%d", rp.listenAddr, rp.httpPort)
	httpsAddr := fmt.Sprintf("%s:%d", rp.listenAddr, rp.httpsPort)

	rp.httpServer = &http.Server{
		Addr:    httpAddr,
		Handler: rp.proxyHandler("reverse"),
	}

	rp.httpsServer = &http.Server{
		Addr:    httpsAddr,
		Handler: rp.proxyHandler("reverse"),
		TLSConfig: &tls.Config{
			GetCertificate: func(info *tls.ClientHelloInfo) (*tls.Certificate, error) {
				return rp.getCert(info.ServerName)
			},
		},
	}

	go func() {
		log.Printf("HTTP reverse proxy listening on %s", httpAddr)
		if err := rp.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("HTTP proxy error: %v", err)
		}
	}()

	go func() {
		ln, err := tls.Listen("tcp", httpsAddr, rp.httpsServer.TLSConfig)
		if err != nil {
			log.Printf("HTTPS proxy listen error: %v", err)
			return
		}
		log.Printf("HTTPS reverse proxy listening on %s", httpsAddr)
		if err := rp.httpsServer.Serve(ln); err != nil && err != http.ErrServerClosed {
			log.Printf("HTTPS proxy error: %v", err)
		}
	}()

	return nil
}

// Stop shuts down both proxy servers.
func (rp *ReverseProxy) Stop() {
	if rp.httpServer != nil {
		rp.httpServer.Close()
	}
	if rp.httpsServer != nil {
		rp.httpsServer.Close()
	}
}

func (rp *ReverseProxy) proxyHandler(source string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		hostname := extractHostname(r.Host)

		rp.mu.RLock()
		rule, ok := rp.rules[hostname]
		rp.mu.RUnlock()

		if !ok || !rule.Enabled {
			http.Error(w, "No proxy rule for this host", http.StatusBadGateway)
			return
		}

		backendIP := rp.resolveAutoIP()
		if rule.BackendIP != nil {
			backendIP = *rule.BackendIP
		}
		backendURL := fmt.Sprintf("%s://%s:%d", rule.BackendProtocol, backendIP, rule.BackendPort)

		target, err := url.Parse(backendURL)
		if err != nil {
			http.Error(w, "Invalid backend URL", http.StatusInternalServerError)
			return
		}

		proxy := httputil.NewSingleHostReverseProxy(target)
		proxy.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // backend may use self-signed certs
		}

		originalDirector := proxy.Director
		proxy.Director = func(req *http.Request) {
			originalDirector(req)
			req.Host = hostname
			req.Header.Set("X-Forwarded-For", r.RemoteAddr)
			req.Header.Set("X-Forwarded-Host", hostname)
			if r.TLS != nil {
				req.Header.Set("X-Forwarded-Proto", "https")
			} else {
				req.Header.Set("X-Forwarded-Proto", "http")
			}
		}

		proxy.ModifyResponse = func(resp *http.Response) error {
			rewriteLocationHeader(resp, hostname)
			rewriteCookies(resp, hostname)
			return nil
		}

		recorder := &statusRecorder{ResponseWriter: w, statusCode: http.StatusOK}
		proxy.ServeHTTP(recorder, r)

		if rp.logAccess != nil {
			rp.logAccess(models.AccessLog{
				Timestamp:      time.Now(),
				Source:         source,
				ClientIP:       extractIP(r.RemoteAddr),
				Hostname:       hostname,
				Method:         r.Method,
				Path:           r.URL.Path,
				StatusCode:     recorder.statusCode,
				ResponseTimeMs: int(time.Since(start).Milliseconds()),
				Backend:        fmt.Sprintf("%s:%d", backendIP, rule.BackendPort),
			})
		}
	})
}

func extractHostname(host string) string {
	h, _, err := net.SplitHostPort(host)
	if err != nil {
		return host
	}
	return h
}

func extractIP(remoteAddr string) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return remoteAddr
	}
	return host
}

func rewriteLocationHeader(resp *http.Response, frontendHost string) {
	location := resp.Header.Get("Location")
	if location == "" {
		return
	}
	u, err := url.Parse(location)
	if err != nil {
		return
	}
	u.Host = frontendHost
	resp.Header.Set("Location", u.String())
}

func rewriteCookies(resp *http.Response, frontendHost string) {
	cookies := resp.Cookies()
	if len(cookies) == 0 {
		return
	}
	resp.Header.Del("Set-Cookie")
	for _, c := range cookies {
		if c.Domain != "" {
			c.Domain = frontendHost
			if idx := strings.Index(c.Domain, ":"); idx != -1 {
				c.Domain = c.Domain[:idx]
			}
		}
		if c.Path != "" {
			c.Path = "/" + strings.TrimLeft(c.Path, "/")
		}
		resp.Header.Add("Set-Cookie", c.String())
	}
}

// statusRecorder wraps http.ResponseWriter to capture the status code.
type statusRecorder struct {
	http.ResponseWriter
	statusCode int
}

func (sr *statusRecorder) WriteHeader(code int) {
	sr.statusCode = code
	sr.ResponseWriter.WriteHeader(code)
}
