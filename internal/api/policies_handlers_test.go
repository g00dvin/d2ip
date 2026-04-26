package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/goodvin/d2ip/internal/config"
	"github.com/goodvin/d2ip/internal/events"
)

// policyTestKVStore implements config.KVStore for testing.
type policyTestKVStore struct {
	data map[string]string
}

func newPolicyTestKVStore() *policyTestKVStore {
	return &policyTestKVStore{data: make(map[string]string)}
}

func (m *policyTestKVStore) GetAll(ctx context.Context) (map[string]string, error) {
	out := make(map[string]string, len(m.data))
	for k, v := range m.data {
		out[k] = v
	}
	return out, nil
}

func (m *policyTestKVStore) Set(ctx context.Context, key, value string) error {
	m.data[key] = value
	return nil
}

func (m *policyTestKVStore) Delete(ctx context.Context, key string) error {
	delete(m.data, key)
	return nil
}

func setupPolicyServer(t *testing.T) (*Server, *policyTestKVStore) {
	t.Helper()
	kv := newPolicyTestKVStore()
	cfg := config.Defaults()
	bus := events.NewBus()
	watcher := config.NewWatcher(cfg, 1, bus)
	server := New(nil, nil, watcher, kv, nil, nil, nil, bus, nil)
	return server, kv
}

func mustMarshal(t *testing.T, v interface{}) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("json.Marshal error: %v", err)
	}
	return b
}

