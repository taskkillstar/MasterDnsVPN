// ==============================================================================
// MasterDnsVPN
// Author: MasterkinG32
// Github: https://github.com/masterking32
// Year: 2026
// ==============================================================================

package client

import (
	"context"
	"fmt"
	"io"
	"os"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"masterdnsvpn-go/internal/config"
)

// ScoredResolver captures the benchmark and quality metrics for a verified resolver.
type ScoredResolver struct {
	Endpoint     config.ResolverAddress
	UploadMTU    int
	DownloadMTU  int
	RTT          time.Duration
	SpeedKBps    float64
	Score        float64
	QualityTier  string
	QualityBadge string
}

// ScanOptions configures the behavior of the standalone resolver discovery scan.
type ScanOptions struct {
	CandidateResolvers []config.ResolverAddress
	TopN               int
	AutoApply          bool
	IncludeLocalNet    bool
	Parallelism        int
	Timeout            time.Duration
	ResolversPath      string
	OutputWriter       io.Writer
}

// DefaultScanOptions returns practical defaults for resolver discovery.
func DefaultScanOptions() ScanOptions {
	return ScanOptions{
		TopN:            32,
		AutoApply:       true,
		IncludeLocalNet: false,
		Parallelism:     32,
		Timeout:         1500 * time.Millisecond,
	}
}

// ScanAndRankResolvers probes a slice of candidate resolvers using a two-phase funnel:
// Phase 1: Rapid liveness check (short timeout, 0 retries) to eliminate dead hosts.
// Phase 2: Full upload/download MTU and RTT qualification for responsive survivors.
func (c *Client) ScanAndRankResolvers(ctx context.Context, candidates []config.ResolverAddress, opts ScanOptions) ([]ScoredResolver, error) {
	if c == nil || len(candidates) == 0 {
		return nil, fmt.Errorf("no candidates to scan")
	}

	var domain string
	if len(c.cfg.Domains) > 0 {
		domain = c.cfg.Domains[0]
	}
	if domain == "" {
		return nil, fmt.Errorf("no tunnel domain configured for MTU discovery")
	}

	uploadCaps := c.precomputeUploadCaps()
	maxUploadCap := uploadCaps[domain]
	if maxUploadCap <= 0 {
		maxUploadCap = defaultUploadMaxCap
	}

	parallelism := opts.Parallelism
	if parallelism <= 0 {
		parallelism = 32
	}
	if parallelism > len(candidates) {
		parallelism = len(candidates)
	}

	var totalCompleted atomic.Int32
	var totalFound atomic.Int32

	resultsMu := sync.Mutex{}
	var scoredResolvers []ScoredResolver

	jobs := make(chan config.ResolverAddress, parallelism*2)
	var wg sync.WaitGroup

	for workerID := 0; workerID < parallelism; workerID++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for addr := range jobs {
				if ctx.Err() != nil {
					return
				}

				connKey := makeConnectionKey(addr.IP, addr.Port, domain)
				conn := Connection{
					Key:           connKey,
					Domain:        domain,
					Resolver:      addr.IP,
					ResolverPort:  addr.Port,
					ResolverLabel: formatResolverEndpoint(addr.IP, addr.Port),
				}

				// Phase 1: Rapid liveness probe (350ms timeout, 0 retries)
				transport, err := newUDPQueryTransport(conn.ResolverLabel)
				if err != nil {
					totalCompleted.Add(1)
					continue
				}

				live, _, _ := c.sendUploadMTUProbe(
					ctx,
					conn,
					transport,
					minUploadMTUFloor+mtuProbeCodeLength+1,
					350*time.Millisecond,
					mtuProbeOptions{Quiet: true, IsRetry: false},
				)
				_ = transport.conn.Close()

				if !live {
					totalCompleted.Add(1)
					continue
				}

				// Phase 2: Full MTU Discovery & RTT Measurement
				result, reason := c.probeConnectionMTU(ctx, conn, maxUploadCap)
				totalCompleted.Add(1)

				if reason == mtuRejectNone && result.UploadBytes > 0 && result.DownloadBytes > 0 {
					totalFound.Add(1)
					speedKBps, score := CalculateResolverScore(result.DownloadBytes, result.ResolveTime, 0.0)
					tier, badge := ResolverQualityTier(score)

					sr := ScoredResolver{
						Endpoint:     addr,
						UploadMTU:    result.UploadBytes,
						DownloadMTU:  result.DownloadBytes,
						RTT:          result.ResolveTime,
						SpeedKBps:    speedKBps,
						Score:        score,
						QualityTier:  tier,
						QualityBadge: badge,
					}

					resultsMu.Lock()
					scoredResolvers = append(scoredResolvers, sr)
					resultsMu.Unlock()
				}
			}
		}()
	}

	for _, addr := range candidates {
		select {
		case <-ctx.Done():
			break
		case jobs <- addr:
		}
	}
	close(jobs)
	wg.Wait()

	sort.Slice(scoredResolvers, func(i, j int) bool {
		return scoredResolvers[i].Score > scoredResolvers[j].Score
	})

	return scoredResolvers, nil
}

