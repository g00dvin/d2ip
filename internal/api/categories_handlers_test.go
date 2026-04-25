package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/goodvin/d2ip/internal/config"
	"github.com/goodvin/d2ip/internal/sourcereg"
)

// catTestRegistry is a mock implementation of sourcereg.Registry for testing.
type catTestRegistry struct {
	domains  map[string][]string
	prefixes map[string][]netip.Prefix
}

func (m *catTestRegistry) LoadAll(ctx context.Context) error            { return nil }
func (m *catTestRegistry) ListSources() []sourcereg.SourceInfo          { return nil }
func (m *catTestRegistry) GetSource(id string) (sourcereg.Source, bool) { return nil, false }
func (m *catTestRegistry) ListCategories() []sourcereg.CategoryInfo {
	return []sourcereg.CategoryInfo{{Name: "geosite:test", Type: sourcereg.CategoryDomain, Count: 2}}
}
func (m *catTestRegistry) GetDomains(category string) ([]string, error) {
	if d, ok := m.domains[category]; ok {
		return d, nil
	}
	return nil, fmt.Errorf("not found")
}
func (m *catTestRegistry) GetPrefixes(category string) ([]netip.Prefix, error) {
	if p, ok := m.prefixes[category]; ok {
		return p, nil
	}
	return nil, fmt.Errorf("not found")
}
func (m *catTestRegistry) ResolveCategory(category string) (string, string, bool) {
	return "test", "domain", true
}
func (m *catTestRegistry) AddSource(ctx context.Context, cfg sourcereg.SourceConfig) error {
	return nil
}
func (m *catTestRegistry) RemoveSource(ctx context.Context, id string) error { return nil }
func (m *catTestRegistry) Close() error                                      { return nil }

// withChiParam injects a chi route context with a URL parameter into a request.
func withChiParam(t *testing.T, req *http.Request, key, value string) *http.Request {
	t.Helper()
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, value)
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}

// setupCategoriesTestServer creates a Server and config.Watcher for category handler tests.
func setupCategoriesTestServer(t *testing.T) (*Server, *config.Watcher) {
	t.Helper()
	cfg := config.Defaults()
	watcher := config.NewWatcher(cfg, 1, nil)
	server := New(nil, nil, watcher, nil, nil, nil, nil, nil, nil)
	return server, watcher
}

