package config

import (
	"errors"
	"fmt"
	"net"
	"regexp"
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
	errs = append(errs, validatePolicies(c.Routing.Policies)...)
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
	if r.Enabled && strings.TrimSpace(r.StateDir) == "" {
		errs = append(errs, errors.New("routing.state_dir: must not be empty when routing is enabled"))
	}
	return errs
}

func validatePolicies(policies []PolicyConfig) []error {
	var errs []error
	if len(policies) == 0 {
		return nil
	}

	names := make(map[string]struct{})
	tableIDs := make(map[int]struct{})
	nftSets := make(map[string]struct{}) // key: "table.set_v4" or "table.set_v6"

	for i, p := range policies {
		prefix := fmt.Sprintf("routing.policies[%d]", i)

		if p.Name == "" {
			errs = append(errs, fmt.Errorf("%s.name is required", prefix))
			continue
		}
		if matched, _ := regexp.MatchString(`^[a-z0-9_-]+$`, p.Name); !matched {
			errs = append(errs, fmt.Errorf("%s.name must match [a-z0-9_-]+", prefix))
		}
		if _, exists := names[p.Name]; exists {
			errs = append(errs, fmt.Errorf("%s.name %q is duplicate", prefix, p.Name))
		}
		names[p.Name] = struct{}{}

		if !p.Enabled {
			continue
		}

		if len(p.Categories) == 0 {
			errs = append(errs, fmt.Errorf("%s.categories must have at least one entry", prefix))
		}
		for j, cat := range p.Categories {
			if !strings.Contains(cat, ":") {
				errs = append(errs, fmt.Errorf("%s.categories[%d] %q must contain ':'", prefix, j, cat))
			}
		}

		if p.Backend == BackendNone {
			errs = append(errs, fmt.Errorf("%s.backend cannot be 'none' for enabled policy", prefix))
		}
		if p.Backend != BackendIProute2 && p.Backend != BackendNFTables && p.Backend != BackendNone {
			errs = append(errs, fmt.Errorf("%s.backend invalid: %s", prefix, p.Backend))
		}

		if p.Backend == BackendIProute2 {
			if p.Iface == "" {
				errs = append(errs, fmt.Errorf("%s.iface is required for iproute2", prefix))
			}
			if p.TableID < 1 || p.TableID > 252 {
				errs = append(errs, fmt.Errorf("%s.table_id must be in [1,252]", prefix))
			}
			if _, exists := tableIDs[p.TableID]; exists {
				errs = append(errs, fmt.Errorf("%s.table_id %d is duplicate", prefix, p.TableID))
			}
			tableIDs[p.TableID] = struct{}{}
		}

		if p.Backend == BackendNFTables {
			if p.NFTTable == "" || p.NFTSetV4 == "" || p.NFTSetV6 == "" {
				errs = append(errs, fmt.Errorf("%s.nft_table, nft_set_v4, nft_set_v6 are required for nftables", prefix))
			}
			v4Key := p.NFTTable + "." + p.NFTSetV4
			v6Key := p.NFTTable + "." + p.NFTSetV6
			if _, exists := nftSets[v4Key]; exists {
				errs = append(errs, fmt.Errorf("%s.nft_set_v4 %q is duplicate in table %q", prefix, p.NFTSetV4, p.NFTTable))
			}
			if _, exists := nftSets[v6Key]; exists {
				errs = append(errs, fmt.Errorf("%s.nft_set_v6 %q is duplicate in table %q", prefix, p.NFTSetV6, p.NFTTable))
			}
			nftSets[v4Key] = struct{}{}
			nftSets[v6Key] = struct{}{}
		}

		validFormats := map[string]struct{}{"plain": {}, "ipset": {}, "json": {}, "nft": {}, "iptables": {}, "bgp": {}, "yaml": {}}
		if p.ExportFormat != "" {
			if _, ok := validFormats[p.ExportFormat]; !ok {
				errs = append(errs, fmt.Errorf("%s.export_format %q is invalid", prefix, p.ExportFormat))
			}
		}
	}

	return errs
}

func validateScheduler(s SchedulerConfig) []error {
	var errs []error
	if s.DLCRefresh < time.Minute {
		errs = append(errs, fmt.Errorf("scheduler.dlc_refresh: must be >= 1m, got %s", s.DLCRefresh))
	}
	// ResolveCycle == 0 means "disabled" (no scheduled resolves).
	if s.ResolveCycle != 0 && s.ResolveCycle < time.Minute {
		errs = append(errs, fmt.Errorf("scheduler.resolve_cycle: must be >= 1m or 0 (disabled), got %s", s.ResolveCycle))
	}
	return errs
}

func validateLogging(l LoggingConfig) []error {
	var errs []error
	switch strings.ToLower(l.Level) {
	case "debug", "info", "warn", "error", "fatal", "panic":
	default:
		errs = append(errs, fmt.Errorf("logging.level: must be debug|info|warn|error|fatal|panic, got %q", l.Level))
	}
	switch strings.ToLower(l.Format) {
	case "json", "console", "text":
	default:
		errs = append(errs, fmt.Errorf("logging.format: must be json|console|text, got %q", l.Format))
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
