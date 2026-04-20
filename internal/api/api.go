// Package api provides the HTTP API for d2ip pipeline control and status.
package api

import (
	"embed"
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"
	"net/netip"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog/log"

	"github.com/goodvin/d2ip/internal/cache"
	"github.com/goodvin/d2ip/internal/config"
	"github.com/goodvin/d2ip/internal/domainlist"
	"github.com/goodvin/d2ip/internal/logging"
	"github.com/goodvin/d2ip/internal/metrics"
	"github.com/goodvin/d2ip/internal/orchestrator"
	"github.com/goodvin/d2ip/internal/routing"
	"github.com/goodvin/d2ip/internal/source"
)

//go:embed web
var webFS embed.FS

// Server wraps the HTTP API with dependencies.
type Server struct {
	orch        *orchestrator.Orchestrator
	router      routing.Router
	cfgWatcher  *config.Watcher
	kvStore     config.KVStore
	dlProvider  domainlist.ListProvider
	sourceStore source.DLCStore
	cacheAgent  cache.Cache
}

// New creates an API server with dependencies.
func New(
	orch *orchestrator.Orchestrator,
	router routing.Router,
	cfgWatcher *config.Watcher,
	kvStore config.KVStore,
	dlProvider domainlist.ListProvider,
	sourceStore source.DLCStore,
	cacheAgent cache.Cache,
) *Server {
	return &Server{
		orch:        orch,
		router:      router,
		cfgWatcher:  cfgWatcher,
		kvStore:     kvStore,
		dlProvider:  dlProvider,
		sourceStore: sourceStore,
		cacheAgent:  cacheAgent,
	}
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

	// API Routes (registered first to take precedence).
	r.Get("/healthz", s.handleHealth)
	r.Get("/readyz", s.handleReady)
	r.Post("/pipeline/run", s.handlePipelineRun)
	r.Get("/pipeline/status", s.handlePipelineStatus)
	r.Post("/routing/dry-run", s.handleRoutingDryRun)
	r.Post("/routing/rollback", s.handleRoutingRollback)
	r.Get("/routing/snapshot", s.handleRoutingSnapshot)
	r.Get("/metrics", s.handleMetrics)
	r.Get("/api/settings", s.handleSettingsGet)
	r.Put("/api/settings", s.handleSettingsPut)
	r.Delete("/api/settings/{key}", s.handleSettingsDelete)
	r.Get("/api/pipeline/history", s.handlePipelineHistory)
	r.Post("/pipeline/cancel", s.handlePipelineCancel)

	// Categories API.
	r.Get("/api/categories", s.handleCategoriesList)
	r.Get("/api/categories/{code}/domains", s.handleCategoryDomains)
	r.Post("/api/categories", s.handleCategoriesAdd)
	r.Delete("/api/categories/{code}", s.handleCategoriesDelete)

	// Cache API.
	r.Get("/api/cache/stats", s.handleCacheStats)
	r.Post("/api/cache/purge", s.handleCachePurge)
	r.Post("/api/cache/vacuum", s.handleCacheVacuum)
	r.Get("/api/cache/entries", s.handleCacheEntries)

	// Static web UI (serve at root and /web/*).
	webRoot, err := fs.Sub(webFS, "web")
	if err != nil {
		log.Warn().Err(err).Msg("api: failed to embed web files")
	} else {
		fileServer := http.FileServer(http.FS(webRoot))
		r.Get("/*", func(w http.ResponseWriter, r *http.Request) {
			// Serve index.html for root path
			if r.URL.Path == "/" {
				http.ServeFileFS(w, r, webRoot, "index.html")
				return
			}
			fileServer.ServeHTTP(w, r)
		})
	}

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
		_ = err
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

// handleRoutingDryRun shows what would change without applying.
func (s *Server) handleRoutingDryRun(w http.ResponseWriter, r *http.Request) {
	var req struct {
		IPv4Prefixes []netip.Prefix `json:"ipv4_prefixes"`
		IPv6Prefixes []netip.Prefix `json:"ipv6_prefixes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	v4Plan, v4Diff, err := s.router.DryRun(r.Context(), req.IPv4Prefixes, routing.FamilyV4)
	if err != nil {
		if errors.Is(err, routing.ErrDisabled) {
			s.jsonError(w, http.StatusServiceUnavailable, "routing disabled")
			return
		}
		log.Error().Err(err).Msg("api: dry-run v4 failed")
		s.jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	v6Plan, v6Diff, err := s.router.DryRun(r.Context(), req.IPv6Prefixes, routing.FamilyV6)
	if err != nil {
		if errors.Is(err, routing.ErrDisabled) {
			s.jsonError(w, http.StatusServiceUnavailable, "routing disabled")
			return
		}
		log.Error().Err(err).Msg("api: dry-run v6 failed")
		s.jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	resp := map[string]interface{}{
		"v4_plan": map[string]interface{}{
			"add":    v4Plan.Add,
			"remove": v4Plan.Remove,
		},
		"v6_plan": map[string]interface{}{
			"add":    v6Plan.Add,
			"remove": v6Plan.Remove,
		},
		"v4_diff": v4Diff,
		"v6_diff": v6Diff,
	}
	s.jsonOK(w, resp)
}

// handleRoutingRollback rolls back to previous state.
func (s *Server) handleRoutingRollback(w http.ResponseWriter, r *http.Request) {
	if err := s.router.Rollback(r.Context()); err != nil {
		if errors.Is(err, routing.ErrDisabled) {
			s.jsonError(w, http.StatusServiceUnavailable, "routing disabled")
			return
		}
		log.Error().Err(err).Msg("api: rollback failed")
		s.jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.jsonOK(w, map[string]string{"status": "ok"})
}

// handleRoutingSnapshot shows current applied state.
func (s *Server) handleRoutingSnapshot(w http.ResponseWriter, r *http.Request) {
	snapshot := s.router.Snapshot()
	resp := map[string]interface{}{
		"backend":    snapshot.Backend,
		"applied_at": snapshot.AppliedAt,
		"v4":         snapshot.V4,
		"v6":         snapshot.V6,
	}
	s.jsonOK(w, resp)
}
