package api

import (
	"context"
	"encoding/json"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"os"
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

func TestHandleSourcesList_NilRegistry(t *testing.T) {
	server, _ := setupSourceServer(t)
	server.registry = nil

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
	if len(sources) != 0 {
		t.Errorf("expected empty sources, got %d", len(sources))
	}
}

func TestHandleSourceCreate_InvalidJSON(t *testing.T) {
	server, _ := setupSourceServer(t)

	body := `{"bad"`
	req := httptest.NewRequest(http.MethodPost, "/api/sources", strings.NewReader(body))
	rr := httptest.NewRecorder()
	server.handleSourceCreate(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleSourceCreate_MissingID(t *testing.T) {
	server, _ := setupSourceServer(t)

	body := `{"provider":"plaintext","prefix":"plain"}`
	req := httptest.NewRequest(http.MethodPost, "/api/sources", strings.NewReader(body))
	rr := httptest.NewRecorder()
	server.handleSourceCreate(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleSourceCreate_MissingPrefix(t *testing.T) {
	server, _ := setupSourceServer(t)

	body := `{"id":"src3","provider":"plaintext"}`
	req := httptest.NewRequest(http.MethodPost, "/api/sources", strings.NewReader(body))
	rr := httptest.NewRecorder()
	server.handleSourceCreate(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleSourceCreate_MissingProvider(t *testing.T) {
	server, _ := setupSourceServer(t)

	body := `{"id":"src3","prefix":"plain"}`
	req := httptest.NewRequest(http.MethodPost, "/api/sources", strings.NewReader(body))
	rr := httptest.NewRecorder()
	server.handleSourceCreate(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

// mockSource implements sourcereg.Source for testing.
type mockSource struct {
	info sourcereg.SourceInfo
}

func (m *mockSource) ID() string                                 { return m.info.ID }
func (m *mockSource) Prefix() string                             { return m.info.Prefix }
func (m *mockSource) Provider() sourcereg.SourceType             { return sourcereg.SourceType(m.info.Provider) }
func (m *mockSource) Load(ctx context.Context) error             { return nil }
func (m *mockSource) Close() error                               { return nil }
func (m *mockSource) Categories() []string                       { return nil }
func (m *mockSource) Info() sourcereg.SourceInfo                 { return m.info }
func (m *mockSource) IsDomainSource() bool                       { return false }
func (m *mockSource) IsPrefixSource() bool                       { return false }
func (m *mockSource) AsDomainSource() sourcereg.DomainSource     { return nil }
func (m *mockSource) AsPrefixSource() sourcereg.PrefixSource     { return nil }

// sourceTestRegistryWithGet extends sourceTestRegistry to support GetSource.
type sourceTestRegistryWithGet struct {
	sourceTestRegistry
	src sourcereg.Source
}

func (m *sourceTestRegistryWithGet) GetSource(id string) (sourcereg.Source, bool) {
	if m.src != nil && m.src.ID() == id {
		return m.src, true
	}
	return nil, false
}

func TestHandleSourceGet_Success(t *testing.T) {
	reg := &sourceTestRegistryWithGet{
		src: &mockSource{info: sourcereg.SourceInfo{ID: "src1", Provider: "v2flygeosite", Prefix: "geosite"}},
	}
	server, _ := setupSourceServer(t)
	server.registry = reg

	req := httptest.NewRequest(http.MethodGet, "/api/sources/src1", nil)
	req = withChiParam(t, req, "id", "src1")
	rr := httptest.NewRecorder()
	server.handleSourceGet(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}
	if resp["id"] != "src1" {
		t.Errorf("expected id 'src1', got %v", resp["id"])
	}
}

func TestHandleSourceGet_NotFound(t *testing.T) {
	reg := &sourceTestRegistryWithGet{}
	server, _ := setupSourceServer(t)
	server.registry = reg

	req := httptest.NewRequest(http.MethodGet, "/api/sources/missing", nil)
	req = withChiParam(t, req, "id", "missing")
	rr := httptest.NewRecorder()
	server.handleSourceGet(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleSourceGet_NilRegistry(t *testing.T) {
	server, _ := setupSourceServer(t)
	server.registry = nil

	req := httptest.NewRequest(http.MethodGet, "/api/sources/src1", nil)
	req = withChiParam(t, req, "id", "src1")
	rr := httptest.NewRecorder()
	server.handleSourceGet(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleSourceGet_EmptyID(t *testing.T) {
	server, _ := setupSourceServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/sources/", nil)
	req = withChiParam(t, req, "id", "")
	rr := httptest.NewRecorder()
	server.handleSourceGet(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleSourceUpdate_Success(t *testing.T) {
	reg := &sourceTestRegistryWithGet{}
	server, _ := setupSourceServer(t)
	server.registry = reg

	body := `{"provider":"plaintext","prefix":"plain","enabled":true}`
	req := httptest.NewRequest(http.MethodPut, "/api/sources/src1", strings.NewReader(body))
	req = withChiParam(t, req, "id", "src1")
	rr := httptest.NewRecorder()
	server.handleSourceUpdate(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	if len(reg.added) != 1 {
		t.Fatalf("expected 1 updated source, got %d", len(reg.added))
	}
	if reg.added[0].ID != "src1" {
		t.Errorf("expected id 'src1', got %s", reg.added[0].ID)
	}
}

func TestHandleSourceUpdate_InvalidJSON(t *testing.T) {
	server, _ := setupSourceServer(t)

	body := `{"bad"`
	req := httptest.NewRequest(http.MethodPut, "/api/sources/src1", strings.NewReader(body))
	req = withChiParam(t, req, "id", "src1")
	rr := httptest.NewRecorder()
	server.handleSourceUpdate(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleSourceUpdate_NilRegistry(t *testing.T) {
	server, _ := setupSourceServer(t)
	server.registry = nil

	body := `{}`
	req := httptest.NewRequest(http.MethodPut, "/api/sources/src1", strings.NewReader(body))
	req = withChiParam(t, req, "id", "src1")
	rr := httptest.NewRecorder()
	server.handleSourceUpdate(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleSourceUpdate_EmptyID(t *testing.T) {
	server, _ := setupSourceServer(t)

	body := `{}`
	req := httptest.NewRequest(http.MethodPut, "/api/sources/", strings.NewReader(body))
	req = withChiParam(t, req, "id", "")
	rr := httptest.NewRecorder()
	server.handleSourceUpdate(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleSourceDelete_Success(t *testing.T) {
	reg := &sourceTestRegistryWithGet{}
	server, _ := setupSourceServer(t)
	server.registry = reg

	req := httptest.NewRequest(http.MethodDelete, "/api/sources/src1", nil)
	req = withChiParam(t, req, "id", "src1")
	rr := httptest.NewRecorder()
	server.handleSourceDelete(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleSourceDelete_NilRegistry(t *testing.T) {
	server, _ := setupSourceServer(t)
	server.registry = nil

	req := httptest.NewRequest(http.MethodDelete, "/api/sources/src1", nil)
	req = withChiParam(t, req, "id", "src1")
	rr := httptest.NewRecorder()
	server.handleSourceDelete(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleSourceDelete_EmptyID(t *testing.T) {
	server, _ := setupSourceServer(t)

	req := httptest.NewRequest(http.MethodDelete, "/api/sources/", nil)
	req = withChiParam(t, req, "id", "")
	rr := httptest.NewRecorder()
	server.handleSourceDelete(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleSourceRefresh_Success(t *testing.T) {
	reg := &sourceTestRegistryWithGet{
		src: &mockSource{info: sourcereg.SourceInfo{ID: "src1", Provider: "v2flygeosite", Prefix: "geosite"}},
	}
	server, _ := setupSourceServer(t)
	server.registry = reg

	req := httptest.NewRequest(http.MethodPost, "/api/sources/src1/refresh", nil)
	req = withChiParam(t, req, "id", "src1")
	rr := httptest.NewRecorder()
	server.handleSourceRefresh(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleSourceRefresh_NotFound(t *testing.T) {
	reg := &sourceTestRegistryWithGet{}
	server, _ := setupSourceServer(t)
	server.registry = reg

	req := httptest.NewRequest(http.MethodPost, "/api/sources/missing/refresh", nil)
	req = withChiParam(t, req, "id", "missing")
	rr := httptest.NewRecorder()
	server.handleSourceRefresh(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleSourceRefresh_NilRegistry(t *testing.T) {
	server, _ := setupSourceServer(t)
	server.registry = nil

	req := httptest.NewRequest(http.MethodPost, "/api/sources/src1/refresh", nil)
	req = withChiParam(t, req, "id", "src1")
	rr := httptest.NewRecorder()
	server.handleSourceRefresh(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleSourceRefresh_EmptyID(t *testing.T) {
	server, _ := setupSourceServer(t)

	req := httptest.NewRequest(http.MethodPost, "/api/sources//refresh", nil)
	req = withChiParam(t, req, "id", "")
	rr := httptest.NewRecorder()
	server.handleSourceRefresh(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleSourceFetchLegacy_Success(t *testing.T) {
	reg := &sourceTestRegistryWithGet{}
	server, _ := setupSourceServer(t)
	server.registry = reg

	req := httptest.NewRequest(http.MethodPost, "/api/source/fetch", nil)
	rr := httptest.NewRecorder()
	server.handleSourceFetchLegacy(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleSourceFetchLegacy_NilRegistry(t *testing.T) {
	server, _ := setupSourceServer(t)
	server.registry = nil

	req := httptest.NewRequest(http.MethodPost, "/api/source/fetch", nil)
	rr := httptest.NewRecorder()
	server.handleSourceFetchLegacy(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleSourceUpload_Success(t *testing.T) {
	server, _ := setupSourceServer(t)

	var b strings.Builder
	w := multipart.NewWriter(&b)
	fw, err := w.CreateFormFile("file", "test.txt")
	if err != nil {
		t.Fatal(err)
	}
	fw.Write([]byte("example.com\n"))
	w.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/sources/upload", strings.NewReader(b.String()))
	req.Header.Set("Content-Type", w.FormDataContentType())
	rr := httptest.NewRecorder()
	server.handleSourceUpload(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}
	if resp["path"] == "" {
		t.Error("expected path in response")
	}

	// Clean up
	os.Remove(resp["path"])
}

func TestHandleSourceUpload_NotTxt(t *testing.T) {
	server, _ := setupSourceServer(t)

	var b strings.Builder
	w := multipart.NewWriter(&b)
	fw, err := w.CreateFormFile("file", "test.jpg")
	if err != nil {
		t.Fatal(err)
	}
	fw.Write([]byte("data"))
	w.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/sources/upload", strings.NewReader(b.String()))
	req.Header.Set("Content-Type", w.FormDataContentType())
	rr := httptest.NewRecorder()
	server.handleSourceUpload(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleSourceUpload_NoFile(t *testing.T) {
	server, _ := setupSourceServer(t)

	var b strings.Builder
	w := multipart.NewWriter(&b)
	w.WriteField("other", "value")
	w.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/sources/upload", strings.NewReader(b.String()))
	req.Header.Set("Content-Type", w.FormDataContentType())
	rr := httptest.NewRecorder()
	server.handleSourceUpload(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleSourceUpload_ParseFormError(t *testing.T) {
	server, _ := setupSourceServer(t)

	req := httptest.NewRequest(http.MethodPost, "/api/sources/upload", strings.NewReader("not multipart"))
	req.Header.Set("Content-Type", "text/plain")
	rr := httptest.NewRecorder()
	server.handleSourceUpload(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

// errorAddRegistry is a registry where AddSource returns an error.
type errorAddRegistry struct {
	sourceTestRegistry
}

func (m *errorAddRegistry) AddSource(ctx context.Context, cfg sourcereg.SourceConfig) error {
	return errors.New("add error")
}

func TestHandleSourceCreate_AddSourceError(t *testing.T) {
	reg := &errorAddRegistry{}
	server, _ := setupSourceServer(t)
	server.registry = reg

	body := `{"id":"src2","provider":"plaintext","prefix":"plain","enabled":true}`
	req := httptest.NewRequest(http.MethodPost, "/api/sources", strings.NewReader(body))
	rr := httptest.NewRecorder()
	server.handleSourceCreate(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", rr.Code, rr.Body.String())
	}
}

// errorRemoveRegistry is a registry where RemoveSource returns an error.
type errorRemoveRegistry struct {
	sourceTestRegistry
}

func (m *errorRemoveRegistry) RemoveSource(ctx context.Context, id string) error {
	return errors.New("remove error")
}

func TestHandleSourceDelete_RemoveSourceError(t *testing.T) {
	reg := &errorRemoveRegistry{}
	server, _ := setupSourceServer(t)
	server.registry = reg

	req := httptest.NewRequest(http.MethodDelete, "/api/sources/src1", nil)
	req = withChiParam(t, req, "id", "src1")
	rr := httptest.NewRecorder()
	server.handleSourceDelete(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", rr.Code, rr.Body.String())
	}
}
