// ==============================================================================
// MasterDnsVPN
// Author: MasterkinG32
// Github: https://github.com/masterking32
// Year: 2026
// ==============================================================================

//go:build windows

package client

import (
	"net/netip"
	"strings"

	"golang.org/x/sys/windows/registry"
)

func detectPlatformLocalNetwork() LocalNetworkInfo {
	var info LocalNetworkInfo
	seenDNS := make(map[string]struct{})

	addDNS := func(val string) {
		for _, raw := range strings.FieldsFunc(val, func(r rune) bool {
			return r == ',' || r == ' ' || r == ';'
		}) {
			raw = strings.TrimSpace(raw)
			if addr, err := netip.ParseAddr(raw); err == nil && addr.IsValid() && !addr.IsLoopback() && !addr.IsUnspecified() {
				ip := addr.String()
				if _, seen := seenDNS[ip]; !seen {
					seenDNS[ip] = struct{}{}
					info.DNSServers = append(info.DNSServers, ip)
				}
			}
		}
	}

	setGateway := func(val string) {
		if info.Gateway != "" {
			return
		}
		for _, raw := range strings.FieldsFunc(val, func(r rune) bool {
			return r == ',' || r == ' ' || r == ';'
		}) {
			raw = strings.TrimSpace(raw)
			if addr, err := netip.ParseAddr(raw); err == nil && addr.IsValid() && !addr.IsLoopback() && !addr.IsUnspecified() {
				info.Gateway = addr.String()
				return
			}
		}
	}

	// 1. Check Global Tcpip Parameters
	if k, err := registry.OpenKey(registry.LOCAL_MACHINE, `SYSTEM\CurrentControlSet\Services\Tcpip\Parameters`, registry.QUERY_VALUE); err == nil {
		if s, _, err := k.GetStringValue("NameServer"); err == nil {
			addDNS(s)
		}
		if s, _, err := k.GetStringValue("DhcpNameServer"); err == nil {
			addDNS(s)
		}
		_ = k.Close()
	}

	// 2. Check each Adapter Interface under Tcpip\Parameters\Interfaces
	if kInterfaces, err := registry.OpenKey(registry.LOCAL_MACHINE, `SYSTEM\CurrentControlSet\Services\Tcpip\Parameters\Interfaces`, registry.READ); err == nil {
		if subkeys, err := kInterfaces.ReadSubKeyNames(-1); err == nil {
			for _, sub := range subkeys {
				if ifaceKey, err := registry.OpenKey(kInterfaces, sub, registry.QUERY_VALUE); err == nil {
					if s, _, err := ifaceKey.GetStringValue("NameServer"); err == nil {
						addDNS(s)
					}
					if s, _, err := ifaceKey.GetStringValue("DhcpNameServer"); err == nil {
						addDNS(s)
					}
					if s, _, err := ifaceKey.GetStringValue("DefaultGateway"); err == nil {
						setGateway(s)
					}
					if s, _, err := ifaceKey.GetStringValue("DhcpDefaultGateway"); err == nil {
						setGateway(s)
					}
					if ss, _, err := ifaceKey.GetStringsValue("DefaultGateway"); err == nil {
						for _, g := range ss {
							setGateway(g)
						}
					}
					if ss, _, err := ifaceKey.GetStringsValue("DhcpDefaultGateway"); err == nil {
						for _, g := range ss {
							setGateway(g)
						}
					}
					if ss, _, err := ifaceKey.GetStringsValue("NameServer"); err == nil {
						for _, dns := range ss {
							addDNS(dns)
						}
					}
					if ss, _, err := ifaceKey.GetStringsValue("DhcpNameServer"); err == nil {
						for _, dns := range ss {
							addDNS(dns)
						}
					}
					_ = ifaceKey.Close()
				}
			}
		}
		_ = kInterfaces.Close()
	}

	return info
}
