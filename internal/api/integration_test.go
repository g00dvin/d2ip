package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/goodvin/d2ip/internal/cache"
	"github.com/goodvin/d2ip/internal/config"
	"github.com/goodvin/d2ip/internal/events"
	"github.com/goodvin/d2ip/internal/orchestrator"
	"github.com/goodvin/d2ip/internal/routing"
	"github.com/goodvin/d2ip/internal/sourcereg"
)

type mockRouter struct {
	snapshot  routing.RouterState
	dryRunErr error
	rollback  func(ctx context.Context) error
}

func (m *mockRouter) Caps() error { return nil }
func (m *mockRouter) Plan(_ context.Context, desired []netip.Prefix, f routing.Family) (routing.Plan, error) {
	return routing.Plan{Family: f, Add: desired}, nil
}
func (m *mockRouter) Apply(_ context.Context, _ routing.Plan) error { return nil }
func (m *mockRouter) Snapshot() routing.RouterState                 { return m.snapshot }
func (m *mockRouter) Rollback(ctx context.Context) error {
	if m.rollback != nil {
		return m.rollback(ctx)
	}
	return nil
}
func (m *mockRouter) DryRun(_ context.Context, desired []netip.Prefix, f routing.Family) (routing.Plan, string, error) {
	if m.dryRunErr != nil {
		return routing.Plan{}, "", m.dryRunErr
	}
	return routing.Plan{Family: f, Add: desired}, "mock diff output", nil
}

type mockRegistry struct {
	sources    []sourcereg.SourceInfo
	categories []sourcereg.CategoryInfo
	domains    map[string][]string
}

func (m *mockRegistry) AddSource(ctx context.Context, cfg sourcereg.SourceConfig) error { return nil }
func (m *mockRegistry) RemoveSource(ctx context.Context, id string) error               { return nil }
func (m *mockRegistry) LoadAll(ctx context.Context) error                               { return nil }
func (m *mockRegistry) Close() error                                                    { return nil }
func (m *mockRegistry) ListSources() []sourcereg.SourceInfo                             { return m.sources }
func (m *mockRegistry) GetSource(id string) (sourcereg.Source, bool)                    { return nil, false }
func (m *mockRegistry) ListCategories() []sourcereg.CategoryInfo                        { return m.categories }
func (m *mockRegistry) GetDomains(category string) ([]string, error) {
	if d, ok := m.domains[category]; ok {
		return d, nil
	}
	return nil, fmt.Errorf("category not found: %s", category)
}
func (m *mockRegistry) GetPrefixes(category string) ([]netip.Prefix, error) { return nil, nil }
func (m *mockRegistry) ResolveCategory(category string) (string, string, bool) {
	for _, c := range m.categories {
		if c.Name == category {
			return "mock", string(c.Type), true
		}
	}
	return "", "", false
}

