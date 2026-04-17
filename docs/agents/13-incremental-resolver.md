# Agent 13 — Incremental Resolver Updates

**Model:** Opus (complex concurrency + state management)  
**Priority:** 🟡 LOW  
**Effort:** 16 hours  
**Iteration:** 9

## Goal

Implement incremental DNS resolution to only re-resolve domains that changed or are stale (cache expired), skipping unchanged domains. This reduces pipeline run time by 50%+ for incremental changes.

## Background

**Current behavior (FULL batch resolution):**
```
1. Fetch dlc.dat (10k domains)
2. Parse all 10k domains
3. Resolve ALL 10k domains (even if cached, even if unchanged)
4. Upsert all results to cache
5. Aggregate and export
```

**Problem:** If only 100 domains changed (1%), we still resolve all 10k (99% waste).

**Goal (INCREMENTAL resolution):**
```
1. Fetch dlc.dat (10k domains)
2. Parse all 10k domains
3. Compare with previous run → 9,900 unchanged, 100 changed/new
4. Check cache for unchanged domains → 9,800 still valid (TTL not expired), 100 stale
5. Resolve only 200 domains (100 changed + 100 stale)
6. Upsert only 200 results
7. Aggregate and export (still full export, but faster input)
```

**Benefit:** 50-95% reduction in DNS queries, faster pipeline runs, less load on upstream DNS.

## Files Involved

### Modified Files
- `internal/orchestrator/orchestrator.go` — Add change detection step
- `internal/domainlist/provider.go` — Track domain hashes/fingerprints
- `internal/resolver/resolver.go` — Support partial batches
- `internal/cache/cache.go` — Check TTL expiry for domains

### New Files
- `internal/orchestrator/diff.go` — Domain diff logic (changed, stale, unchanged)
- `internal/cache/migrations/003_incremental.sql` — Add `last_parsed_hash` column

## Requirements

### 1. Domain Change Detection

**Approach:** Hash domain list to detect changes.

```go
// internal/orchestrator/diff.go (NEW)

package orchestrator

import (
    "crypto/sha256"
    "encoding/hex"
    "sort"
)

type DomainDiff struct {
    Changed    []string // New or modified domains
    Stale      []string // Cached but TTL expired
    Unchanged  []string // Cached and fresh
    Removed    []string // In cache but not in current list
}

// ComputeDiff compares current domains with cache state
func (o *Orchestrator) ComputeDiff(ctx context.Context, currentDomains []string) (*DomainDiff, error) {
    // 1. Compute fingerprint of current domain list
    fingerprint := computeFingerprint(currentDomains)
    
    // 2. Get previous fingerprint from cache metadata
    prevFingerprint, err := o.cache.GetMetadata("domain_list_fingerprint")
    if err != nil || prevFingerprint == "" {
        // First run or no previous fingerprint → resolve all
        return &DomainDiff{
            Changed: currentDomains,
        }, nil
    }
    
    // 3. If fingerprint unchanged, check individual domain TTLs
    if fingerprint == prevFingerprint {
        // No new domains, but some may be stale
        stale, fresh, err := o.cache.CheckStaleDomains(ctx, currentDomains)
        if err != nil {
            return nil, err
        }
        
        return &DomainDiff{
            Changed:   []string{}, // No changes
            Stale:     stale,
            Unchanged: fresh,
        }, nil
    }
    
    // 4. Fingerprint changed → detect added/removed domains
    prevDomains, err := o.cache.GetDomainList()
    if err != nil {
        // Fallback: resolve all
        return &DomainDiff{Changed: currentDomains}, nil
    }
    
    prevSet := toSet(prevDomains)
    currSet := toSet(currentDomains)
    
    var changed, removed []string
    
    // Find new domains (in current but not in previous)
    for domain := range currSet {
        if !prevSet[domain] {
            changed = append(changed, domain)
        }
    }
    
    // Find removed domains (in previous but not in current)
    for domain := range prevSet {
        if !currSet[domain] {
            removed = append(removed, domain)
        }
    }
    
    // Find unchanged domains (in both lists)
    var unchanged []string
    for domain := range currSet {
        if prevSet[domain] {
            unchanged = append(unchanged, domain)
        }
    }
    
    // Check TTL for unchanged domains
    stale, fresh, err := o.cache.CheckStaleDomains(ctx, unchanged)
    if err != nil {
        // Fallback: treat all unchanged as stale
        stale = unchanged
        fresh = []string{}
    }
    
    // Merge changed + stale as domains to resolve
    toResolve := append(changed, stale...)
    
    return &DomainDiff{
        Changed:   toResolve,
        Stale:     stale,
        Unchanged: fresh,
        Removed:   removed,
    }, nil
}

func computeFingerprint(domains []string) string {
    // Sort to ensure consistent hash regardless of order
    sorted := make([]string, len(domains))
    copy(sorted, domains)
    sort.Strings(sorted)
    
    h := sha256.New()
    for _, domain := range sorted {
        h.Write([]byte(domain))
        h.Write([]byte("\n"))
    }
    return hex.EncodeToString(h.Sum(nil))
}

func toSet(domains []string) map[string]bool {
    set := make(map[string]bool, len(domains))
    for _, d := range domains {
        set[d] = true
    }
    return set
}
```

