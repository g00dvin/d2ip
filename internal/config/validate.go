package config

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"
)

// Validate returns a slice of every rule violation found in c. An empty slice
// means the configuration is acceptable. The binary MUST refuse to start when
// this returns non-empty.
//
// Rules are sourced from docs/CONFIG.md; when adding new constraints keep the
// error messages actionable (include field name and offending value).
func (c *Config) Validate() []error {
	var errs []error

	if strings.TrimSpace(c.Listen) == "" {
		errs = append(errs, errors.New("listen: must not be empty"))
	} else if _, _, err := net.SplitHostPort(c.Listen); err != nil {
		errs = append(errs, fmt.Errorf("listen: invalid address %q: %w", c.Listen, err))
	}

	errs = append(errs, validateSource(c.Source)...)
	errs = append(errs, validateCategories(c.Categories)...)
	errs = append(errs, validateResolver(c.Resolver)...)
	errs = append(errs, validateCache(c.Cache)...)
	errs = append(errs, validateAggregation(c.Aggregation)...)
	errs = append(errs, validateExport(c.Export)...)
	errs = append(errs, validateRouting(c.Routing)...)
	errs = append(errs, validateScheduler(c.Scheduler)...)
	errs = append(errs, validateLogging(c.Logging)...)
	errs = append(errs, validateMetrics(c.Metrics)...)

	return errs
}

func validateSource(s SourceConfig) []error {
	var errs []error
	if strings.TrimSpace(s.URL) == "" {
		errs = append(errs, errors.New("source.url: must not be empty"))
	} else if !strings.HasPrefix(s.URL, "http://") && !strings.HasPrefix(s.URL, "https://") {
		errs = append(errs, fmt.Errorf("source.url: must be http(s) URL, got %q", s.URL))
	}
	if strings.TrimSpace(s.CachePath) == "" {
		errs = append(errs, errors.New("source.cache_path: must not be empty"))
	}
	if s.RefreshInterval < time.Minute {
		errs = append(errs, fmt.Errorf("source.refresh_interval: must be >= 1m, got %s", s.RefreshInterval))
	}
	if s.HTTPTimeout < time.Second {
		errs = append(errs, fmt.Errorf("source.http_timeout: must be >= 1s, got %s", s.HTTPTimeout))
	}
	return errs
}

func validateCategories(cats []CategoryConfig) []error {
	// An empty list is allowed at load time (operator may populate via the Web
	// UI before the first pipeline run); the Orchestrator will no-op cleanly.
	var errs []error
	seen := make(map[string]struct{}, len(cats))
	for i, c := range cats {
		code := strings.TrimSpace(c.Code)
		if code == "" {
			errs = append(errs, fmt.Errorf("categories[%d].code: must not be empty", i))
			continue
		}
		if !strings.Contains(code, ":") {
			errs = append(errs, fmt.Errorf("categories[%d].code: expected form 'geosite:<name>', got %q", i, code))
		}
		if _, dup := seen[code]; dup {
			errs = append(errs, fmt.Errorf("categories[%d].code: duplicate %q", i, code))
		}
		seen[code] = struct{}{}
	}
	return errs
}

func validateResolver(r ResolverConfig) []error {
	var errs []error
	if strings.TrimSpace(r.Upstream) == "" {
		errs = append(errs, errors.New("resolver.upstream: must not be empty"))
	} else if host, port, err := net.SplitHostPort(r.Upstream); err != nil {
		errs = append(errs, fmt.Errorf("resolver.upstream: invalid host:port %q: %w", r.Upstream, err))
	} else {
		if host == "" {
			errs = append(errs, fmt.Errorf("resolver.upstream: missing host in %q", r.Upstream))
		}
		if p, perr := strconv.Atoi(port); perr != nil || p < 1 || p > 65535 {
			errs = append(errs, fmt.Errorf("resolver.upstream: invalid port in %q", r.Upstream))
		}
	}
	switch r.Network {
	case "udp", "tcp", "tcp-tls":
	default:
		errs = append(errs, fmt.Errorf("resolver.network: must be udp|tcp|tcp-tls, got %q", r.Network))
	}
	if r.Concurrency < 1 || r.Concurrency > 4096 {
		errs = append(errs, fmt.Errorf("resolver.concurrency: must be in [1,4096], got %d", r.Concurrency))
	}
	if r.QPS < 1 || r.QPS > 100000 {
		errs = append(errs, fmt.Errorf("resolver.qps: must be in [1,100000], got %d", r.QPS))
	}
	if r.Timeout < 100*time.Millisecond {
		errs = append(errs, fmt.Errorf("resolver.timeout: must be >= 100ms, got %s", r.Timeout))
	}
	if r.Retries < 0 || r.Retries > 10 {
		errs = append(errs, fmt.Errorf("resolver.retries: must be in [0,10], got %d", r.Retries))
	}
	if r.BackoffBase <= 0 {
		errs = append(errs, fmt.Errorf("resolver.backoff_base: must be > 0, got %s", r.BackoffBase))
	}
	if r.BackoffMax < r.BackoffBase {
		errs = append(errs, fmt.Errorf("resolver.backoff_max (%s) must be >= resolver.backoff_base (%s)", r.BackoffMax, r.BackoffBase))
	}
	if !r.EnableV4 && !r.EnableV6 {
		errs = append(errs, errors.New("resolver: at least one of enable_v4 or enable_v6 must be true"))
	}
	return errs
}

