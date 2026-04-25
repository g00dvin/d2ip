package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/goodvin/d2ip/internal/config"
)

// settingsTestKVStore is a simple in-memory KVStore for settings handler tests.
type settingsTestKVStore struct {
	data map[string]string
}

func (m *settingsTestKVStore) GetAll(_ context.Context) (map[string]string, error) {
	return m.data, nil
}
func (m *settingsTestKVStore) Set(_ context.Context, key, value string) error {
	if m.data == nil {
		m.data = make(map[string]string)
	}
	m.data[key] = value
	return nil
}
func (m *settingsTestKVStore) Delete(_ context.Context, key string) error {
	delete(m.data, key)
	return nil
}

func setupSettingsTestServer(t *testing.T) (*Server, *config.Watcher, *settingsTestKVStore) {
	t.Helper()
	cfg := config.Defaults()
	watcher := config.NewWatcher(cfg, 1, nil)
	kv := &settingsTestKVStore{data: make(map[string]string)}
	server := New(nil, nil, watcher, kv, nil, nil, nil, nil, nil)
	return server, watcher, kv
}

func TestHandleSettingsGet_WithNilKVStore(t *testing.T) {
	cfg := config.Defaults()
	watcher := config.NewWatcher(cfg, 1, nil)

	s := &Server{cfgWatcher: watcher, kvStore: nil}
	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Get("/api/settings", s.handleSettingsGet)

	req := httptest.NewRequest(http.MethodGet, "/api/settings", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
}

func TestHandleSettingsPut_Success(t *testing.T) {
	s, _, kv := setupSettingsTestServer(t)
	r := chi.NewRouter()
	r.Put("/api/settings", s.handleSettingsPut)

	body := `{"resolver.qps": "999", "resolver.upstream": "8.8.8.8:53"}`
	req := httptest.NewRequest(http.MethodPut, "/api/settings", strings.NewReader(body))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	if kv.data["resolver.qps"] != "999" {
		t.Errorf("expected resolver.qps=999, got %s", kv.data["resolver.qps"])
	}
	if kv.data["resolver.upstream"] != "8.8.8.8:53" {
		t.Errorf("expected resolver.upstream=8.8.8.8:53, got %s", kv.data["resolver.upstream"])
	}
}

func TestHandleSettingsPut_EmptyBody(t *testing.T) {
	s, _, _ := setupSettingsTestServer(t)
	r := chi.NewRouter()
	r.Put("/api/settings", s.handleSettingsPut)

	body := `{}`
	req := httptest.NewRequest(http.MethodPut, "/api/settings", strings.NewReader(body))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for empty body, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleSettingsPut_InvalidJSON(t *testing.T) {
	s, _, _ := setupSettingsTestServer(t)
	r := chi.NewRouter()
	r.Put("/api/settings", s.handleSettingsPut)

	body := `{"bad"`
	req := httptest.NewRequest(http.MethodPut, "/api/settings", strings.NewReader(body))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestHandleSettingsPut_NilKVStore(t *testing.T) {
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

func TestHandleSettingsDelete_Success(t *testing.T) {
	s, _, kv := setupSettingsTestServer(t)
	kv.data["resolver.qps"] = "500"

	r := chi.NewRouter()
	r.Delete("/api/settings/{key}", s.handleSettingsDelete)

	req := httptest.NewRequest(http.MethodDelete, "/api/settings/resolver.qps", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	if _, ok := kv.data["resolver.qps"]; ok {
		t.Error("expected resolver.qps to be deleted from kvStore")
	}
}

func TestHandleSettingsDelete_MissingKey(t *testing.T) {
	s, _, _ := setupSettingsTestServer(t)
	r := chi.NewRouter()
	r.Delete("/api/settings/{key}", s.handleSettingsDelete)

	// Hit the route with an empty key parameter.
	req := httptest.NewRequest(http.MethodDelete, "/api/settings/", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestHandleSettingsDelete_NilKVStore(t *testing.T) {
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

func TestStructToMap(t *testing.T) {
	type nested struct {
		Inner string `json:"inner"`
	}
	type testStruct struct {
		Name   string `json:"name"`
		Value  int    `json:"value"`
		Nested nested `json:"nested"`
	}

	v := testStruct{Name: "test", Value: 42, Nested: nested{Inner: "hello"}}
	m := structToMap(v)

	if m["name"] != "test" {
		t.Errorf("expected name=test, got %v", m["name"])
	}
	if m["value"] != float64(42) {
		// JSON unmarshals numbers as float64.
		t.Errorf("expected value=42, got %v", m["value"])
	}
	if m["nested.inner"] != "hello" {
		t.Errorf("expected nested.inner=hello, got %v", m["nested.inner"])
	}
}

func TestFlattenMap(t *testing.T) {
	m := map[string]interface{}{
		"a": 1,
		"b": map[string]interface{}{
			"c": 2,
			"d": map[string]interface{}{
				"e": 3,
			},
		},
	}

	flat := flattenMap(m, "")

	expected := map[string]interface{}{
		"a":     1,
		"b.c":   2,
		"b.d.e": 3,
	}

	if len(flat) != len(expected) {
		t.Fatalf("expected %d keys, got %d", len(expected), len(flat))
	}
	for k, v := range expected {
		if flat[k] != v {
			t.Errorf("expected %s=%v, got %v", k, v, flat[k])
		}
	}
}

func TestReloadConfig(t *testing.T) {
	s, watcher, kv := setupSettingsTestServer(t)
	kv.data["resolver.qps"] = "999"

	if err := s.reloadConfig(context.Background()); err != nil {
		t.Fatalf("reloadConfig failed: %v", err)
	}

	current := watcher.Current().Config
	if current.Resolver.QPS != 999 {
		t.Errorf("expected QPS=999 after reload, got %d", current.Resolver.QPS)
	}
}
