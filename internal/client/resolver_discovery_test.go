// ==============================================================================
// MasterDnsVPN
// Author: MasterkinG32
// Github: https://github.com/masterking32
// Year: 2026
// ==============================================================================

package client

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"masterdnsvpn-go/internal/config"
)

func TestDefaultScanOptions(t *testing.T) {
	opts := DefaultScanOptions()
	if opts.TopN != 32 {
		t.Errorf("Expected TopN=32, got %d", opts.TopN)
	}
	if !opts.AutoApply {
		t.Errorf("Expected AutoApply=true, got %v", opts.AutoApply)
	}
	if opts.Parallelism != 32 {
		t.Errorf("Expected Parallelism=32, got %d", opts.Parallelism)
	}
}

func TestPrintScanSummaryTable(t *testing.T) {
	var buf bytes.Buffer

	// Test empty table
	PrintScanSummaryTable(&buf, nil, 10)
	if !strings.Contains(buf.String(), "No working resolvers found") {
		t.Errorf("Expected empty message, got %s", buf.String())
	}

	buf.Reset()
	resolvers := []ScoredResolver{
		{
			Endpoint: config.ResolverAddress{
				IP:   "1.1.1.1",
				Port: 53,
			},
			UploadMTU:    140,
			DownloadMTU:  1420,
			RTT:          15 * time.Millisecond,
			SpeedKBps:    92.4,
			Score:        89.7,
			QualityTier:  "EXCELLENT",
			QualityBadge: "🟢 EXCELLENT",
		},
		{
			Endpoint: config.ResolverAddress{
				IP:   "8.8.8.8",
				Port: 5353,
			},
			UploadMTU:    130,
			DownloadMTU:  1280,
			RTT:          45 * time.Millisecond,
			SpeedKBps:    27.8,
			Score:        25.5,
			QualityTier:  "GOOD",
			QualityBadge: "🟡 GOOD",
		},
	}

	PrintScanSummaryTable(&buf, resolvers, 10)
	out := buf.String()

	if !strings.Contains(out, "1.1.1.1") {
		t.Errorf("Expected table to contain 1.1.1.1, got:\n%s", out)
	}
	if !strings.Contains(out, "8.8.8.8:5353") {
		t.Errorf("Expected table to contain 8.8.8.8:5353, got:\n%s", out)
	}
	if !strings.Contains(out, "EXCELLENT") {
		t.Errorf("Expected table to contain EXCELLENT, got:\n%s", out)
	}
	if !strings.Contains(out, "Ping/RTT") {
		t.Errorf("Expected table header with Ping/RTT, got:\n%s", out)
	}
}

func TestScanAndRankResolvers_NoCandidates(t *testing.T) {
	c := &Client{}
	_, err := c.ScanAndRankResolvers(context.Background(), nil, DefaultScanOptions())
	if err == nil {
		t.Fatal("Expected error for empty candidates, got nil")
	}
}
