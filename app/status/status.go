package status

import (
	"crypto/tls"
	"fmt"
	"net"
	"time"
)

// PortHealth represents the health check result for a single port.
type PortHealth struct {
	Service  string `json:"service"`
	Address  string `json:"address"`
	Port     int    `json:"port"`
	Protocol string `json:"protocol"`
	Bound    bool   `json:"bound"`
	Error    string `json:"error,omitempty"`
}

// OverviewStatus represents the system status summary.
type OverviewStatus struct {
	Version     string    `json:"version"`
	Uptime      string    `json:"uptime"`
	StartedAt   time.Time `json:"started_at"`
	ActiveRules int       `json:"active_rules"`
}

// CheckPortBind checks if a port is currently bound (listening).
func CheckPortBind(addr string, port int, protocol string) bool {
	target := net.JoinHostPort(addr, fmt.Sprintf("%d", port))
	switch protocol {
	case "tcp":
		conn, err := net.DialTimeout("tcp", target, 2*time.Second)
		if err != nil {
			return false
		}
		conn.Close()
		return true
	case "tls":
		conn, err := tls.DialWithDialer(
			&net.Dialer{Timeout: 2 * time.Second},
			"tcp", target,
			&tls.Config{InsecureSkipVerify: true, ServerName: "localhost"}, //nolint:gosec // health check against self-signed certs
		)
		if err != nil {
			return false
		}
		conn.Close()
		return true
	case "udp":
		conn, err := net.DialTimeout("udp", target, 2*time.Second)
		if err != nil {
			return false
		}
		conn.Close()
		return true
	}
	return false
}

// RunHealthChecks performs health checks on all service ports for each listen address.
func RunHealthChecks(listenAddrs []string, httpPort, httpsPort, dnsPort, proxyPort, adminPort int) []PortHealth {
	checks := []struct {
		service  string
		port     int
		protocol string
	}{
		{"HTTP Proxy", httpPort, "tcp"},
		{"HTTPS Proxy", httpsPort, "tls"},
		{"DNS (TCP)", dnsPort, "tcp"},
		{"DNS (UDP)", dnsPort, "udp"},
		{"Forward Proxy", proxyPort, "tcp"},
		{"Admin UI", adminPort, "tcp"},
	}

	var results []PortHealth
	for _, addr := range listenAddrs {
		for _, c := range checks {
			bound := CheckPortBind(addr, c.port, c.protocol)
			results = append(results, PortHealth{
				Service:  c.service,
				Address:  addr,
				Port:     c.port,
				Protocol: c.protocol,
				Bound:    bound,
			})
		}
	}
	return results
}
