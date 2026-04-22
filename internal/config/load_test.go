package config

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// clearEnv unsets every D2IP_* variable so a test starts from a clean slate.
// t.Setenv restores individual keys, but tests that list *all* overrides
// need a blanket sweep to avoid pollution from the host.
func clearEnv(t *testing.T) {
	t.Helper()
	for _, kv := range os.Environ() {
		if strings.HasPrefix(kv, EnvPrefix+"_") {
			k := strings.SplitN(kv, "=", 2)[0]
			t.Setenv(k, "") // will be restored; viper treats empty as unset for most types
			if err := os.Unsetenv(k); err != nil {
				t.Fatalf("unset %s: %v", k, err)
			}
		}
	}
}

func TestLoad_Defaults(t *testing.T) {
	clearEnv(t)

	cfg, err := Load(LoadOptions{})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	want := Defaults()
	if cfg.Listen != want.Listen {
		t.Errorf("Listen: got %q want %q", cfg.Listen, want.Listen)
	}
	if cfg.Resolver.QPS != want.Resolver.QPS {
		t.Errorf("Resolver.QPS: got %d want %d", cfg.Resolver.QPS, want.Resolver.QPS)
	}
	if cfg.Cache.TTL != want.Cache.TTL {
		t.Errorf("Cache.TTL: got %s want %s", cfg.Cache.TTL, want.Cache.TTL)
	}
	if cfg.Aggregation.Level != want.Aggregation.Level {
		t.Errorf("Aggregation.Level: got %q want %q", cfg.Aggregation.Level, want.Aggregation.Level)
	}
	if cfg.Routing.Enabled {
		t.Errorf("Routing.Enabled: defaults must be false")
	}
	if !cfg.Resolver.EnableV4 || !cfg.Resolver.EnableV6 {
		t.Errorf("Resolver: v4/v6 must default to true")
	}
}

func TestLoad_EnvOverride(t *testing.T) {
	clearEnv(t)

	t.Setenv("D2IP_RESOLVER_QPS", "500")
	t.Setenv("D2IP_RESOLVER_CONCURRENCY", "128")
	t.Setenv("D2IP_CACHE_TTL", "12h")
	t.Setenv("D2IP_AGGREGATION_LEVEL", "aggressive")
	t.Setenv("D2IP_LISTEN", "127.0.0.1:9000")
	t.Setenv("D2IP_METRICS_ENABLED", "false")

	cfg, err := Load(LoadOptions{})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.Resolver.QPS != 500 {
		t.Errorf("Resolver.QPS: got %d want 500", cfg.Resolver.QPS)
	}
	if cfg.Resolver.Concurrency != 128 {
		t.Errorf("Resolver.Concurrency: got %d want 128", cfg.Resolver.Concurrency)
	}
	if cfg.Cache.TTL != 12*time.Hour {
		t.Errorf("Cache.TTL: got %s want 12h", cfg.Cache.TTL)
	}
	if cfg.Aggregation.Level != AggAggressive {
		t.Errorf("Aggregation.Level: got %q want aggressive", cfg.Aggregation.Level)
	}
	if cfg.Listen != "127.0.0.1:9000" {
		t.Errorf("Listen: got %q want 127.0.0.1:9000", cfg.Listen)
	}
	if cfg.Metrics.Enabled {
		t.Errorf("Metrics.Enabled: expected false")
	}
}

func TestLoad_CategoriesFromEnvJSON(t *testing.T) {
	clearEnv(t)

	t.Setenv("D2IP_CATEGORIES", `[{"code":"geosite:ru"},{"code":"geosite:google","attrs":["cn"]}]`)

	cfg, err := Load(LoadOptions{})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(cfg.Categories) != 2 {
		t.Fatalf("Categories: got %d entries want 2", len(cfg.Categories))
	}
	if cfg.Categories[0].Code != "geosite:ru" {
		t.Errorf("Categories[0].Code: %q", cfg.Categories[0].Code)
	}
	if cfg.Categories[1].Code != "geosite:google" ||
		len(cfg.Categories[1].Attrs) != 1 || cfg.Categories[1].Attrs[0] != "cn" {
		t.Errorf("Categories[1]: %+v", cfg.Categories[1])
	}
}

