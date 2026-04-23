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

// ensureAllDomainsWithStatus inserts/updates all domain rows with their
// resolve_status and last_resolved_at. It returns a map of domain name → id.
func (c *SQLiteCache) ensureAllDomainsWithStatus(ctx context.Context, domains []string, statuses []domainStatus) (map[string]int64, error) {
	if len(domains) == 0 {
		return make(map[string]int64), nil
	}

	tx, err := c.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tx for domains: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Step 1: INSERT OR IGNORE for all domains.
	for i := 0; i < len(domains); i += maxParamsPerStmt {
		end := i + maxParamsPerStmt
		if end > len(domains) {
			end = len(domains)
		}
		batch := domains[i:end]

		placeholders := make([]string, len(batch))
		args := make([]interface{}, len(batch))
		for j, name := range batch {
			placeholders[j] = "(?)"
			args[j] = name
		}

		stmt := "INSERT INTO domains(name) VALUES " + strings.Join(placeholders, ",") + " ON CONFLICT(name) DO NOTHING"
		if _, err := tx.ExecContext(ctx, stmt, args...); err != nil {
			return nil, fmt.Errorf("insert domains batch [%d:%d]: %w", i, end, err)
		}
	}

	// Step 2: Update resolve_status and last_resolved_at for each domain.
	for _, ds := range statuses {
		_, err := tx.ExecContext(ctx,
			`UPDATE domains SET resolve_status=?, last_resolved_at=? WHERE name=?`,
			ds.resolveStatus, ds.lastResolvedAt, ds.name)
		if err != nil {
			return nil, fmt.Errorf("update domain status %q: %w", ds.name, err)
		}
	}

	// Step 3: SELECT all ids.
	idMap := make(map[string]int64, len(domains))
	for i := 0; i < len(domains); i += maxParamsPerStmt {
		end := i + maxParamsPerStmt
		if end > len(domains) {
			end = len(domains)
		}
		batch := domains[i:end]

		placeholders := make([]string, len(batch))
		args := make([]interface{}, len(batch))
		for j, name := range batch {
			placeholders[j] = "?"
			args[j] = name
		}

		query := "SELECT id, name FROM domains WHERE name IN (" + strings.Join(placeholders, ",") + ")"
		rows, err := tx.QueryContext(ctx, query, args...)
		if err != nil {
			return nil, fmt.Errorf("select domain ids batch [%d:%d]: %w", i, end, err)
		}

		for rows.Next() {
			var id int64
			var name string
			if err := rows.Scan(&id, &name); err != nil {
				rows.Close()
				return nil, fmt.Errorf("scan domain row: %w", err)
			}
			idMap[name] = id
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return nil, fmt.Errorf("iterate domain rows: %w", err)
		}
		rows.Close()
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit domains tx: %w", err)
	}

	return idMap, nil
}