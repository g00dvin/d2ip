package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/goodvin/d2ip/internal/config"
	"github.com/goodvin/d2ip/internal/orchestrator"
)

// pipelineTestOrchestrator is a mock orchestrator for pipeline handler tests.
type pipelineTestOrchestrator struct {
	history   []orchestrator.PipelineReport
	cancelErr error
}

func (m *pipelineTestOrchestrator) Run(ctx context.Context, req orchestrator.PipelineRequest) (orchestrator.PipelineReport, error) {
	return orchestrator.PipelineReport{}, nil
}
func (m *pipelineTestOrchestrator) Cancel() error {
	return m.cancelErr
}
func (m *pipelineTestOrchestrator) Status() orchestrator.RunStatus {
	return orchestrator.RunStatus{}
}
func (m *pipelineTestOrchestrator) History() []orchestrator.PipelineReport {
	return m.history
}

func setupPipelineTestServer(t *testing.T, orch *pipelineTestOrchestrator) *Server {
	t.Helper()
	cfg := config.Defaults()
	watcher := config.NewWatcher(cfg, 1, nil)
	if orch == nil {
		return &Server{cfgWatcher: watcher}
	}
	server := New(orch, nil, watcher, nil, nil, nil, nil, nil, nil)
	return server
}

func TestHandlePipelineHistory_WithOrchestrator(t *testing.T) {
	mock := &pipelineTestOrchestrator{
		history: []orchestrator.PipelineReport{
			{RunID: 1, Domains: 10, Duration: time.Second},
			{RunID: 2, Domains: 20, Duration: 2 * time.Second},
		},
	}
	s := setupPipelineTestServer(t, mock)
	r := chi.NewRouter()
	r.Get("/api/pipeline/history", s.handlePipelineHistory)

	req := httptest.NewRequest(http.MethodGet, "/api/pipeline/history", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	history, ok := resp["history"].([]interface{})
	if !ok {
		t.Fatalf("expected history array, got %T", resp["history"])
	}
	if len(history) != 2 {
		t.Errorf("expected 2 history items, got %d", len(history))
	}
}

func TestHandlePipelineCancel_Success(t *testing.T) {
	mock := &pipelineTestOrchestrator{cancelErr: nil}
	s := setupPipelineTestServer(t, mock)
	r := chi.NewRouter()
	r.Post("/pipeline/cancel", s.handlePipelineCancel)

	req := httptest.NewRequest(http.MethodPost, "/pipeline/cancel", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}
	if resp["status"] != "cancelled" {
		t.Errorf("expected status 'cancelled', got %q", resp["status"])
	}
}

func TestHandlePipelineCancel_NotRunning(t *testing.T) {
	mock := &pipelineTestOrchestrator{cancelErr: orchestrator.ErrNotRunning}
	s := setupPipelineTestServer(t, mock)
	r := chi.NewRouter()
	r.Post("/pipeline/cancel", s.handlePipelineCancel)

	req := httptest.NewRequest(http.MethodPost, "/pipeline/cancel", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}
	if resp["status"] != "not running" {
		t.Errorf("expected status 'not running', got %q", resp["status"])
	}
}

func TestHandlePipelineCancel_OtherError(t *testing.T) {
	mock := &pipelineTestOrchestrator{cancelErr: errors.New("some error")}
	s := setupPipelineTestServer(t, mock)
	r := chi.NewRouter()
	r.Post("/pipeline/cancel", s.handlePipelineCancel)

	req := httptest.NewRequest(http.MethodPost, "/pipeline/cancel", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", rec.Code)
	}
}

func TestHandlePipelineCancel_NilOrchestrator(t *testing.T) {
	s := setupPipelineTestServer(t, nil)
	r := chi.NewRouter()
	r.Post("/pipeline/cancel", s.handlePipelineCancel)

	req := httptest.NewRequest(http.MethodPost, "/pipeline/cancel", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}
}
