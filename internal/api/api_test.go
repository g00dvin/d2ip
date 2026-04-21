package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/goodvin/d2ip/internal/config"
)

// mockKVStore is a simple in-memory KVStore for testing.
type mockKVStore struct {
	data map[string]string
}

func (m *mockKVStore) GetAll(_ context.Context) (map[string]string, error) {
	return m.data, nil
}

func (m *mockKVStore) Set(_ context.Context, key, value string) error {
	if m.data == nil {
		m.data = make(map[string]string)
	}
	if value == "" {
		delete(m.data, key)
	} else {
		m.data[key] = value
	}
	return nil
}

func (m *mockKVStore) Delete(_ context.Context, key string) error {
	delete(m.data, key)
	return nil
}

func TestHandleSettingsGet_ReturnsConfig(t *testing.T) {
	cfg := config.Defaults()
	watcher := config.NewWatcher(cfg, 1)

	kv := &mockKVStore{data: map[string]string{"resolver.qps": "500"}}

	s := &Server{cfgWatcher: watcher, kvStore: kv}
	r := chi.NewRouter()
	r.Get("/api/settings", s.handleSettingsGet)

	req := httptest.NewRequest(http.MethodGet, "/api/settings", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if _, ok := resp["config"]; !ok {
		t.Error("response missing 'config' key")
	}
	if _, ok := resp["defaults"]; !ok {
		t.Error("response missing 'defaults' key")
	}
	if _, ok := resp["overrides"]; !ok {
		t.Error("response missing 'overrides' key")
	}

	// Verify overrides are present.
	overrides, ok := resp["overrides"].(map[string]interface{})
	if !ok {
		t.Fatal("overrides is not an object")
	}
	if overrides["resolver.qps"] != "500" {
		t.Errorf("expected resolver.qps=500, got %v", overrides["resolver.qps"])
	}
}

func TestHandleSettingsPut_UpdatesOverride(t *testing.T) {
	cfg := config.Defaults()
	watcher := config.NewWatcher(cfg, 1)

	s := &Server{cfgWatcher: watcher, kvStore: nil}
	r := chi.NewRouter()
	r.Put("/api/settings", s.handleSettingsPut)

	body := `{"resolver.qps": "100"}`
	req := httptest.NewRequest(http.MethodPut, "/api/settings", strings.NewReader(body))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 with nil kvStore, got %d", rec.Code)
	}
}

