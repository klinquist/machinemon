package client

import (
	"net"
	"sort"
)

// ListInterfaceIPs returns non-loopback IP addresses from active interfaces.
func ListInterfaceIPs() []string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil
	}

	seen := make(map[string]struct{})
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
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			default:
				continue
			}

			if ip == nil || ip.IsLoopback() || ip.IsMulticast() || ip.IsUnspecified() {
				continue
			}
			ip = ip.To16()
			if ip == nil {
				continue
			}

			s := ip.String()
			if _, ok := seen[s]; ok {
				continue
			}
			seen[s] = struct{}{}
			ips = append(ips, s)
		}
	}

	sort.Strings(ips)
	return ips
}
