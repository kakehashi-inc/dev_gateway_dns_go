package modules

import "testing"

func TestIsWildcard(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"*.example.com", true},
		{"*.local", true},
		{"app.example.com", false},
		{"example.com", false},
		{"*", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := IsWildcard(tt.input); got != tt.want {
			t.Errorf("IsWildcard(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestWildcardBase(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"*.example.com", "example.com"},
		{"*.local", "local"},
		{"app.example.com", "app.example.com"},
		{"example.com", "example.com"},
		{"*", "*"},
		{"", ""},
	}
	for _, tt := range tests {
		if got := WildcardBase(tt.input); got != tt.want {
			t.Errorf("WildcardBase(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestWildcardDomain(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"sub.example.com", "example.com"},
		{"a.b.example.com", "b.example.com"},
		{"app.local", "local"},
		{"localhost", ""},
		{"", ""},
	}
	for _, tt := range tests {
		if got := WildcardDomain(tt.input); got != tt.want {
			t.Errorf("WildcardDomain(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