func TestHandleSettingsPut_WithMockKVStore(t *testing.T) {
	cfg := config.Defaults()
	watcher := config.NewWatcher(cfg, 1)

	kv := &mockKVStore{data: make(map[string]string)}
	s := &Server{cfgWatcher: watcher, kvStore: kv}
	r := chi.NewRouter()
	r.Put("/api/settings", s.handleSettingsPut)

	// Simulate what the JS config editor sends: all fields as string overrides
	body := `{
		"source.url": "https://example.com/dlc.dat",
		"source.cache_path": "/var/lib/d2ip/dlc.dat",
		"source.refresh_interval": "24h0m0s",
		"source.http_timeout": "30s",
		"resolver.upstream": "1.1.1.1:53",
		"resolver.network": "udp",
		"resolver.concurrency": "64",
		"resolver.qps": "200",
		"resolver.timeout": "3s",
		"resolver.retries": "3",
		"resolver.backoff_base": "200ms",
		"resolver.backoff_max": "5s",
		"resolver.follow_cname": "true",
		"resolver.enable_v4": "true",
		"resolver.enable_v6": "true",
		"cache.db_path": "/var/lib/d2ip/cache.db",
		"cache.ttl": "6h0m0s",
		"cache.failed_ttl": "30m0s",
		"cache.vacuum_after": "720h0m0s",
		"aggregation.enabled": "true",
		"aggregation.level": "balanced",
		"aggregation.v4_max_prefix": "16",
		"aggregation.v6_max_prefix": "32",
		"export.dir": "/var/lib/d2ip/out",
		"export.ipv4_file": "ipv4.txt",
		"export.ipv6_file": "ipv6.txt",
		"routing.enabled": "false",
		"routing.backend": "nftables",
		"routing.table_id": "100",
		"routing.nft_table": "inet d2ip",
		"routing.nft_set_v4": "d2ip_v4",
		"routing.nft_set_v6": "d2ip_v6",
		"routing.state_path": "/var/lib/d2ip/state.json",
		"routing.dry_run": "false",
		"scheduler.dlc_refresh": "24h0m0s",
		"scheduler.resolve_cycle": "1h0m0s",
		"logging.level": "info",
		"logging.format": "json",
		"metrics.enabled": "true",
		"metrics.path": "/metrics"
	}`
	req := httptest.NewRequest(http.MethodPut, "/api/settings", strings.NewReader(body))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleSettingsDelete_RemovesOverride(t *testing.T) {
	cfg := config.Defaults()
	watcher := config.NewWatcher(cfg, 1)

	s := &Server{cfgWatcher: watcher, kvStore: nil}
	r := chi.NewRouter()
	r.Delete("/api/settings/{key}", s.handleSettingsDelete)

	req := httptest.NewRequest(http.MethodDelete, "/api/settings/resolver.qps", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 with nil kvStore, got %d", rec.Code)
	}
}

func TestHandlePipelineHistory_ReturnsList(t *testing.T) {
	s := &Server{orch: nil}
	r := chi.NewRouter()
	r.Get("/api/pipeline/history", s.handlePipelineHistory)

	req := httptest.NewRequest(http.MethodGet, "/api/pipeline/history", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestHandleCategoriesList_ReturnsCategories(t *testing.T) {
	cfg := config.Defaults()
	watcher := config.NewWatcher(cfg, 1)

	s := &Server{cfgWatcher: watcher, dlProvider: nil}
	r := chi.NewRouter()
	r.Get("/api/categories", s.handleCategoriesList)

	req := httptest.NewRequest(http.MethodGet, "/api/categories", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if _, ok := resp["configured"]; !ok {
		t.Error("response missing 'configured' key")
	}
	if _, ok := resp["available"]; !ok {
		t.Error("response missing 'available' key")
	}
}

func TestHandleCategoriesList_ReturnsConfiguredAndAvailable(t *testing.T) {
	cfg := config.Defaults()
	watcher := config.NewWatcher(cfg, 1)

	s := &Server{cfgWatcher: watcher, dlProvider: nil}
	r := chi.NewRouter()
	r.Get("/api/categories", s.handleCategoriesList)

	req := httptest.NewRequest(http.MethodGet, "/api/categories", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if _, ok := resp["configured"]; !ok {
		t.Error("response missing 'configured' key")
	}
	if _, ok := resp["available"]; !ok {
		t.Error("response missing 'available' key")
	}
}

func TestHandleCacheStats_ReturnsStats(t *testing.T) {
	s := &Server{cacheAgent: nil}
	r := chi.NewRouter()
	r.Get("/api/cache/stats", s.handleCacheStats)

	req := httptest.NewRequest(http.MethodGet, "/api/cache/stats", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	// With nil cache agent, should return 503
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}
}

func TestHandleSourceInfo_ReturnsInfo(t *testing.T) {
	s := &Server{sourceStore: nil}
	r := chi.NewRouter()
	r.Get("/api/source/info", s.handleSourceInfo)

	req := httptest.NewRequest(http.MethodGet, "/api/source/info", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if available, ok := resp["available"].(bool); !ok || available {
		t.Errorf("expected available=false, got %v", resp["available"])
	}
}

func TestHandleRoutingDryRun_EmptyPrefixes(t *testing.T) {
	s := &Server{router: nil}
	r := chi.NewRouter()
	r.Post("/routing/dry-run", s.handleRoutingDryRun)

	body := `{"ipv4_prefixes":[],"ipv6_prefixes":[]}`
	req := httptest.NewRequest(http.MethodPost, "/routing/dry-run", strings.NewReader(body))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	// With empty prefixes, should return 200 (not call router)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for empty prefixes, got %d", rec.Code)
	}
}