func TestPolicyCreate(t *testing.T) {
	server, kv := setupPolicyServer(t)

	policy := config.PolicyConfig{
		Name:       "p1",
		Enabled:    true,
		Categories: []string{"geosite:ru"},
		Backend:    config.BackendNFTables,
		NFTTable:   "inet d2ip",
		NFTSetV4:   "set_v4",
		NFTSetV6:   "set_v6",
	}
	body := mustMarshal(t, policy)

	req := httptest.NewRequest("POST", "/api/policies", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	server.handlePolicyCreate(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	cfg := server.cfgWatcher.Current().Config
	if len(cfg.Routing.Policies) != 1 {
		t.Fatalf("expected 1 policy in config, got %d", len(cfg.Routing.Policies))
	}
	if cfg.Routing.Policies[0].Name != "p1" {
		t.Errorf("expected policy name 'p1', got %s", cfg.Routing.Policies[0].Name)
	}

	val, ok := kv.data["routing.policies"]
	if !ok {
		t.Fatal("expected routing.policies to be persisted in kvStore")
	}
	if !strings.Contains(val, `"name":"p1"`) {
		t.Errorf("expected kvStore value to contain policy name, got %s", val)
	}
}

func TestPolicyCreate_DuplicateName(t *testing.T) {
	server, _ := setupPolicyServer(t)

	policy := config.PolicyConfig{
		Name:       "p1",
		Enabled:    true,
		Categories: []string{"geosite:ru"},
		Backend:    config.BackendNFTables,
		NFTTable:   "inet d2ip",
		NFTSetV4:   "set_v4",
		NFTSetV6:   "set_v6",
	}
	body := mustMarshal(t, policy)

	// First create
	req1 := httptest.NewRequest("POST", "/api/policies", bytes.NewReader(body))
	rr1 := httptest.NewRecorder()
	server.handlePolicyCreate(rr1, req1)
	if rr1.Code != http.StatusOK {
		t.Fatalf("first create expected 200, got %d", rr1.Code)
	}

	// Second create (same name)
	req2 := httptest.NewRequest("POST", "/api/policies", bytes.NewReader(body))
	rr2 := httptest.NewRecorder()
	server.handlePolicyCreate(rr2, req2)

	if rr2.Code != http.StatusConflict {
		t.Fatalf("expected status 409, got %d: %s", rr2.Code, rr2.Body.String())
	}
}

func TestPolicyUpdate(t *testing.T) {
	server, kv := setupPolicyServer(t)

	// Create initial policy
	policy := config.PolicyConfig{
		Name:       "p1",
		Enabled:    true,
		Categories: []string{"geosite:ru"},
		Backend:    config.BackendNFTables,
		NFTTable:   "inet d2ip",
		NFTSetV4:   "set_v4",
		NFTSetV6:   "set_v6",
	}
	body := mustMarshal(t, policy)
	req := httptest.NewRequest("POST", "/api/policies", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	server.handlePolicyCreate(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("create expected 200, got %d", rr.Code)
	}

	// Update policy
	updated := config.PolicyConfig{
		Name:    "p1",
		Enabled: false,
		Backend: config.BackendIProute2,
		TableID: 100,
		Iface:   "eth0",
	}
	body = mustMarshal(t, updated)
	req = httptest.NewRequest("PUT", "/api/policies/p1", bytes.NewReader(body))
	req = withChiParam(t, req, "name", "p1")
	rr = httptest.NewRecorder()
	server.handlePolicyUpdate(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	cfg := server.cfgWatcher.Current().Config
	if len(cfg.Routing.Policies) != 1 {
		t.Fatalf("expected 1 policy, got %d", len(cfg.Routing.Policies))
	}
	p := cfg.Routing.Policies[0]
	if p.Backend != config.BackendIProute2 {
		t.Errorf("expected backend 'iproute2', got %s", p.Backend)
	}
	if p.Enabled != false {
		t.Errorf("expected enabled false, got %v", p.Enabled)
	}
	if p.TableID != 100 {
		t.Errorf("expected table_id 100, got %d", p.TableID)
	}

	val, ok := kv.data["routing.policies"]
	if !ok {
		t.Fatal("expected routing.policies to be persisted in kvStore")
	}
	if !strings.Contains(val, `"backend":"iproute2"`) {
		t.Errorf("expected kvStore value to contain updated backend, got %s", val)
	}
}

func TestPolicyUpdate_NotFound(t *testing.T) {
	server, _ := setupPolicyServer(t)

	updated := config.PolicyConfig{
		Name:       "nonexistent",
		Enabled:    true,
		Categories: []string{"geosite:ru"},
		Backend:    config.BackendNFTables,
		NFTTable:   "inet d2ip",
		NFTSetV4:   "set_v4",
		NFTSetV6:   "set_v6",
	}
	body := mustMarshal(t, updated)
	req := httptest.NewRequest("PUT", "/api/policies/nonexistent", bytes.NewReader(body))
	req = withChiParam(t, req, "name", "nonexistent")
	rr := httptest.NewRecorder()
	server.handlePolicyUpdate(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestPolicyDelete(t *testing.T) {
	server, kv := setupPolicyServer(t)

	// Create policy
	policy := config.PolicyConfig{
		Name:       "p1",
		Enabled:    true,
		Categories: []string{"geosite:ru"},
		Backend:    config.BackendNFTables,
		NFTTable:   "inet d2ip",
		NFTSetV4:   "set_v4",
		NFTSetV6:   "set_v6",
	}
	body := mustMarshal(t, policy)
	req := httptest.NewRequest("POST", "/api/policies", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	server.handlePolicyCreate(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("create expected 200, got %d", rr.Code)
	}

	// Delete policy
	req = httptest.NewRequest("DELETE", "/api/policies/p1", nil)
	req = withChiParam(t, req, "name", "p1")
	rr = httptest.NewRecorder()
	server.handlePolicyDelete(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	cfg := server.cfgWatcher.Current().Config
	if len(cfg.Routing.Policies) != 0 {
		t.Fatalf("expected 0 policies, got %d", len(cfg.Routing.Policies))
	}

	val, ok := kv.data["routing.policies"]
	if !ok {
		t.Fatal("expected routing.policies to be persisted in kvStore")
	}
	if strings.TrimSpace(val) != "[]" {
		t.Errorf("expected kvStore value to be '[]', got %s", val)
	}
}

func TestPolicyDelete_NotFound(t *testing.T) {
	server, _ := setupPolicyServer(t)

	req := httptest.NewRequest("DELETE", "/api/policies/nonexistent", nil)
	req = withChiParam(t, req, "name", "nonexistent")
	rr := httptest.NewRecorder()
	server.handlePolicyDelete(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandlePoliciesList(t *testing.T) {
	server, _ := setupPolicyServer(t)

	// Create a policy first
	policy := config.PolicyConfig{
		Name:       "p1",
		Enabled:    true,
		Categories: []string{"geosite:ru"},
		Backend:    config.BackendNFTables,
		NFTTable:   "inet d2ip",
		NFTSetV4:   "set_v4",
		NFTSetV6:   "set_v6",
	}
	body := mustMarshal(t, policy)
	req := httptest.NewRequest("POST", "/api/policies", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	server.handlePolicyCreate(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("create expected 200, got %d", rr.Code)
	}

	req = httptest.NewRequest("GET", "/api/policies", nil)
	rr = httptest.NewRecorder()
	server.handlePoliciesList(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	policies, ok := resp["policies"].([]interface{})
	if !ok {
		t.Fatalf("expected policies array, got %T", resp["policies"])
	}
	if len(policies) != 1 {
		t.Errorf("expected 1 policy, got %d", len(policies))
	}
}

func TestHandlePolicyGet_Success(t *testing.T) {
	server, _ := setupPolicyServer(t)

	policy := config.PolicyConfig{
		Name:       "p1",
		Enabled:    true,
		Categories: []string{"geosite:ru"},
		Backend:    config.BackendNFTables,
		NFTTable:   "inet d2ip",
		NFTSetV4:   "set_v4",
		NFTSetV6:   "set_v6",
	}
	body := mustMarshal(t, policy)
	req := httptest.NewRequest("POST", "/api/policies", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	server.handlePolicyCreate(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("create expected 200, got %d", rr.Code)
	}

	req = httptest.NewRequest("GET", "/api/policies/p1", nil)
	req = withChiParam(t, req, "name", "p1")
	rr = httptest.NewRecorder()
	server.handlePolicyGet(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}
	if resp["name"] != "p1" {
		t.Errorf("expected name 'p1', got %v", resp["name"])
	}
}

func TestHandlePolicyGet_NotFound(t *testing.T) {
	server, _ := setupPolicyServer(t)

	req := httptest.NewRequest("GET", "/api/policies/nonexistent", nil)
	req = withChiParam(t, req, "name", "nonexistent")
	rr := httptest.NewRecorder()
	server.handlePolicyGet(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandlePolicyCreate_InvalidJSON(t *testing.T) {
	server, _ := setupPolicyServer(t)

	req := httptest.NewRequest("POST", "/api/policies", strings.NewReader("bad json"))
	rr := httptest.NewRecorder()
	server.handlePolicyCreate(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandlePolicyUpdate_NameMismatch(t *testing.T) {
	server, _ := setupPolicyServer(t)

	policy := config.PolicyConfig{Name: "p1", Enabled: true, Backend: config.BackendNFTables}
	body := mustMarshal(t, policy)
	req := httptest.NewRequest("PUT", "/api/policies/p1", bytes.NewReader(body))
	req = withChiParam(t, req, "name", "p1")
	rr := httptest.NewRecorder()
	server.handlePolicyUpdate(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for not-found policy, got %d: %s", rr.Code, rr.Body.String())
	}

	// Now test name mismatch
	policy2 := config.PolicyConfig{Name: "p2", Enabled: true, Backend: config.BackendNFTables}
	body = mustMarshal(t, policy2)
	req = httptest.NewRequest("PUT", "/api/policies/p1", bytes.NewReader(body))
	req = withChiParam(t, req, "name", "p1")
	rr = httptest.NewRecorder()
	server.handlePolicyUpdate(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for name mismatch, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandlePolicyDelete_MissingName(t *testing.T) {
	server, _ := setupPolicyServer(t)

	req := httptest.NewRequest("DELETE", "/api/policies/", nil)
	req = withChiParam(t, req, "name", "")
	rr := httptest.NewRecorder()
	server.handlePolicyDelete(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rr.Code, rr.Body.String())
	}
}
