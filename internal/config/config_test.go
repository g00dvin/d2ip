package config

import (
	"reflect"
	"testing"
	"time"
)

func TestDefaults(t *testing.T) {
	cfg := Defaults()

	tests := []struct {
		name     string
		got      any
		expected any
	}{
		{name: "Listen", got: cfg.Listen, expected: ":9099"},
		{name: "Source.URL", got: cfg.Source.URL, expected: "https://github.com/v2fly/domain-list-community/releases/latest/download/dlc.dat"},
		{name: "Source.CachePath", got: cfg.Source.CachePath, expected: "/var/lib/d2ip/dlc.dat"},
		{name: "Source.RefreshInterval", got: cfg.Source.RefreshInterval, expected: 24 * time.Hour},
		{name: "Source.HTTPTimeout", got: cfg.Source.HTTPTimeout, expected: 30 * time.Second},
		{name: "Resolver.Upstream", got: cfg.Resolver.Upstream, expected: "1.1.1.1:53"},
		{name: "Resolver.Network", got: cfg.Resolver.Network, expected: "udp"},
		{name: "Resolver.Concurrency", got: cfg.Resolver.Concurrency, expected: 64},
		{name: "Resolver.QPS", got: cfg.Resolver.QPS, expected: 200},
		{name: "Resolver.Timeout", got: cfg.Resolver.Timeout, expected: 3 * time.Second},
		{name: "Resolver.Retries", got: cfg.Resolver.Retries, expected: 3},
		{name: "Resolver.BackoffBase", got: cfg.Resolver.BackoffBase, expected: 200 * time.Millisecond},
		{name: "Resolver.BackoffMax", got: cfg.Resolver.BackoffMax, expected: 5 * time.Second},
		{name: "Resolver.FollowCNAME", got: cfg.Resolver.FollowCNAME, expected: true},
		{name: "Resolver.EnableV4", got: cfg.Resolver.EnableV4, expected: true},
		{name: "Resolver.EnableV6", got: cfg.Resolver.EnableV6, expected: true},
		{name: "Cache.DBPath", got: cfg.Cache.DBPath, expected: "/var/lib/d2ip/cache.db"},
		{name: "Cache.TTL", got: cfg.Cache.TTL, expected: 6 * time.Hour},
		{name: "Cache.FailedTTL", got: cfg.Cache.FailedTTL, expected: 30 * time.Minute},
		{name: "Cache.VacuumAfter", got: cfg.Cache.VacuumAfter, expected: 720 * time.Hour},
		{name: "Aggregation.Enabled", got: cfg.Aggregation.Enabled, expected: true},
		{name: "Aggregation.Level", got: cfg.Aggregation.Level, expected: AggBalanced},
		{name: "Aggregation.V4MaxPrefix", got: cfg.Aggregation.V4MaxPrefix, expected: 16},
		{name: "Aggregation.V6MaxPrefix", got: cfg.Aggregation.V6MaxPrefix, expected: 32},
		{name: "Export.Dir", got: cfg.Export.Dir, expected: "/var/lib/d2ip/out"},
		{name: "Export.IPv4File", got: cfg.Export.IPv4File, expected: "ipv4.txt"},
		{name: "Export.IPv6File", got: cfg.Export.IPv6File, expected: "ipv6.txt"},
		{name: "Routing.Enabled", got: cfg.Routing.Enabled, expected: false},
		{name: "Routing.StateDir", got: cfg.Routing.StateDir, expected: "/var/lib/d2ip"},
		{name: "Scheduler.DLCRefresh", got: cfg.Scheduler.DLCRefresh, expected: 24 * time.Hour},
		{name: "Scheduler.ResolveCycle", got: cfg.Scheduler.ResolveCycle, expected: 1 * time.Hour},
		{name: "Logging.Level", got: cfg.Logging.Level, expected: "info"},
		{name: "Logging.Format", got: cfg.Logging.Format, expected: "json"},
		{name: "Metrics.Enabled", got: cfg.Metrics.Enabled, expected: true},
		{name: "Metrics.Path", got: cfg.Metrics.Path, expected: "/metrics"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !reflect.DeepEqual(tt.got, tt.expected) {
				t.Errorf("got %v, expected %v", tt.got, tt.expected)
			}
		})
	}

	// Validate Sources slice (should have exactly one default source).
	if len(cfg.Sources) != 1 {
		t.Fatalf("expected 1 default source, got %d", len(cfg.Sources))
	}
	src := cfg.Sources[0]
	if src.ID != "default-geosite" {
		t.Errorf("expected source ID 'default-geosite', got %s", src.ID)
	}
	if src.Provider != "v2flygeosite" {
		t.Errorf("expected source Provider 'v2flygeosite', got %s", src.Provider)
	}
	if src.Prefix != "geosite" {
		t.Errorf("expected source Prefix 'geosite', got %s", src.Prefix)
	}
	if !src.Enabled {
		t.Errorf("expected source Enabled true")
	}
	if src.Config == nil {
		t.Fatalf("expected source Config to be non-nil")
	}
	expectedConfig := map[string]any{
		"url":              "https://github.com/v2fly/domain-list-community/releases/latest/download/dlc.dat",
		"cache_path":       "/var/lib/d2ip/dlc.dat",
		"refresh_interval": "24h",
		"http_timeout":     "30s",
	}
	if !reflect.DeepEqual(src.Config, expectedConfig) {
		t.Errorf("expected source Config %v, got %v", expectedConfig, src.Config)
	}

}

