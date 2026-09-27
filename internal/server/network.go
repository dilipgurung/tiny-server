package server

import (
	"fmt"
	"net"
)

// formatAddress renders a listen address for an IP, bracketing IPv6 literals.
func formatAddress(ip net.IP, port string) string {
	if v4 := ip.To4(); v4 != nil {
		return fmt.Sprintf("http://%s:%s", v4.String(), port)
	}
	return fmt.Sprintf("http://[%s]:%s", ip.To16().String(), port)
}

// usableIP reports whether ip is worth advertising as a listen address.
// Link-local addresses (fe80::/10, 169.254/16) are skipped: a URL such as
// http://[fe80::1]:8000 carries no zone, so no other device can reach it.
// macOS assigns fe80::1 to lo0, which IsLoopback does not recognise, and it
// used to be chosen as the QR code target ahead of the real LAN address.
func usableIP(ip net.IP) bool {
	return ip != nil && !ip.IsUnspecified() &&
		!ip.IsLinkLocalUnicast() && !ip.IsLinkLocalMulticast()
}

// GetNetworkAddresses lists the URLs the server is reachable on and picks
// one as host, the address encoded in the QR code. The host prefers a
// non-loopback IPv4 address, since that is what a phone on the same network
// can most reliably reach, then any non-loopback address, then loopback.
func GetNetworkAddresses(port string) (string, []string, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return "", nil, err
	}

	var ips []net.IP
	for _, iface := range interfaces {
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			switch v := addr.(type) {
			case *net.IPNet:
				ips = append(ips, v.IP)
			case *net.IPAddr:
				ips = append(ips, v.IP)
			}
		}
	}

	host, addresses := selectAddresses(ips, port)
	return host, addresses, nil
}

// selectAddresses filters and de-duplicates ips into listen URLs and picks
// the QR code host from them. It is separate from GetNetworkAddresses so the
// selection rules can be tested without depending on the host's interfaces.
func selectAddresses(ips []net.IP, port string) (string, []string) {
	var addresses []string
	seen := make(map[string]bool)
	var hostV4, hostAny string

	for _, ip := range ips {
		if !usableIP(ip) {
			continue
		}
		formatted := formatAddress(ip, port)
		if seen[formatted] {
			continue
		}
		seen[formatted] = true
		addresses = append(addresses, formatted)

		if ip.IsLoopback() {
			continue
		}
		if hostV4 == "" && ip.To4() != nil {
			hostV4 = formatted
		}
		if hostAny == "" {
			hostAny = formatted
		}
	}

	host := hostV4
	if host == "" {
		host = hostAny
	}
	if host == "" && len(addresses) > 0 {
		host = addresses[0]
	}
	return host, addresses
}