### 2. Cache TTL Check

Add method to check which domains are stale:

```go
// internal/cache/cache.go (MODIFIED)

// CheckStaleDomains returns domains with expired TTL (stale) and valid TTL (fresh)
func (c *SQLiteCache) CheckStaleDomains(ctx context.Context, domains []string) (stale, fresh []string, err error) {
    if len(domains) == 0 {
        return nil, nil, nil
    }
    
    // Build query: SELECT domain FROM domains WHERE domain IN (?) AND updated_at < ?
    staleCutoff := time.Now().Add(-c.ttl) // Domains updated before this are stale
    
    placeholders := make([]string, len(domains))
    args := make([]interface{}, len(domains)+1)
    for i, d := range domains {
        placeholders[i] = "?"
        args[i] = d
    }
    args[len(domains)] = staleCutoff
    
    query := fmt.Sprintf(`
        SELECT domain FROM domains
        WHERE domain IN (%s) AND updated_at < ?
    `, strings.Join(placeholders, ","))
    
    rows, err := c.db.QueryContext(ctx, query, args...)
    if err != nil {
        return nil, nil, err
    }
    defer rows.Close()
    
    staleSet := make(map[string]bool)
    for rows.Next() {
        var domain string
        if err := rows.Scan(&domain); err != nil {
            return nil, nil, err
        }
        staleSet[domain] = true
        stale = append(stale, domain)
    }
    
    // Domains not in staleSet are fresh
    for _, domain := range domains {
        if !staleSet[domain] {
            fresh = append(fresh, domain)
        }
    }
    
    return stale, fresh, rows.Err()
}

// GetMetadata retrieves a metadata key-value pair
func (c *SQLiteCache) GetMetadata(key string) (string, error) {
    var value string
    err := c.db.QueryRow(`SELECT value FROM metadata WHERE key = ?`, key).Scan(&value)
    if err == sql.ErrNoRows {
        return "", nil
    }
    return value, err
}

// SetMetadata stores a metadata key-value pair
func (c *SQLiteCache) SetMetadata(key, value string) error {
    _, err := c.db.Exec(`
        INSERT INTO metadata (key, value, updated_at)
        VALUES (?, ?, CURRENT_TIMESTAMP)
        ON CONFLICT(key) DO UPDATE SET
            value = excluded.value,
            updated_at = CURRENT_TIMESTAMP
    `, key, value)
    return err
}

// GetDomainList returns all domains in cache
func (c *SQLiteCache) GetDomainList() ([]string, error) {
    rows, err := c.db.Query(`SELECT domain FROM domains ORDER BY domain`)
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    
    var domains []string
    for rows.Next() {
        var domain string
        if err := rows.Scan(&domain); err != nil {
            return nil, err
        }
        domains = append(domains, domain)
    }
    return domains, rows.Err()
}
```

### 3. Schema Migration

Add metadata table for storing fingerprints:

```sql
-- internal/cache/migrations/003_incremental.sql (NEW)

CREATE TABLE IF NOT EXISTS metadata (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_metadata_updated_at ON metadata(updated_at);
```

### 4. Orchestrator Integration

Modify `Run()` to use incremental resolution:

```go
// internal/orchestrator/orchestrator.go (MODIFIED)

func (o *Orchestrator) Run(ctx context.Context, req PipelineRequest) (*PipelineReport, error) {
    // ... existing steps 1-2: fetch + parse ...
    
    // Step 3: Compute diff (NEW)
    diff, err := o.ComputeDiff(ctx, domains)
    if err != nil {
        return nil, fmt.Errorf("compute diff: %w", err)
    }
    
    report.DomainsTotal = len(domains)
    report.DomainsChanged = len(diff.Changed)
    report.DomainsStale = len(diff.Stale)
    report.DomainsUnchanged = len(diff.Unchanged)
    report.DomainsRemoved = len(diff.Removed)
    
    // Skip resolution if nothing changed and nothing stale
    toResolve := diff.Changed
    if len(toResolve) == 0 {
        o.logger.Info("No domains changed or stale, skipping resolution")
        
        // Still need to aggregate + export (may have removed domains)
        goto aggregation
    }
    
    // Step 4: Resolve only changed/stale domains
    o.logger.Info("Resolving domains", "count", len(toResolve))
    resultCh := o.resolver.ResolveBatch(ctx, toResolve)
    
    // ... existing upsert logic ...
    
aggregation:
    // Step 5-7: Aggregate + export (uses full domain list from cache)
    // ... existing logic ...
    
    // Step 8: Update fingerprint in cache
    fingerprint := computeFingerprint(domains)
    if err := o.cache.SetMetadata("domain_list_fingerprint", fingerprint); err != nil {
        o.logger.Warn("Failed to save fingerprint", "error", err)
    }
    
    return report, nil
}
```

### 5. PipelineReport Extension

```go
// internal/orchestrator/types.go (MODIFIED)

type PipelineReport struct {
    // ... existing fields ...
    
    // Incremental resolution stats (NEW)
    DomainsTotal     int `json:"domains_total"`
    DomainsChanged   int `json:"domains_changed"`
    DomainsStale     int `json:"domains_stale"`
    DomainsUnchanged int `json:"domains_unchanged"`
    DomainsRemoved   int `json:"domains_removed"`
}
```

## Acceptance Criteria

- [ ] Change detection works (SHA256 fingerprint)
- [ ] Unchanged domains skipped if TTL valid
- [ ] Stale domains (TTL expired) re-resolved
- [ ] New domains resolved
- [ ] Removed domains deleted from cache
- [ ] Metrics track incremental stats (domains_changed, domains_stale, etc.)
- [ ] Full export still happens (aggregates all cached results)
- [ ] First run (no previous fingerprint) resolves all domains (fallback)
- [ ] All tests still pass
- [ ] Pipeline run time reduced by 50%+ for incremental changes

## Edge Cases

1. **First run:** No previous fingerprint → resolve all (expected)
2. **Empty domain list:** All removed → clear cache, no resolution
3. **All domains stale:** TTL expired for all → full resolution (fallback)
4. **Cache corruption:** Missing fingerprint or domain list → full resolution (fallback)
5. **Concurrent runs:** Single-flight enforcement prevents race (existing)
6. **TTL changes:** If config `cache.ttl` changes, next run may have different stale set

## Performance Goals

**Baseline (full resolution):**
- 10k domains × 50ms avg = 500s = ~8 minutes

**Incremental (1% changed):**
- 100 changed + 50 stale = 150 domains × 50ms = 7.5s = ~8 seconds
- **Speed-up: 60x faster**

**Incremental (10% changed):**
- 1000 changed + 500 stale = 1500 domains × 50ms = 75s = ~1 minute
- **Speed-up: 8x faster**

**Incremental (100% changed):**
- Same as full resolution (no penalty for worst case)

## Non-Goals

- Partial aggregation (still aggregate full cache, but faster input)
- Partial export (still export full ipv4.txt/ipv6.txt)
- Partial routing apply (still apply full state, but faster pipeline)
- TTL respect from DNS responses (still use internal TTL only)

## Testing Strategy

1. **Unit tests:**
   - `ComputeDiff()` with various domain lists (added, removed, unchanged)
   - Fingerprint computation (order-independent)
   - `CheckStaleDomains()` with mocked cache

2. **Integration tests:**
   - First run → resolve all
   - Second run (no changes) → resolve 0
   - Second run (1 new domain) → resolve 1
   - Second run (100 stale) → resolve 100
   - Domain removed → delete from cache

3. **Performance benchmark:**
   - 10k domains, 1% changed → measure time improvement
   - Compare against baseline (full resolution)

## Deliverables

1. **Diff logic** (`internal/orchestrator/diff.go`)
2. **Cache methods** (`CheckStaleDomains`, `GetMetadata`, `SetMetadata`)
3. **Migration** (`003_incremental.sql`)
4. **Orchestrator integration** (modified `Run()`)
5. **Tests** (unit + integration)
6. **Metrics** (domains_changed, domains_stale, etc.)
7. **Documentation** (`docs/ARCHITECTURE.md` updated)

## Success Metrics

- ✅ 50%+ reduction in pipeline run time for incremental changes
- ✅ Zero domains resolved when nothing changed (and not stale)
- ✅ Fallback to full resolution when needed (first run, cache corruption)
- ✅ All tests pass
- ✅ Metrics show incremental stats (visible in Prometheus)
