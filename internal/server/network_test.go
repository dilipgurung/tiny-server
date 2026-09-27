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

func TestSelectAddresses(t *testing.T) {
	ips := []net.IP{
		net.ParseIP("127.0.0.1"),
		net.ParseIP("::1"),
		net.ParseIP("fe80::1"), // macOS lo0: not IsLoopback, but unreachable
		net.ParseIP("2001:db8::5"),
		net.ParseIP("192.168.1.25"),
		net.ParseIP("fe80::e457:efff:fe50:f9a2"),
		net.ParseIP("169.254.10.1"),
		net.ParseIP("192.168.1.25"), // duplicate across interfaces
		net.ParseIP("0.0.0.0"),
	}
	host, addresses := selectAddresses(ips, "8000")

	if want := "http://192.168.1.25:8000"; host != want {
		t.Errorf("host = %q, want %q", host, want)
	}
	want := []string{
		"http://127.0.0.1:8000",
		"http://[::1]:8000",
		"http://[2001:db8::5]:8000",
		"http://192.168.1.25:8000",
	}
	if strings.Join(addresses, ",") != strings.Join(want, ",") {
		t.Errorf("addresses = %v, want %v", addresses, want)
	}
}

func TestSelectAddressesHostFallbacks(t *testing.T) {
	host, _ := selectAddresses([]net.IP{net.ParseIP("::1"), net.ParseIP("2001:db8::5")}, "80")
	if want := "http://[2001:db8::5]:80"; host != want {
		t.Errorf("IPv6-only host = %q, want %q", host, want)
	}
	host, _ = selectAddresses([]net.IP{net.ParseIP("fe80::1"), net.ParseIP("127.0.0.1")}, "80")
	if want := "http://127.0.0.1:80"; host != want {
		t.Errorf("loopback-only host = %q, want %q", host, want)
	}
	host, addresses := selectAddresses(nil, "80")
	if host != "" || len(addresses) != 0 {
		t.Errorf("no IPs: host = %q, addresses = %v; want empty", host, addresses)
	}
}