func TestClone_Independent(t *testing.T) {
	original := Defaults()
	// Ensure we have Sources to mutate.
	original.Sources = []SourceItemConfig{
		{
			ID:       "src1",
			Provider: "v2flygeosite",
			Enabled:  true,
			Config:   map[string]any{"key": "value"},
		},
	}

	cloned := original.Clone()

	// Mutate Sources in clone.
	cloned.Sources[0].Config["key"] = "mutated"
	cloned.Sources[0].Config["newkey"] = "newvalue"
	cloned.Sources = append(cloned.Sources, SourceItemConfig{ID: "src2"})

	// Verify original is unaffected.
	if original.Sources[0].Config["key"] != "value" {
		t.Errorf("original Sources[0].Config was mutated: %v", original.Sources[0].Config)
	}
	if _, ok := original.Sources[0].Config["newkey"]; ok {
		t.Errorf("original Sources[0].Config received unexpected key")
	}
	if len(original.Sources) != 1 {
		t.Errorf("original Sources length changed: %d", len(original.Sources))
	}
}

func TestClone_PoliciesDeepCopy(t *testing.T) {
	agg := &AggregationConfig{
		Enabled:     true,
		Level:       AggAggressive,
		V4MaxPrefix: 8,
		V6MaxPrefix: 16,
	}
	original := Defaults()
	original.Routing.Policies = []PolicyConfig{
		{
			Name:         "policy1",
			Enabled:      true,
			Categories:   []string{"cat1", "cat2"},
			Backend:      BackendNFTables,
			TableID:      100,
			Iface:        "eth0",
			NFTTable:     "inet d2ip",
			NFTSetV4:     "set_v4",
			NFTSetV6:     "set_v6",
			DryRun:       false,
			ExportFormat: "plain",
			Aggregation:  agg,
		},
	}

	cloned := original.Clone()

	// Mutate nested slices and pointers in the clone.
	cloned.Routing.Policies[0].Categories[0] = "mutated"
	cloned.Routing.Policies[0].Categories = append(cloned.Routing.Policies[0].Categories, "cat3")
	cloned.Routing.Policies[0].Aggregation.Level = AggOff
	cloned.Routing.Policies[0].Aggregation.V4MaxPrefix = 32
	cloned.Routing.Policies[0].Aggregation.Enabled = false
	// Replace the pointer entirely.
	cloned.Routing.Policies[0].Aggregation = &AggregationConfig{Level: AggConservative}

	// Verify original is unaffected.
	if original.Routing.Policies[0].Categories[0] != "cat1" {
		t.Errorf("original Policy Categories[0] was mutated: %s", original.Routing.Policies[0].Categories[0])
	}
	if len(original.Routing.Policies[0].Categories) != 2 {
		t.Errorf("original Policy Categories length changed: %d", len(original.Routing.Policies[0].Categories))
	}
	if original.Routing.Policies[0].Aggregation == nil {
		t.Fatalf("original Policy Aggregation is nil")
	}
	if original.Routing.Policies[0].Aggregation.Level != AggAggressive {
		t.Errorf("original Policy Aggregation.Level was mutated: %s", original.Routing.Policies[0].Aggregation.Level)
	}
	if original.Routing.Policies[0].Aggregation.V4MaxPrefix != 8 {
		t.Errorf("original Policy Aggregation.V4MaxPrefix was mutated: %d", original.Routing.Policies[0].Aggregation.V4MaxPrefix)
	}
	if !original.Routing.Policies[0].Aggregation.Enabled {
		t.Errorf("original Policy Aggregation.Enabled was mutated")
	}
}

func TestClone_NilSlices(t *testing.T) {
	original := Config{
		Listen:  ":9099",
		Sources: nil,
		Routing: RoutingConfig{
			Policies: nil,
		},
	}

	cloned := original.Clone()

	// Verify no panic and clone is usable.
	if cloned.Listen != ":9099" {
		t.Errorf("expected Listen ':9099', got %s", cloned.Listen)
	}
	if cloned.Sources != nil {
		t.Errorf("expected nil Sources, got %v", cloned.Sources)
	}
	if cloned.Routing.Policies != nil {
		t.Errorf("expected nil Policies, got %v", cloned.Routing.Policies)
	}
}

func TestClone_PolicyNilAggregation(t *testing.T) {
	original := Defaults()
	original.Routing.Policies = []PolicyConfig{
		{
			Name:        "policy1",
			Enabled:     true,
			Categories:  []string{"cat1"},
			Backend:     BackendNFTables,
			Aggregation: nil,
		},
	}

	cloned := original.Clone()

	// Verify original is unaffected and Aggregation remains nil.
	if original.Routing.Policies[0].Aggregation != nil {
		t.Errorf("original Policy Aggregation expected nil, got %v", original.Routing.Policies[0].Aggregation)
	}
	if cloned.Routing.Policies[0].Aggregation != nil {
		t.Errorf("cloned Policy Aggregation expected nil, got %v", cloned.Routing.Policies[0].Aggregation)
	}

	// Verify both original and clone are usable.
	if original.Routing.Policies[0].Categories[0] != "cat1" {
		t.Errorf("original Policy Categories[0] was mutated: %s", original.Routing.Policies[0].Categories[0])
	}
	if cloned.Routing.Policies[0].Categories[0] != "cat1" {
		t.Errorf("cloned Policy Categories[0] unexpected value: %s", cloned.Routing.Policies[0].Categories[0])
	}
}
