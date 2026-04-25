package config

import (
	"reflect"
	"testing"
	"time"

	"github.com/spf13/viper"
)

func TestCoerceKVValue_Durations(t *testing.T) {
	tests := []struct {
		name    string
		key     string
		raw     string
		want    time.Duration
		wantErr bool
	}{
		{"source.refresh_interval hours", "source.refresh_interval", "1h", time.Hour, false},
		{"resolver.timeout seconds", "resolver.timeout", "30s", 30 * time.Second, false},
		{"cache.ttl hours", "cache.ttl", "6h", 6 * time.Hour, false},
		{"scheduler.dlc_refresh hours", "scheduler.dlc_refresh", "24h", 24 * time.Hour, false},
		{"source.http_timeout minutes", "source.http_timeout", "5m", 5 * time.Minute, false},
		{"resolver.backoff_base ms", "resolver.backoff_base", "200ms", 200 * time.Millisecond, false},
		{"resolver.backoff_max seconds", "resolver.backoff_max", "5s", 5 * time.Second, false},
		{"scheduler.resolve_cycle hours", "scheduler.resolve_cycle", "1h", time.Hour, false},
		{"cache.failed_ttl minutes", "cache.failed_ttl", "30m", 30 * time.Minute, false},
		{"cache.vacuum_after hours", "cache.vacuum_after", "720h", 720 * time.Hour, false},
		{"invalid duration", "resolver.timeout", "notaduration", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := coerceKVValue(tt.key, tt.raw)
			if (err != nil) != tt.wantErr {
				t.Fatalf("coerceKVValue(%q, %q) error = %v, wantErr %v", tt.key, tt.raw, err, tt.wantErr)
			}
			if err != nil {
				return
			}
			d, ok := got.(time.Duration)
			if !ok {
				t.Fatalf("coerceKVValue(%q, %q) = %T, want time.Duration", tt.key, tt.raw, got)
			}
			if d != tt.want {
				t.Fatalf("coerceKVValue(%q, %q) = %v, want %v", tt.key, tt.raw, d, tt.want)
			}
		})
	}
}

func TestCoerceKVValue_Integers(t *testing.T) {
	tests := []struct {
		name    string
		key     string
		raw     string
		want    int
		wantErr bool
	}{
		{"resolver.concurrency", "resolver.concurrency", "64", 64, false},
		{"resolver.qps", "resolver.qps", "200", 200, false},
		{"aggregation.v4_max_prefix", "aggregation.v4_max_prefix", "16", 16, false},
		{"resolver.retries", "resolver.retries", "3", 3, false},
		{"aggregation.v6_max_prefix", "aggregation.v6_max_prefix", "32", 32, false},
		{"routing.table_id", "routing.table_id", "100", 100, false},
		{"invalid integer", "resolver.qps", "abc", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := coerceKVValue(tt.key, tt.raw)
			if (err != nil) != tt.wantErr {
				t.Fatalf("coerceKVValue(%q, %q) error = %v, wantErr %v", tt.key, tt.raw, err, tt.wantErr)
			}
			if err != nil {
				return
			}
			n, ok := got.(int)
			if !ok {
				t.Fatalf("coerceKVValue(%q, %q) = %T, want int", tt.key, tt.raw, got)
			}
			if n != tt.want {
				t.Fatalf("coerceKVValue(%q, %q) = %d, want %d", tt.key, tt.raw, n, tt.want)
			}
		})
	}
}

func TestCoerceKVValue_Booleans(t *testing.T) {
	tests := []struct {
		name    string
		key     string
		raw     string
		want    bool
		wantErr bool
	}{
		{"resolver.follow_cname true", "resolver.follow_cname", "true", true, false},
		{"routing.enabled false", "routing.enabled", "false", false, false},
		{"metrics.enabled 1", "metrics.enabled", "1", true, false},
		{"resolver.enable_v4 0", "resolver.enable_v4", "0", false, false},
		{"aggregation.enabled TRUE", "aggregation.enabled", "TRUE", true, false},
		{"routing.dry_run FALSE", "routing.dry_run", "FALSE", false, false},
		{"invalid boolean", "routing.enabled", "maybe", false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := coerceKVValue(tt.key, tt.raw)
			if (err != nil) != tt.wantErr {
				t.Fatalf("coerceKVValue(%q, %q) error = %v, wantErr %v", tt.key, tt.raw, err, tt.wantErr)
			}
			if err != nil {
				return
			}
			b, ok := got.(bool)
			if !ok {
				t.Fatalf("coerceKVValue(%q, %q) = %T, want bool", tt.key, tt.raw, got)
			}
			if b != tt.want {
				t.Fatalf("coerceKVValue(%q, %q) = %v, want %v", tt.key, tt.raw, b, tt.want)
			}
		})
	}
}

