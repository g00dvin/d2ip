package sourcereg

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/netip"
	"sync"
)

var _ Registry = (*DBRegistry)(nil)

// DBRegistry implements Registry using SQLite for persistence.
type DBRegistry struct {
	db      *sql.DB
	sources map[string]Source
	mu      sync.RWMutex
}

// NewDBRegistry creates a new registry backed by SQLite.
func NewDBRegistry(db *sql.DB) (*DBRegistry, error) {
	if err := migrate(db); err != nil {
		return nil, fmt.Errorf("sourcereg: migrate: %w", err)
	}
	return &DBRegistry{
		db:      db,
		sources: make(map[string]Source),
	}, nil
}

func migrate(db *sql.DB) error {
	// For simplicity, exec the migration inline. In production, use a proper migration tool.
	schema := `
CREATE TABLE IF NOT EXISTS sources (
    id TEXT PRIMARY KEY,
    provider TEXT NOT NULL,
    prefix TEXT NOT NULL UNIQUE,
    enabled INTEGER NOT NULL DEFAULT 1,
    config_json TEXT NOT NULL DEFAULT '{}',
    created_at INTEGER NOT NULL DEFAULT (unixepoch()),
    updated_at INTEGER NOT NULL DEFAULT (unixepoch())
);
CREATE INDEX IF NOT EXISTS idx_sources_prefix ON sources(prefix);
CREATE INDEX IF NOT EXISTS idx_sources_enabled ON sources(enabled);
`
	_, err := db.Exec(schema)
	return err
}

