package cache

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"

	// Register the modernc.org/sqlite driver under the "sqlite" name.
	_ "modernc.org/sqlite"

	"github.com/goodvin/d2ip/migrations"
)

// SQLiteCache is the modernc.org/sqlite-backed Cache implementation. It is
// safe for concurrent use from multiple goroutines; mu serializes only the
// Close path and the vacuum-pragma path (where a single, exclusive
// connection is required).
type SQLiteCache struct {
	db     *sql.DB
	path   string // original user-visible path (":memory:" or filesystem)
	closed bool
	mu     sync.Mutex
}

// Ensure *SQLiteCache satisfies the Cache contract at compile time.
var _ Cache = (*SQLiteCache)(nil)

// Open returns a *SQLiteCache backed by the SQLite file at dbPath. Pass
// ":memory:" for an ephemeral in-process database (used by tests). The
// PRAGMAs from docs/SCHEMA.md are applied and any pending migrations from
// the embedded migrations.FS are executed.
//
// For file-backed databases the connection string sets _pragma parameters
// inline so that every connection in the pool (modernc.org/sqlite opens a
// fresh SQLite connection per *sql.Conn) inherits the same settings.
func Open(ctx context.Context, dbPath string) (*SQLiteCache, error) {
	if strings.TrimSpace(dbPath) == "" {
		return nil, errors.New("cache.Open: empty db path")
	}

	dsn := buildDSN(dbPath)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("cache.Open: sql.Open: %w", err)
	}

	// modernc.org/sqlite is safe for concurrent use but a single writer
	// serializes under the hood. For :memory: we MUST cap to a single
	// connection otherwise each conn gets its own private database.
	if dbPath == ":memory:" || strings.Contains(dbPath, "mode=memory") {
		db.SetMaxOpenConns(1)
	} else {
		// A modest pool: reads are cheap, writer will serialize anyway.
		db.SetMaxOpenConns(8)
		db.SetMaxIdleConns(2)
		db.SetConnMaxIdleTime(5 * time.Minute)
	}

	c := &SQLiteCache{db: db, path: dbPath}

	if err := c.applyPragmas(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("cache.Open: pragmas: %w", err)
	}

	if err := c.migrate(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("cache.Open: migrate: %w", err)
	}

	return c, nil
}

// buildDSN assembles the modernc.org/sqlite DSN with the mandatory
// PRAGMAs from docs/SCHEMA.md. The driver accepts `_pragma=...` query
// parameters which it applies on every fresh SQLite connection, which is
// important because PRAGMA settings are per-connection in SQLite.
func buildDSN(dbPath string) string {
	if dbPath == ":memory:" {
		// Shared cache so the single-conn pool sees a consistent image.
		// (Technically unnecessary with MaxOpenConns(1), but defensive.)
		return "file::memory:?cache=shared&_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)&_pragma=temp_store(2)"
	}

	// Build a URI-form DSN with escaped path so spaces/unicode work.
	q := url.Values{}
	q.Add("_pragma", "journal_mode(WAL)")
	q.Add("_pragma", "synchronous(NORMAL)")
	q.Add("_pragma", "foreign_keys(1)")
	q.Add("_pragma", "busy_timeout(5000)")
	q.Add("_pragma", "temp_store(2)") // MEMORY

	return "file:" + dbPath + "?" + q.Encode()
}

// applyPragmas issues the PRAGMAs once on a fresh connection and verifies
// that WAL mode is actually in effect (journal_mode is persistent in the
// database file, but we re-assert it to handle first-creation).
func (c *SQLiteCache) applyPragmas(ctx context.Context) error {
	// For :memory: journal_mode cannot be WAL; SQLite silently downgrades.
	if c.path != ":memory:" {
		if _, err := c.db.ExecContext(ctx, "PRAGMA journal_mode=WAL"); err != nil {
			return fmt.Errorf("set journal_mode=WAL: %w", err)
		}
	}
	for _, stmt := range []string{
		"PRAGMA synchronous=NORMAL",
		"PRAGMA foreign_keys=ON",
		"PRAGMA busy_timeout=5000",
		"PRAGMA temp_store=MEMORY",
	} {
		if _, err := c.db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("%s: %w", stmt, err)
		}
	}
	return nil
}

