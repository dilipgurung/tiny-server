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

func GetNetworkAddresses(port string) (string, []string, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return "", nil, err
	}

	var host string
	var addresses []string

	for _, iface := range interfaces {
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip == nil || ip.IsUnspecified() {
				continue
			}

			formatted := formatAddress(ip, port)
			addresses = append(addresses, formatted)
			if !ip.IsLoopback() && host == "" {
				host = formatted
			}
		}
	}

	if host == "" && len(addresses) > 0 {
		host = addresses[0]
	}

	return host, addresses, nil
}
