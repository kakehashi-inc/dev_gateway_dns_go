package dns

import (
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
)

// UpstreamMap manages NIC IP to upstream DNS server mappings.
type UpstreamMap struct {
	mu       sync.RWMutex
	mapping  map[string][]string // NIC IP -> []upstream DNS addresses
	fallback []string
}

// NewUpstreamMap creates an UpstreamMap with the given fallback DNS servers.
func NewUpstreamMap(fallback []string) *UpstreamMap {
	return &UpstreamMap{
		mapping:  make(map[string][]string),
		fallback: fallback,
	}
}

// Build detects each NIC's DNS servers and populates the mapping.
func (u *UpstreamMap) Build() error {
	u.mu.Lock()
	defer u.mu.Unlock()

	ifaces, err := net.Interfaces()
	if err != nil {
		return fmt.Errorf("failed to list interfaces: %w", err)
	}

	systemDNS := parseResolvConf()

	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			ipNet, ok := addr.(*net.IPNet)
			if !ok || ipNet.IP.To4() == nil {
				continue
			}
			dns := systemDNS
			if len(dns) == 0 {
				dns = u.fallback
			}
			u.mapping[ipNet.IP.String()] = dns
		}
	}
	return nil
}

// Resolve returns the upstream DNS servers for a given NIC IP.
func (u *UpstreamMap) Resolve(nicIP string) []string {
	u.mu.RLock()
	defer u.mu.RUnlock()
	if servers, ok := u.mapping[nicIP]; ok && len(servers) > 0 {
		return servers
	}
	return u.fallback
}

// parseResolvConf reads /etc/resolv.conf for nameserver entries.
func parseResolvConf() []string {
	data, err := os.ReadFile("/etc/resolv.conf")
	if err != nil {
		return nil
	}
	var servers []string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "nameserver") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				servers = append(servers, fields[1])
			}
		}
	}
	return servers
}
