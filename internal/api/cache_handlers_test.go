package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/goodvin/d2ip/internal/cache"
	"github.com/goodvin/d2ip/internal/config"
	"github.com/goodvin/d2ip/internal/events"
)

func setupCacheTestServer(t *testing.T) (*Server, *cache.SQLiteCache) {
	t.Helper()
	ctx := context.Background()
	db, err := cache.Open(ctx, ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.Defaults()
	bus := events.NewBus()
	watcher := config.NewWatcher(cfg, 1, bus)
	server := New(nil, nil, watcher, nil, nil, nil, db, bus, nil)
	return server, db
}

func TestCacheStats(t *testing.T) {
	server, db := setupCacheTestServer(t)
	defer db.Close()

	req := httptest.NewRequest(http.MethodGet, "/api/cache/stats", nil)
	rr := httptest.NewRecorder()
	server.handleCacheStats(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	keys := []string{
		"domains", "domains_valid", "domains_failed", "domains_nxdomain",
		"records_total", "records_v4", "records_v6", "records_valid", "records_failed", "records_nxdomain",
		"oldest_updated", "newest_updated",
	}
	for _, k := range keys {
		if _, ok := resp[k]; !ok {
			t.Errorf("response missing key %q", k)
		}
	}
}

func TestCacheStats_NoCache(t *testing.T) {
	server := &Server{cacheAgent: nil}

	req := httptest.NewRequest(http.MethodGet, "/api/cache/stats", nil)
	rr := httptest.NewRecorder()
	server.handleCacheStats(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}
	if resp["error"] != "cache unavailable" {
		t.Errorf("expected error 'cache unavailable', got %q", resp["error"])
	}
}

func TestCachePurge(t *testing.T) {
	server, db := setupCacheTestServer(t)
	defer db.Close()

	body := `{"pattern":"*","older":"1h","failed":true}`
	req := httptest.NewRequest(http.MethodPost, "/api/cache/purge", strings.NewReader(body))
	rr := httptest.NewRecorder()
	server.handleCachePurge(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}
	if resp["status"] != "ok" {
		t.Errorf("expected status 'ok', got %q", resp["status"])
	}
	if resp["message"] != "purge requires cache.DeleteByPattern — not yet implemented" {
		t.Errorf("unexpected message: %q", resp["message"])
	}
}

func TestCacheVacuum(t *testing.T) {
	server, db := setupCacheTestServer(t)
	defer db.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/cache/vacuum", nil)
	rr := httptest.NewRecorder()
	server.handleCacheVacuum(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}
	if resp["status"] != "ok" {
		t.Errorf("expected status 'ok', got %v", resp["status"])
	}
	if _, ok := resp["deleted"]; !ok {
		t.Error("response missing 'deleted' key")
	}
}

func TestCacheEntries_MissingDomain(t *testing.T) {
	server, db := setupCacheTestServer(t)
	defer db.Close()

	req := httptest.NewRequest(http.MethodGet, "/api/cache/entries", nil)
	rr := httptest.NewRecorder()
	server.handleCacheEntries(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}
	if resp["error"] != "domain query parameter is required" {
		t.Errorf("expected error 'domain query parameter is required', got %q", resp["error"])
	}
}

func TestCacheEntries_NotImplemented(t *testing.T) {
	server, db := setupCacheTestServer(t)
	defer db.Close()

	req := httptest.NewRequest(http.MethodGet, "/api/cache/entries?domain=example.com", nil)
	rr := httptest.NewRecorder()
	server.handleCacheEntries(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}
	if resp["error"] != "domain-level lookup not yet implemented" {
		t.Errorf("expected error 'domain-level lookup not yet implemented', got %q", resp["error"])
	}
}
