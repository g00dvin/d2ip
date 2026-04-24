package cache

import (
	"context"
	"database/sql"
	"fmt"
	"net/netip"
	"sort"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

// Snapshot returns all currently-valid IPv4 and IPv6 addresses, deduplicated
// and sorted canonically.
func (c *SQLiteCache) Snapshot(ctx context.Context) (ipv4, ipv6 []netip.Addr, err error) {
	v4, err := c.snapshotFamily(ctx, "A")
	if err != nil {
		return nil, nil, fmt.Errorf("snapshot v4: %w", err)
	}

	v6, err := c.snapshotFamily(ctx, "AAAA")
	if err != nil {
		return nil, nil, fmt.Errorf("snapshot v6: %w", err)
	}

	return v4, v6, nil
}

// snapshotFamily queries and parses addresses for a single family.
func (c *SQLiteCache) snapshotFamily(ctx context.Context, typ string) ([]netip.Addr, error) {
	query := "SELECT DISTINCT ip FROM records WHERE type=? AND status='valid' ORDER BY ip"
	rows, err := c.db.QueryContext(ctx, query, typ)
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}
	defer rows.Close()

	var out []netip.Addr
	var parseErrors int
	for rows.Next() {
		var ipStr string
		if err := rows.Scan(&ipStr); err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}

		addr, err := netip.ParseAddr(ipStr)
		if err != nil {
			parseErrors++
			log.Warn().Str("ip", ipStr).Str("type", typ).Err(err).Msg("cache: snapshot: unparseable IP, skipping")
			continue
		}
		out = append(out, addr)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate: %w", err)
	}

	if parseErrors > 0 {
		log.Warn().Int("count", parseErrors).Str("type", typ).Msg("cache: snapshot: skipped unparseable IPs")
	}

	// Already sorted by ORDER BY ip, but netip.Addr comparison != string comparison.
	// Re-sort by canonical byte form for determinism.
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i].As16(), out[j].As16()
		for k := 0; k < 16; k++ {
			if a[k] != b[k] {
				return a[k] < b[k]
			}
		}
		return false
	})

	return out, nil
}

// NeedsRefresh returns the subset of `domains` that are stale according to
// the TTL policy (see docs/agents/04-cache.md).
//
// A domain is considered fresh when:
//   - It exists in the domains table with a known resolve_status
//   - Its last_resolved_at is recent enough given the TTL for that status:
//     'valid' → ttl, 'failed'/'nxdomain' → failedTTL, 'unknown' → stale
//
// Domains not present in the domains table are always stale.
func (c *SQLiteCache) NeedsRefresh(ctx context.Context, domains []string, ttl, failedTTL time.Duration) ([]string, error) {
	if len(domains) == 0 {
		return nil, nil
	}

	now := time.Now().Unix()
	validThreshold := now - int64(ttl.Seconds())
	failedThreshold := now - int64(failedTTL.Seconds())

	staleSet := make(map[string]bool, len(domains))

	// Query in batches (SQLite param limit).
	for i := 0; i < len(domains); i += maxParamsPerStmt {
		end := i + maxParamsPerStmt
		if end > len(domains) {
			end = len(domains)
		}
		batch := domains[i:end]

		if err := c.checkStaleBatch(ctx, batch, validThreshold, failedThreshold, staleSet); err != nil {
			return nil, err
		}
	}

	// Domains not in the result set are stale (no row in domains table).
	for _, name := range domains {
		if _, seen := staleSet[name]; !seen {
			staleSet[name] = true
		}
	}

	// Collect stale domain names.
	var stale []string
	for name, isStale := range staleSet {
		if isStale {
			stale = append(stale, name)
		}
	}

	sort.Strings(stale) // deterministic output
	return stale, nil
}

// checkStaleBatch queries a batch of domains and marks fresh ones in staleSet.
func (c *SQLiteCache) checkStaleBatch(ctx context.Context, domains []string, validThresh, failedThresh int64, staleSet map[string]bool) error {
	placeholders := make([]string, len(domains))
	args := make([]interface{}, len(domains))
	for i, name := range domains {
		placeholders[i] = "?"
		args[i] = name
	}

	// Use domain-level resolve_status and last_resolved_at instead of
	// aggregating from the records table. This correctly handles domains
	// that resolved to zero IPs (failed/nxdomain).
	query := `
		SELECT name, resolve_status, last_resolved_at
		FROM domains
		WHERE name IN (` + strings.Join(placeholders, ",") + `)
	`

	rows, err := c.db.QueryContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("query batch: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var name string
		var resolveStatus string
		var lastResolvedAt sql.NullInt64

		if err := rows.Scan(&name, &resolveStatus, &lastResolvedAt); err != nil {
			return fmt.Errorf("scan: %w", err)
		}

		// Domain exists but was never resolved → stale.
		if !lastResolvedAt.Valid || resolveStatus == "unknown" {
			staleSet[name] = true
			continue
		}

		// Choose threshold based on resolve status.
		threshold := validThresh
		if resolveStatus == "failed" || resolveStatus == "nxdomain" {
			threshold = failedThresh
		}

		if lastResolvedAt.Int64 >= threshold {
			staleSet[name] = false
		} else {
			staleSet[name] = true
		}
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate: %w", err)
	}

	return nil
}

// SnapshotForDomains returns the resolved IPv4 and IPv6 addresses for the
// given domain list. Addresses are not deduplicated or sorted.
func (c *SQLiteCache) SnapshotForDomains(ctx context.Context, domains []string) (ipv4 []netip.Addr, ipv6 []netip.Addr, err error) {
	if len(domains) == 0 {
		return nil, nil, nil
	}

	placeholders := make([]string, len(domains))
	args := make([]any, 0, len(domains)+1)
	args = append(args, "valid")
	for i, d := range domains {
		placeholders[i] = "?"
		args = append(args, d)
	}

	query := fmt.Sprintf(
		"SELECT d.name, r.ip, r.type FROM records r JOIN domains d ON r.domain_id = d.id WHERE r.status = ? AND d.name IN (%s)",
		strings.Join(placeholders, ","),
	)

	rows, err := c.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, nil, fmt.Errorf("cache: snapshot query: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var domain string
		var ipStr string
		var typ string
		if err := rows.Scan(&domain, &ipStr, &typ); err != nil {
			continue
		}
		addr, err := netip.ParseAddr(ipStr)
		if err != nil {
			log.Warn().Str("ip", ipStr).Str("domain", domain).Err(err).Msg("cache: snapshot: unparseable IP, skipping")
			continue
		}
		if typ == "A" || addr.Is4() {
			ipv4 = append(ipv4, addr)
		} else {
			ipv6 = append(ipv6, addr)
		}
	}
	return ipv4, ipv6, rows.Err()
}