// migrate executes every *.sql migration embedded in migrations.FS whose
// numeric prefix is greater than the current max(version) in
// schema_version. Migrations are applied in lexicographic order inside a
// single transaction per file; a failure aborts the transaction (so the
// version row is never inserted for a half-applied migration).
func (c *SQLiteCache) migrate(ctx context.Context) error {
	// Ensure schema_version exists so we can read the current level even
	// on a pristine database (the 0001 migration creates it, but we must
	// be able to SELECT before running 0001 too).
	if _, err := c.db.ExecContext(ctx, `
        CREATE TABLE IF NOT EXISTS schema_version (
            version    INTEGER PRIMARY KEY,
            applied_at INTEGER NOT NULL
        )
    `); err != nil {
		return fmt.Errorf("bootstrap schema_version: %w", err)
	}

	current, err := c.currentSchemaVersion(ctx)
	if err != nil {
		return err
	}

	entries, err := fs.ReadDir(migrations.FS, ".")
	if err != nil {
		return fmt.Errorf("read embedded migrations: %w", err)
	}

	files := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		files = append(files, e.Name())
	}
	sort.Strings(files)

	for _, name := range files {
		ver, ok := parseMigrationVersion(name)
		if !ok {
			log.Warn().Str("file", name).Msg("cache: skipping migration with unparseable version prefix")
			continue
		}
		if ver <= current {
			continue
		}

		body, err := fs.ReadFile(migrations.FS, name)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", name, err)
		}

		if err := c.applyMigration(ctx, ver, name, string(body)); err != nil {
			return err
		}
		log.Info().Str("file", name).Int("version", ver).Msg("cache: migration applied")
	}

	return nil
}

// currentSchemaVersion returns the max(version) already applied, or 0 if
// the schema_version table is empty.
func (c *SQLiteCache) currentSchemaVersion(ctx context.Context) (int, error) {
	var v sql.NullInt64
	row := c.db.QueryRowContext(ctx, "SELECT COALESCE(MAX(version),0) FROM schema_version")
	if err := row.Scan(&v); err != nil {
		return 0, fmt.Errorf("read schema_version: %w", err)
	}
	if !v.Valid {
		return 0, nil
	}
	return int(v.Int64), nil
}

// applyMigration runs a single migration body inside a transaction. The
// schema_version row is inserted in the same tx so that either the whole
// migration + bookkeeping commits together, or nothing does.
func (c *SQLiteCache) applyMigration(ctx context.Context, version int, name, body string) error {
	tx, err := c.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx for %s: %w", name, err)
	}
	defer func() { _ = tx.Rollback() }() // no-op if committed

	if _, err := tx.ExecContext(ctx, body); err != nil {
		return fmt.Errorf("exec %s: %w", name, err)
	}
	// The 0001_init migration already inserts its own version row; the
	// ON CONFLICT DO NOTHING there means our upsert below is harmless.
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO schema_version(version, applied_at) VALUES (?, ?)
         ON CONFLICT(version) DO NOTHING`,
		version, time.Now().Unix()); err != nil {
		return fmt.Errorf("record version %d: %w", version, err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit %s: %w", name, err)
	}
	return nil
}

// parseMigrationVersion extracts the leading integer prefix of a
// migration filename (e.g. "0001_init.sql" → 1). Returns false if the
// name is not in the expected "NNNN_*.sql" form.
func parseMigrationVersion(name string) (int, bool) {
	// Strip extension.
	stem := strings.TrimSuffix(name, ".sql")
	// Take the part before the first underscore.
	sep := strings.IndexByte(stem, '_')
	var numPart string
	if sep < 0 {
		numPart = stem
	} else {
		numPart = stem[:sep]
	}
	if numPart == "" {
		return 0, false
	}
	var n int
	for _, r := range numPart {
		if r < '0' || r > '9' {
			return 0, false
		}
		n = n*10 + int(r-'0')
	}
	return n, true
}

// DB exposes the underlying *sql.DB for advanced callers (metrics, tests).
// Most consumers should use the Cache interface methods instead.
func (c *SQLiteCache) DB() *sql.DB { return c.db }

// Close releases the database connection. Idempotent.
func (c *SQLiteCache) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil
	}
	c.closed = true
	return c.db.Close()
}
