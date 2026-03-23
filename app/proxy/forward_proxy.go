package proxy

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"
	"time"

	"dev_gateway_dns/app/models"
)

// ForwardProxy handles HTTP forward proxy for clients that cannot change DNS settings.
type ForwardProxy struct {
	mu            sync.RWMutex
	rules         map[string]*models.ProxyRule
	servers       []*http.Server
	listenAddrs   []string
	port          int
	getCert       func(hostname string) (*tls.Certificate, error)
	logAccess     func(entry models.AccessLog)
	resolveAutoIP func() string
}

// NewForwardProxy creates a new forward proxy.
func NewForwardProxy(listenAddrs []string, port int,
	getCert func(string) (*tls.Certificate, error),
	logAccess func(models.AccessLog),
	resolveAutoIP func() string,
) *ForwardProxy {
	return &ForwardProxy{
		rules:         make(map[string]*models.ProxyRule),
		listenAddrs:   listenAddrs,
		port:          port,
		getCert:       getCert,
		logAccess:     logAccess,
		resolveAutoIP: resolveAutoIP,
	}
}

// SetRules replaces the proxy rules.
func (fp *ForwardProxy) SetRules(rules map[string]*models.ProxyRule) {
	fp.mu.Lock()
	defer fp.mu.Unlock()
	fp.rules = rules
}

// UpdateRule updates a single rule.
func (fp *ForwardProxy) UpdateRule(rule *models.ProxyRule) {
	fp.mu.Lock()
	defer fp.mu.Unlock()
	if rule.Enabled {
		fp.rules[rule.Hostname] = rule
	} else {
		delete(fp.rules, rule.Hostname)
	}
}

// RemoveRule removes a rule by hostname.
func (fp *ForwardProxy) RemoveRule(hostname string) {
	fp.mu.Lock()
	defer fp.mu.Unlock()
	delete(fp.rules, hostname)
}

// Start begins listening for forward proxy connections on each listen address.
func (fp *ForwardProxy) Start() error {
	for _, ip := range fp.listenAddrs {
		addr := fmt.Sprintf("%s:%d", ip, fp.port)
		srv := &http.Server{
			Addr:    addr,
			Handler: fp,
		}
		fp.servers = append(fp.servers, srv)

		go func(a string, s *http.Server) {
			log.Printf("Forward proxy listening on %s", a)
			if err := s.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Printf("Forward proxy error (%s): %v", a, err)
			}
		}(addr, srv)
	}
	return nil
}

// Stop shuts down all forward proxy instances.
func (fp *ForwardProxy) Stop() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for _, srv := range fp.servers {
		srv.Shutdown(ctx)
	}
}

// ServeHTTP handles both HTTP requests and HTTPS CONNECT tunnels.
func (fp *ForwardProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodConnect {
		fp.handleConnect(w, r)
		return
	}
	fp.handleHTTP(w, r)
}

func (fp *ForwardProxy) handleHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	hostname := extractHostname(r.Host)

	fp.mu.RLock()
	rule, matched := fp.rules[hostname]
	fp.mu.RUnlock()

	if matched && rule.Enabled {
		backendIP := fp.resolveAutoIP()
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
			req.Header.Set("X-Forwarded-Proto", "http")
		}
		proxy.ModifyResponse = func(resp *http.Response) error {
			rewriteLocationHeader(resp, hostname)
			rewriteCookies(resp, hostname)
			return nil
		}

		recorder := &statusRecorder{ResponseWriter: w, statusCode: http.StatusOK}
		proxy.ServeHTTP(recorder, r)

		if fp.logAccess != nil {
			fp.logAccess(models.AccessLog{
				Timestamp:      time.Now().UTC(),
				Source:         "forward",
				ClientIP:       extractIP(r.RemoteAddr),
				Hostname:       hostname,
				Method:         r.Method,
				Path:           r.URL.Path,
				StatusCode:     recorder.statusCode,
				ResponseTimeMs: int(time.Since(start).Milliseconds()),
				Backend:        fmt.Sprintf("%s:%d", backendIP, rule.BackendPort),
			})
		}
		return
	}

	// Not matched: forward to external
	resp, err := http.DefaultTransport.RoundTrip(r)
	if err != nil {
		http.Error(w, "Forward proxy error", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	copyHeaders(w.Header(), resp.Header)
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

func (fp *ForwardProxy) handleConnect(w http.ResponseWriter, r *http.Request) {
	hostname := extractHostname(r.Host)

	fp.mu.RLock()
	rule, matched := fp.rules[hostname]
	fp.mu.RUnlock()

	if matched && rule.Enabled {
		fp.handleConnectWithIntercept(w, r, rule)
		return
	}

	// Tunnel through to external
	targetConn, err := net.DialTimeout("tcp", r.Host, 10*time.Second)
	if err != nil {
		http.Error(w, "Could not connect to target", http.StatusBadGateway)
		return
	}

	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "Hijacking not supported", http.StatusInternalServerError)
		targetConn.Close()
		return
	}

	w.WriteHeader(http.StatusOK)
	clientConn, _, err := hijacker.Hijack()
	if err != nil {
		targetConn.Close()
		return
	}

	go transfer(targetConn, clientConn)
	go transfer(clientConn, targetConn)
}

func (fp *ForwardProxy) handleConnectWithIntercept(w http.ResponseWriter, r *http.Request, rule *models.ProxyRule) {
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "Hijacking not supported", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	clientConn, _, err := hijacker.Hijack()
	if err != nil {
		return
	}

	cert, err := fp.getCert(rule.Hostname)
	if err != nil {
		clientConn.Close()
		return
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{*cert},
	}
	tlsConn := tls.Server(clientConn, tlsConfig)
	if err := tlsConn.Handshake(); err != nil {
		tlsConn.Close()
		return
	}

	backendIP := fp.resolveAutoIP()
	if rule.BackendIP != nil {
		backendIP = *rule.BackendIP
	}
	backendURL := fmt.Sprintf("%s://%s:%d", rule.BackendProtocol, backendIP, rule.BackendPort)
	target, err := url.Parse(backendURL)
	if err != nil {
		tlsConn.Close()
		return
	}

	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.Transport = &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // backend may use self-signed certs
	}

	srv := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			req.Host = rule.Hostname
			req.Header.Set("X-Forwarded-For", r.RemoteAddr)
			req.Header.Set("X-Forwarded-Host", rule.Hostname)
			req.Header.Set("X-Forwarded-Proto", "https")
			proxy.ServeHTTP(w, req)
		}),
		TLSConfig: tlsConfig,
	}

	conn := &singleConnListener{conn: tlsConn}
	srv.Serve(conn)
}

func transfer(dst io.WriteCloser, src io.ReadCloser) {
	defer dst.Close()
	defer src.Close()
	io.Copy(dst, src)
}

func copyHeaders(dst, src http.Header) {
	for k, vv := range src {
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}

// singleConnListener wraps a single net.Conn as a net.Listener for http.Server.Serve.
type singleConnListener struct {
	conn net.Conn
	once sync.Once
}

func (l *singleConnListener) Accept() (net.Conn, error) {
	var c net.Conn
	l.once.Do(func() { c = l.conn })
	if c != nil {
		return c, nil
	}
	return nil, fmt.Errorf("listener closed")
}

func (l *singleConnListener) Close() error   { return nil }
func (l *singleConnListener) Addr() net.Addr { return l.conn.LocalAddr() }
