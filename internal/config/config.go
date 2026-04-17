// Package config defines the d2ip runtime configuration schema, loading,
// validation, and thread-safe hot-reload plumbing.
//
// Resolution order (highest wins): ENV (D2IP_*) > kv_settings overrides > defaults.
// The optional config.yaml is a *seed* for first-run only; code-level defaults
// are authoritative.
package config

import "time"

// Config is the full runtime configuration for d2ip. It mirrors docs/CONFIG.md.
// Every field carries a default in Defaults(); YAML, kv_settings, and ENV are
// overrides only.
type Config struct {
	// Listen is the HTTP listener address for the admin/API/metrics server.
	// Hot-reload does NOT apply to this field — changing it requires restart.
	Listen string `mapstructure:"listen" json:"listen" yaml:"listen"`

	Source      SourceConfig      `mapstructure:"source" json:"source" yaml:"source"`
	Categories  []CategoryConfig  `mapstructure:"categories" json:"categories" yaml:"categories"`
	Resolver    ResolverConfig    `mapstructure:"resolver" json:"resolver" yaml:"resolver"`
	Cache       CacheConfig       `mapstructure:"cache" json:"cache" yaml:"cache"`
	Aggregation AggregationConfig `mapstructure:"aggregation" json:"aggregation" yaml:"aggregation"`
	Export      ExportConfig      `mapstructure:"export" json:"export" yaml:"export"`
	Routing     RoutingConfig     `mapstructure:"routing" json:"routing" yaml:"routing"`
	Scheduler   SchedulerConfig   `mapstructure:"scheduler" json:"scheduler" yaml:"scheduler"`
	Logging     LoggingConfig     `mapstructure:"logging" json:"logging" yaml:"logging"`
	Metrics     MetricsConfig     `mapstructure:"metrics" json:"metrics" yaml:"metrics"`
}

// SourceConfig controls the domain-list-community (geosite) source.
type SourceConfig struct {
	URL             string        `mapstructure:"url" json:"url" yaml:"url"`
	CachePath       string        `mapstructure:"cache_path" json:"cache_path" yaml:"cache_path"`
	RefreshInterval time.Duration `mapstructure:"refresh_interval" json:"refresh_interval" yaml:"refresh_interval"`
	HTTPTimeout     time.Duration `mapstructure:"http_timeout" json:"http_timeout" yaml:"http_timeout"`
}

// CategoryConfig selects a geosite category (and optional @attribute filter).
type CategoryConfig struct {
	// Code is the geosite category code, e.g. "geosite:ru" or "geosite:google".
	Code string `mapstructure:"code" json:"code" yaml:"code"`
	// Attrs is an optional list of @attribute filters applied with AND semantics.
	Attrs []string `mapstructure:"attrs" json:"attrs" yaml:"attrs"`
}

// ResolverConfig controls the DNS resolver worker pool.
type ResolverConfig struct {
	Upstream    string        `mapstructure:"upstream" json:"upstream" yaml:"upstream"`
	Network     string        `mapstructure:"network" json:"network" yaml:"network"` // udp|tcp|tcp-tls
	Concurrency int           `mapstructure:"concurrency" json:"concurrency" yaml:"concurrency"`
	QPS         int           `mapstructure:"qps" json:"qps" yaml:"qps"`
	Timeout     time.Duration `mapstructure:"timeout" json:"timeout" yaml:"timeout"`
	Retries     int           `mapstructure:"retries" json:"retries" yaml:"retries"`
	BackoffBase time.Duration `mapstructure:"backoff_base" json:"backoff_base" yaml:"backoff_base"`
	BackoffMax  time.Duration `mapstructure:"backoff_max" json:"backoff_max" yaml:"backoff_max"`
	FollowCNAME bool          `mapstructure:"follow_cname" json:"follow_cname" yaml:"follow_cname"`
	EnableV4    bool          `mapstructure:"enable_v4" json:"enable_v4" yaml:"enable_v4"`
	EnableV6    bool          `mapstructure:"enable_v6" json:"enable_v6" yaml:"enable_v6"`
}

// CacheConfig controls the SQLite-backed resolve/result cache.
type CacheConfig struct {
	DBPath      string        `mapstructure:"db_path" json:"db_path" yaml:"db_path"`
	TTL         time.Duration `mapstructure:"ttl" json:"ttl" yaml:"ttl"`                      // DNS TTL is ignored; this is internal
	FailedTTL   time.Duration `mapstructure:"failed_ttl" json:"failed_ttl" yaml:"failed_ttl"` // short retry for failures
	VacuumAfter time.Duration `mapstructure:"vacuum_after" json:"vacuum_after" yaml:"vacuum_after"`
}

// AggregationLevel enumerates the supported aggregation aggressiveness levels.
type AggregationLevel string

const (
	AggOff          AggregationLevel = "off"
	AggConservative AggregationLevel = "conservative"
	AggBalanced     AggregationLevel = "balanced"
	AggAggressive   AggregationLevel = "aggressive"
)

// AggregationConfig controls prefix-aggregation aggressiveness and ceilings.
type AggregationConfig struct {
	Enabled     bool             `mapstructure:"enabled" json:"enabled" yaml:"enabled"`
	Level       AggregationLevel `mapstructure:"level" json:"level" yaml:"level"`
	V4MaxPrefix int              `mapstructure:"v4_max_prefix" json:"v4_max_prefix" yaml:"v4_max_prefix"`
	V6MaxPrefix int              `mapstructure:"v6_max_prefix" json:"v6_max_prefix" yaml:"v6_max_prefix"`
}

