// ==============================================================================
// MasterDnsVPN
// Author: MasterkinG32
// Github: https://github.com/masterking32
// Year: 2026
// ==============================================================================

package client

import (
	"fmt"
	"net/netip"
	"strings"

	"masterdnsvpn-go/internal/config"
)

// LocalNetworkInfo holds detected local DNS servers and default gateway.
type LocalNetworkInfo struct {
	DNSServers []string
	Gateway    string
}

// DetectLocalNetwork discovers active DNS servers and default gateway from the OS.
func DetectLocalNetwork() LocalNetworkInfo {
	return detectPlatformLocalNetwork()
}

// DeriveSubnetNeighborhood derives a /24 subnet string for an IPv4 address.
// For instance: "194.225.150.11" -> "194.225.150.0/24".
// Returns an error if the IP is not valid IPv4 or is loopback/unspecified.
func DeriveSubnetNeighborhood(ipStr string) (string, error) {
	ipStr = strings.TrimSpace(ipStr)
	addr, err := netip.ParseAddr(ipStr)
	if err != nil {
		return "", fmt.Errorf("invalid IP: %w", err)
	}

	if !addr.IsValid() || addr.IsLoopback() || addr.IsUnspecified() || addr.IsMulticast() {
		return "", fmt.Errorf("ip %s is not suitable for subnet derivation", ipStr)
	}

	if !addr.Is4() {
		return "", fmt.Errorf("subnet neighborhood derivation is only supported for IPv4")
	}

	prefix, err := addr.Prefix(24)
	if err != nil {
		return "", err
	}
	return prefix.Masked().String(), nil
}

// ExpandSubnetToResolverAddresses takes a /24 CIDR (or other prefix <= /20)
// and returns all usable host addresses as config.ResolverAddress with default port 53.
func ExpandSubnetToResolverAddresses(cidr string, port int) ([]config.ResolverAddress, error) {
	if port <= 0 || port > 65535 {
		port = 53
	}

	prefix, err := netip.ParsePrefix(strings.TrimSpace(cidr))
	if err != nil {
		return nil, fmt.Errorf("invalid subnet prefix: %w", err)
	}

	prefix = prefix.Masked()
	if prefix.Bits() < 16 {
		return nil, fmt.Errorf("subnet prefix /%d is too large to safely scan", prefix.Bits())
	}

	first := prefix.Addr()
	if !first.Is4() {
		return nil, fmt.Errorf("only IPv4 subnets are supported for expansion")
	}

	firstBytes := first.As4()
	totalHosts := 1 << (32 - prefix.Bits())
	if totalHosts <= 2 {
		return nil, fmt.Errorf("subnet too small")
	}

	endpoints := make([]config.ResolverAddress, 0, totalHosts-2)
	// Skip network address (index 0) and broadcast (last index) for /24 to /30
	baseInt := uint32(firstBytes[0])<<24 | uint32(firstBytes[1])<<16 | uint32(firstBytes[2])<<8 | uint32(firstBytes[3])
	for i := 1; i < totalHosts-1; i++ {
		current := baseInt + uint32(i)
		ip := netip.AddrFrom4([4]byte{
			byte(current >> 24),
			byte(current >> 16),
			byte(current >> 8),
			byte(current),
		})
		endpoints = append(endpoints, config.ResolverAddress{
			IP:   ip.String(),
			Port: port,
		})
	}

	return endpoints, nil
}

// GetLocalDiscoveryCandidates queries the local network and compiles a deduplicated list
// of candidate resolver endpoints, including:
// 1. Explicitly configured DNS servers (DHCP / static)
// 2. Default gateway IP (often running a DNS proxy, essential for captive portal bypass)
// 3. The surrounding /24 subnet for each detected IPv4 DNS server and gateway.
func GetLocalDiscoveryCandidates(includeSubnets bool) ([]config.ResolverAddress, LocalNetworkInfo) {
	info := DetectLocalNetwork()

	seen := make(map[string]struct{})
	var candidates []config.ResolverAddress

	addCandidate := func(ip string, port int) {
		ip = strings.TrimSpace(ip)
		if ip == "" {
			return
		}
		if port <= 0 || port > 65535 {
			port = 53
		}
		key := fmt.Sprintf("%s:%d", ip, port)
		if _, exists := seen[key]; exists {
			return
		}
		seen[key] = struct{}{}
		candidates = append(candidates, config.ResolverAddress{
			IP:   ip,
			Port: port,
		})
	}

	// 1. Add specific DNS servers
	for _, dns := range info.DNSServers {
		addCandidate(dns, 53)
	}

	// 2. Add default gateway
	if info.Gateway != "" {
		addCandidate(info.Gateway, 53)
	}

	// 3. Derive /24 subnets if requested
	if includeSubnets {
		subnetsToExpand := make(map[string]struct{})
		for _, dns := range info.DNSServers {
			if subnet, err := DeriveSubnetNeighborhood(dns); err == nil {
				subnetsToExpand[subnet] = struct{}{}
			}
		}
		if info.Gateway != "" {
			if subnet, err := DeriveSubnetNeighborhood(info.Gateway); err == nil {
				subnetsToExpand[subnet] = struct{}{}
			}
		}

		for subnet := range subnetsToExpand {
			if expanded, err := ExpandSubnetToResolverAddresses(subnet, 53); err == nil {
				for _, ep := range expanded {
					addCandidate(ep.IP, ep.Port)
				}
			}
		}
	}

	return candidates, info
}
