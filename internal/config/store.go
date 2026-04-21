package config

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/viper"
)

// KVStore is the contract the config package expects from a persistent
// key-value overrides store (the kv_settings SQLite table, in practice).
// The config package does NOT implement this — the cache agent owns SQLite
// and supplies a concrete implementation.
//
// Keys use dotted notation matching mapstructure paths, e.g.
//
//	resolver.qps         → "500"
//	cache.ttl            → "6h"
//	aggregation.enabled  → "true"
//	categories           → `[{"code":"geosite:ru"}]`  (JSON)
//
// Values are always strings; the config layer parses them.
type KVStore interface {
	// GetAll returns every override row as a dotted-key → raw-string map.
	GetAll(ctx context.Context) (map[string]string, error)
	// Set persists a single override. Empty value deletes the row.
	Set(ctx context.Context, key, value string) error
	// Delete removes an override row (no-op if absent).
	Delete(ctx context.Context, key string) error
}

// ApplyOverrides merges kv_settings rows into cfg IN PLACE using the same
// precedence rules as Load (kv rows beat defaults/YAML but are beaten by ENV,
// which the caller is expected to re-apply afterwards if relevant).
//
// It is exposed for callers that already have a loaded *Config and want to
// layer overrides on top without re-reading the filesystem — typically the
// hot-reload path invoked by PATCH /settings.
//
// Returns the first parse error encountered (if any). Unknown keys are
// ignored so that forward-compatible rows do not break older binaries.
func ApplyOverrides(cfg *Config, kv map[string]string) error {
	if cfg == nil {
		return fmt.Errorf("config: ApplyOverrides: nil config")
	}
	if len(kv) == 0 {
		return nil
	}

	// Round-trip through viper so we benefit from the same decode hooks
	// (duration parsing, slice decoding, type coercion) used by Load.
	v := viper.New()
	applyDefaultsToViper(v, *cfg)

	// Seed viper with the current cfg values so that keys not present in kv
	// survive the round-trip.
	seedViperFromConfig(v, *cfg)

	if err := applyKVToViper(v, kv); err != nil {
		return err
	}

	next := *cfg
	if err := v.Unmarshal(&next); err != nil {
		return fmt.Errorf("config: unmarshal after kv overrides: %w", err)
	}

	// Categories require special handling: a "categories" row carries JSON.
	if raw, ok := kv["categories"]; ok && strings.TrimSpace(raw) != "" {
		cats, err := parseCategoriesEnv(raw)
		if err != nil {
			return fmt.Errorf("config: parse categories override: %w", err)
		}
		next.Categories = cats
	}

	*cfg = next
	return nil
}

// applyKVToViper sets each kv row on v with appropriate type coercion.
// Durations are parsed into time.Duration so that downstream Unmarshal sees
// the already-typed value (viper’s decode hooks would handle the string form
// equally well, but pre-parsing gives us early error reporting).
//
// ENV vars take precedence over KV: if an env var is set for a key, the KV
// value is skipped to respect the precedence order ENV > kv > YAML > defaults.
func applyKVToViper(v *viper.Viper, kv map[string]string) error {
	for key, raw := range kv {
		key = strings.ToLower(strings.TrimSpace(key))
		if key == "" {
			continue
		}
		// "categories" is handled by the caller as JSON; skip here.
		if key == "categories" {
			continue
		}
		// Skip if ENV var is set (ENV beats KV).
		envKey := strings.ToUpper(strings.ReplaceAll(key, ".", "_"))
		if os.Getenv("D2IP_"+envKey) != "" {
			continue
		}
		val, err := coerceKVValue(key, raw)
		if err != nil {
			return fmt.Errorf("kv[%s]: %w", key, err)
		}
		v.Set(key, val)
	}
	return nil
}

// coerceKVValue converts a kv string into the expected Go type for the given
// dotted key. Unknown keys pass through as strings — viper’s unmarshal layer
// will either decode or ignore them.
func coerceKVValue(key, raw string) (any, error) {
	raw = strings.TrimSpace(raw)
	switch key {
	// Durations.
	case "source.refresh_interval", "source.http_timeout",
		"resolver.timeout", "resolver.backoff_base", "resolver.backoff_max",
		"cache.ttl", "cache.failed_ttl", "cache.vacuum_after",
		"scheduler.dlc_refresh", "scheduler.resolve_cycle":
		d, err := parseDuration(raw)
		if err != nil {
			return nil, err
		}
		return d, nil

	// Integers.
	case "resolver.concurrency", "resolver.qps", "resolver.retries",
		"aggregation.v4_max_prefix", "aggregation.v6_max_prefix",
		"routing.table_id":
		n, err := strconv.Atoi(raw)
		if err != nil {
			return nil, fmt.Errorf("invalid integer %q: %w", raw, err)
		}
		return n, nil

	// Booleans.
	case "resolver.follow_cname", "resolver.enable_v4", "resolver.enable_v6",
		"aggregation.enabled", "routing.enabled", "routing.dry_run",
		"metrics.enabled":
		b, err := strconv.ParseBool(raw)
		if err != nil {
			return nil, fmt.Errorf("invalid boolean %q: %w", raw, err)
		}
		return b, nil
	}

	// Default: pass through as string.
	return raw, nil
}