// PrintScanSummaryTable formats and prints a readable terminal summary table of discovered resolvers.
func PrintScanSummaryTable(w io.Writer, resolvers []ScoredResolver, maxRows int) {
	if w == nil {
		w = os.Stdout
	}

	if len(resolvers) == 0 {
		_, _ = fmt.Fprintln(w, "\n❌ No working resolvers found during discovery.")
		return
	}

	rowsToPrint := len(resolvers)
	if maxRows > 0 && maxRows < rowsToPrint {
		rowsToPrint = maxRows
	}

	_, _ = fmt.Fprintln(w, "")
	_, _ = fmt.Fprintln(w, "┌──────┬─────────────────────────┬──────────┬──────────┬──────────┬─────────────┬──────────┬───────────┐")
	_, _ = fmt.Fprintln(w, "│ Rank │ Resolver Endpoint       │ Ping/RTT │ Upload   │ Download │ Speed Est.  │ Score    │ Quality   │")
	_, _ = fmt.Fprintln(w, "├──────┼─────────────────────────┼──────────┼──────────┼──────────┼─────────────┼──────────┼───────────┤")

	for i := 0; i < rowsToPrint; i++ {
		r := resolvers[i]
		ep := config.FormatResolverAddressForFile(r.Endpoint)
		rttStr := formatResolverRTT(r.RTT)
		speedStr := FormatResolverSpeed(r.SpeedKBps)

		_, _ = fmt.Fprintf(w, "│ %4d │ %-23s │ %-8s │ %-8s │ %-8s │ %-11s │ %8.1f │ %-9s │\n",
			i+1,
			truncateString(ep, 23),
			rttStr,
			fmt.Sprintf("%d B", r.UploadMTU),
			fmt.Sprintf("%d B", r.DownloadMTU),
			speedStr,
			r.Score,
			r.QualityTier,
		)
	}
	_, _ = fmt.Fprintln(w, "└──────┴─────────────────────────┴──────────┴──────────┴──────────┴─────────────┴──────────┴───────────┘")
	if len(resolvers) > rowsToPrint {
		_, _ = fmt.Fprintf(w, "   ... and %d more qualified resolvers not shown.\n", len(resolvers)-rowsToPrint)
	}
	_, _ = fmt.Fprintln(w, "")
}

// RunScanMode executes the full standalone discovery workflow:
// loads candidates, runs the 2-phase scanner, displays the summary table,
// and optionally persists the top resolvers into client_resolvers.txt managed header.
func (c *Client) RunScanMode(ctx context.Context, opts ScanOptions) error {
	out := opts.OutputWriter
	if out == nil {
		out = os.Stdout
	}

	_, _ = fmt.Fprintln(out, "==============================================================================")
	_, _ = fmt.Fprintln(out, "⚡ MasterDnsVPN Resolver Discovery & Benchmark")
	_, _ = fmt.Fprintln(out, "==============================================================================")

	candidateMap := make(map[string]config.ResolverAddress)

	// 1. Load from file if provided or default
	resolversPath := opts.ResolversPath
	if resolversPath == "" {
		resolversPath = c.cfg.ResolversPath()
	}
	if resolversPath != "" {
		if fileEndpoints, _, err := config.LoadClientResolvers(resolversPath); err == nil {
			for _, ep := range fileEndpoints {
				key := config.FormatResolverAddressForFile(ep)
				candidateMap[key] = ep
			}
			_, _ = fmt.Fprintf(out, "📁 Loaded %d candidates from: %s\n", len(fileEndpoints), resolversPath)
		}
	}

	// 2. Add explicit candidates from options
	for _, ep := range opts.CandidateResolvers {
		key := config.FormatResolverAddressForFile(ep)
		candidateMap[key] = ep
	}

	// 3. Auto-detect local network if requested
	if opts.IncludeLocalNet || c.cfg.AutoDetectLocalDns {
		localCandidates, netInfo := GetLocalDiscoveryCandidates(true)
		_, _ = fmt.Fprintf(out, "🌐 Local network detected: DNS=%v, Gateway=%s (%d targets derived)\n",
			netInfo.DNSServers, netInfo.Gateway, len(localCandidates))
		for _, ep := range localCandidates {
			key := config.FormatResolverAddressForFile(ep)
			candidateMap[key] = ep
		}
	}

	if len(candidateMap) == 0 {
		return fmt.Errorf("no candidate resolvers found to scan (specify -resolvers or -auto-local)")
	}

	candidates := make([]config.ResolverAddress, 0, len(candidateMap))
	for _, ep := range candidateMap {
		candidates = append(candidates, ep)
	}

	_, _ = fmt.Fprintf(out, "🔍 Testing %d unique candidate resolvers (concurrency: %d)...\n", len(candidates), opts.Parallelism)
	started := time.Now()

	scored, err := c.ScanAndRankResolvers(ctx, candidates, opts)
	if err != nil {
		return err
	}

	elapsed := time.Since(started).Round(time.Millisecond)
	_, _ = fmt.Fprintf(out, "🏁 Scan completed in %s. Found %d healthy resolvers.\n", elapsed, len(scored))

	PrintScanSummaryTable(out, scored, opts.TopN)

	// 4. Auto-apply best resolvers if enabled
	if opts.AutoApply && len(scored) > 0 && resolversPath != "" {
		topCount := opts.TopN
		if topCount <= 0 {
			topCount = 32
		}
		if topCount > len(scored) {
			topCount = len(scored)
		}

		topEndpoints := make([]config.ResolverAddress, topCount)
		for i := 0; i < topCount; i++ {
			topEndpoints[i] = scored[i].Endpoint
		}

		if err := config.UpdateResolversFileWithRanked(resolversPath, topEndpoints); err != nil {
			_, _ = fmt.Fprintf(out, "⚠️ Failed to save ranked resolvers to %s: %v\n", resolversPath, err)
		} else {
			_, _ = fmt.Fprintf(out, "💾 Successfully saved top %d verified resolvers to managed block in:\n   %s\n", topCount, resolversPath)
			_, _ = fmt.Fprintln(out, "   (All custom comments and subnets below the block remain untouched.)")
		}
	}

	return nil
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
