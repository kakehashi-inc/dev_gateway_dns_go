package modules

import (
	"fmt"
	"net"
)

// NICInfo holds network interface information.
type NICInfo struct {
	Name string   `json:"name"`
	IPs  []string `json:"ips"`
}

// DetectNICs returns all active network interfaces with their IPv4 addresses.
func DetectNICs() ([]NICInfo, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("failed to list interfaces: %w", err)
	}

	var nics []NICInfo
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		var ips []string
		for _, addr := range addrs {
			ipNet, ok := addr.(*net.IPNet)
			if !ok {
				continue
			}
			if ipNet.IP.To4() != nil {
				ips = append(ips, ipNet.IP.String())
			}
		}
		if len(ips) > 0 {
			nics = append(nics, NICInfo{Name: iface.Name, IPs: ips})
		}
	}
	return nics, nil
}

// GetAllNICIPs returns all IPv4 addresses from active non-loopback interfaces.
func GetAllNICIPs() ([]string, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	var ips []string
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
			if !ok {
				continue
			}
			if ipNet.IP.To4() != nil {
				ips = append(ips, ipNet.IP.String())
			}
		}
	}
	return ips, nil
}

// ResolveListenIPs determines the IP list based on the listen address setting.
// If listen is "0.0.0.0", returns all NIC IPs. Otherwise returns the specified addresses.
func ResolveListenIPs(listenAddresses []string) ([]string, error) {
	var result []string
	for _, addr := range listenAddresses {
		if addr == "0.0.0.0" {
			allIPs, err := GetAllNICIPs()
			if err != nil {
				return nil, err
			}
			result = append(result, allIPs...)
		} else {
			result = append(result, addr)
		}
	}
	return result, nil
}
