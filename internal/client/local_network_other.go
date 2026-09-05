// ==============================================================================
// MasterDnsVPN
// Author: MasterkinG32
// Github: https://github.com/masterking32
// Year: 2026
// ==============================================================================

//go:build !windows

package client

import (
	"bufio"
	"fmt"
	"net/netip"
	"os"
	"strings"
)

func detectPlatformLocalNetwork() LocalNetworkInfo {
	var info LocalNetworkInfo
	seenDNS := make(map[string]struct{})

	// 1. Read /etc/resolv.conf for nameservers
	if file, err := os.Open("/etc/resolv.conf"); err == nil {
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if strings.HasPrefix(line, "nameserver") {
				fields := strings.Fields(line)
				if len(fields) >= 2 {
					raw := fields[1]
					if addr, err := netip.ParseAddr(raw); err == nil && addr.IsValid() && !addr.IsLoopback() && !addr.IsUnspecified() {
						ip := addr.String()
						if _, seen := seenDNS[ip]; !seen {
							seenDNS[ip] = struct{}{}
							info.DNSServers = append(info.DNSServers, ip)
						}
					}
				}
			}
		}
		_ = file.Close()
	}

	// 2. Read /proc/net/route on Linux for default gateway
	if file, err := os.Open("/proc/net/route"); err == nil {
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			fields := strings.Fields(scanner.Text())
			if len(fields) >= 3 && fields[1] == "00000000" { // Destination 0.0.0.0
				var g1, g2, g3, g4 uint8
				if n, _ := fmt.Sscanf(fields[2], "%02X%02X%02X%02X", &g4, &g3, &g2, &g1); n == 4 {
					gatewayIP := fmt.Sprintf("%d.%d.%d.%d", g1, g2, g3, g4)
					if addr, err := netip.ParseAddr(gatewayIP); err == nil && addr.IsValid() && !addr.IsUnspecified() {
						info.Gateway = addr.String()
						break
					}
				}
			}
		}
		_ = file.Close()
	}

	return info
}
