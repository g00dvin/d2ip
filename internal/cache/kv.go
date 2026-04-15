package cache

import (
	"context"
	"fmt"
)

// KVStore implementation (satisfies config.KVStore interface).

// GetAll returns all kv_settings rows as a map.
func (c *SQLiteCache) GetAll(ctx context.Context) (map[string]string, error) {
	rows, err := c.db.QueryContext(ctx, "SELECT key, value FROM kv_settings")
	if err != nil {
		return nil, fmt.Errorf("query kv_settings: %w", err)
	}
	defer rows.Close()

	kv := make(map[string]string)
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return nil, fmt.Errorf("scan kv row: %w", err)
		}
		kv[key] = value
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate kv rows: %w", err)
	}

	return kv, nil
}

// Set upserts a key-value pair.
func (c *SQLiteCache) Set(ctx context.Context, key, value string) error {
	_, err := c.db.ExecContext(ctx,
		"INSERT INTO kv_settings(key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value=excluded.value",
		key, value)
	if err != nil {
		return fmt.Errorf("upsert kv_settings: %w", err)
	}
	return nil
}

// Delete removes a key.
func (c *SQLiteCache) Delete(ctx context.Context, key string) error {
	_, err := c.db.ExecContext(ctx, "DELETE FROM kv_settings WHERE key=?", key)
	if err != nil {
		return fmt.Errorf("delete kv_settings: %w", err)
	}
	return nil
}