func TestLoad_YAMLSeed(t *testing.T) {
	clearEnv(t)

	dir := t.TempDir()
	yaml := filepath.Join(dir, "config.yaml")
	const body = `
listen: ":7070"
resolver:
  qps: 321
  timeout: 4s
cache:
  ttl: 2h
categories:
  - code: geosite:ru
`
	if err := os.WriteFile(yaml, []byte(body), 0o600); err != nil {
		t.Fatalf("write yaml: %v", err)
	}

	cfg, err := Load(LoadOptions{ConfigFile: yaml})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Listen != ":7070" {
		t.Errorf("Listen: %q", cfg.Listen)
	}
	if cfg.Resolver.QPS != 321 {
		t.Errorf("QPS: %d", cfg.Resolver.QPS)
	}
	if cfg.Resolver.Timeout != 4*time.Second {
		t.Errorf("Timeout: %s", cfg.Resolver.Timeout)
	}
	if cfg.Cache.TTL != 2*time.Hour {
		t.Errorf("Cache.TTL: %s", cfg.Cache.TTL)
	}
	if len(cfg.Categories) != 1 || cfg.Categories[0].Code != "geosite:ru" {
		t.Errorf("Categories: %+v", cfg.Categories)
	}
}

func TestLoad_DefaultSearchPaths(t *testing.T) {
	clearEnv(t)

	dir := t.TempDir()
	yaml := filepath.Join(dir, "config.yaml")
	const body = `
listen: ":8080"
resolver:
  qps: 777
`
	if err := os.WriteFile(yaml, []byte(body), 0o600); err != nil {
		t.Fatalf("write yaml: %v", err)
	}

	cfg, err := Load(LoadOptions{SearchPaths: []string{dir}})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Listen != ":8080" {
		t.Errorf("Listen: got %q, want :8080", cfg.Listen)
	}
	if cfg.Resolver.QPS != 777 {
		t.Errorf("QPS: got %d, want 777", cfg.Resolver.QPS)
	}
}

func TestLoad_DefaultSearchPathsFallsBackToDefaults(t *testing.T) {
	clearEnv(t)

	cfg, err := Load(LoadOptions{})
	if err != nil {
		t.Fatalf("Load with no config file: %v", err)
	}
	want := Defaults()
	if cfg.Listen != want.Listen {
		t.Errorf("Listen: got %q, want %q", cfg.Listen, want.Listen)
	}
}

func TestLoad_EnvBeatsYAMLBeatsKVBeatsDefaults(t *testing.T) {
	// Precedence: ENV > kv > YAML > defaults (per docs/agents/08-config.md §2,
	// with YAML treated as a first-run seed that kv and ENV override).
	clearEnv(t)

	dir := t.TempDir()
	yaml := filepath.Join(dir, "config.yaml")
	const body = `
resolver:
  qps: 100
`
	if err := os.WriteFile(yaml, []byte(body), 0o600); err != nil {
		t.Fatalf("write yaml: %v", err)
	}

	// kv overrides should beat YAML.
	kv := map[string]string{"resolver.qps": "250"}

	// ENV should beat everything.
	t.Setenv("D2IP_RESOLVER_QPS", "777")

	cfg, err := Load(LoadOptions{ConfigFile: yaml, KVOverrides: kv})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Resolver.QPS != 777 {
		t.Errorf("QPS: got %d want 777 (ENV should win)", cfg.Resolver.QPS)
	}
}

