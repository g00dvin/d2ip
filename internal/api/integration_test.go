package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/goodvin/d2ip/internal/cache"
	"github.com/goodvin/d2ip/internal/config"
	"github.com/goodvin/d2ip/internal/domainlist"
	"github.com/goodvin/d2ip/internal/routing"
	"github.com/goodvin/d2ip/internal/source"
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
func (m *mockRouter) Snapshot() routing.RouterState                    { return m.snapshot }
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

type mockDLCStore struct{}

func (m *mockDLCStore) Get(_ context.Context, _ time.Duration) (string, source.Version, error) {
	return "", source.Version{}, nil
}
func (m *mockDLCStore) ForceRefresh(_ context.Context) (string, source.Version, error) {
	return "", source.Version{}, nil
}
func (m *mockDLCStore) Info() source.Version {
	return source.Version{FetchedAt: time.Now(), Size: 42}
}

type mockListProvider struct {
	rules      []domainlist.Rule
	categories []string
}

func (m *mockListProvider) Load(_ string) error { return nil }
func (m *mockListProvider) Select(sel []domainlist.CategorySelector) ([]domainlist.Rule, error) {
	return m.rules, nil
}
func (m *mockListProvider) Categories() []string { return m.categories }

func newTestServer(t *testing.T, opts ...func(*Server)) *Server {
	t.Helper()
	cfg := config.Defaults()
	watcher := config.NewWatcher(cfg, 1)
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

func withSource(st source.DLCStore) func(*Server) {
	return func(s *Server) { s.sourceStore = st }
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
	watcher := config.NewWatcher(cfg, 1)

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
	watcher := config.NewWatcher(cfg, 1)
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
	watcher := config.NewWatcher(cfg, 1)
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

func TestSourceInfo_NilStore_ReturnsNotAvailable(t *testing.T) {
	s := newTestServer(t)
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/source/info")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var body map[string]interface{}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(t, false, body["available"])
}

func TestSourceInfo_WithMockStore(t *testing.T) {
	s := newTestServer(t, withSource(&mockDLCStore{}))
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/source/info")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var body map[string]interface{}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(t, true, body["available"])
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

func TestCategoriesList_WithProvider(t *testing.T) {
	cfg := config.Defaults()
	cfg.Categories = []config.CategoryConfig{{Code: "geosite:ru"}}
	watcher := config.NewWatcher(cfg, 1)

	provider := &mockListProvider{
		rules:      []domainlist.Rule{{Type: domainlist.RuleFull, Value: "example.ru"}},
		categories: []string{"ru", "google", "facebook"},
	}

	s := &Server{cfgWatcher: watcher, dlProvider: provider}
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/categories")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var body map[string]interface{}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Contains(t, body, "configured")
	assert.Contains(t, body, "available")

	configured, ok := body["configured"].([]interface{})
	require.True(t, ok)
	assert.NotEmpty(t, configured, "should have configured categories")

	available, ok := body["available"].([]interface{})
	require.True(t, ok)
	assert.NotEmpty(t, available, "should have available categories")
}

func TestCategoriesAdd_AddsWithPrefix(t *testing.T) {
	cfg := config.Defaults()
	watcher := config.NewWatcher(cfg, 1)
	kv := &mockKVStore{data: make(map[string]string)}

	s := &Server{cfgWatcher: watcher, kvStore: kv, dlProvider: nil}
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()

	body := `{"code": "google"}`
	resp, err := http.Post(srv.URL+"/api/categories", "application/json", strings.NewReader(body))
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	snap := watcher.Current()
	found := false
	for _, cat := range snap.Config.Categories {
		if cat.Code == "geosite:google" {
			found = true
		}
	}
	assert.True(t, found, "geosite:google should be in config categories after add")
}

func TestCategoriesAdd_Duplicate_ReturnsConflict(t *testing.T) {
	cfg := config.Defaults()
	cfg.Categories = []config.CategoryConfig{{Code: "geosite:google"}}
	watcher := config.NewWatcher(cfg, 1)
	kv := &mockKVStore{data: make(map[string]string)}

	s := &Server{cfgWatcher: watcher, kvStore: kv}
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()

	body := `{"code": "geosite:google"}`
	resp, err := http.Post(srv.URL+"/api/categories", "application/json", strings.NewReader(body))
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusConflict, resp.StatusCode)
}

func TestCategoriesAdd_EmptyCode_Returns400(t *testing.T) {
	cfg := config.Defaults()
	watcher := config.NewWatcher(cfg, 1)
	kv := &mockKVStore{data: make(map[string]string)}

	s := &Server{cfgWatcher: watcher, kvStore: kv}
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()

	body := `{"code": ""}`
	resp, err := http.Post(srv.URL+"/api/categories", "application/json", strings.NewReader(body))
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestCategoriesDelete_RemovesCategory(t *testing.T) {
	cfg := config.Defaults()
	cfg.Categories = []config.CategoryConfig{{Code: "geosite:ru"}}
	watcher := config.NewWatcher(cfg, 1)
	kv := &mockKVStore{data: make(map[string]string)}

	s := &Server{cfgWatcher: watcher, kvStore: kv}
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()

	req, _ := http.NewRequest(http.MethodDelete, srv.URL+"/api/categories/geosite:ru", nil)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	snap := watcher.Current()
	for _, cat := range snap.Config.Categories {
		assert.NotEqual(t, "geosite:ru", cat.Code, "geosite:ru should be removed")
	}
}

func TestCategoriesDelete_NotFound_Returns404(t *testing.T) {
	cfg := config.Defaults()
	watcher := config.NewWatcher(cfg, 1)
	kv := &mockKVStore{data: make(map[string]string)}

	s := &Server{cfgWatcher: watcher, kvStore: kv}
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()

	req, _ := http.NewRequest(http.MethodDelete, srv.URL+"/api/categories/geosite:nonexistent", nil)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestCategoryDomains_WithProvider(t *testing.T) {
	rules := []domainlist.Rule{
		{Type: domainlist.RuleFull, Value: "example.com"},
		{Type: domainlist.RuleFull, Value: "test.ru"},
	}
	provider := &mockListProvider{rules: rules, categories: []string{"ru"}}
	cfg := config.Defaults()
	watcher := config.NewWatcher(cfg, 1)

	s := &Server{cfgWatcher: watcher, dlProvider: provider}
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/categories/ru/domains")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var body map[string]interface{}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(t, "ru", body["code"])
	assert.Contains(t, body, "domains")
	assert.Contains(t, body, "total")
}

func TestCategoryDomains_NilProvider_Returns503(t *testing.T) {
	cfg := config.Defaults()
	watcher := config.NewWatcher(cfg, 1)
	s := &Server{cfgWatcher: watcher, dlProvider: nil}
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/categories/ru/domains")
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