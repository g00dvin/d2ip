package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/goodvin/d2ip/internal/config"
	"github.com/goodvin/d2ip/internal/metrics"
	"github.com/goodvin/d2ip/internal/orchestrator"
	"github.com/goodvin/d2ip/internal/routing"
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
	watcher := config.NewWatcher(cfg, 1, nil)

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
	watcher := config.NewWatcher(cfg, 1, nil)

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
	watcher := config.NewWatcher(cfg, 1, nil)

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
	watcher := config.NewWatcher(cfg, 1, nil)

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
	watcher := config.NewWatcher(cfg, 1, nil)

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
	watcher := config.NewWatcher(cfg, 1, nil)

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

func TestHandleSourcesList_ReturnsEmpty(t *testing.T) {
	s := &Server{registry: nil}
	r := chi.NewRouter()
	r.Get("/api/sources", s.handleSourcesList)

	req := httptest.NewRequest(http.MethodGet, "/api/sources", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	sources, ok := resp["sources"].([]interface{})
	if !ok {
		t.Errorf("expected sources array, got %T", resp["sources"])
	} else if len(sources) != 0 {
		t.Errorf("expected empty sources, got %d", len(sources))
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

// mockOrchestrator implements pipelineOrchestrator for testing.
type mockOrchestrator struct {
	runReport orchestrator.PipelineReport
	runErr    error
	status    orchestrator.RunStatus
}

func (m *mockOrchestrator) Run(ctx context.Context, req orchestrator.PipelineRequest) (orchestrator.PipelineReport, error) {
	return m.runReport, m.runErr
}
func (m *mockOrchestrator) Cancel() error                                         { return nil }
func (m *mockOrchestrator) Status() orchestrator.RunStatus                       { return m.status }
func (m *mockOrchestrator) History() []orchestrator.PipelineReport               { return nil }

func TestHandleVersion(t *testing.T) {
	cfg := config.Defaults()
	watcher := config.NewWatcher(cfg, 1, nil)
	s := &Server{cfgWatcher: watcher}
	s.SetVersion("1.0.0", "2024-01-01T00:00:00Z")

	r := chi.NewRouter()
	r.Get("/api/version", s.handleVersion)

	req := httptest.NewRequest(http.MethodGet, "/api/version", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}
	if resp["version"] != "1.0.0" {
		t.Errorf("expected version 1.0.0, got %s", resp["version"])
	}
	if resp["build_time"] != "2024-01-01T00:00:00Z" {
		t.Errorf("expected build_time, got %s", resp["build_time"])
	}
}

func TestHandleMetrics(t *testing.T) {
	metrics.Setup()
	cfg := config.Defaults()
	watcher := config.NewWatcher(cfg, 1, nil)
	s := &Server{cfgWatcher: watcher}

	r := chi.NewRouter()
	r.Get("/metrics", s.handleMetrics)

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	if !strings.Contains(rec.Body.String(), "go_") {
		t.Error("expected prometheus metrics output")
	}
}

func TestHandlePipelineRun_Success(t *testing.T) {
	cfg := config.Defaults()
	watcher := config.NewWatcher(cfg, 1, nil)
	mock := &mockOrchestrator{runReport: orchestrator.PipelineReport{RunID: 42}}
	s := &Server{cfgWatcher: watcher, orch: mock}

	r := chi.NewRouter()
	r.Post("/pipeline/run", s.handlePipelineRun)

	req := httptest.NewRequest(http.MethodPost, "/pipeline/run", strings.NewReader(`{}`))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}
	if resp["run_id"] != float64(42) {
		t.Errorf("expected run_id 42, got %v", resp["run_id"])
	}
}

func TestHandlePipelineRun_Busy(t *testing.T) {
	cfg := config.Defaults()
	watcher := config.NewWatcher(cfg, 1, nil)
	mock := &mockOrchestrator{runErr: orchestrator.ErrBusy}
	s := &Server{cfgWatcher: watcher, orch: mock}

	r := chi.NewRouter()
	r.Post("/pipeline/run", s.handlePipelineRun)

	req := httptest.NewRequest(http.MethodPost, "/pipeline/run", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandlePipelineRun_Error(t *testing.T) {
	cfg := config.Defaults()
	watcher := config.NewWatcher(cfg, 1, nil)
	mock := &mockOrchestrator{runErr: errors.New("boom")}
	s := &Server{cfgWatcher: watcher, orch: mock}

	r := chi.NewRouter()
	r.Post("/pipeline/run", s.handlePipelineRun)

	req := httptest.NewRequest(http.MethodPost, "/pipeline/run", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandlePipelineStatus_Success(t *testing.T) {
	cfg := config.Defaults()
	watcher := config.NewWatcher(cfg, 1, nil)
	mock := &mockOrchestrator{status: orchestrator.RunStatus{Running: true}}
	s := &Server{cfgWatcher: watcher, orch: mock}

	r := chi.NewRouter()
	r.Get("/pipeline/status", s.handlePipelineStatus)

	req := httptest.NewRequest(http.MethodGet, "/pipeline/status", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}
	if resp["running"] != true {
		t.Errorf("expected running true, got %v", resp["running"])
	}
}

// mockPolicyRouter implements routing.PolicyRouter for testing.
type mockPolicyRouter struct {
	snapshot routing.RouterState
}

func (m *mockPolicyRouter) Caps(ctx context.Context, policy config.PolicyConfig) error { return nil }
func (m *mockPolicyRouter) ApplyPolicy(ctx context.Context, policy config.PolicyConfig, v4, v6 []netip.Prefix) error {
	return nil
}
func (m *mockPolicyRouter) DryRunPolicy(ctx context.Context, policy config.PolicyConfig, v4, v6 []netip.Prefix) (routing.Plan, routing.Plan, string, string, error) {
	return routing.Plan{}, routing.Plan{}, "", "", nil
}
func (m *mockPolicyRouter) RollbackPolicy(ctx context.Context, policyName string) error { return nil }
func (m *mockPolicyRouter) SnapshotPolicy(policyName string) routing.RouterState {
	return m.snapshot
}

func TestHandleRoutingSnapshot_WithPolicies(t *testing.T) {
	cfg := config.Defaults()
	cfg.Routing.Policies = []config.PolicyConfig{
		{Name: "p1", Enabled: true, Backend: config.BackendNFTables},
	}
	watcher := config.NewWatcher(cfg, 1, nil)
	mockRtr := &mockPolicyRouter{snapshot: routing.RouterState{Backend: "nftables", V4: []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")}}}
	s := &Server{cfgWatcher: watcher, policyRtr: mockRtr}

	r := chi.NewRouter()
	r.Get("/routing/snapshot", s.handleRoutingSnapshot)

	req := httptest.NewRequest(http.MethodGet, "/routing/snapshot", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}
	if resp["backend"] != "policies" {
		t.Errorf("expected backend 'policies', got %v", resp["backend"])
	}
}

func TestHandleExportDownload_MissingParams(t *testing.T) {
	cfg := config.Defaults()
	watcher := config.NewWatcher(cfg, 1, nil)
	s := &Server{cfgWatcher: watcher}

	r := chi.NewRouter()
	r.Get("/api/export/download", s.handleExportDownload)

	req := httptest.NewRequest(http.MethodGet, "/api/export/download", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleExportDownload_NotFound(t *testing.T) {
	cfg := config.Defaults()
	watcher := config.NewWatcher(cfg, 1, nil)
	s := &Server{cfgWatcher: watcher}

	r := chi.NewRouter()
	r.Get("/api/export/download", s.handleExportDownload)

	req := httptest.NewRequest(http.MethodGet, "/api/export/download?policy=test&type=ipv4", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleExportDownload_Success(t *testing.T) {
	cfg := config.Defaults()
	cfg.Export.Dir = t.TempDir()
	watcher := config.NewWatcher(cfg, 1, nil)
	s := &Server{cfgWatcher: watcher}

	// Create the export file
	policyDir := filepath.Join(cfg.Export.Dir, "test")
	os.MkdirAll(policyDir, 0755)
	os.WriteFile(filepath.Join(policyDir, "ipv4.txt"), []byte("10.0.0.0/8\n"), 0644)

	r := chi.NewRouter()
	r.Get("/api/export/download", s.handleExportDownload)

	req := httptest.NewRequest(http.MethodGet, "/api/export/download?policy=test&type=ipv4", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	if !strings.Contains(rec.Body.String(), "10.0.0.0/8") {
		t.Errorf("expected prefix in response, got %s", rec.Body.String())
	}
}

func TestHandleConfigExport(t *testing.T) {
	cfg := config.Defaults()
	watcher := config.NewWatcher(cfg, 1, nil)
	kv := &mockKVStore{data: map[string]string{"resolver.qps": "500"}}
	s := &Server{cfgWatcher: watcher, kvStore: kv}

	r := chi.NewRouter()
	r.Get("/api/config/export", s.handleConfigExport)

	req := httptest.NewRequest(http.MethodGet, "/api/config/export", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	if !strings.Contains(rec.Header().Get("Content-Disposition"), "d2ip-config.json") {
		t.Errorf("expected attachment disposition, got %s", rec.Header().Get("Content-Disposition"))
	}
}

func TestHandleConfigImport_InvalidJSON(t *testing.T) {
	cfg := config.Defaults()
	watcher := config.NewWatcher(cfg, 1, nil)
	s := &Server{cfgWatcher: watcher}

	r := chi.NewRouter()
	r.Post("/api/config/import", s.handleConfigImport)

	req := httptest.NewRequest(http.MethodPost, "/api/config/import", strings.NewReader("bad json"))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleConfigImport_NilKVStore(t *testing.T) {
	cfg := config.Defaults()
	watcher := config.NewWatcher(cfg, 1, nil)
	s := &Server{cfgWatcher: watcher, kvStore: nil}

	r := chi.NewRouter()
	r.Post("/api/config/import", s.handleConfigImport)

	body := `{"overrides":{"resolver.qps":"999"}}`
	req := httptest.NewRequest(http.MethodPost, "/api/config/import", strings.NewReader(body))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleConfigImport_Success(t *testing.T) {
	cfg := config.Defaults()
	watcher := config.NewWatcher(cfg, 1, nil)
	kv := &mockKVStore{data: make(map[string]string)}
	s := &Server{cfgWatcher: watcher, kvStore: kv}

	r := chi.NewRouter()
	r.Post("/api/config/import", s.handleConfigImport)

	body := `{"overrides":{"resolver.qps":"999"}}`
	req := httptest.NewRequest(http.MethodPost, "/api/config/import", strings.NewReader(body))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	if kv.data["resolver.qps"] != "999" {
		t.Errorf("expected resolver.qps=999 in kvStore, got %s", kv.data["resolver.qps"])
	}
}

// errorKVStore is a mock KVStore that returns an error from GetAll.
type errorKVStore struct {
	mockKVStore
}

func (m *errorKVStore) GetAll(_ context.Context) (map[string]string, error) {
	return nil, errors.New("getall error")
}

func TestHandleConfigImport_ReloadError(t *testing.T) {
	cfg := config.Defaults()
	watcher := config.NewWatcher(cfg, 1, nil)
	kv := &errorKVStore{mockKVStore{data: make(map[string]string)}}
	s := &Server{cfgWatcher: watcher, kvStore: kv}

	r := chi.NewRouter()
	r.Post("/api/config/import", s.handleConfigImport)

	body := `{"overrides":{"resolver.qps":"999"}}`
	req := httptest.NewRequest(http.MethodPost, "/api/config/import", strings.NewReader(body))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleSettingsPut_ReloadError(t *testing.T) {
	cfg := config.Defaults()
	watcher := config.NewWatcher(cfg, 1, nil)
	kv := &errorKVStore{mockKVStore{data: make(map[string]string)}}
	s := &Server{cfgWatcher: watcher, kvStore: kv}

	r := chi.NewRouter()
	r.Put("/api/settings", s.handleSettingsPut)

	body := `{"resolver.qps": "100"}`
	req := httptest.NewRequest(http.MethodPut, "/api/settings", strings.NewReader(body))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleRoutingRollback_RouterError(t *testing.T) {
	cfg := config.Defaults()
	watcher := config.NewWatcher(cfg, 1, nil)
	s := &Server{cfgWatcher: watcher, router: &mockRouter{rollback: func(_ context.Context) error { return errors.New("boom") }}}

	r := chi.NewRouter()
	r.Post("/routing/rollback", s.handleRoutingRollback)

	req := httptest.NewRequest(http.MethodPost, "/routing/rollback", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleRoutingDryRun_V4Error(t *testing.T) {
	cfg := config.Defaults()
	watcher := config.NewWatcher(cfg, 1, nil)
	s := &Server{cfgWatcher: watcher, router: &mockRouter{dryRunErr: errors.New("boom")}}

	r := chi.NewRouter()
	r.Post("/routing/dry-run", s.handleRoutingDryRun)

	body := `{"ipv4_prefixes":["10.0.0.0/8"],"ipv6_prefixes":[]}`
	req := httptest.NewRequest(http.MethodPost, "/routing/dry-run", strings.NewReader(body))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleRoutingDryRun_V6Disabled(t *testing.T) {
	cfg := config.Defaults()
	watcher := config.NewWatcher(cfg, 1, nil)
	s := &Server{cfgWatcher: watcher, router: &mockRouter{dryRunErr: routing.ErrDisabled}}

	r := chi.NewRouter()
	r.Post("/routing/dry-run", s.handleRoutingDryRun)

	body := `{"ipv4_prefixes":[],"ipv6_prefixes":["2001:db8::/32"]}`
	req := httptest.NewRequest(http.MethodPost, "/routing/dry-run", strings.NewReader(body))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d: %s", rec.Code, rec.Body.String())
	}
}
