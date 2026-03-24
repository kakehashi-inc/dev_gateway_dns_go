package status

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"time"

	mdns "codeberg.org/miekg/dns"
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

const healthCheckTimeout = 5 * time.Second

// checkHTTPProxy sends an HTTP request to /health and verifies the proxy responds with 200.
func checkHTTPProxy(addr string, port int) bool {
	client := &http.Client{Timeout: healthCheckTimeout}
	resp, err := client.Get(fmt.Sprintf("http://%s:%d/health", addr, port))
	if err != nil {
		return false
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// checkHTTPSProxy sends an HTTPS request to /health with certificate verification disabled.
func checkHTTPSProxy(addr string, httpsPort int) bool {
	client := &http.Client{
		Timeout: healthCheckTimeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true, //nolint:gosec // health check against self-signed certs
			},
		},
	}
	resp, err := client.Get(fmt.Sprintf("https://%s:%d/health", addr, httpsPort))
	if err != nil {
		return false
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// checkDNS sends a DNS A query and verifies a response is returned.
func checkDNS(addr string, port int, network string) bool {
	client := &mdns.Client{}
	msg := mdns.NewMsg("localhost.", mdns.TypeA)
	target := net.JoinHostPort(addr, fmt.Sprintf("%d", port))
	ctx, cancel := context.WithTimeout(context.Background(), healthCheckTimeout)
	defer cancel()
	resp, _, err := client.Exchange(ctx, msg, network, target)
	if err != nil {
		return false
	}
	return resp != nil
}

// checkForwardProxy sends an HTTP request through the forward proxy to an external site.
func checkForwardProxy(addr string, port int) bool {
	proxyURL, _ := url.Parse(fmt.Sprintf("http://%s:%d", addr, port))
	client := &http.Client{
		Timeout: healthCheckTimeout,
		Transport: &http.Transport{
			Proxy: http.ProxyURL(proxyURL),
		},
	}
	resp, err := client.Get("http://www.gstatic.com/generate_204")
	if err != nil {
		return false
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	return resp.StatusCode == http.StatusNoContent
}

// checkAdminUI sends an HTTP request to the admin API endpoint.
func checkAdminUI(addr string, port int) bool {
	client := &http.Client{Timeout: healthCheckTimeout}
	resp, err := client.Get(fmt.Sprintf("http://%s:%d/api/v1/status/overview", addr, port))
	if err != nil {
		return false
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// RunHealthChecks performs functional health checks on all services for each listen address.
func RunHealthChecks(listenAddrs []string, httpPort, httpsPort, dnsPort, proxyPort, adminPort int) []PortHealth {
	type check struct {
		service  string
		port     int
		protocol string
		fn       func(addr string) bool
	}

	checks := []check{
		{"HTTP Proxy", httpPort, "tcp", func(addr string) bool {
			return checkHTTPProxy(addr, httpPort)
		}},
		{"HTTPS Proxy", httpsPort, "tls", func(addr string) bool {
			return checkHTTPSProxy(addr, httpsPort)
		}},
		{"DNS (TCP)", dnsPort, "tcp", func(addr string) bool {
			return checkDNS(addr, dnsPort, "tcp")
		}},
		{"DNS (UDP)", dnsPort, "udp", func(addr string) bool {
			return checkDNS(addr, dnsPort, "udp")
		}},
		{"Forward Proxy", proxyPort, "tcp", func(addr string) bool {
			return checkForwardProxy(addr, proxyPort)
		}},
		{"Admin UI", adminPort, "tcp", func(addr string) bool {
			return checkAdminUI(addr, adminPort)
		}},
	}

	var results []PortHealth
	for _, addr := range listenAddrs {
		for _, c := range checks {
			bound := c.fn(addr)
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
