package cache

import (
	"context"
	"fmt"
	"strings"
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
		status := r.Status.String()
		updatedAt := r.ResolvedAt.Unix()

		for _, addr := range r.IPv4 {
			out = append(out, recordRowWithDomain{
				domain:    r.Domain,
				ip:        addr.String(),
				typ:       "A",
				updatedAt: updatedAt,
				status:    status,
			})
		}
		for _, addr := range r.IPv6 {
			out = append(out, recordRowWithDomain{
				domain:    r.Domain,
				ip:        addr.String(),
				typ:       "AAAA",
				updatedAt: updatedAt,
				status:    status,
			})
		}
	}
	return out
}

// UpsertBatch idempotently writes a batch of ResolveResult entries.
// Large batches are split into multiple transactions (maxRowsPerTx cap).
func (c *SQLiteCache) UpsertBatch(ctx context.Context, results []ResolveResult) error {
	if len(results) == 0 {
		return nil
	}

	rows := expandResultsWithDomain(results)
	if len(rows) == 0 {
		// All results were failures with no IPs — nothing to insert.
		return nil
	}

	// Collect unique domains.
	domainSet := make(map[string]struct{}, len(results))
	for _, row := range rows {
		domainSet[row.domain] = struct{}{}
	}
	domains := make([]string, 0, len(domainSet))
	for name := range domainSet {
		domains = append(domains, name)
	}

	// Split into transactions.
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

		idMap, err := c.ensureDomains(ctx, tx, domains)
		if err != nil {
			_ = tx.Rollback()
			return err
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
