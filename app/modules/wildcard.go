package modules

import "strings"

// IsWildcard returns true if the hostname is a wildcard pattern (e.g., "*.example.com").
func IsWildcard(hostname string) bool {
	return strings.HasPrefix(hostname, "*.")
}

// WildcardBase returns the base domain of a wildcard hostname.
// "*.example.com" -> "example.com". Returns the input unchanged if not a wildcard.
func WildcardBase(hostname string) string {
	if strings.HasPrefix(hostname, "*.") {
		return hostname[2:]
	}
	return hostname
}

// WildcardDomain extracts the parent domain from a hostname for wildcard matching.
// "sub.example.com" -> "example.com", "a.b.example.com" -> "b.example.com".
// Returns "" if the hostname has no dots (no parent domain).
func WildcardDomain(hostname string) string {
	idx := strings.Index(hostname, ".")
	if idx < 0 {
		return ""
	}
	return hostname[idx+1:]
}
