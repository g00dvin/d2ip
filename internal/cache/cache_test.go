package cache

import (
	"context"
	"fmt"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func openTestDB(t *testing.T) *SQLiteCache {
	t.Helper()
	c, err := Open(context.Background(), ":memory:")
	require.NoError(t, err, "Open(:memory:) should succeed")
	t.Cleanup(func() { _ = c.Close() })
	return c
}

func mustParseAddr(s string) netip.Addr {
	addr, err := netip.ParseAddr(s)
	if err != nil {
		panic(fmt.Sprintf("failed to parse addr %q: %v", s, err))
	}
	return addr
}

func TestOpen_CreatesDatabaseAndRunsMigrations(t *testing.T) {
	c := openTestDB(t)

	var version int
	err := c.db.QueryRowContext(context.Background(), "SELECT COALESCE(MAX(version),0) FROM schema_version").Scan(&version)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, version, 2, "at least v2 migrations should be applied")

	_, err = c.db.ExecContext(context.Background(), "SELECT resolve_status FROM domains LIMIT 0")
	assert.NoError(t, err, "resolve_status column should exist after v2 migration")
}

func TestUpsertBatch_ValidResults_InsertsRecords(t *testing.T) {
	c := openTestDB(t)
	ctx := context.Background()

	now := time.Now()
	results := []ResolveResult{
		{
			Domain:     "example.com",
			IPv4:       []netip.Addr{mustParseAddr("1.2.3.4"), mustParseAddr("5.6.7.8")},
			IPv6:       []netip.Addr{mustParseAddr("::1")},
			Status:     StatusValid,
			ResolvedAt: now,
		},
	}

	err := c.UpsertBatch(ctx, results)
	require.NoError(t, err)

	stats, err := c.Stats(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(1), stats.Domains)
	assert.Equal(t, int64(3), stats.RecordsTotal)
	assert.Equal(t, int64(2), stats.RecordsV4)
	assert.Equal(t, int64(1), stats.RecordsV6)
	assert.Equal(t, int64(3), stats.RecordsValid)
}

func TestUpsertBatch_FailedResults_PersistsDomainStatus(t *testing.T) {
	c := openTestDB(t)
	ctx := context.Background()

	now := time.Now()
	results := []ResolveResult{
		{
			Domain:     "failed.example.com",
			Status:     StatusFailed,
			ResolvedAt: now,
			Err:        fmt.Errorf("SERVFAIL"),
		},
	}

	err := c.UpsertBatch(ctx, results)
	require.NoError(t, err)

	var resolveStatus string
	var lastResolvedAt int64
	err = c.db.QueryRowContext(ctx,
		"SELECT resolve_status, last_resolved_at FROM domains WHERE name=?",
		"failed.example.com").Scan(&resolveStatus, &lastResolvedAt)
	require.NoError(t, err)
	assert.Equal(t, "failed", resolveStatus)
	assert.Greater(t, lastResolvedAt, int64(0))

	stats, err := c.Stats(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(0), stats.RecordsTotal, "no IP records for failed domain")
}

func TestUpsertBatch_NXDomainResults_PersistsStatus(t *testing.T) {
	c := openTestDB(t)
	ctx := context.Background()

	now := time.Now()
	results := []ResolveResult{
		{
			Domain:     "nxdomain.example.com",
			Status:     StatusNXDomain,
			ResolvedAt: now,
			Err:        fmt.Errorf("NXDOMAIN"),
		},
	}

	err := c.UpsertBatch(ctx, results)
	require.NoError(t, err)

	var resolveStatus string
	err = c.db.QueryRowContext(ctx,
		"SELECT resolve_status FROM domains WHERE name=?",
		"nxdomain.example.com").Scan(&resolveStatus)
	require.NoError(t, err)
	assert.Equal(t, "nxdomain", resolveStatus)

	stats, err := c.Stats(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(0), stats.RecordsTotal, "no IP records for nxdomain domain")
}

func TestUpsertBatch_EmptyBatch_ReturnsNil(t *testing.T) {
	c := openTestDB(t)
	ctx := context.Background()

	err := c.UpsertBatch(ctx, []ResolveResult{})
	require.NoError(t, err, "empty batch should return nil")

	err = c.UpsertBatch(ctx, nil)
	require.NoError(t, err, "nil batch should return nil")
}

func TestUpsertBatch_Idempotent_UpsertDoesNotCreateDuplicates(t *testing.T) {
	c := openTestDB(t)
	ctx := context.Background()

	now := time.Now()
	results := []ResolveResult{
		{
			Domain:     "example.com",
			IPv4:       []netip.Addr{mustParseAddr("1.2.3.4")},
			Status:     StatusValid,
			ResolvedAt: now,
		},
	}

	err := c.UpsertBatch(ctx, results)
	require.NoError(t, err)

	later := now.Add(1 * time.Hour)
	results[0].ResolvedAt = later
	results[0].IPv4 = []netip.Addr{mustParseAddr("1.2.3.4")}
	err = c.UpsertBatch(ctx, results)
	require.NoError(t, err)

	stats, err := c.Stats(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(1), stats.Domains, "domain count should not increase on re-upsert")
	assert.Equal(t, int64(1), stats.RecordsTotal, "record count should not increase on re-upsert")
}

