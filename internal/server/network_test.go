package server

import (
	"net"
	"strings"
	"testing"
)

func TestFormatAddress(t *testing.T) {
	tests := []struct {
		name string
		ip   net.IP
		port string
		want string
	}{
		{"IPv4 loopback", net.ParseIP("127.0.0.1"), "8000", "http://127.0.0.1:8000"},
		{"IPv4 private", net.ParseIP("192.168.1.10"), "9000", "http://192.168.1.10:9000"},
		{"IPv6 loopback", net.ParseIP("::1"), "8000", "http://[::1]:8000"},
		{"IPv6 link-local", net.ParseIP("fe80::1"), "8000", "http://[fe80::1]:8000"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatAddress(tt.ip, tt.port); got != tt.want {
				t.Errorf("formatAddress(%s, %s) = %q, want %q", tt.ip, tt.port, got, tt.want)
			}
		})
	}
}

func TestGetNetworkAddresses(t *testing.T) {
	host, addresses, err := GetNetworkAddresses("8000")
	if err != nil {
		t.Fatalf("GetNetworkAddresses: %v", err)
	}
	if len(addresses) == 0 {
		t.Fatal("expected at least one address")
	}
	if host == "" {
		t.Fatal("expected non-empty host")
	}
	for _, a := range addresses {
		if !strings.HasPrefix(a, "http://") {
			t.Errorf("address %q missing http:// prefix", a)
		}
		// IPv6 addresses must be bracketed; IPv4 must not contain a bracket.
		if strings.Contains(a, "[") != strings.Contains(a, "]") {
			t.Errorf("address %q has mismatched brackets", a)
		}
	}
}

// TestGetNetworkAddressesIncludesLoopback verifies that at least the
// loopback address (IPv4 or IPv6) is reported on a typical host.
func TestGetNetworkAddressesIncludesLoopback(t *testing.T) {
	_, addresses, err := GetNetworkAddresses("8000")
	if err != nil {
		t.Fatalf("GetNetworkAddresses: %v", err)
	}
	found := false
	for _, a := range addresses {
		if strings.Contains(a, "127.0.0.1") || strings.Contains(a, "[::1]") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected a loopback address in %v", addresses)
	}
}
