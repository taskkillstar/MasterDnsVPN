// ==============================================================================
// MasterDnsVPN
// Author: MasterkinG32
// Github: https://github.com/masterking32
// Year: 2026
// ==============================================================================

package client

import (
	"testing"
)

func TestDeriveSubnetNeighborhood(t *testing.T) {
	tests := []struct {
		name    string
		ip      string
		want    string
		wantErr bool
	}{
		{"Standard IPv4", "194.225.150.11", "194.225.150.0/24", false},
		{"Private Class A", "10.201.34.5", "10.201.34.0/24", false},
		{"Private Class C", "192.168.1.1", "192.168.1.0/24", false},
		{"Loopback Rejected", "127.0.0.1", "", true},
		{"Unspecified Rejected", "0.0.0.0", "", true},
		{"IPv6 Rejected", "2001:4860:4860::8888", "", true},
		{"Invalid String", "not-an-ip", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := DeriveSubnetNeighborhood(tt.ip)
			if (err != nil) != tt.wantErr {
				t.Fatalf("DeriveSubnetNeighborhood(%q) error = %v, wantErr %v", tt.ip, err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("DeriveSubnetNeighborhood(%q) = %v, want %v", tt.ip, got, tt.want)
			}
		})
	}
}

func TestExpandSubnetToResolverAddresses(t *testing.T) {
	endpoints, err := ExpandSubnetToResolverAddresses("192.168.1.0/24", 53)
	if err != nil {
		t.Fatalf("ExpandSubnetToResolverAddresses failed: %v", err)
	}

	// /24 has 256 addresses, minus network (.0) and broadcast (.255) = 254
	if len(endpoints) != 254 {
		t.Errorf("Expected 254 endpoints for /24, got %d", len(endpoints))
	}
	if len(endpoints) > 0 {
		if endpoints[0].IP != "192.168.1.1" || endpoints[0].Port != 53 {
			t.Errorf("First endpoint mismatch: got %+v, want 192.168.1.1:53", endpoints[0])
		}
		if endpoints[len(endpoints)-1].IP != "192.168.1.254" {
			t.Errorf("Last endpoint mismatch: got %+v, want 192.168.1.254:53", endpoints[len(endpoints)-1])
		}
	}
}

func TestDetectLocalNetworkDoesNotPanic(t *testing.T) {
	info := DetectLocalNetwork()
	t.Logf("Detected local network: DNS=%v, Gateway=%s", info.DNSServers, info.Gateway)
}
