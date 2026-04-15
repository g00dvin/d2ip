package cache

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// ensureDomains bulk-inserts domain names (idempotent) and returns a map of
// name → domain_id. This is the first step of UpsertBatch: we must have an
// id before inserting records.
func (c *SQLiteCache) ensureDomains(ctx context.Context, tx *sql.Tx, domains []string) (map[string]int64, error) {
	if len(domains) == 0 {
		return make(map[string]int64), nil
	}

	// Step 1: INSERT OR IGNORE in batches (SQLite param limit ~999).
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

	// Step 2: SELECT all ids in batches.
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

	return idMap, nil
}
