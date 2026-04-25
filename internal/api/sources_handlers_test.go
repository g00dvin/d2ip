package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"

	"github.com/goodvin/d2ip/internal/config"
	"github.com/goodvin/d2ip/internal/events"
	"github.com/goodvin/d2ip/internal/sourcereg"
)

// sourceTestRegistry implements sourcereg.Registry for testing.
type sourceTestRegistry struct {
	sources []sourcereg.SourceInfo
	added   []sourcereg.SourceConfig
}

func (m *sourceTestRegistry) ListSources() []sourcereg.SourceInfo {
	return m.sources
}

func (m *sourceTestRegistry) AddSource(ctx context.Context, cfg sourcereg.SourceConfig) error {
	m.added = append(m.added, cfg)
	return nil
}

func (m *sourceTestRegistry) RemoveSource(ctx context.Context, id string) error { return nil }
func (m *sourceTestRegistry) LoadAll(ctx context.Context) error                 { return nil }
func (m *sourceTestRegistry) Close() error                                      { return nil }
func (m *sourceTestRegistry) GetSource(id string) (sourcereg.Source, bool)      { return nil, false }
func (m *sourceTestRegistry) ListCategories() []sourcereg.CategoryInfo          { return nil }
func (m *sourceTestRegistry) GetDomains(category string) ([]string, error)      { return nil, nil }
func (m *sourceTestRegistry) GetPrefixes(category string) ([]netip.Prefix, error) {
	return nil, nil
}
func (m *sourceTestRegistry) ResolveCategory(category string) (sourceID string, catType string, found bool) {
	return "", "", false
}

func setupSourceServer(t *testing.T) (*Server, *sourceTestRegistry) {
	t.Helper()
	reg := &sourceTestRegistry{}
	cfg := config.Defaults()
	bus := events.NewBus()
	watcher := config.NewWatcher(cfg, 1, bus)
	server := New(nil, nil, watcher, nil, nil, nil, nil, bus, reg)
	return server, reg
}

func TestSourcesList(t *testing.T) {
	server, reg := setupSourceServer(t)
	reg.sources = []sourcereg.SourceInfo{
		{ID: "src1", Provider: "v2flygeosite", Prefix: "geosite", Enabled: true},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/sources", nil)
	rr := httptest.NewRecorder()
	server.handleSourcesList(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	sources, ok := resp["sources"].([]interface{})
	if !ok {
		t.Fatalf("expected sources array, got %T", resp["sources"])
	}
	if len(sources) != 1 {
		t.Fatalf("expected 1 source, got %d", len(sources))
	}

	src := sources[0].(map[string]interface{})
	if src["id"] != "src1" {
		t.Errorf("expected id 'src1', got %v", src["id"])
	}
}

func TestSourceCreate(t *testing.T) {
	server, reg := setupSourceServer(t)

	body := `{"id":"src2","provider":"plaintext","prefix":"plain","enabled":true,"config":{"path":"/tmp/test.txt"}}`
	req := httptest.NewRequest(http.MethodPost, "/api/sources", strings.NewReader(body))
	rr := httptest.NewRecorder()
	server.handleSourceCreate(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	if len(reg.added) != 1 {
		t.Fatalf("expected 1 added source, got %d", len(reg.added))
	}

	if reg.added[0].ID != "src2" {
		t.Errorf("expected added source id 'src2', got %s", reg.added[0].ID)
	}

	var resp map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}
	if resp["status"] != "ok" {
		t.Errorf("expected status 'ok', got %s", resp["status"])
	}
}