// AddSource creates a new source from config and persists it.
func (r *DBRegistry) AddSource(ctx context.Context, cfg SourceConfig) error {
	if cfg.ID == "" {
		return fmt.Errorf("sourcereg: source id is required")
	}
	if cfg.Prefix == "" {
		return fmt.Errorf("sourcereg: source prefix is required")
	}
	if cfg.Provider == "" {
		return fmt.Errorf("sourcereg: source provider is required")
	}

	// Validate prefix format: lowercase letters, numbers, hyphens only
	for _, ch := range cfg.Prefix {
		if !((ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') || ch == '-') {
			return fmt.Errorf("sourcereg: prefix must be lowercase alphanumeric or hyphen")
		}
	}

	// Validate and create source first
	src, err := createSource(cfg)
	if err != nil {
		return err
	}

	configJSON, err := json.Marshal(cfg.Config)
	if err != nil {
		return fmt.Errorf("sourcereg: marshal config: %w", err)
	}

	_, err = r.db.ExecContext(ctx, `
		INSERT INTO sources (id, provider, prefix, enabled, config_json)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			provider = excluded.provider,
			prefix = excluded.prefix,
			enabled = excluded.enabled,
			config_json = excluded.config_json,
			updated_at = unixepoch()
	`, cfg.ID, string(cfg.Provider), cfg.Prefix, boolToInt(cfg.Enabled), string(configJSON))
	if err != nil {
		return fmt.Errorf("sourcereg: insert source: %w", err)
	}

	// Update in-memory map
	r.mu.Lock()
	old, hadOld := r.sources[cfg.ID]
	r.sources[cfg.ID] = src
	r.mu.Unlock()
	if hadOld {
		_ = old.Close()
	}

	return nil
}

// RemoveSource deletes a source by ID.
func (r *DBRegistry) RemoveSource(ctx context.Context, id string) error {
	r.mu.Lock()
	src, hadSrc := r.sources[id]
	delete(r.sources, id)
	r.mu.Unlock()

	_, err := r.db.ExecContext(ctx, `DELETE FROM sources WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("sourcereg: delete source: %w", err)
	}
	if hadSrc {
		_ = src.Close()
	}
	return nil
}

// GetSource returns a source by ID.
func (r *DBRegistry) GetSource(id string) (Source, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.sources[id]
	return s, ok
}

// ListSources returns all persisted source configs.
func (r *DBRegistry) ListSources() []SourceInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]SourceInfo, 0, len(r.sources))
	for _, s := range r.sources {
		out = append(out, s.Info())
	}
	return out
}

// LoadAll loads all enabled sources from DB into memory.
func (r *DBRegistry) LoadAll(ctx context.Context) error {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, provider, prefix, enabled, config_json FROM sources
	`)
	if err != nil {
		return fmt.Errorf("sourcereg: query sources: %w", err)
	}
	defer rows.Close()

	var skipped int
	newSources := make(map[string]Source)
	for rows.Next() {
		var cfg SourceConfig
		var configJSON string
		var enabledInt int
		if err := rows.Scan(&cfg.ID, &cfg.Provider, &cfg.Prefix, &enabledInt, &configJSON); err != nil {
			skipped++
			continue // skip malformed rows
		}
		cfg.Enabled = enabledInt == 1
		if err := json.Unmarshal([]byte(configJSON), &cfg.Config); err != nil {
			skipped++
			continue
		}

		src, err := createSource(cfg)
		if err != nil {
			skipped++
			continue // skip invalid sources
		}
		newSources[cfg.ID] = src
	}

	if len(newSources) == 0 && skipped > 0 {
		return fmt.Errorf("sourcereg: all %d source rows were malformed", skipped)
	}

	// Load each enabled source (do not hold registry lock during I/O)
	for _, src := range newSources {
		if !src.Info().Enabled {
			continue
		}
		if err := src.Load(ctx); err != nil {
			// Log but continue — don't let one bad source break others
			// In real code, use a logger
			_ = err
		}
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("sourcereg: rows error: %w", err)
	}

	// Swap under lock and close old sources
	r.mu.Lock()
	oldSources := r.sources
	r.sources = newSources
	r.mu.Unlock()

	for _, src := range oldSources {
		_ = src.Close()
	}

	return nil
}

// ListCategories returns all categories across all loaded sources.
func (r *DBRegistry) ListCategories() []CategoryInfo {
	// Collect source references and category names under lock, then
	// call source methods without holding the registry lock.
	type pair struct {
		src  Source
		cat  string
	}
	var pairs []pair

	r.mu.RLock()
	for _, src := range r.sources {
		for _, cat := range src.Categories() {
			pairs = append(pairs, pair{src: src, cat: cat})
		}
	}
	r.mu.RUnlock()

	out := make([]CategoryInfo, 0, len(pairs))
	for _, p := range pairs {
		info := p.src.Info()
		catType := CategoryDomain
		if p.src.IsPrefixSource() {
			catType = CategoryPrefix
		}
		count := 0
		if ds := p.src.AsDomainSource(); ds != nil {
			if d, err := ds.GetDomains(p.cat); err == nil {
				count = len(d)
			}
		} else if ps := p.src.AsPrefixSource(); ps != nil {
			if pr, err := ps.GetPrefixes(p.cat); err == nil {
				count = len(pr)
			}
		}
		out = append(out, CategoryInfo{
			Name:     p.cat,
			SourceID: info.ID,
			Type:     catType,
			Count:    count,
		})
	}
	return out
}

// GetDomains collects domains for a category from its owning source.
func (r *DBRegistry) GetDomains(category string) ([]string, error) {
	src, ok := r.findSourceForCategory(category)
	if !ok {
		return nil, fmt.Errorf("sourcereg: no source found for category %q", category)
	}
	ds := src.AsDomainSource()
	if ds == nil {
		return nil, fmt.Errorf("sourcereg: source %q does not provide domains", src.ID())
	}
	return ds.GetDomains(category)
}

// GetPrefixes collects prefixes for a category from its owning source.
func (r *DBRegistry) GetPrefixes(category string) ([]netip.Prefix, error) {
	src, ok := r.findSourceForCategory(category)
	if !ok {
		return nil, fmt.Errorf("sourcereg: no source found for category %q", category)
	}
	ps := src.AsPrefixSource()
	if ps == nil {
		return nil, fmt.Errorf("sourcereg: source %q does not provide prefixes", src.ID())
	}
	return ps.GetPrefixes(category)
}

// ResolveCategory finds which source owns a category.
func (r *DBRegistry) ResolveCategory(category string) (sourceID string, catType string, found bool) {
	src, ok := r.findSourceForCategory(category)
	if !ok {
		return "", "", false
	}
	if src.IsDomainSource() {
		return src.ID(), string(CategoryDomain), true
	}
	return src.ID(), string(CategoryPrefix), true
}

// Close releases all resources held by the registry and its sources.
func (r *DBRegistry) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, src := range r.sources {
		_ = src.Close()
	}
	r.sources = nil
	return nil
}

func (r *DBRegistry) findSourceForCategory(category string) (Source, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, src := range r.sources {
		for _, cat := range src.Categories() {
			if cat == category {
				return src, true
			}
		}
	}
	return nil, false
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