// seedViperFromConfig writes the current Config back into viper so that keys
// not targeted by kv overrides retain their current value through Unmarshal.
func seedViperFromConfig(v *viper.Viper, c Config) {
	v.Set("listen", c.Listen)

	v.Set("source.url", c.Source.URL)
	v.Set("source.cache_path", c.Source.CachePath)
	v.Set("source.refresh_interval", c.Source.RefreshInterval)
	v.Set("source.http_timeout", c.Source.HTTPTimeout)

	// Preserve categories as a slice of maps viper can re-encode.
	cats := make([]map[string]any, 0, len(c.Categories))
	for _, cat := range c.Categories {
		cats = append(cats, map[string]any{
			"code":  cat.Code,
			"attrs": cat.Attrs,
		})
	}
	v.Set("categories", cats)

	v.Set("resolver.upstream", c.Resolver.Upstream)
	v.Set("resolver.network", c.Resolver.Network)
	v.Set("resolver.concurrency", c.Resolver.Concurrency)
	v.Set("resolver.qps", c.Resolver.QPS)
	v.Set("resolver.timeout", c.Resolver.Timeout)
	v.Set("resolver.retries", c.Resolver.Retries)
	v.Set("resolver.backoff_base", c.Resolver.BackoffBase)
	v.Set("resolver.backoff_max", c.Resolver.BackoffMax)
	v.Set("resolver.follow_cname", c.Resolver.FollowCNAME)
	v.Set("resolver.enable_v4", c.Resolver.EnableV4)
	v.Set("resolver.enable_v6", c.Resolver.EnableV6)

	v.Set("cache.db_path", c.Cache.DBPath)
	v.Set("cache.ttl", c.Cache.TTL)
	v.Set("cache.failed_ttl", c.Cache.FailedTTL)
	v.Set("cache.vacuum_after", c.Cache.VacuumAfter)

	v.Set("aggregation.enabled", c.Aggregation.Enabled)
	v.Set("aggregation.level", string(c.Aggregation.Level))
	v.Set("aggregation.v4_max_prefix", c.Aggregation.V4MaxPrefix)
	v.Set("aggregation.v6_max_prefix", c.Aggregation.V6MaxPrefix)

	v.Set("export.dir", c.Export.Dir)
	v.Set("export.ipv4_file", c.Export.IPv4File)
	v.Set("export.ipv6_file", c.Export.IPv6File)

	v.Set("routing.enabled", c.Routing.Enabled)
	v.Set("routing.backend", string(c.Routing.Backend))
	v.Set("routing.table_id", c.Routing.TableID)
	v.Set("routing.iface", c.Routing.Iface)
	v.Set("routing.nft_table", c.Routing.NFTTable)
	v.Set("routing.nft_set_v4", c.Routing.NFTSetV4)
	v.Set("routing.nft_set_v6", c.Routing.NFTSetV6)
	v.Set("routing.state_path", c.Routing.StatePath)
	v.Set("routing.dry_run", c.Routing.DryRun)

	v.Set("scheduler.dlc_refresh", c.Scheduler.DLCRefresh)
	v.Set("scheduler.resolve_cycle", c.Scheduler.ResolveCycle)

	v.Set("logging.level", c.Logging.Level)
	v.Set("logging.format", c.Logging.Format)

	v.Set("metrics.enabled", c.Metrics.Enabled)
	v.Set("metrics.path", c.Metrics.Path)
}

// parseCategoriesEnv decodes either:
//   - a JSON array: `[{"code":"geosite:ru","attrs":["cn"]},...]`
//   - or a comma-separated list of codes: `geosite:ru,geosite:google`
//
// It is used for both the D2IP_CATEGORIES env var and the `categories` kv row.
func parseCategoriesEnv(raw string) ([]CategoryConfig, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return []CategoryConfig{}, nil
	}
	if strings.HasPrefix(raw, "[") {
		var cats []CategoryConfig
		if err := json.Unmarshal([]byte(raw), &cats); err != nil {
			return nil, fmt.Errorf("categories json: %w", err)
		}
		return cats, nil
	}
	// Comma-separated codes.
	parts := strings.Split(raw, ",")
	out := make([]CategoryConfig, 0, len(parts))
	for _, p := range parts {
		code := strings.TrimSpace(p)
		if code == "" {
			continue
		}
		out = append(out, CategoryConfig{Code: code})
	}
	return out, nil
}
