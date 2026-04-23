package cache

import (
	"context"
	"fmt"
	"strings"
)

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

	for _, ds := range statuses {
		_, err := tx.ExecContext(ctx,
			`UPDATE domains SET resolve_status=?, last_resolved_at=? WHERE name=?`,
			ds.resolveStatus, ds.lastResolvedAt, ds.name)
		if err != nil {
			return nil, fmt.Errorf("update domain status %q: %w", ds.name, err)
		}
	}

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