func TestLoad_KVWithoutEnv(t *testing.T) {
	clearEnv(t)
	kv := map[string]string{
		"resolver.qps":        "999",
		"cache.ttl":           "8h",
		"aggregation.enabled": "false",
	}
	cfg, err := Load(LoadOptions{KVOverrides: kv})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Resolver.QPS != 999 {
		t.Errorf("QPS: %d", cfg.Resolver.QPS)
	}
	if cfg.Cache.TTL != 8*time.Hour {
		t.Errorf("Cache.TTL: %s", cfg.Cache.TTL)
	}
	if cfg.Aggregation.Enabled {
		t.Errorf("Aggregation.Enabled: expected false")
	}
}

func TestValidate(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(c *Config)
		wantErr string // substring; empty means expect no error
	}{
		{name: "defaults ok", mutate: func(*Config) {}},
		{
			name:    "qps too low",
			mutate:  func(c *Config) { c.Resolver.QPS = 0 },
			wantErr: "resolver.qps",
		},
		{
			name:    "qps too high",
			mutate:  func(c *Config) { c.Resolver.QPS = 200001 },
			wantErr: "resolver.qps",
		},
		{
			name:    "concurrency too high",
			mutate:  func(c *Config) { c.Resolver.Concurrency = 99999 },
			wantErr: "resolver.concurrency",
		},
		{
			name:    "concurrency zero",
			mutate:  func(c *Config) { c.Resolver.Concurrency = 0 },
			wantErr: "resolver.concurrency",
		},
		{
			name:    "cache ttl too small",
			mutate:  func(c *Config) { c.Cache.TTL = 10 * time.Second },
			wantErr: "cache.ttl",
		},
		{
			name:    "v4 max prefix out of range",
			mutate:  func(c *Config) { c.Aggregation.V4MaxPrefix = 40 },
			wantErr: "v4_max_prefix",
		},
		{
			name:    "v6 max prefix out of range",
			mutate:  func(c *Config) { c.Aggregation.V6MaxPrefix = 8 },
			wantErr: "v6_max_prefix",
		},
		{
			name:    "unknown aggregation level",
			mutate:  func(c *Config) { c.Aggregation.Level = "extreme" },
			wantErr: "aggregation.level",
		},
		{
			name:    "routing enabled with backend none",
			mutate:  func(c *Config) { c.Routing.Enabled = true; c.Routing.Backend = BackendNone },
			wantErr: "routing.enabled",
		},
		{
			name:    "unknown routing backend",
			mutate:  func(c *Config) { c.Routing.Backend = "pfctl" },
			wantErr: "routing.backend",
		},
		{
			name:    "both v4 and v6 disabled",
			mutate:  func(c *Config) { c.Resolver.EnableV4 = false; c.Resolver.EnableV6 = false },
			wantErr: "enable_v4",
		},
		{
			name:    "bad resolver upstream",
			mutate:  func(c *Config) { c.Resolver.Upstream = "not-a-host" },
			wantErr: "resolver.upstream",
		},
		{
			name:    "bad resolver network",
			mutate:  func(c *Config) { c.Resolver.Network = "https" },
			wantErr: "resolver.network",
		},
		{
			name:    "bad listen",
			mutate:  func(c *Config) { c.Listen = "not-a-port" },
			wantErr: "listen",
		},
		{
			name: "duplicate categories",
			mutate: func(c *Config) {
				c.Categories = []CategoryConfig{{Code: "geosite:ru"}, {Code: "geosite:ru"}}
			},
			wantErr: "duplicate",
		},
		{
			name: "category missing colon",
			mutate: func(c *Config) {
				c.Categories = []CategoryConfig{{Code: "ru"}}
			},
			wantErr: "categories[0]",
		},
		{
			name:    "ipv4 equals ipv6 filename",
			mutate:  func(c *Config) { c.Export.IPv6File = c.Export.IPv4File },
			wantErr: "export.ipv",
		},
		{
			name:    "metrics path no slash",
			mutate:  func(c *Config) { c.Metrics.Path = "metrics" },
			wantErr: "metrics.path",
		},
		{
			name:    "logging level unknown",
			mutate:  func(c *Config) { c.Logging.Level = "loud" },
			wantErr: "logging.level",
		},
		{
			name:    "backoff_max < backoff_base",
			mutate:  func(c *Config) { c.Resolver.BackoffMax = 1 * time.Millisecond },
			wantErr: "backoff_max",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := Defaults()
			tc.mutate(&cfg)
			errs := cfg.Validate()
			joined := errors.Join(errs...)
			if tc.wantErr == "" {
				if joined != nil {
					t.Fatalf("expected no errors, got: %v", joined)
				}
				return
			}
			if joined == nil {
				t.Fatalf("expected error containing %q, got nil", tc.wantErr)
			}
			if !strings.Contains(joined.Error(), tc.wantErr) {
				t.Fatalf("error %q does not contain %q", joined.Error(), tc.wantErr)
			}
		})
	}
}

