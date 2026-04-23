package cache

import (
	"context"
	"fmt"
	"strings"

	"github.com/rs/zerolog/log"
)

// recordRowWithDomain represents a single DNS record to upsert.
type recordRowWithDomain struct {
	domain    string
	ip        string
	typ       string
	updatedAt int64
	status    string
}

func expandResultsWithDomain(results []ResolveResult) []recordRowWithDomain {
	var out []recordRowWithDomain
	for _, r := range results {
		// Record status must satisfy the CHECK(status IN ('valid','failed'))
		// constraint on the records table. NXDomain maps to 'failed' in records
		// (the domain-level resolve_status column keeps the distinction).
		recStatus := "valid"
		if r.Status != StatusValid {
			recStatus = "failed"
		}
		updatedAt := r.ResolvedAt.Unix()

		for _, addr := range r.IPv4 {
			out = append(out, recordRowWithDomain{
				domain:    r.Domain,
				ip:        addr.String(),
				typ:       "A",
				updatedAt: updatedAt,
				status:    recStatus,
			})
		}
		for _, addr := range r.IPv6 {
			out = append(out, recordRowWithDomain{
				domain:    r.Domain,
				ip:        addr.String(),
				typ:       "AAAA",
				updatedAt: updatedAt,
				status:    recStatus,
			})
		}
	}
	return out
}

// domainStatus represents per-domain resolution metadata to persist.
type domainStatus struct {
	name           string
	resolveStatus  string
	lastResolvedAt int64
}

func buildDomainStatuses(results []ResolveResult) []domainStatus {
	out := make([]domainStatus, 0, len(results))
	for _, r := range results {
		out = append(out, domainStatus{
			name:           r.Domain,
			resolveStatus:  r.Status.String(),
			lastResolvedAt: r.ResolvedAt.Unix(),
		})
	}
	return out
}

// UpsertBatch idempotently writes a batch of ResolveResult entries.
// It always ensures domain rows exist (with resolve_status updated) so
// that NeedsRefresh can honour failedTTL for domains with zero IPs.
// Record rows are only written when a result has IP addresses.
func (c *SQLiteCache) UpsertBatch(ctx context.Context, results []ResolveResult) error {
	if len(results) == 0 {
		return nil
	}

	validCount := 0
	failedCount := 0
	nxdomainCount := 0
	for _, r := range results {
		switch r.Status {
		case StatusValid:
			validCount++
		case StatusNXDomain:
			nxdomainCount++
		default:
			failedCount++
		}
	}

	log.Info().
		Int("total", len(results)).
		Int("valid", validCount).
		Int("failed", failedCount).
		Int("nxdomain", nxdomainCount).
		Msg("cache: upsert batch status breakdown")

	rows := expandResultsWithDomain(results)
	domainStatuses := buildDomainStatuses(results)

	// Collect unique domain names from ALL results (not just those with IPs).
	allDomains := make([]string, 0, len(results))
	seen := make(map[string]struct{}, len(results))
	for _, ds := range domainStatuses {
		if _, ok := seen[ds.name]; !ok {
			seen[ds.name] = struct{}{}
			allDomains = append(allDomains, ds.name)
		}
	}

	// Ensure domain rows exist first.
	idMap, err := c.ensureAllDomainsWithStatus(ctx, allDomains, domainStatuses)
	if err != nil {
		return fmt.Errorf("ensure domains: %w", err)
	}

	// If no IP records to write, we're done — domain statuses were updated above.
	if len(rows) == 0 {
		return nil
	}

	// Split record upserts into transactions.
	for i := 0; i < len(rows); i += maxRowsPerTx {
		end := i + maxRowsPerTx
		if end > len(rows) {
			end = len(rows)
		}
		batch := rows[i:end]

		tx, err := c.db.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("begin tx: %w", err)
		}

		// Build bulk upsert.
		placeholders := make([]string, len(batch))
		args := make([]interface{}, 0, len(batch)*5)
		for j, row := range batch {
			domainID, ok := idMap[row.domain]
			if !ok {
				_ = tx.Rollback()
				return fmt.Errorf("domain %q missing from idMap after ensureDomains", row.domain)
			}
			placeholders[j] = "(?,?,?,?,?)"
			args = append(args, domainID, row.ip, row.typ, row.updatedAt, row.status)
		}

		stmt := `INSERT INTO records(domain_id, ip, type, updated_at, status) VALUES ` +
			strings.Join(placeholders, ",") +
			` ON CONFLICT(domain_id, ip, type) DO UPDATE SET updated_at=excluded.updated_at, status=excluded.status`

		if _, err := tx.ExecContext(ctx, stmt, args...); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("upsert records [%d:%d]: %w", i, end, err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit tx [%d:%d]: %w", i, end, err)
		}
	}

	return nil
}