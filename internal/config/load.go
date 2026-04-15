package config

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// EnvPrefix is the prefix used for environment variable overrides.
// e.g. resolver.qps → D2IP_RESOLVER_QPS.
const EnvPrefix = "D2IP"

// LoadOptions configures Load.
type LoadOptions struct {
	// ConfigFile is an optional path to a YAML config (seed for first run).
	// Empty string disables file loading. If the file does not exist, this is
	// not an error — defaults + ENV are used.
	ConfigFile string

	// SearchPaths is an optional list of directories to search for "config.yaml"
	// when ConfigFile is empty.
	SearchPaths []string

	// KVOverrides is an optional map of kv_settings rows (dotted-key → string
	// value) that are merged between defaults and ENV. When non-nil it takes
	// precedence over YAML but is itself overridden by ENV. In the canonical
	// pipeline these rows are passed in by the cache agent at startup.
	KVOverrides map[string]string
}

// Load assembles the effective Config from defaults, YAML seed, kv_settings
// overrides, and ENV (highest wins). It also validates the result and returns
// a joined error if any rule fails.
func Load(opts LoadOptions) (*Config, error) {
	v := viper.New()

	// Seed defaults first so Unmarshal fills zero-valued fields.
	applyDefaultsToViper(v, Defaults())

	// YAML seed (optional).
	if opts.ConfigFile != "" {
		v.SetConfigFile(opts.ConfigFile)
	} else {
		v.SetConfigName("config")
		v.SetConfigType("yaml")
		for _, p := range opts.SearchPaths {
			v.AddConfigPath(p)
		}
	}
	if err := v.ReadInConfig(); err != nil {
		// Missing config file is not fatal — defaults + ENV still work.
		var nfErr viper.ConfigFileNotFoundError
		if !errors.As(err, &nfErr) && !os.IsNotExist(err) {
			return nil, fmt.Errorf("config: read file: %w", err)
		}
	}

	// kv_settings overrides (between YAML and ENV).
	if len(opts.KVOverrides) > 0 {
		if err := applyKVToViper(v, opts.KVOverrides); err != nil {
			return nil, fmt.Errorf("config: apply kv overrides: %w", err)
		}
	}

	// ENV: prefix D2IP_, dot→underscore mapping on keys.
	v.SetEnvPrefix(EnvPrefix)
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	cfg := Defaults()
	// viper enables StringToTimeDuration and StringToSlice by default in
	// defaultDecoderConfig; no extra hooks are needed for our schema.
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("config: unmarshal: %w", err)
	}

	// Categories can arrive from ENV as JSON (D2IP_CATEGORIES='[{"code":"geosite:ru"}]').
	// Viper’s AutomaticEnv does not populate slice-of-struct fields, so check
	// the raw env var and decode it explicitly as a fallback.
	if raw := os.Getenv(EnvPrefix + "_CATEGORIES"); raw != "" {
		cats, err := parseCategoriesEnv(raw)
		if err != nil {
			return nil, fmt.Errorf("config: parse %s_CATEGORIES: %w", EnvPrefix, err)
		}
		cfg.Categories = cats
	}

	if errs := cfg.Validate(); len(errs) > 0 {
		return nil, fmt.Errorf("config: validation failed: %w", errors.Join(errs...))
	}

	return &cfg, nil
}

// applyDefaultsToViper seeds viper with the default values so that Unmarshal
// picks up any field that hasn’t been overridden by YAML/ENV.
func applyDefaultsToViper(v *viper.Viper, d Config) {
	v.SetDefault("listen", d.Listen)

	v.SetDefault("source.url", d.Source.URL)
	v.SetDefault("source.cache_path", d.Source.CachePath)
	v.SetDefault("source.refresh_interval", d.Source.RefreshInterval)
	v.SetDefault("source.http_timeout", d.Source.HTTPTimeout)

	// Categories: leave empty default; operator must configure at least one.
	v.SetDefault("categories", []map[string]any{})

	v.SetDefault("resolver.upstream", d.Resolver.Upstream)
	v.SetDefault("resolver.network", d.Resolver.Network)
	v.SetDefault("resolver.concurrency", d.Resolver.Concurrency)
	v.SetDefault("resolver.qps", d.Resolver.QPS)
	v.SetDefault("resolver.timeout", d.Resolver.Timeout)
	v.SetDefault("resolver.retries", d.Resolver.Retries)
	v.SetDefault("resolver.backoff_base", d.Resolver.BackoffBase)
	v.SetDefault("resolver.backoff_max", d.Resolver.BackoffMax)
	v.SetDefault("resolver.follow_cname", d.Resolver.FollowCNAME)
	v.SetDefault("resolver.enable_v4", d.Resolver.EnableV4)
	v.SetDefault("resolver.enable_v6", d.Resolver.EnableV6)

	v.SetDefault("cache.db_path", d.Cache.DBPath)
	v.SetDefault("cache.ttl", d.Cache.TTL)
	v.SetDefault("cache.failed_ttl", d.Cache.FailedTTL)
	v.SetDefault("cache.vacuum_after", d.Cache.VacuumAfter)

	v.SetDefault("aggregation.enabled", d.Aggregation.Enabled)
	v.SetDefault("aggregation.level", string(d.Aggregation.Level))
	v.SetDefault("aggregation.v4_max_prefix", d.Aggregation.V4MaxPrefix)
	v.SetDefault("aggregation.v6_max_prefix", d.Aggregation.V6MaxPrefix)

	v.SetDefault("export.dir", d.Export.Dir)
	v.SetDefault("export.ipv4_file", d.Export.IPv4File)
	v.SetDefault("export.ipv6_file", d.Export.IPv6File)

	v.SetDefault("routing.enabled", d.Routing.Enabled)
	v.SetDefault("routing.backend", string(d.Routing.Backend))
	v.SetDefault("routing.table_id", d.Routing.TableID)
	v.SetDefault("routing.nft_table", d.Routing.NFTTable)
	v.SetDefault("routing.nft_set_v4", d.Routing.NFTSetV4)
	v.SetDefault("routing.nft_set_v6", d.Routing.NFTSetV6)
	v.SetDefault("routing.state_path", d.Routing.StatePath)
	v.SetDefault("routing.dry_run", d.Routing.DryRun)

	v.SetDefault("scheduler.dlc_refresh", d.Scheduler.DLCRefresh)
	v.SetDefault("scheduler.resolve_cycle", d.Scheduler.ResolveCycle)

	v.SetDefault("logging.level", d.Logging.Level)
	v.SetDefault("logging.format", d.Logging.Format)

	v.SetDefault("metrics.enabled", d.Metrics.Enabled)
	v.SetDefault("metrics.path", d.Metrics.Path)
}

// Duration parsing helper used for kv_settings overrides and tests.
// It accepts the same formats as time.ParseDuration plus bare integers
// interpreted as seconds for ergonomic CLI input.
func parseDuration(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty duration")
	}
	if d, err := time.ParseDuration(s); err == nil {
		return d, nil
	}
	// Fallback: integer seconds.
	var secs int64
	_, err := fmt.Sscanf(s, "%d", &secs)
	if err != nil {
		return 0, fmt.Errorf("invalid duration %q: %w", s, err)
	}
	return time.Duration(secs) * time.Second, nil
}