func validateCache(c CacheConfig) []error {
	var errs []error
	if strings.TrimSpace(c.DBPath) == "" {
		errs = append(errs, errors.New("cache.db_path: must not be empty"))
	}
	if c.TTL < time.Minute {
		errs = append(errs, fmt.Errorf("cache.ttl: must be >= 1m, got %s", c.TTL))
	}
	if c.FailedTTL < time.Second {
		errs = append(errs, fmt.Errorf("cache.failed_ttl: must be >= 1s, got %s", c.FailedTTL))
	}
	if c.VacuumAfter < time.Hour {
		errs = append(errs, fmt.Errorf("cache.vacuum_after: must be >= 1h, got %s", c.VacuumAfter))
	}
	return errs
}

func validateAggregation(a AggregationConfig) []error {
	var errs []error
	switch a.Level {
	case AggOff, AggConservative, AggBalanced, AggAggressive:
	default:
		errs = append(errs, fmt.Errorf("aggregation.level: must be off|conservative|balanced|aggressive, got %q", a.Level))
	}
	if a.V4MaxPrefix < 8 || a.V4MaxPrefix > 32 {
		errs = append(errs, fmt.Errorf("aggregation.v4_max_prefix: must be in [8,32], got %d", a.V4MaxPrefix))
	}
	if a.V6MaxPrefix < 16 || a.V6MaxPrefix > 128 {
		errs = append(errs, fmt.Errorf("aggregation.v6_max_prefix: must be in [16,128], got %d", a.V6MaxPrefix))
	}
	return errs
}

func validateExport(e ExportConfig) []error {
	var errs []error
	if strings.TrimSpace(e.Dir) == "" {
		errs = append(errs, errors.New("export.dir: must not be empty"))
	}
	if strings.TrimSpace(e.IPv4File) == "" {
		errs = append(errs, errors.New("export.ipv4_file: must not be empty"))
	}
	if strings.TrimSpace(e.IPv6File) == "" {
		errs = append(errs, errors.New("export.ipv6_file: must not be empty"))
	}
	if e.IPv4File == e.IPv6File {
		errs = append(errs, errors.New("export.ipv4_file and export.ipv6_file must differ"))
	}
	return errs
}

func validateRouting(r RoutingConfig) []error {
	var errs []error
	switch r.Backend {
	case BackendNone, BackendNFTables, BackendIProute2:
	default:
		errs = append(errs, fmt.Errorf("routing.backend: must be none|nftables|iproute2, got %q", r.Backend))
	}
	if r.Enabled && r.Backend == BackendNone {
		errs = append(errs, errors.New("routing.enabled=true requires routing.backend != none"))
	}
	if r.Backend == BackendIProute2 {
		if r.TableID < 1 || r.TableID > 252 {
			// 253,254,255 are reserved (default, main, local).
			errs = append(errs, fmt.Errorf("routing.table_id: must be in [1,252] for iproute2, got %d", r.TableID))
		}
	}
	if r.Backend == BackendNFTables {
		if strings.TrimSpace(r.NFTTable) == "" {
			errs = append(errs, errors.New("routing.nft_table: must not be empty when backend=nftables"))
		}
		if strings.TrimSpace(r.NFTSetV4) == "" {
			errs = append(errs, errors.New("routing.nft_set_v4: must not be empty when backend=nftables"))
		}
		if strings.TrimSpace(r.NFTSetV6) == "" {
			errs = append(errs, errors.New("routing.nft_set_v6: must not be empty when backend=nftables"))
		}
	}
	if r.Enabled && strings.TrimSpace(r.StatePath) == "" {
		errs = append(errs, errors.New("routing.state_path: must not be empty when routing is enabled"))
	}
	return errs
}

func validateScheduler(s SchedulerConfig) []error {
	var errs []error
	if s.DLCRefresh < time.Minute {
		errs = append(errs, fmt.Errorf("scheduler.dlc_refresh: must be >= 1m, got %s", s.DLCRefresh))
	}
	if s.ResolveCycle < time.Minute {
		errs = append(errs, fmt.Errorf("scheduler.resolve_cycle: must be >= 1m, got %s", s.ResolveCycle))
	}
	return errs
}

func validateLogging(l LoggingConfig) []error {
	var errs []error
	switch strings.ToLower(l.Level) {
	case "debug", "info", "warn", "error", "fatal", "panic":
	default:
		errs = append(errs, fmt.Errorf("logging.level: must be debug|info|warn|error, got %q", l.Level))
	}
	switch strings.ToLower(l.Format) {
	case "json", "console", "text":
	default:
		errs = append(errs, fmt.Errorf("logging.format: must be json|console, got %q", l.Format))
	}
	return errs
}

func validateMetrics(m MetricsConfig) []error {
	var errs []error
	if m.Enabled {
		if !strings.HasPrefix(m.Path, "/") {
			errs = append(errs, fmt.Errorf("metrics.path: must start with '/', got %q", m.Path))
		}
	}
	return errs
}