func newTestServer(t *testing.T, opts ...func(*Server)) *Server {
	t.Helper()
	cfg := config.Defaults()
	watcher := config.NewWatcher(cfg, 1, nil)
	kv := &mockKVStore{data: make(map[string]string)}

	s := &Server{
		cfgWatcher: watcher,
		kvStore:    kv,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

func withCache(c cache.Cache) func(*Server) {
	return func(s *Server) { s.cacheAgent = c }
}

func withRouter(r routing.Router) func(*Server) {
	return func(s *Server) { s.router = r }
}

func withRegistry(r sourcereg.Registry) func(*Server) {
	return func(s *Server) { s.registry = r }
}

func TestHealthz_ReturnsOK(t *testing.T) {
	s := newTestServer(t)
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/healthz")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var body map[string]string
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(t, "ok", body["status"])
}

func TestReadyz_ReturnsReady(t *testing.T) {
	s := newTestServer(t)
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/readyz")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var body map[string]string
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(t, "ready", body["status"])
}

func TestPipelineStatus_NilOrchestrator_ReturnsUnavailable(t *testing.T) {
	s := newTestServer(t)
	r := s.Handler()
	req := httptest.NewRequest(http.MethodGet, "/pipeline/status", nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
}

func TestPipelineRun_NilOrchestrator_ReturnsUnavailable(t *testing.T) {
	s := newTestServer(t)
	r := s.Handler()
	req := httptest.NewRequest(http.MethodPost, "/pipeline/run", nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
}

func TestPipelineCancel_NilOrchestrator_ReturnsUnavailable(t *testing.T) {
	s := newTestServer(t)
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/pipeline/cancel", "application/json", nil)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
}

func TestPipelineHistory_NilOrchestrator_ReturnsEmptyList(t *testing.T) {
	s := newTestServer(t)
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/pipeline/history")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var body map[string]interface{}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	history, ok := body["history"].([]interface{})
	require.True(t, ok)
	assert.Empty(t, history)
}

func TestSettingsGet_WithOverrides(t *testing.T) {
	kv := &mockKVStore{data: map[string]string{"cache.ttl": "12h0m0s"}}
	cfg := config.Defaults()
	watcher := config.NewWatcher(cfg, 1, nil)

	s := &Server{cfgWatcher: watcher, kvStore: kv}
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/settings")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var body map[string]interface{}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Contains(t, body, "config")
	assert.Contains(t, body, "defaults")
	assert.Contains(t, body, "overrides")

	overrides := body["overrides"].(map[string]interface{})
	assert.Equal(t, "12h0m0s", overrides["cache.ttl"])
}

func TestSettingsPut_AndDelete(t *testing.T) {
	cfg := config.Defaults()
	watcher := config.NewWatcher(cfg, 1, nil)
	kv := &mockKVStore{data: make(map[string]string)}

	s := &Server{cfgWatcher: watcher, kvStore: kv}
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()

	putBody := `{"resolver.qps": "500"}`
	putReq, err := http.NewRequest(http.MethodPut, srv.URL+"/api/settings", strings.NewReader(putBody))
	require.NoError(t, err)
	putReq.Header.Set("Content-Type", "application/json")
	putResp, err := http.DefaultClient.Do(putReq)
	require.NoError(t, err)
	defer putResp.Body.Close()
	assert.Equal(t, http.StatusOK, putResp.StatusCode)

	assert.Equal(t, "500", kv.data["resolver.qps"])

	delReq, _ := http.NewRequest(http.MethodDelete, srv.URL+"/api/settings/resolver.qps", nil)
	delResp, err := http.DefaultClient.Do(delReq)
	require.NoError(t, err)
	defer delResp.Body.Close()
	assert.Equal(t, http.StatusOK, delResp.StatusCode)

	_, exists := kv.data["resolver.qps"]
	assert.False(t, exists, "override should be removed after DELETE")
}

func TestSettingsPut_NilKVStore_Returns500(t *testing.T) {
	cfg := config.Defaults()
	watcher := config.NewWatcher(cfg, 1, nil)
	s := &Server{cfgWatcher: watcher, kvStore: nil}
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()

	putReq, err := http.NewRequest(http.MethodPut, srv.URL+"/api/settings", strings.NewReader(`{"key":"val"}`))
	require.NoError(t, err)
	putReq.Header.Set("Content-Type", "application/json")
	putResp, err := http.DefaultClient.Do(putReq)
	require.NoError(t, err)
	defer putResp.Body.Close()
	assert.Equal(t, http.StatusInternalServerError, putResp.StatusCode)
}

func TestCacheStats_WithRealCache(t *testing.T) {
	c, err := cache.Open(context.Background(), ":memory:")
	require.NoError(t, err)
	defer c.Close()

	ctx := context.Background()
	now := time.Now()
	ip4, _ := netip.ParseAddr("10.0.0.1")
	require.NoError(t, c.UpsertBatch(ctx, []cache.ResolveResult{
		{Domain: "test.example.com", IPv4: []netip.Addr{ip4}, Status: cache.StatusValid, ResolvedAt: now},
	}))

	s := newTestServer(t, withCache(c))
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/cache/stats")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var body map[string]interface{}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(t, float64(1), body["domains"])
	assert.Equal(t, float64(1), body["records_total"])
	assert.Equal(t, float64(1), body["records_v4"])
}

func TestCacheVacuum_WithRealCache(t *testing.T) {
	c, err := cache.Open(context.Background(), ":memory:")
	require.NoError(t, err)
	defer c.Close()

	ctx := context.Background()
	oldTime := time.Now().Add(-48 * time.Hour)
	ip4, _ := netip.ParseAddr("10.0.0.1")
	require.NoError(t, c.UpsertBatch(ctx, []cache.ResolveResult{
		{Domain: "old.example.com", IPv4: []netip.Addr{ip4}, Status: cache.StatusValid, ResolvedAt: oldTime},
	}))

	s := newTestServer(t, withCache(c))
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/api/cache/vacuum", "application/json", nil)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var body map[string]interface{}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Contains(t, body, "deleted")
	assert.Contains(t, body, "status")
}

func TestCacheStats_NilCache_Returns503(t *testing.T) {
	s := newTestServer(t)
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/cache/stats")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
}

func TestSourcesList_NilRegistry_ReturnsEmpty(t *testing.T) {
	s := newTestServer(t)
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/sources")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var body map[string]interface{}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	sources, ok := body["sources"].([]interface{})
	require.True(t, ok)
	assert.Empty(t, sources)
}

func TestSourcesList_WithMockRegistry(t *testing.T) {
	reg := &mockRegistry{
		sources: []sourcereg.SourceInfo{
			{ID: "src1", Provider: "plaintext", Prefix: "corp"},
		},
	}
	s := newTestServer(t, withRegistry(reg))
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/sources")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var body map[string]interface{}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	sources, ok := body["sources"].([]interface{})
	require.True(t, ok)
	assert.Len(t, sources, 1)
}

func TestRoutingSnapshot_WithMockRouter(t *testing.T) {
	appliedAt := time.Now().Truncate(time.Second)
	snap := routing.RouterState{
		Backend:   "mock",
		AppliedAt: appliedAt,
		V4:        []netip.Prefix{netip.PrefixFrom(netip.MustParseAddr("10.0.0.0"), 24)},
		V6:        nil,
	}
	s := newTestServer(t, withRouter(&mockRouter{snapshot: snap}))
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/routing/snapshot")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var body map[string]interface{}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Contains(t, body, "backend")
	assert.Contains(t, body, "applied_at")
	assert.Contains(t, body, "v4")
	assert.Contains(t, body, "v6")
}

func TestRoutingDryRun_WithMockRouter(t *testing.T) {
	s := newTestServer(t, withRouter(&mockRouter{}))
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()

	body := `{"ipv4_prefixes":["10.0.0.0/8"],"ipv6_prefixes":[]}`
	resp, err := http.Post(srv.URL+"/routing/dry-run", "application/json", strings.NewReader(body))
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result map[string]interface{}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
	assert.Contains(t, result, "v4_plan")
	assert.Contains(t, result, "v6_plan")
}

func TestRoutingDryRun_EmptyPrefixes_ReturnsEmpty(t *testing.T) {
	s := newTestServer(t)
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()

	body := `{"ipv4_prefixes":[],"ipv6_prefixes":[]}`
	resp, err := http.Post(srv.URL+"/routing/dry-run", "application/json", strings.NewReader(body))
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result map[string]interface{}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
	assert.Contains(t, result, "v4_plan")
	assert.Contains(t, result, "message")
}

func TestRoutingDryRun_InvalidJSON_Returns400(t *testing.T) {
	s := newTestServer(t, withRouter(&mockRouter{}))
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/routing/dry-run", "application/json", strings.NewReader("invalid"))
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestRoutingDryRun_DisabledRouter_Returns503(t *testing.T) {
	s := newTestServer(t, withRouter(&mockRouter{dryRunErr: routing.ErrDisabled}))
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()

	body := `{"ipv4_prefixes":["10.0.0.0/8"],"ipv6_prefixes":[]}`
	resp, err := http.Post(srv.URL+"/routing/dry-run", "application/json", strings.NewReader(body))
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
}

func TestRoutingRollback_WithMockRouter(t *testing.T) {
	s := newTestServer(t, withRouter(&mockRouter{}))
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/routing/rollback", "application/json", nil)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var body map[string]string
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(t, "ok", body["status"])
}

func TestRoutingRollback_DisabledRouter_Returns503(t *testing.T) {
	s := newTestServer(t, withRouter(&mockRouter{
		rollback: func(_ context.Context) error { return routing.ErrDisabled },
	}))
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/routing/rollback", "application/json", nil)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
}

func TestCategoriesList_WithRegistry(t *testing.T) {
	reg := &mockRegistry{
		categories: []sourcereg.CategoryInfo{
			{Name: "geosite:ru", Type: sourcereg.CategoryDomain},
			{Name: "geosite:google", Type: sourcereg.CategoryDomain},
		},
	}
	s := &Server{registry: reg}
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/categories")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var body map[string]interface{}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Contains(t, body, "available")

	available, ok := body["available"].([]interface{})
	require.True(t, ok)
	assert.Len(t, available, 2, "should have available categories")
}

func TestCategoriesAdd_Returns405(t *testing.T) {
	cfg := config.Defaults()
	watcher := config.NewWatcher(cfg, 1, nil)
	kv := &mockKVStore{data: make(map[string]string)}

	s := &Server{cfgWatcher: watcher, kvStore: kv, dlProvider: nil}
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()

	body := `{"code": "google"}`
	resp, err := http.Post(srv.URL+"/api/categories", "application/json", strings.NewReader(body))
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusMethodNotAllowed, resp.StatusCode)
}

func TestCategoriesDelete_Returns405(t *testing.T) {
	cfg := config.Defaults()
	watcher := config.NewWatcher(cfg, 1, nil)
	kv := &mockKVStore{data: make(map[string]string)}

	s := &Server{cfgWatcher: watcher, kvStore: kv}
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()

	req, _ := http.NewRequest(http.MethodDelete, srv.URL+"/api/categories/geosite:ru", nil)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusMethodNotAllowed, resp.StatusCode)
}

func TestCategoryDomains_WithRegistry(t *testing.T) {
	reg := &mockRegistry{
		categories: []sourcereg.CategoryInfo{
			{Name: "geosite:ru", Type: sourcereg.CategoryDomain},
		},
		domains: map[string][]string{
			"geosite:ru": {"example.com", "test.ru"},
		},
	}
	s := &Server{registry: reg}
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/categories/geosite:ru/domains")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var body map[string]interface{}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(t, "geosite:ru", body["code"])
	assert.Contains(t, body, "domains")
	assert.Contains(t, body, "total")
}

func TestCategoryDomains_NilRegistry_Returns503(t *testing.T) {
	s := &Server{registry: nil}
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/categories/geosite:ru/domains")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
}

func TestCacheEntries_RequiresDomainParam(t *testing.T) {
	c, err := cache.Open(context.Background(), ":memory:")
	require.NoError(t, err)
	defer c.Close()

	s := newTestServer(t, withCache(c))
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/cache/entries")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestCacheEntries_WithDomainParam(t *testing.T) {
	c, err := cache.Open(context.Background(), ":memory:")
	require.NoError(t, err)
	defer c.Close()

	ctx := context.Background()
	now := time.Now()
	ip4, _ := netip.ParseAddr("10.0.0.1")
	require.NoError(t, c.UpsertBatch(ctx, []cache.ResolveResult{
		{Domain: "test.example.com", IPv4: []netip.Addr{ip4}, Status: cache.StatusValid, ResolvedAt: now},
	}))

	s := newTestServer(t, withCache(c))
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/cache/entries?domain=test.example.com")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)

	var body map[string]interface{}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Contains(t, body, "error")
}

func TestCachePurge_ReturnsPlaceholderResponse(t *testing.T) {
	c, err := cache.Open(context.Background(), ":memory:")
	require.NoError(t, err)
	defer c.Close()

	s := newTestServer(t, withCache(c))
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/api/cache/purge", "application/json", strings.NewReader(`{}`))
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var body map[string]interface{}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(t, "ok", body["status"])
}

func TestCacheVacuum_NilCache_Returns503(t *testing.T) {
	s := newTestServer(t)
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/api/cache/vacuum", "application/json", nil)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
}

func setupTestServer(t *testing.T) (*httptest.Server, func()) {
	t.Helper()
	s := newTestServer(t)
	srv := httptest.NewServer(s.Handler())
	return srv, srv.Close
}

func TestPoliciesAPI(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	// Test GET /api/policies (empty list)
	resp, err := http.Get(srv.URL + "/api/policies")
	if err != nil {
		t.Fatalf("get policies: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var listResp struct {
		Policies []interface{} `json:"policies"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		t.Fatalf("decode policies list: %v", err)
	}
	resp.Body.Close()

	// Test GET /api/policies/{name} (not found)
	resp, err = http.Get(srv.URL + "/api/policies/nonexistent")
	if err != nil {
		t.Fatalf("get policy: %v", err)
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

// fakeRegistry is a functional in-memory registry for integration tests.
type fakeRegistry struct {
	mu         sync.Mutex
	sources    map[string]*fakeSource
	categories []sourcereg.CategoryInfo
	domains    map[string][]string
}

type fakeSource struct {
	info sourcereg.SourceInfo
}

func newFakeRegistry() *fakeRegistry {
	return &fakeRegistry{
		sources: make(map[string]*fakeSource),
		domains: make(map[string][]string),
	}
}

func (f *fakeSource) ID() string                             { return f.info.ID }
func (f *fakeSource) Prefix() string                         { return f.info.Prefix }
func (f *fakeSource) Provider() sourcereg.SourceType         { return sourcereg.SourceType(f.info.Provider) }
func (f *fakeSource) Load(ctx context.Context) error         { return nil }
func (f *fakeSource) Close() error                           { return nil }
func (f *fakeSource) Categories() []string                   { return f.info.Categories }
func (f *fakeSource) Info() sourcereg.SourceInfo             { return f.info }
func (f *fakeSource) IsDomainSource() bool                   { return true }
func (f *fakeSource) IsPrefixSource() bool                   { return false }
func (f *fakeSource) AsDomainSource() sourcereg.DomainSource { return nil }
func (f *fakeSource) AsPrefixSource() sourcereg.PrefixSource { return nil }

func (r *fakeRegistry) AddSource(ctx context.Context, cfg sourcereg.SourceConfig) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sources[cfg.ID] = &fakeSource{info: sourcereg.SourceInfo{
		ID:       cfg.ID,
		Provider: string(cfg.Provider),
		Prefix:   cfg.Prefix,
		Enabled:  cfg.Enabled,
	}}
	return nil
}

func (r *fakeRegistry) RemoveSource(ctx context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.sources, id)
	return nil
}

func (r *fakeRegistry) LoadAll(ctx context.Context) error { return nil }
func (r *fakeRegistry) Close() error                      { return nil }

func (r *fakeRegistry) ListSources() []sourcereg.SourceInfo {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []sourcereg.SourceInfo
	for _, s := range r.sources {
		out = append(out, s.Info())
	}
	return out
}

func (r *fakeRegistry) GetSource(id string) (sourcereg.Source, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.sources[id]
	if !ok {
		return nil, false
	}
	return s, true
}

func (r *fakeRegistry) ListCategories() []sourcereg.CategoryInfo { return r.categories }

func (r *fakeRegistry) GetDomains(category string) ([]string, error) {
	if d, ok := r.domains[category]; ok {
		return d, nil
	}
	return nil, fmt.Errorf("category not found: %s", category)
}

func (r *fakeRegistry) GetPrefixes(category string) ([]netip.Prefix, error) { return nil, nil }

func (r *fakeRegistry) ResolveCategory(category string) (string, string, bool) {
	for _, c := range r.categories {
		if c.Name == category {
			return "fake", string(c.Type), true
		}
	}
	return "", "", false
}

func TestIntegration_SourcesCRUD(t *testing.T) {
	reg := newFakeRegistry()
	s := newTestServer(t, withRegistry(reg))
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()

	// Create
	body := `{"id":"src1","provider":"plaintext","prefix":"corp"}`
	resp, err := http.Post(srv.URL+"/api/sources", "application/json", strings.NewReader(body))
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	// List
	resp, err = http.Get(srv.URL + "/api/sources")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var listBody map[string]interface{}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&listBody))
	resp.Body.Close()
	sources, ok := listBody["sources"].([]interface{})
	require.True(t, ok)
	require.Len(t, sources, 1)

	// Get
	resp, err = http.Get(srv.URL + "/api/sources/src1")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var getBody map[string]interface{}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&getBody))
	resp.Body.Close()
	assert.Equal(t, "src1", getBody["id"])

	// Update
	upBody := `{"id":"src1","provider":"plaintext","prefix":"newcorp"}`
	req, err := http.NewRequest(http.MethodPut, srv.URL+"/api/sources/src1", strings.NewReader(upBody))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	resp, err = http.DefaultClient.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	// Verify update
	resp, err = http.Get(srv.URL + "/api/sources/src1")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&getBody))
	resp.Body.Close()
	assert.Equal(t, "newcorp", getBody["prefix"])

	// Refresh
	resp, err = http.Post(srv.URL+"/api/sources/src1/refresh", "application/json", nil)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var refreshBody map[string]interface{}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&refreshBody))
	resp.Body.Close()
	assert.Equal(t, "ok", refreshBody["status"])

	// Delete
	req, err = http.NewRequest(http.MethodDelete, srv.URL+"/api/sources/src1", nil)
	require.NoError(t, err)
	resp, err = http.DefaultClient.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	// Verify delete
	resp, err = http.Get(srv.URL + "/api/sources/src1")
	require.NoError(t, err)
	require.Equal(t, http.StatusNotFound, resp.StatusCode)
	resp.Body.Close()
}

func TestIntegration_PoliciesCRUD(t *testing.T) {
	s := newTestServer(t)
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()

	// Create
	body := `{"name":"p1","enabled":true,"backend":"iproute2","categories":["geosite:ru"],"iface":"eth0","table_id":100}`
	resp, err := http.Post(srv.URL+"/api/policies", "application/json", strings.NewReader(body))
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	// List
	resp, err = http.Get(srv.URL + "/api/policies")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var listBody map[string]interface{}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&listBody))
	resp.Body.Close()
	policies, ok := listBody["policies"].([]interface{})
	require.True(t, ok)
	require.Len(t, policies, 1)

	// Get
	resp, err = http.Get(srv.URL + "/api/policies/p1")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var getBody map[string]interface{}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&getBody))
	resp.Body.Close()
	assert.Equal(t, "p1", getBody["name"])

	// Update
	upBody := `{"name":"p1","enabled":true,"backend":"iproute2","categories":["geosite:ru","geosite:us"],"iface":"eth0","table_id":100}`
	req, err := http.NewRequest(http.MethodPut, srv.URL+"/api/policies/p1", strings.NewReader(upBody))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	resp, err = http.DefaultClient.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	// Verify update
	resp, err = http.Get(srv.URL + "/api/policies/p1")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&getBody))
	resp.Body.Close()
	cats, ok := getBody["categories"].([]interface{})
	require.True(t, ok)
	assert.Len(t, cats, 2)

	// Duplicate create returns 409
	resp, err = http.Post(srv.URL+"/api/policies", "application/json", strings.NewReader(body))
	require.NoError(t, err)
	require.Equal(t, http.StatusConflict, resp.StatusCode)
	resp.Body.Close()

	// Delete
	req, err = http.NewRequest(http.MethodDelete, srv.URL+"/api/policies/p1", nil)
	require.NoError(t, err)
	resp, err = http.DefaultClient.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	// Verify delete
	resp, err = http.Get(srv.URL + "/api/policies/p1")
	require.NoError(t, err)
	require.Equal(t, http.StatusNotFound, resp.StatusCode)
	resp.Body.Close()
}

func TestIntegration_CategoriesAndDomains(t *testing.T) {
	reg := newFakeRegistry()
	reg.categories = []sourcereg.CategoryInfo{
		{Name: "geosite:ru", Type: sourcereg.CategoryDomain},
	}
	reg.domains = map[string][]string{
		"geosite:ru": {"example.com", "test.ru"},
	}
	s := newTestServer(t, withRegistry(reg))
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()

	// List categories
	resp, err := http.Get(srv.URL + "/api/categories")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var body map[string]interface{}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	resp.Body.Close()
	available, ok := body["available"].([]interface{})
	require.True(t, ok)
	assert.Len(t, available, 1)

	// Get domains
	resp, err = http.Get(srv.URL + "/api/categories/geosite:ru/domains")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	resp.Body.Close()
	assert.Equal(t, "geosite:ru", body["code"])
	domains, ok := body["domains"].([]interface{})
	require.True(t, ok)
	assert.Len(t, domains, 2)
}

func TestIntegration_CacheOps(t *testing.T) {
	c, err := cache.Open(context.Background(), ":memory:")
	require.NoError(t, err)
	defer c.Close()

	ctx := context.Background()
	now := time.Now()
	ip4, _ := netip.ParseAddr("10.0.0.1")
	require.NoError(t, c.UpsertBatch(ctx, []cache.ResolveResult{
		{Domain: "test.example.com", IPv4: []netip.Addr{ip4}, Status: cache.StatusValid, ResolvedAt: now},
	}))

	s := newTestServer(t, withCache(c))
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()

	// Stats
	resp, err := http.Get(srv.URL + "/api/cache/stats")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var stats map[string]interface{}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&stats))
	resp.Body.Close()
	assert.Equal(t, float64(1), stats["domains"])

	// Entries (requires domain param)
	resp, err = http.Get(srv.URL + "/api/cache/entries?domain=test.example.com")
	require.NoError(t, err)
	require.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
	resp.Body.Close()

	// Purge
	resp, err = http.Post(srv.URL+"/api/cache/purge", "application/json", strings.NewReader(`{}`))
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var purgeBody map[string]interface{}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&purgeBody))
	resp.Body.Close()
	assert.Equal(t, "ok", purgeBody["status"])

	// Vacuum
	resp, err = http.Post(srv.URL+"/api/cache/vacuum", "application/json", nil)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var vacBody map[string]interface{}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&vacBody))
	resp.Body.Close()
	assert.Contains(t, vacBody, "deleted")
}

func TestIntegration_SettingsCRUD(t *testing.T) {
	s := newTestServer(t)
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()

	// Get
	resp, err := http.Get(srv.URL + "/api/settings")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var body map[string]interface{}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	resp.Body.Close()
	assert.Contains(t, body, "config")
	assert.Contains(t, body, "defaults")
	assert.Contains(t, body, "overrides")

	// Put
	putBody := `{"resolver.qps": "500"}`
	req, err := http.NewRequest(http.MethodPut, srv.URL+"/api/settings", strings.NewReader(putBody))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	resp, err = http.DefaultClient.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	// Get again
	resp, err = http.Get(srv.URL + "/api/settings")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	resp.Body.Close()
	overrides := body["overrides"].(map[string]interface{})
	assert.Equal(t, "500", overrides["resolver.qps"])

	// Delete
	req, err = http.NewRequest(http.MethodDelete, srv.URL+"/api/settings/resolver.qps", nil)
	require.NoError(t, err)
	resp, err = http.DefaultClient.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	// Verify delete
	resp, err = http.Get(srv.URL + "/api/settings")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	resp.Body.Close()
	overrides = body["overrides"].(map[string]interface{})
	_, exists := overrides["resolver.qps"]
	assert.False(t, exists)
}

func TestIntegration_PipelineOps(t *testing.T) {
	orch := &mockOrchestrator{
		runReport: orchestrator.PipelineReport{RunID: 42},
		status:    orchestrator.RunStatus{Running: false},
	}
	s := newTestServer(t)
	s.orch = orch
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()

	// Run
	resp, err := http.Post(srv.URL+"/pipeline/run", "application/json", strings.NewReader(`{}`))
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var runBody map[string]interface{}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&runBody))
	resp.Body.Close()
	assert.Equal(t, float64(42), runBody["run_id"])

	// Status
	resp, err = http.Get(srv.URL + "/pipeline/status")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var statusBody map[string]interface{}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&statusBody))
	resp.Body.Close()
	assert.Equal(t, false, statusBody["running"])

	// History
	resp, err = http.Get(srv.URL + "/api/pipeline/history")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var histBody map[string]interface{}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&histBody))
	resp.Body.Close()
	if history, ok := histBody["history"].([]interface{}); ok {
		assert.Empty(t, history)
	} else {
		assert.Nil(t, histBody["history"])
	}

	// Cancel
	resp, err = http.Post(srv.URL+"/pipeline/cancel", "application/json", nil)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var cancelBody map[string]string
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&cancelBody))
	resp.Body.Close()
	assert.Equal(t, "cancelled", cancelBody["status"])
}

func TestIntegration_RoutingOps(t *testing.T) {
	appliedAt := time.Now().Truncate(time.Second)
	snap := routing.RouterState{
		Backend:   "mock",
		AppliedAt: appliedAt,
		V4:        []netip.Prefix{netip.PrefixFrom(netip.MustParseAddr("10.0.0.0"), 24)},
		V6:        nil,
	}
	s := newTestServer(t, withRouter(&mockRouter{snapshot: snap}))
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()

	// Snapshot
	resp, err := http.Get(srv.URL + "/routing/snapshot")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var body map[string]interface{}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	resp.Body.Close()
	assert.Contains(t, body, "backend")
	assert.Contains(t, body, "v4")

	// Dry-run
	drBody := `{"ipv4_prefixes":["10.0.0.0/8"],"ipv6_prefixes":[]}`
	resp, err = http.Post(srv.URL+"/routing/dry-run", "application/json", strings.NewReader(drBody))
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var drResp map[string]interface{}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&drResp))
	resp.Body.Close()
	assert.Contains(t, drResp, "v4_plan")
	assert.Contains(t, drResp, "v6_plan")

	// Rollback
	resp, err = http.Post(srv.URL+"/routing/rollback", "application/json", nil)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var rbBody map[string]string
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&rbBody))
	resp.Body.Close()
	assert.Equal(t, "ok", rbBody["status"])
}

func TestIntegration_ExportAndConfig(t *testing.T) {
	cfg := config.Defaults()
	cfg.Export.Dir = t.TempDir()
	watcher := config.NewWatcher(cfg, 1, nil)
	kv := &mockKVStore{data: make(map[string]string)}
	s := &Server{cfgWatcher: watcher, kvStore: kv}
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()

	// Create export file
	policyDir := filepath.Join(cfg.Export.Dir, "test")
	require.NoError(t, os.MkdirAll(policyDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(policyDir, "ipv4.txt"), []byte("10.0.0.0/8\n"), 0644))

	// Download export
	resp, err := http.Get(srv.URL + "/api/export/download?policy=test&type=ipv4")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	data, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	resp.Body.Close()
	assert.Contains(t, string(data), "10.0.0.0/8")

	// Config export
	resp, err = http.Get(srv.URL + "/api/config/export")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "attachment; filename=d2ip-config.json", resp.Header.Get("Content-Disposition"))
	resp.Body.Close()

	// Config import
	importBody := `{"overrides":{"resolver.qps":"999"}}`
	resp, err = http.Post(srv.URL+"/api/config/import", "application/json", strings.NewReader(importBody))
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var impBody map[string]string
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&impBody))
	resp.Body.Close()
	assert.Equal(t, "ok", impBody["status"])
	assert.Equal(t, "999", kv.data["resolver.qps"])
}

func TestIntegration_EventsSSE(t *testing.T) {
	bus := events.NewBus()
	defer bus.Close()

	s := newTestServer(t)
	s.eventBus = bus
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/events")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "text/event-stream", resp.Header.Get("Content-Type"))
}
