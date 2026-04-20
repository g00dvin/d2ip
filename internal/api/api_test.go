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
