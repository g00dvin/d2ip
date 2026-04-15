package cache

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// Stats returns record counts and age bounds in O(few queries).
func (c *SQLiteCache) Stats(ctx context.Context) (Stats, error) {
	var s Stats

	// Count domains.
	row := c.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM domains")
	if err := row.Scan(&s.Domains); err != nil {
		return s, fmt.Errorf("count domains: %w", err)
	}

	// Count records by type and status.
	row = c.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM records")
	if err := row.Scan(&s.RecordsTotal); err != nil {
		return s, fmt.Errorf("count records total: %w", err)
	}

	row = c.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM records WHERE type='A'")
	if err := row.Scan(&s.RecordsV4); err != nil {
		return s, fmt.Errorf("count v4: %w", err)
	}

	row = c.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM records WHERE type='AAAA'")
	if err := row.Scan(&s.RecordsV6); err != nil {
		return s, fmt.Errorf("count v6: %w", err)
	}

	row = c.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM records WHERE status='valid'")
	if err := row.Scan(&s.RecordsValid); err != nil {
		return s, fmt.Errorf("count valid: %w", err)
	}

	row = c.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM records WHERE status='failed'")
	if err := row.Scan(&s.RecordsFail); err != nil {
		return s, fmt.Errorf("count failed: %w", err)
	}

	// Age bounds.
	var oldest, newest sql.NullInt64
	row = c.db.QueryRowContext(ctx, "SELECT MIN(updated_at), MAX(updated_at) FROM records")
	if err := row.Scan(&oldest, &newest); err != nil {
		return s, fmt.Errorf("age bounds: %w", err)
	}

	if oldest.Valid {
		s.OldestUpdatedAt = oldest.Int64
	}
	if newest.Valid {
		s.NewestUpdatedAt = newest.Int64
	}

	return s, nil
}

// Vacuum deletes old records and runs PRAGMA incremental_vacuum if worthwhile.
func (c *SQLiteCache) Vacuum(ctx context.Context, olderThan time.Duration) (int, error) {
	threshold := time.Now().Add(-olderThan).Unix()

	res, err := c.db.ExecContext(ctx, "DELETE FROM records WHERE updated_at < ?", threshold)
	if err != nil {
		return 0, fmt.Errorf("delete old records: %w", err)
	}

	deleted, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("rows affected: %w", err)
	}

	// Check freelist size (cheap heuristic).
	var freelist int64
	row := c.db.QueryRowContext(ctx, "PRAGMA freelist_count")
	if err := row.Scan(&freelist); err != nil {
		return int(deleted), fmt.Errorf("freelist_count: %w (deleted %d rows)", err, deleted)
	}

	// If freelist > 10% of page_count, run incremental_vacuum.
	var pageCount int64
	row = c.db.QueryRowContext(ctx, "PRAGMA page_count")
	if err := row.Scan(&pageCount); err != nil {
		return int(deleted), fmt.Errorf("page_count: %w", err)
	}

	if pageCount > 0 && float64(freelist)/float64(pageCount) > 0.1 {
		// VACUUM requires an exclusive lock; serialize with mu.
		c.mu.Lock()
		defer c.mu.Unlock()

		if _, err := c.db.ExecContext(ctx, "PRAGMA incremental_vacuum"); err != nil {
			return int(deleted), fmt.Errorf("incremental_vacuum: %w", err)
		}
	}

	return int(deleted), nil
}
