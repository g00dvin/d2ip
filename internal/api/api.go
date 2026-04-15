// Package api provides the HTTP API for d2ip pipeline control and status.
package api

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog/log"

	"github.com/goodvin/d2ip/internal/logging"
	"github.com/goodvin/d2ip/internal/metrics"
	"github.com/goodvin/d2ip/internal/orchestrator"
)

// Server wraps the HTTP API with dependencies.
type Server struct {
	orch *orchestrator.Orchestrator
}

// New creates an API server with dependencies.
func New(orch *orchestrator.Orchestrator) *Server {
	return &Server{orch: orch}
}

// Handler returns the configured chi router.
func (s *Server) Handler() http.Handler {
	r := chi.NewRouter()

	// Middleware stack.
	r.Use(middleware.RequestID)
	r.Use(logging.RequestIDMiddleware)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Compress(5))

	// Routes.
	r.Get("/healthz", s.handleHealth)
	r.Get("/readyz", s.handleReady)
	r.Post("/pipeline/run", s.handlePipelineRun)
	r.Get("/pipeline/status", s.handlePipelineStatus)
	r.Get("/metrics", s.handleMetrics)

	return r
}

// handleHealth returns 200 if the process is alive.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

// handleReady returns 200 if dependencies are healthy (stub for Iteration 0).
func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	// TODO: check DB connection, last successful run age.
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ready"}`))
}

// handlePipelineRun triggers a new pipeline execution.
func (s *Server) handlePipelineRun(w http.ResponseWriter, r *http.Request) {
	var req orchestrator.PipelineRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// Empty body is fine; use defaults.
	}

	report, err := s.orch.Run(r.Context(), req)
	if err != nil {
		if err == orchestrator.ErrBusy {
			s.jsonError(w, http.StatusConflict, "pipeline already running")
			return
		}
		log.Error().Err(err).Msg("api: pipeline run failed")
		s.jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.jsonOK(w, report)
}

// handlePipelineStatus returns the current/last run status.
func (s *Server) handlePipelineStatus(w http.ResponseWriter, r *http.Request) {
	status := s.orch.Status()
	s.jsonOK(w, status)
}

// handleMetrics serves Prometheus metrics.
func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	promhttp.HandlerFor(metrics.Registry, promhttp.HandlerOpts{}).ServeHTTP(w, r)
}

// jsonOK writes a 200 JSON response.
func (s *Server) jsonOK(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(v)
}

// jsonError writes an error JSON response.
func (s *Server) jsonError(w http.ResponseWriter, code int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}
