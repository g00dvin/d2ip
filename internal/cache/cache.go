// Package cache is the persistence agent for d2ip. It owns the SQLite
// database file, the schema migrations, and every read/write path for DNS
// resolution results, runtime kv overrides, category selections, and run
// history.
//
// # Concurrency model
//
// The package is built around a single *sql.DB handle in WAL mode, which
// allows multiple concurrent readers with a single serialized writer.
// SQLite enforces the serialization at the file level, and the Go database
// layer makes *sql.DB safe for concurrent use. The busy_timeout PRAGMA
// (5s) is relied upon to absorb short writer-contention bursts.
//
// # Upsert semantics
//
// Record writes are fully idempotent: the UNIQUE(domain_id, ip, type)
// constraint plus the ON CONFLICT DO UPDATE upsert guarantees that
// re-inserting the same resolve result mutates only the updated_at and
// status columns. No new row is created, so repeated pipeline runs do
// zero record-row allocation when nothing has changed (see the package
// acceptance criteria in docs/agents/04-cache.md).
//
// # TTL model
//
// DNS-reported TTL values are explicitly ignored. Freshness is governed
// entirely by the caller-supplied `ttl` (for status='valid') and
// `failedTTL` (for status='failed') durations. A domain with no records
// at all is always stale.
package cache

import (
	"context"
	"net/netip"
	"time"
)

// Status is the lifecycle outcome of a single ResolveResult as observed by
// the resolver. NXDOMAIN is surfaced separately by the resolver but is
// persisted as 'nxdomain' in the domains table (the records table still
// uses 'failed' for both NXDomain and Failed since those rows only exist
// when a domain has IP addresses).
type Status uint8

// Status enumeration. The zero value is StatusValid because the most
// common code path writes successful resolutions; callers MUST set the
// status explicitly when writing failures.
const (
	StatusValid Status = iota
	StatusFailed
	StatusNXDomain
	StatusUnknown
)

// String renders the Status in the canonical lowercase form used in the
// records.status column (subject to the CHECK constraint).
func (s Status) String() string {
	switch s {
	case StatusValid:
		return "valid"
	case StatusFailed:
		return "failed"
	case StatusNXDomain:
		return "nxdomain"
	case StatusUnknown:
		return "unknown"
	default:
		return "failed"
	}
}

// ResolveResult is the unit of work the cache accepts. Exactly one of
// IPv4 / IPv6 / Err is typically populated; a successful result may
// carry both address families. Failed results carry an empty address
// list and a non-nil Err plus Status=StatusFailed/StatusNXDomain.
//
// This type is duplicated (rather than imported from internal/resolver)
// to keep the cache package import-free at the domain boundary. The
// resolver and orchestrator construct values of this shape directly.
type ResolveResult struct {
	Domain     string
	IPv4       []netip.Addr
	IPv6       []netip.Addr
	Status     Status
	ResolvedAt time.Time
	Err        error
}

// Stats is a cheap snapshot of record-counts by type and status. The
// implementation computes every field with a handful of COUNT queries;
// it does NOT scan the full table.
type Stats struct {
	Domains         int64
	DomainsValid    int64
	DomainsFailed   int64
	DomainsNXDomain int64
	RecordsTotal    int64
	RecordsV4       int64
	RecordsV6       int64
	RecordsValid    int64
	RecordsFail     int64
	RecordsNXDomain int64
	// OldestUpdatedAt is the unix seconds of the oldest still-present
	// record. Zero if the table is empty.
	OldestUpdatedAt int64
	// NewestUpdatedAt is the unix seconds of the freshest record.
	// Zero if the table is empty.
	NewestUpdatedAt int64
}

// Cache is the contract consumed by the orchestrator and web UI. The
// concrete implementation is *SQLiteCache (see sqlite.go).
type Cache interface {
	// NeedsRefresh returns the subset of `domains` that must be re-resolved.
	// Rules:
	//   - domain not present in the domains table → stale.
	//   - domain present but no records           → stale.
	//   - MAX(updated_at) older than now-ttl for status='valid' rows → stale.
	//   - MAX(updated_at) older than now-failedTTL for status='failed' rows → stale.
	NeedsRefresh(ctx context.Context, domains []string, ttl, failedTTL time.Duration) (stale []string, err error)

	// UpsertBatch idempotently persists a batch of ResolveResult values.
	// It opens its own transaction(s) and caps each transaction at a soft
	// limit of 1000 record rows to stay under SQLite's parameter ceiling.
	UpsertBatch(ctx context.Context, results []ResolveResult) error

	// Snapshot returns the deduplicated set of currently-valid IPv4 and
	// IPv6 addresses, ordered canonically by string representation.
	Snapshot(ctx context.Context) (ipv4, ipv6 []netip.Addr, err error)

	// SnapshotForDomains returns the resolved IPv4 and IPv6 addresses for
	// the given domain list. Addresses are not deduplicated or sorted.
	SnapshotForDomains(ctx context.Context, domains []string) (ipv4, ipv6 []netip.Addr, err error)

	// Stats returns record counts by type and status in O(few queries).
	Stats(ctx context.Context) (Stats, error)

	// Vacuum deletes records with updated_at older than now-olderThan and
	// runs `PRAGMA incremental_vacuum` / `VACUUM` if the freelist warrants
	// it. Returns the number of deleted rows.
	Vacuum(ctx context.Context, olderThan time.Duration) (deleted int, err error)

	// Close releases the underlying *sql.DB. Safe to call multiple times.
	Close() error
}

// maxParamsPerStmt is the upper bound on placeholder count for a single
// prepared statement. SQLite's default SQLITE_MAX_VARIABLE_NUMBER is 999
// on legacy builds and 32766 on modern builds; we stay well under both.
const maxParamsPerStmt = 900

// maxRowsPerTx is the soft transaction cap. Large batches are split into
// multiple transactions to bound WAL growth and keep write latency
// predictable even under multi-thousand-domain resolves.
const maxRowsPerTx = 1000