func TestNeedsRefresh_FreshDomain_ReturnsEmpty(t *testing.T) {
	c := openTestDB(t)
	ctx := context.Background()

	now := time.Now()
	results := []ResolveResult{
		{
			Domain:     "fresh.example.com",
			IPv4:       []netip.Addr{mustParseAddr("1.2.3.4")},
			Status:     StatusValid,
			ResolvedAt: now,
		},
	}

	err := c.UpsertBatch(ctx, results)
	require.NoError(t, err)

	stale, err := c.NeedsRefresh(ctx, []string{"fresh.example.com"}, 6*time.Hour, 30*time.Minute)
	require.NoError(t, err)
	assert.Empty(t, stale, "freshly resolved domain should not be stale")
}

func TestNeedsRefresh_StaleDomain_ReturnsDomain(t *testing.T) {
	c := openTestDB(t)
	ctx := context.Background()

	oldTime := time.Now().Add(-7 * time.Hour)
	results := []ResolveResult{
		{
			Domain:     "stale.example.com",
			IPv4:       []netip.Addr{mustParseAddr("1.2.3.4")},
			Status:     StatusValid,
			ResolvedAt: oldTime,
		},
	}

	err := c.UpsertBatch(ctx, results)
	require.NoError(t, err)

	stale, err := c.NeedsRefresh(ctx, []string{"stale.example.com"}, 6*time.Hour, 30*time.Minute)
	require.NoError(t, err)
	assert.Contains(t, stale, "stale.example.com", "old domain should be stale")
}

func TestNeedsRefresh_UnknownDomain_ReturnsDomain(t *testing.T) {
	c := openTestDB(t)
	ctx := context.Background()

	stale, err := c.NeedsRefresh(ctx, []string{"unknown.example.com"}, 6*time.Hour, 30*time.Minute)
	require.NoError(t, err)
	assert.Contains(t, stale, "unknown.example.com", "unknown domain should be stale")
}

func TestNeedsRefresh_FailedDomain_RespectsFailedTTL(t *testing.T) {
	c := openTestDB(t)
	ctx := context.Background()

	recentTime := time.Now().Add(-10 * time.Minute)
	results := []ResolveResult{
		{
			Domain:     "recently-failed.example.com",
			Status:     StatusFailed,
			ResolvedAt: recentTime,
		},
	}

	err := c.UpsertBatch(ctx, results)
	require.NoError(t, err)

	stale, err := c.NeedsRefresh(ctx, []string{"recently-failed.example.com"}, 6*time.Hour, 30*time.Minute)
	require.NoError(t, err)
	assert.Empty(t, stale, "recently failed domain within failedTTL should not be stale")

	oldFailedTime := time.Now().Add(-1 * time.Hour)
	results[0].ResolvedAt = oldFailedTime
	err = c.UpsertBatch(ctx, results)
	require.NoError(t, err)

	stale, err = c.NeedsRefresh(ctx, []string{"recently-failed.example.com"}, 6*time.Hour, 30*time.Minute)
	require.NoError(t, err)
	assert.Contains(t, stale, "recently-failed.example.com", "old failed domain beyond failedTTL should be stale")
}

func TestNeedsRefresh_EmptyInput_ReturnsNil(t *testing.T) {
	c := openTestDB(t)
	ctx := context.Background()

	stale, err := c.NeedsRefresh(ctx, []string{}, 6*time.Hour, 30*time.Minute)
	require.NoError(t, err)
	assert.Nil(t, stale)
}

func TestStats_ReturnsCorrectCounts(t *testing.T) {
	c := openTestDB(t)
	ctx := context.Background()

	now := time.Now()
	results := []ResolveResult{
		{
			Domain:     "a.example.com",
			IPv4:       []netip.Addr{mustParseAddr("10.0.0.1"), mustParseAddr("10.0.0.2")},
			IPv6:       []netip.Addr{mustParseAddr("::1")},
			Status:     StatusValid,
			ResolvedAt: now,
		},
		{
			Domain:     "b.example.com",
			IPv4:       []netip.Addr{mustParseAddr("10.0.0.3")},
			Status:     StatusValid,
			ResolvedAt: now,
		},
		{
			Domain:     "c.example.com",
			Status:     StatusFailed,
			ResolvedAt: now,
			Err:        fmt.Errorf("timeout"),
		},
	}

	err := c.UpsertBatch(ctx, results)
	require.NoError(t, err)

	stats, err := c.Stats(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(3), stats.Domains)
	assert.Equal(t, int64(4), stats.RecordsTotal)
	assert.Equal(t, int64(3), stats.RecordsV4)
	assert.Equal(t, int64(1), stats.RecordsV6)
	assert.Equal(t, int64(4), stats.RecordsValid)
	assert.Equal(t, int64(0), stats.RecordsFail)
}