func TestLoad_InvalidConfigRejected(t *testing.T) {
	clearEnv(t)
	t.Setenv("D2IP_RESOLVER_QPS", "0") // below minimum

	_, err := Load(LoadOptions{})
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if !strings.Contains(err.Error(), "resolver.qps") {
		t.Errorf("expected qps error, got: %v", err)
	}
}

func TestDurationParsing(t *testing.T) {
	cases := []struct {
		in   string
		want time.Duration
		err  bool
	}{
		{"6h", 6 * time.Hour, false},
		{"30m", 30 * time.Minute, false},
		{"500ms", 500 * time.Millisecond, false},
		{"2h30m", 2*time.Hour + 30*time.Minute, false},
		{"45", 45 * time.Second, false}, // bare integer → seconds
		{"", 0, true},
		{"banana", 0, true},
	}
	for _, tc := range cases {
		got, err := parseDuration(tc.in)
		if tc.err {
			if err == nil {
				t.Errorf("parseDuration(%q): want error, got %s", tc.in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("parseDuration(%q): unexpected err %v", tc.in, err)
			continue
		}
		if got != tc.want {
			t.Errorf("parseDuration(%q): got %s want %s", tc.in, got, tc.want)
		}
	}
}

func TestApplyOverrides(t *testing.T) {
	cfg := Defaults()
	kv := map[string]string{
		"resolver.qps":        "321",
		"resolver.timeout":    "7s",
		"cache.ttl":           "90m",
		"aggregation.enabled": "false",
		"categories":          `[{"code":"geosite:ru"}]`,
	}
	if err := ApplyOverrides(&cfg, kv); err != nil {
		t.Fatalf("ApplyOverrides: %v", err)
	}
	if cfg.Resolver.QPS != 321 {
		t.Errorf("QPS: %d", cfg.Resolver.QPS)
	}
	if cfg.Resolver.Timeout != 7*time.Second {
		t.Errorf("Timeout: %s", cfg.Resolver.Timeout)
	}
	if cfg.Cache.TTL != 90*time.Minute {
		t.Errorf("Cache.TTL: %s", cfg.Cache.TTL)
	}
	if cfg.Aggregation.Enabled {
		t.Errorf("Aggregation.Enabled: expected false")
	}
	if len(cfg.Categories) != 1 || cfg.Categories[0].Code != "geosite:ru" {
		t.Errorf("Categories: %+v", cfg.Categories)
	}
}

func TestApplyOverrides_InvalidValue(t *testing.T) {
	cfg := Defaults()
	err := ApplyOverrides(&cfg, map[string]string{"resolver.qps": "not-a-number"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestWatcher_PublishAndSubscribe(t *testing.T) {
	w := NewWatcher(Defaults(), 1)
	defer w.Close()

	sub, cancel := w.Subscribe()
	defer cancel()

	next := Defaults()
	next.Resolver.QPS = 1234
	if err := w.Publish(next); err != nil {
		t.Fatalf("Publish: %v", err)
	}

	select {
	case snap := <-sub:
		if snap.Config.Resolver.QPS != 1234 {
			t.Errorf("snapshot QPS: %d", snap.Config.Resolver.QPS)
		}
		if snap.Version < 2 {
			t.Errorf("snapshot version: %d (expected >= 2)", snap.Version)
		}
	case <-time.After(time.Second):
		t.Fatal("no snapshot delivered within 1s")
	}

	cur := w.Current()
	if cur.Config.Resolver.QPS != 1234 {
		t.Errorf("Current QPS: %d", cur.Config.Resolver.QPS)
	}
}

func TestWatcher_RejectsInvalid(t *testing.T) {
	w := NewWatcher(Defaults(), 1)
	defer w.Close()

	bad := Defaults()
	bad.Resolver.QPS = -1
	if err := w.Publish(bad); err == nil {
		t.Fatal("expected validation error on Publish, got nil")
	}
	// state unchanged
	if w.Current().Config.Resolver.QPS != Defaults().Resolver.QPS {
		t.Fatal("invalid Publish mutated state")
	}
}

func TestWatcher_MultipleSubscribersConcurrent(t *testing.T) {
	// Use buffer=5 to ensure all 3 publishes can be queued without coalescing.
	w := NewWatcher(Defaults(), 5)
	defer w.Close()

	const subs = 8
	var wg sync.WaitGroup
	wg.Add(subs)
	received := make([]int, subs)
	cancels := make([]func(), subs)
	for i := 0; i < subs; i++ {
		ch, cancel := w.Subscribe()
		cancels[i] = cancel
		go func(i int, ch <-chan Snapshot) {
			defer wg.Done()
			for range ch {
				received[i]++
				if received[i] >= 3 {
					return
				}
			}
		}(i, ch)
	}
	defer func() {
		for _, c := range cancels {
			c()
		}
	}()

	// Give goroutines time to start consuming before we publish rapidly.
	time.Sleep(10 * time.Millisecond)

	for i := 0; i < 3; i++ {
		next := Defaults()
		next.Resolver.QPS = 100 + i
		if err := w.Publish(next); err != nil {
			t.Fatalf("Publish %d: %v", i, err)
		}
		// Small delay between publishes to avoid overwhelming the coalescing logic.
		time.Sleep(5 * time.Millisecond)
	}

	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("subscribers did not all receive 3 updates within 2s")
	}
}

func TestWatcher_CancelStopsDelivery(t *testing.T) {
	w := NewWatcher(Defaults(), 1)
	defer w.Close()

	sub, cancel := w.Subscribe()
	cancel()

	// After cancel, channel should be closed.
	if _, ok := <-sub; ok {
		t.Fatal("channel should be closed after cancel")
	}
}

func TestKVStoreInterface_Compile(t *testing.T) {
	// Compile-time assertion that the in-memory fake satisfies KVStore.
	var _ KVStore = (*memoryKV)(nil)

	m := &memoryKV{data: map[string]string{"resolver.qps": "500"}}
	ctx := context.Background()
	got, err := m.GetAll(ctx)
	if err != nil {
		t.Fatalf("GetAll: %v", err)
	}
	if got["resolver.qps"] != "500" {
		t.Errorf("GetAll: %v", got)
	}
}

// memoryKV is a trivial in-memory KVStore used for tests and as a reference
// implementation shape.
type memoryKV struct {
	mu   sync.Mutex
	data map[string]string
}

func (m *memoryKV) GetAll(_ context.Context) (map[string]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make(map[string]string, len(m.data))
	for k, v := range m.data {
		out[k] = v
	}
	return out, nil
}

func (m *memoryKV) Set(_ context.Context, key, value string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.data == nil {
		m.data = map[string]string{}
	}
	if value == "" {
		delete(m.data, key)
		return nil
	}
	m.data[key] = value
	return nil
}

func (m *memoryKV) Delete(_ context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.data, key)
	return nil
}
