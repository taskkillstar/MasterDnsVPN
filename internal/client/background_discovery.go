// ==============================================================================
// MasterDnsVPN
// Author: MasterkinG32
// Github: https://github.com/masterking32
// Year: 2026
// ==============================================================================

package client

import (
	"context"
	"sync"
	"time"

	"masterdnsvpn-go/internal/config"
	"masterdnsvpn-go/internal/logger"
)

// startBackgroundDiscovery runs an asynchronous, low-impact background probe
// that sweeps unprobed candidates and subnets, promotes healthy resolvers into
// the active balancer pool, and persists top performers to client_resolvers.txt.
func (c *Client) startBackgroundDiscovery(ctx context.Context) {
	if c == nil || !c.cfg.BackgroundDiscoveryEnabled || c.balancer == nil {
		return
	}

	var domain string
	if len(c.cfg.Domains) > 0 {
		domain = c.cfg.Domains[0]
	}
	if domain == "" {
		return
	}

	// Wait 2 seconds after startup to ensure foreground tunnel traffic has stabilized
	select {
	case <-ctx.Done():
		return
	case <-time.After(2 * time.Second):
	}

	// 1. Collect candidate endpoints to probe
	candidatesMap := make(map[string]config.ResolverAddress)

	// Inactive connections already registered in balancer
	allConns := c.balancer.AllConnections()
	for _, conn := range allConns {
		if !conn.IsValid && conn.UploadMTUBytes <= 0 && conn.DownloadMTUBytes <= 0 {
			ep := config.ResolverAddress{
				IP:   conn.Resolver,
				Port: conn.ResolverPort,
			}
			key := formatResolverEndpoint(ep.IP, ep.Port)
			candidatesMap[key] = ep
		}
	}

	// Auto-detected local network targets if enabled
	if c.cfg.AutoDetectLocalDns {
		localEndpoints, _ := GetLocalDiscoveryCandidates(true)
		for _, ep := range localEndpoints {
			key := formatResolverEndpoint(ep.IP, ep.Port)
			// Only consider if not already an active connection
			connKey := makeConnectionKey(ep.IP, ep.Port, domain)
			if existingConn, ok := c.balancer.GetConnectionByKey(connKey); !ok || !existingConn.IsValid {
				candidatesMap[key] = ep
			}
		}
	}

	if len(candidatesMap) == 0 {
		return
	}

	candidates := make([]config.ResolverAddress, 0, len(candidatesMap))
	for _, ep := range candidatesMap {
		candidates = append(candidates, ep)
	}

	workerCount := c.cfg.BackgroundDiscoveryWorkers
	if workerCount < 1 {
		workerCount = 1
	}
	if workerCount > 8 {
		workerCount = 8
	}

	uploadCaps := c.precomputeUploadCaps()
	maxUploadCap := uploadCaps[domain]
	if maxUploadCap <= 0 {
		maxUploadCap = defaultUploadMaxCap
	}

	if c.log != nil && c.log.Enabled(logger.LevelInfo) {
		c.log.Infof("⚡ <green>[FastStart Discovery]</green> Background discovery launched for <cyan>%d</cyan> candidate resolver(s) with <cyan>%d</cyan> worker(s).", len(candidates), workerCount)
	}

	jobs := make(chan config.ResolverAddress, workerCount*2)
	var wg sync.WaitGroup

	for i := 0; i < workerCount; i++ {
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

				// Phase 1: Rapid liveness check (350ms, quiet)
				transport, err := newUDPQueryTransport(conn.ResolverLabel)
				if err != nil {
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
					// Gentle pacing between dead probes
					select {
					case <-ctx.Done():
						return
					case <-time.After(50 * time.Millisecond):
					}
					continue
				}

				// Phase 2: Full MTU discovery & qualification
				result, reason := c.probeConnectionMTU(ctx, conn, maxUploadCap)
				if reason == mtuRejectNone && result.UploadBytes > 0 && result.DownloadBytes > 0 {
					// Register into balancer
					c.balancer.ApplyMTUProbeResult(
						conn.Key,
						result.UploadBytes,
						result.UploadChars,
						result.DownloadBytes,
						result.ResolveTime,
						true,
					)
					c.balancer.SetConnectionValidity(conn.Key, true)

					speedKBps, score := CalculateResolverScore(result.DownloadBytes, result.ResolveTime, 0.0)
					speedStr := FormatResolverSpeed(speedKBps)

					if c.log != nil && c.log.Enabled(logger.LevelInfo) {
						c.log.Infof("⚡ <green>[FastStart Discovery]</green> Verified resolver <cyan>%s</cyan> (Speed: <cyan>%s</cyan>, Score: <cyan>%.1f</cyan>)",
							conn.ResolverLabel, speedStr, score)
					}

					// Update auto-ranked header block in client_resolvers.txt
					c.SaveRankedResolversToFile()
				}

				// Pacing between checks to protect tunnel throughput
				select {
				case <-ctx.Done():
					return
				case <-time.After(80 * time.Millisecond):
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

	if c.log != nil && c.log.Enabled(logger.LevelInfo) {
		c.log.Infof("⚡ <green>[FastStart Discovery]</green> Background candidate sweep completed.")
	}
}