// ExportConfig controls on-disk export artifacts.
type ExportConfig struct {
	Dir      string `mapstructure:"dir" json:"dir" yaml:"dir"`
	IPv4File string `mapstructure:"ipv4_file" json:"ipv4_file" yaml:"ipv4_file"`
	IPv6File string `mapstructure:"ipv6_file" json:"ipv6_file" yaml:"ipv6_file"`
}

// RoutingBackend enumerates supported routing backends.
type RoutingBackend string

const (
	BackendNone     RoutingBackend = "none"
	BackendNFTables RoutingBackend = "nftables"
	BackendIProute2 RoutingBackend = "iproute2"
)

// RoutingConfig controls kernel routing integration (SAFE default: disabled).
type RoutingConfig struct {
	Enabled   bool           `mapstructure:"enabled" json:"enabled" yaml:"enabled"`
	Backend   RoutingBackend `mapstructure:"backend" json:"backend" yaml:"backend"`
	TableID   int            `mapstructure:"table_id" json:"table_id" yaml:"table_id"`
	NFTTable  string         `mapstructure:"nft_table" json:"nft_table" yaml:"nft_table"`
	NFTSetV4  string         `mapstructure:"nft_set_v4" json:"nft_set_v4" yaml:"nft_set_v4"`
	NFTSetV6  string         `mapstructure:"nft_set_v6" json:"nft_set_v6" yaml:"nft_set_v6"`
	StatePath string         `mapstructure:"state_path" json:"state_path" yaml:"state_path"`
	DryRun    bool           `mapstructure:"dry_run" json:"dry_run" yaml:"dry_run"`
}

// SchedulerConfig controls background cadences.
type SchedulerConfig struct {
	DLCRefresh   time.Duration `mapstructure:"dlc_refresh" json:"dlc_refresh" yaml:"dlc_refresh"`
	ResolveCycle time.Duration `mapstructure:"resolve_cycle" json:"resolve_cycle" yaml:"resolve_cycle"`
}

// LoggingConfig controls zerolog output.
type LoggingConfig struct {
	Level  string `mapstructure:"level" json:"level" yaml:"level"`    // debug|info|warn|error
	Format string `mapstructure:"format" json:"format" yaml:"format"` // json|console
}

// MetricsConfig controls Prometheus metrics exposition.
type MetricsConfig struct {
	Enabled bool   `mapstructure:"enabled" json:"enabled" yaml:"enabled"`
	Path    string `mapstructure:"path" json:"path" yaml:"path"`
}

// Defaults returns a Config populated with the built-in defaults from
// docs/CONFIG.md. This is the source of truth for default values — YAML/ENV/kv
// are overrides only.
func Defaults() Config {
	return Config{
		Listen: ":9099",
		Source: SourceConfig{
			URL:             "https://github.com/v2fly/domain-list-community/releases/latest/download/dlc.dat",
			CachePath:       "/var/lib/d2ip/dlc.dat",
			RefreshInterval: 24 * time.Hour,
			HTTPTimeout:     30 * time.Second,
		},
		Categories: []CategoryConfig{},
		Resolver: ResolverConfig{
			Upstream:    "1.1.1.1:53",
			Network:     "udp",
			Concurrency: 64,
			QPS:         200,
			Timeout:     3 * time.Second,
			Retries:     3,
			BackoffBase: 200 * time.Millisecond,
			BackoffMax:  5 * time.Second,
			FollowCNAME: true,
			EnableV4:    true,
			EnableV6:    true,
		},
		Cache: CacheConfig{
			DBPath:      "/var/lib/d2ip/cache.db",
			TTL:         6 * time.Hour,
			FailedTTL:   30 * time.Minute,
			VacuumAfter: 720 * time.Hour,
		},
		Aggregation: AggregationConfig{
			Enabled:     true,
			Level:       AggBalanced,
			V4MaxPrefix: 16,
			V6MaxPrefix: 32,
		},
		Export: ExportConfig{
			Dir:      "/var/lib/d2ip/out",
			IPv4File: "ipv4.txt",
			IPv6File: "ipv6.txt",
		},
		Routing: RoutingConfig{
			Enabled:   false,
			Backend:   BackendNFTables,
			TableID:   100,
			NFTTable:  "inet d2ip",
			NFTSetV4:  "d2ip_v4",
			NFTSetV6:  "d2ip_v6",
			StatePath: "/var/lib/d2ip/state.json",
			DryRun:    false,
		},
		Scheduler: SchedulerConfig{
			DLCRefresh:   24 * time.Hour,
			ResolveCycle: 1 * time.Hour,
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "json",
		},
		Metrics: MetricsConfig{
			Enabled: true,
			Path:    "/metrics",
		},
	}
}

// Clone returns a deep copy of the Config. Slices (Categories, Attrs) are
// copied so the returned value is safe to mutate without affecting the source.
func (c Config) Clone() Config {
	out := c
	if c.Categories != nil {
		out.Categories = make([]CategoryConfig, len(c.Categories))
		for i, cat := range c.Categories {
			ccat := cat
			if cat.Attrs != nil {
				ccat.Attrs = make([]string, len(cat.Attrs))
				copy(ccat.Attrs, cat.Attrs)
			}
			out.Categories[i] = ccat
		}
	}
	return out
}
