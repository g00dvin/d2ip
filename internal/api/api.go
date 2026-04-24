// Package api provides the HTTP API for d2ip pipeline control and status.
package api

import (
	"compress/gzip"
	"context"
	"embed"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"net/http"
	"net/netip"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog/log"

	"github.com/goodvin/d2ip/internal/cache"
	"github.com/goodvin/d2ip/internal/config"
	"github.com/goodvin/d2ip/internal/domainlist"
	"github.com/goodvin/d2ip/internal/events"
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
	eventBus    *events.Bus
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
	eventBus *events.Bus,
) *Server {
	return &Server{
		orch:        orch,
		router:      router,
		cfgWatcher:  cfgWatcher,
		kvStore:     kvStore,
		dlProvider:  dlProvider,
		sourceStore: sourceStore,
		cacheAgent:  cacheAgent,
		eventBus:    eventBus,
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

	// SSE endpoint must NOT be compressed — compression buffers response data
	// and breaks the streaming protocol (flusher.Flush() has no effect).
	r.Get("/events", s.handleEvents)

	// All other routes get compression.
	r.Group(func(cr chi.Router) {
		cr.Use(middleware.Compress(5))

		// API Routes.
		cr.Get("/healthz", s.handleHealth)
		cr.Get("/readyz", s.handleReady)
		cr.Post("/pipeline/run", s.handlePipelineRun)
		cr.Get("/pipeline/status", s.handlePipelineStatus)
		cr.Post("/routing/dry-run", s.handleRoutingDryRun)
		cr.Post("/routing/rollback", s.handleRoutingRollback)
		cr.Get("/routing/snapshot", s.handleRoutingSnapshot)
		cr.Get("/metrics", s.handleMetrics)
		cr.Get("/api/settings", s.handleSettingsGet)
		cr.Put("/api/settings", s.handleSettingsPut)
		cr.Delete("/api/settings/{key}", s.handleSettingsDelete)
		cr.Get("/api/pipeline/history", s.handlePipelineHistory)
		cr.Post("/pipeline/cancel", s.handlePipelineCancel)

		// Categories API.
		cr.Get("/api/categories", s.handleCategoriesList)
		cr.Get("/api/categories/{code}/domains", s.handleCategoryDomains)
		cr.Post("/api/categories", s.handleCategoriesAdd)
		cr.Delete("/api/categories/{code}", s.handleCategoriesDelete)

		// Cache API.
		cr.Get("/api/cache/stats", s.handleCacheStats)
		cr.Post("/api/cache/purge", s.handleCachePurge)
		cr.Post("/api/cache/vacuum", s.handleCacheVacuum)
		cr.Get("/api/cache/entries", s.handleCacheEntries)

		// Source API.
		cr.Get("/api/source/info", s.handleSourceInfo)

		// Static web UI (serve at root and /web/*).
		webRoot, err := fs.Sub(webFS, "web")
		if err != nil {
			log.Warn().Err(err).Msg("api: failed to embed web files")
		} else {
			cr.Get("/*", func(w http.ResponseWriter, r *http.Request) {
				// Serve index.html for root path
				if r.URL.Path == "/" {
					serveEmbeddedFile(w, r, webRoot, "index.html")
					return
				}
				// Strip /web/ prefix to get the file path within the embedded FS
				name := strings.TrimPrefix(r.URL.Path, "/web/")
				if name == "" {
					name = "index.html"
				}
				serveEmbeddedFile(w, r, webRoot, name)
			})
		}
	})

	return r
}

// serveEmbeddedFile serves a file from the embedded FS with correct MIME types.
// Embedded assets are pre-gzipped during build to reduce binary size.
func serveEmbeddedFile(w http.ResponseWriter, r *http.Request, embedFS fs.FS, name string) {
	// Determine MIME type by extension
	switch {
	case strings.HasSuffix(name, ".css"):
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
	case strings.HasSuffix(name, ".js"):
		w.Header().Set("Content-Type", "application/javascript")
	case strings.HasSuffix(name, ".html"):
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
	default:
		w.Header().Set("Content-Type", "application/octet-stream")
	}

	// Cache control: never cache index.html (SPA entry point), but cache
	// hashed assets aggressively since their filenames change on every build.
	if name == "index.html" {
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")
	} else if strings.Contains(name, ".") {
		// Hashed assets like index.abc123.js — cache for 1 year.
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	}

	f, err := embedFS.Open(name)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		http.Error(w, "stat error", http.StatusInternalServerError)
		return
	}

	// Embedded assets are pre-gzipped during build to reduce binary size.
	// Serve with Content-Encoding: gzip so browsers decompress automatically.
	// Fallback to on-the-fly decompression for clients that don't accept gzip.
	if strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
		w.Header().Set("Content-Encoding", "gzip")
		w.Header().Set("Vary", "Accept-Encoding")
		w.Header().Set("Content-Length", strconv.FormatInt(stat.Size(), 10))
		if _, err := io.Copy(w, f); err != nil {
			log.Warn().Err(err).Str("file", name).Msg("api: failed to serve gzipped asset")
		}
	} else {
		gr, err := gzip.NewReader(f)
		if err != nil {
			http.Error(w, "decompression error", http.StatusInternalServerError)
			return
		}
		defer gr.Close()
		if _, err := io.Copy(w, gr); err != nil {
			log.Warn().Err(err).Str("file", name).Msg("api: failed to serve decompressed asset")
		}
	}
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
// The pipeline runs with a background-derived context so that
// client disconnects don't cancel the long-running job.
func (s *Server) handlePipelineRun(w http.ResponseWriter, r *http.Request) {
	if s.orch == nil {
		s.jsonError(w, http.StatusServiceUnavailable, "orchestrator not initialized")
		return
	}

	var req orchestrator.PipelineRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// Empty body is fine; use defaults.
		_ = err
	}

	report, err := s.orch.Run(context.Background(), req)
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
	if s.orch == nil {
		s.jsonError(w, http.StatusServiceUnavailable, "orchestrator not initialized")
		return
	}
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

	// Handle empty prefix arrays without calling router
	if len(req.IPv4Prefixes) == 0 && len(req.IPv6Prefixes) == 0 {
		s.jsonOK(w, map[string]interface{}{
			"v4_plan": map[string]interface{}{"add": []interface{}{}, "remove": []interface{}{}},
			"v6_plan": map[string]interface{}{"add": []interface{}{}, "remove": []interface{}{}},
			"v4_diff": "",
			"v6_diff": "",
			"message": "no prefixes to test",
		})
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