func TestSnapshot_ReturnsValidIPs(t *testing.T) {
	c := openTestDB(t)
	ctx := context.Background()

	now := time.Now()
	results := []ResolveResult{
		{
			Domain:     "example.com",
			IPv4:       []netip.Addr{mustParseAddr("10.0.0.1"), mustParseAddr("10.0.0.2")},
			IPv6:       []netip.Addr{mustParseAddr("2001:db8::1")},
			Status:     StatusValid,
			ResolvedAt: now,
		},
	}

	err := c.UpsertBatch(ctx, results)
	require.NoError(t, err)

	ipv4, ipv6, err := c.Snapshot(ctx)
	require.NoError(t, err)
	assert.Len(t, ipv4, 2)
	assert.Len(t, ipv6, 1)
	assert.Contains(t, ipv4, mustParseAddr("10.0.0.1"))
	assert.Contains(t, ipv4, mustParseAddr("10.0.0.2"))
	assert.Contains(t, ipv6, mustParseAddr("2001:db8::1"))
}

func TestSnapshot_ExcludesFailedRecords(t *testing.T) {
	c := openTestDB(t)
	ctx := context.Background()

	now := time.Now()
	results := []ResolveResult{
		{
			Domain:     "valid.example.com",
			IPv4:       []netip.Addr{mustParseAddr("10.0.0.1")},
			Status:     StatusValid,
			ResolvedAt: now,
		},
		{
			Domain:     "failed.example.com",
			IPv4:       []netip.Addr{mustParseAddr("10.0.0.2")},
			Status:     StatusFailed,
			ResolvedAt: now,
		},
	}

	err := c.UpsertBatch(ctx, results)
	require.NoError(t, err)

	ipv4, _, err := c.Snapshot(ctx)
	require.NoError(t, err)
	assert.Len(t, ipv4, 1)
	assert.Contains(t, ipv4, mustParseAddr("10.0.0.1"))
}

func TestVacuum_DeletesOldRecords(t *testing.T) {
	c := openTestDB(t)
	ctx := context.Background()

	oldTime := time.Now().Add(-48 * time.Hour)
	recentTime := time.Now()

	results := []ResolveResult{
		{
			Domain:     "old.example.com",
			IPv4:       []netip.Addr{mustParseAddr("10.0.0.1")},
			Status:     StatusValid,
			ResolvedAt: oldTime,
		},
		{
			Domain:     "recent.example.com",
			IPv4:       []netip.Addr{mustParseAddr("10.0.0.2")},
			Status:     StatusValid,
			ResolvedAt: recentTime,
		},
	}

	err := c.UpsertBatch(ctx, results)
	require.NoError(t, err)

	deleted, err := c.Vacuum(ctx, 24*time.Hour)
	require.NoError(t, err)
	assert.Equal(t, 1, deleted, "should delete 1 old record")

	stats, err := c.Stats(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(1), stats.RecordsTotal, "only recent record should remain")
}

func TestKV_CRUD(t *testing.T) {
	c := openTestDB(t)
	ctx := context.Background()

	kv, err := c.GetAll(ctx)
	require.NoError(t, err)
	assert.Empty(t, kv, "initially there should be no KV entries")

	err = c.Set(ctx, "resolver.qps", "500")
	require.NoError(t, err)

	err = c.Set(ctx, "cache.ttl", "6h0m0s")
	require.NoError(t, err)

	kv, err = c.GetAll(ctx)
	require.NoError(t, err)
	assert.Equal(t, "500", kv["resolver.qps"])
	assert.Equal(t, "6h0m0s", kv["cache.ttl"])

	err = c.Delete(ctx, "resolver.qps")
	require.NoError(t, err)

	kv, err = c.GetAll(ctx)
	require.NoError(t, err)
	assert.NotContains(t, kv, "resolver.qps")
	assert.Equal(t, "6h0m0s", kv["cache.ttl"])
}

func TestEnsureAllDomainsWithStatus_UpdatesExisting(t *testing.T) {
	c := openTestDB(t)
	ctx := context.Background()

	now := time.Now()
	results := []ResolveResult{
		{
			Domain:     "example.com",
			IPv4:       []netip.Addr{mustParseAddr("1.2.3.4")},
			Status:     StatusValid,
			ResolvedAt: now,
		},
	}

	err := c.UpsertBatch(ctx, results)
	require.NoError(t, err)

	var status string
	err = c.db.QueryRowContext(ctx, "SELECT resolve_status FROM domains WHERE name=?", "example.com").Scan(&status)
	require.NoError(t, err)
	assert.Equal(t, "valid", status)

	later := now.Add(1 * time.Hour)
	results[0].Status = StatusFailed
	results[0].ResolvedAt = later
	results[0].IPv4 = nil
	err = c.UpsertBatch(ctx, results)
	require.NoError(t, err)

	err = c.db.QueryRowContext(ctx, "SELECT resolve_status FROM domains WHERE name=?", "example.com").Scan(&status)
	require.NoError(t, err)
	assert.Equal(t, "failed", status, "status should be updated to failed")
}