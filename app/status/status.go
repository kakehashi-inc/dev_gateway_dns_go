package status

import (
	"fmt"
	"net"
	"time"
)

// PortHealth represents the health check result for a single port.
type PortHealth struct {
	Service  string `json:"service"`
	Port     int    `json:"port"`
	Protocol string `json:"protocol"`
	Bound    bool   `json:"bound"`
	Loopback bool   `json:"loopback"`
	Error    string `json:"error,omitempty"`
}

// OverviewStatus represents the system status summary.
type OverviewStatus struct {
	Version     string       `json:"version"`
	Uptime      string       `json:"uptime"`
	StartedAt   time.Time    `json:"started_at"`
	PortHealth  []PortHealth `json:"port_health"`
	ActiveRules int          `json:"active_rules"`
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

// CheckLoopback performs a loopback connection test.
func CheckLoopback(port int, protocol string) bool {
	return CheckPortBind("127.0.0.1", port, protocol)
}

// RunHealthChecks performs health checks on all service ports.
func RunHealthChecks(httpPort, httpsPort, dnsPort, proxyPort, adminPort int) []PortHealth {
	checks := []struct {
		service  string
		port     int
		protocol string
	}{
		{"HTTP Proxy", httpPort, "tcp"},
		{"HTTPS Proxy", httpsPort, "tcp"},
		{"DNS (TCP)", dnsPort, "tcp"},
		{"DNS (UDP)", dnsPort, "udp"},
		{"Forward Proxy", proxyPort, "tcp"},
		{"Admin UI", adminPort, "tcp"},
	}

	results := make([]PortHealth, len(checks))
	for i, c := range checks {
		bound := CheckLoopback(c.port, c.protocol)
		results[i] = PortHealth{
			Service:  c.service,
			Port:     c.port,
			Protocol: c.protocol,
			Bound:    bound,
			Loopback: bound,
		}
	}
	return results
}