func TestHandleCategoryDomains_ReturnsDomains(t *testing.T) {
	reg := &catTestRegistry{
		domains: map[string][]string{
			"geosite:test": {"example.com", "example.org"},
		},
	}
	server, _ := setupCategoriesTestServer(t)
	server.registry = reg

	req := httptest.NewRequest(http.MethodGet, "/api/categories/geosite:test/domains", nil)
	req = withChiParam(t, req, "code", "geosite:test")
	rec := httptest.NewRecorder()

	server.handleCategoryDomains(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	domains, ok := resp["domains"].([]interface{})
	if !ok {
		t.Fatalf("expected domains array, got %T", resp["domains"])
	}
	if len(domains) != 2 {
		t.Fatalf("expected 2 domains, got %d", len(domains))
	}
	if domains[0] != "example.com" {
		t.Errorf("expected example.com, got %v", domains[0])
	}
	if domains[1] != "example.org" {
		t.Errorf("expected example.org, got %v", domains[1])
	}
}

func TestHandleCategoryDomains_PrefixesFallback(t *testing.T) {
	reg := &catTestRegistry{
		prefixes: map[string][]netip.Prefix{
			"geosite:test": {netip.MustParsePrefix("192.0.2.0/24")},
		},
	}
	server, _ := setupCategoriesTestServer(t)
	server.registry = reg

	req := httptest.NewRequest(http.MethodGet, "/api/categories/geosite:test/domains", nil)
	req = withChiParam(t, req, "code", "geosite:test")
	rec := httptest.NewRecorder()

	server.handleCategoryDomains(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	domains, ok := resp["domains"].([]interface{})
	if !ok {
		t.Fatalf("expected domains array, got %T", resp["domains"])
	}
	if len(domains) != 1 {
		t.Fatalf("expected 1 domain, got %d", len(domains))
	}
	if domains[0] != "192.0.2.0/24" {
		t.Errorf("expected 192.0.2.0/24, got %v", domains[0])
	}
}

func TestHandleCategoryDomains_NotFound(t *testing.T) {
	reg := &catTestRegistry{}
	server, _ := setupCategoriesTestServer(t)
	server.registry = reg

	req := httptest.NewRequest(http.MethodGet, "/api/categories/geosite:test/domains", nil)
	req = withChiParam(t, req, "code", "geosite:test")
	rec := httptest.NewRecorder()

	server.handleCategoryDomains(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleCategoryDomains_Pagination(t *testing.T) {
	domains := make([]string, 150)
	for i := 0; i < 150; i++ {
		domains[i] = fmt.Sprintf("domain%d.example.com", i)
	}
	reg := &catTestRegistry{
		domains: map[string][]string{
			"geosite:test": domains,
		},
	}
	server, _ := setupCategoriesTestServer(t)
	server.registry = reg

	req := httptest.NewRequest(http.MethodGet, "/api/categories/geosite:test/domains?page=2&per_page=50", nil)
	req = withChiParam(t, req, "code", "geosite:test")
	rec := httptest.NewRecorder()

	server.handleCategoryDomains(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	respDomains, ok := resp["domains"].([]interface{})
	if !ok {
		t.Fatalf("expected domains array, got %T", resp["domains"])
	}
	if len(respDomains) != 50 {
		t.Fatalf("expected 50 domains, got %d", len(respDomains))
	}
	if respDomains[0] != "domain50.example.com" {
		t.Errorf("expected domain50.example.com, got %v", respDomains[0])
	}
	if respDomains[49] != "domain99.example.com" {
		t.Errorf("expected domain99.example.com, got %v", respDomains[49])
	}
}

func TestHandleCategoryDomains_MissingCode(t *testing.T) {
	server, _ := setupCategoriesTestServer(t)
	server.registry = &catTestRegistry{}

	req := httptest.NewRequest(http.MethodGet, "/api/categories//domains", nil)
	req = withChiParam(t, req, "code", "")
	rec := httptest.NewRecorder()

	server.handleCategoryDomains(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleCategoryDomains_NilRegistry(t *testing.T) {
	server, _ := setupCategoriesTestServer(t)
	server.registry = nil

	req := httptest.NewRequest(http.MethodGet, "/api/categories/geosite:test/domains", nil)
	req = withChiParam(t, req, "code", "geosite:test")
	rec := httptest.NewRecorder()

	server.handleCategoryDomains(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleCategoriesAdd_Success(t *testing.T) {
	server, watcher := setupCategoriesTestServer(t)

	body := `{"code":"test"}`
	req := httptest.NewRequest(http.MethodPost, "/api/categories", strings.NewReader(body))
	rec := httptest.NewRecorder()

	server.handleCategoriesAdd(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}
	if resp["status"] != "ok" {
		t.Errorf("expected status ok, got %v", resp["status"])
	}

	snapshot := watcher.Current()
	if len(snapshot.Config.Categories) != 1 {
		t.Fatalf("expected 1 category, got %d", len(snapshot.Config.Categories))
	}
	if snapshot.Config.Categories[0].Code != "geosite:test" {
		t.Errorf("expected geosite:test, got %s", snapshot.Config.Categories[0].Code)
	}
}

func TestHandleCategoriesAdd_Duplicate(t *testing.T) {
	server, _ := setupCategoriesTestServer(t)

	body := `{"code":"test"}`
	req1 := httptest.NewRequest(http.MethodPost, "/api/categories", strings.NewReader(body))
	rec1 := httptest.NewRecorder()
	server.handleCategoriesAdd(rec1, req1)
	if rec1.Code != http.StatusOK {
		t.Fatalf("first add expected 200, got %d: %s", rec1.Code, rec1.Body.String())
	}

	req2 := httptest.NewRequest(http.MethodPost, "/api/categories", strings.NewReader(body))
	rec2 := httptest.NewRecorder()
	server.handleCategoriesAdd(rec2, req2)

	if rec2.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", rec2.Code, rec2.Body.String())
	}
}

func TestHandleCategoriesAdd_EmptyCode(t *testing.T) {
	server, _ := setupCategoriesTestServer(t)

	body := `{"code":""}`
	req := httptest.NewRequest(http.MethodPost, "/api/categories", strings.NewReader(body))
	rec := httptest.NewRecorder()

	server.handleCategoriesAdd(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleCategoriesAdd_InvalidJSON(t *testing.T) {
	server, _ := setupCategoriesTestServer(t)

	body := `not json`
	req := httptest.NewRequest(http.MethodPost, "/api/categories", strings.NewReader(body))
	rec := httptest.NewRecorder()

	server.handleCategoriesAdd(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleCategoriesDelete_Success(t *testing.T) {
	server, _ := setupCategoriesTestServer(t)

	// Add category first
	body := `{"code":"test"}`
	reqAdd := httptest.NewRequest(http.MethodPost, "/api/categories", strings.NewReader(body))
	recAdd := httptest.NewRecorder()
	server.handleCategoriesAdd(recAdd, reqAdd)
	if recAdd.Code != http.StatusOK {
		t.Fatalf("add expected 200, got %d: %s", recAdd.Code, recAdd.Body.String())
	}

	// Delete category
	req := httptest.NewRequest(http.MethodDelete, "/api/categories/test", nil)
	req = withChiParam(t, req, "code", "test")
	rec := httptest.NewRecorder()

	server.handleCategoriesDelete(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}
	if resp["status"] != "ok" {
		t.Errorf("expected status ok, got %v", resp["status"])
	}

	snapshot := server.cfgWatcher.Current()
	if len(snapshot.Config.Categories) != 0 {
		t.Fatalf("expected 0 categories, got %d", len(snapshot.Config.Categories))
	}
}

func TestHandleCategoriesDelete_NotFound(t *testing.T) {
	server, _ := setupCategoriesTestServer(t)

	req := httptest.NewRequest(http.MethodDelete, "/api/categories/test", nil)
	req = withChiParam(t, req, "code", "test")
	rec := httptest.NewRecorder()

	server.handleCategoriesDelete(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleCategoriesDelete_MissingCode(t *testing.T) {
	server, _ := setupCategoriesTestServer(t)

	req := httptest.NewRequest(http.MethodDelete, "/api/categories/", nil)
	req = withChiParam(t, req, "code", "")
	rec := httptest.NewRecorder()

	server.handleCategoriesDelete(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleCategoriesList_NilRegistry(t *testing.T) {
	server, _ := setupCategoriesTestServer(t)
	server.registry = nil

	req := httptest.NewRequest(http.MethodGet, "/api/categories", nil)
	rec := httptest.NewRecorder()
	server.handleCategoriesList(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	configured, ok := resp["configured"].([]interface{})
	if !ok {
		t.Fatalf("expected configured array, got %T", resp["configured"])
	}
	if len(configured) != 0 {
		t.Errorf("expected empty configured, got %d", len(configured))
	}

	available, ok := resp["available"].([]interface{})
	if !ok {
		t.Fatalf("expected available array, got %T", resp["available"])
	}
	if len(available) != 0 {
		t.Errorf("expected empty available, got %d", len(available))
	}
}

func TestHandleCategoryDomains_MissingSource(t *testing.T) {
	reg := &catTestRegistry{}
	server, _ := setupCategoriesTestServer(t)
	server.registry = reg

	req := httptest.NewRequest(http.MethodGet, "/api/categories/nonexistent/domains", nil)
	req = withChiParam(t, req, "code", "nonexistent")
	rec := httptest.NewRecorder()

	server.handleCategoryDomains(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}