func TestCoerceKVValue_Policies(t *testing.T) {
	raw := `[{"name":"test-policy","enabled":true,"categories":["geosite:ru"],"backend":"nftables","table_id":100,"iface":"eth0","nft_table":"inet d2ip","nft_set_v4":"ipv4_set","nft_set_v6":"ipv6_set","dry_run":false,"export_format":"txt"}]`

	got, err := coerceKVValue("routing.policies", raw)
	if err != nil {
		t.Fatalf("coerceKVValue(routing.policies, ...) error = %v", err)
	}

	pols, ok := got.([]PolicyConfig)
	if !ok {
		t.Fatalf("coerceKVValue(routing.policies, ...) = %T, want []PolicyConfig", got)
	}
	if len(pols) != 1 {
		t.Fatalf("expected 1 policy, got %d", len(pols))
	}

	want := PolicyConfig{
		Name:         "test-policy",
		Enabled:      true,
		Categories:   []string{"geosite:ru"},
		Backend:      BackendNFTables,
		TableID:      100,
		Iface:        "eth0",
		NFTTable:     "inet d2ip",
		NFTSetV4:     "ipv4_set",
		NFTSetV6:     "ipv6_set",
		DryRun:       false,
		ExportFormat: "txt",
	}

	if !reflect.DeepEqual(pols[0], want) {
		t.Fatalf("policy mismatch:\ngot  %+v\nwant %+v", pols[0], want)
	}
}

func TestCoerceKVValue_Sources(t *testing.T) {
	raw := `[{"id":"src1","provider":"v2flygeosite","prefix":"geosite","enabled":true,"config":{"url":"https://example.com"}}]`

	got, err := coerceKVValue("sources", raw)
	if err != nil {
		t.Fatalf("coerceKVValue(sources, ...) error = %v", err)
	}

	srcs, ok := got.([]SourceItemConfig)
	if !ok {
		t.Fatalf("coerceKVValue(sources, ...) = %T, want []SourceItemConfig", got)
	}
	if len(srcs) != 1 {
		t.Fatalf("expected 1 source, got %d", len(srcs))
	}

	want := SourceItemConfig{
		ID:       "src1",
		Provider: "v2flygeosite",
		Prefix:   "geosite",
		Enabled:  true,
		Config:   map[string]any{"url": "https://example.com"},
	}

	if !reflect.DeepEqual(srcs[0], want) {
		t.Fatalf("source mismatch:\ngot  %+v\nwant %+v", srcs[0], want)
	}
}

func TestParseCategoriesEnv_JSON(t *testing.T) {
	raw := `[{"code":"geosite:ru","attrs":["cn"]},{"code":"geosite:google","attrs":["@ads"]}]`

	got, err := parseCategoriesEnv(raw)
	if err != nil {
		t.Fatalf("parseCategoriesEnv(%q) error = %v", raw, err)
	}

	want := []CategoryConfig{
		{Code: "geosite:ru", Attrs: []string{"cn"}},
		{Code: "geosite:google", Attrs: []string{"@ads"}},
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseCategoriesEnv(%q) = %+v, want %+v", raw, got, want)
	}
}

func TestParseCategoriesEnv_CSV(t *testing.T) {
	raw := "geosite:ru, geosite:google"

	got, err := parseCategoriesEnv(raw)
	if err != nil {
		t.Fatalf("parseCategoriesEnv(%q) error = %v", raw, err)
	}

	want := []CategoryConfig{
		{Code: "geosite:ru"},
		{Code: "geosite:google"},
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseCategoriesEnv(%q) = %+v, want %+v", raw, got, want)
	}
}

func TestSeedViperFromConfig_PreservesPolicies(t *testing.T) {
	cfg := Defaults()
	cfg.Routing.Policies = []PolicyConfig{
		{
			Name:         "policy-a",
			Enabled:      true,
			Categories:   []string{"geosite:ru", "geosite:cn"},
			Backend:      BackendNFTables,
			TableID:      100,
			Iface:        "eth0",
			NFTTable:     "inet d2ip",
			NFTSetV4:     "v4set",
			NFTSetV6:     "v6set",
			DryRun:       true,
			ExportFormat: "txt",
			Aggregation: &AggregationConfig{
				Enabled:     true,
				Level:       AggBalanced,
				V4MaxPrefix: 16,
				V6MaxPrefix: 32,
			},
		},
		{
			Name:       "policy-b",
			Enabled:    false,
			Categories: []string{"geosite:google"},
			Backend:    BackendIProute2,
			TableID:    200,
			Iface:      "eth1",
		},
	}

	v := viper.New()
	applyDefaultsToViper(v, cfg)
	seedViperFromConfig(v, cfg)

	var next Config
	if err := v.Unmarshal(&next); err != nil {
		t.Fatalf("v.Unmarshal error = %v", err)
	}

	if len(next.Routing.Policies) != len(cfg.Routing.Policies) {
		t.Fatalf("expected %d policies, got %d", len(cfg.Routing.Policies), len(next.Routing.Policies))
	}

	for i, want := range cfg.Routing.Policies {
		got := next.Routing.Policies[i]
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("policy[%d] mismatch:\ngot  %+v\nwant %+v", i, got, want)
		}
	}
}
