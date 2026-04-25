// Package api provides the HTTP API for d2ip pipeline control and status.
package api

import (
	"compress/gzip"
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/netip"
	"os"
	"strconv"
	"strings"
	"time"

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
	"github.com/goodvin/d2ip/internal/sourcereg"
)

//go:embed web
var webFS embed.FS

// pipelineOrchestrator defines the methods the API needs from the pipeline orchestrator.
type pipelineOrchestrator interface {
	Run(ctx context.Context, req orchestrator.PipelineRequest) (orchestrator.PipelineReport, error)
	Cancel() error
	Status() orchestrator.RunStatus
	History() []orchestrator.PipelineReport
}

// Server wraps the HTTP API with dependencies.
type Server struct {
	orch        pipelineOrchestrator
	router      routing.Router
	policyRtr   routing.PolicyRouter
	cfgWatcher  *config.Watcher
	kvStore     config.KVStore
	dlProvider  domainlist.ListProvider
	sourceStore source.DLCStore
	cacheAgent  cache.Cache
	eventBus    *events.Bus
	registry    sourcereg.Registry
	version     string
	buildTime   string
}

// New creates an API server with dependencies.
func New(
	orch pipelineOrchestrator,
	router routing.Router,
	cfgWatcher *config.Watcher,
	kvStore config.KVStore,
	dlProvider domainlist.ListProvider,
	sourceStore source.DLCStore,
	cacheAgent cache.Cache,
	eventBus *events.Bus,
	registry sourcereg.Registry,
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
		registry:    registry,
	}
}

// SetVersion sets the build version metadata.
func (s *Server) SetVersion(version, buildTime string) {
	s.version = version
	s.buildTime = buildTime
}

// SetPolicyRouter sets the policy router for policy-aware endpoints.
func (s *Server) SetPolicyRouter(r routing.PolicyRouter) {
	s.policyRtr = r
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
		cr.Get("/api/version", s.handleVersion)
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

		// Sources (multi-source)
		cr.Get("/api/sources", s.handleSourcesList)
		cr.Post("/api/sources", s.handleSourceCreate)
		cr.Get("/api/sources/{id}", s.handleSourceGet)
		cr.Put("/api/sources/{id}", s.handleSourceUpdate)
		cr.Delete("/api/sources/{id}", s.handleSourceDelete)
		cr.Post("/api/sources/{id}/refresh", s.handleSourceRefresh)
		cr.Post("/api/sources/upload", s.handleSourceUpload)

		// Legacy source endpoints (redirect to registry)
		cr.Get("/api/source/info", s.handleSourcesList)
		cr.Post("/api/source/fetch", s.handleSourceFetchLegacy)

		// Export API.
		cr.Get("/api/export/download", s.handleExportDownload)

		// Config export/import.
		cr.Get("/api/config/export", s.handleConfigExport)
		cr.Post("/api/config/import", s.handleConfigImport)

		// Policies API.
		cr.Get("/api/policies", s.handlePoliciesList)
		cr.Get("/api/policies/{name}", s.handlePolicyGet)
		cr.Post("/api/policies", s.handlePolicyCreate)
		cr.Put("/api/policies/{name}", s.handlePolicyUpdate)
		cr.Delete("/api/policies/{name}", s.handlePolicyDelete)

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

// handleVersion returns the build version metadata.
func (s *Server) handleVersion(w http.ResponseWriter, r *http.Request) {
	s.jsonOK(w, map[string]string{
		"version":    s.version,
		"build_time": s.buildTime,
	})
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
// When policy routing is configured, returns aggregated policy state.
func (s *Server) handleRoutingSnapshot(w http.ResponseWriter, r *http.Request) {
	cfg := s.cfgWatcher.Current().Config

	// If policies are configured, return aggregated policy routing state.
	if len(cfg.Routing.Policies) > 0 && s.policyRtr != nil {
		var totalV4, totalV6 int
		var anyApplied bool
		var lastApplied time.Time
		for _, p := range cfg.Routing.Policies {
			if !p.Enabled {
				continue
			}
			state := s.policyRtr.SnapshotPolicy(p.Name)
			if state.Backend != "" && state.Backend != "none" {
				anyApplied = true
				if state.AppliedAt.After(lastApplied) {
					lastApplied = state.AppliedAt
				}
				totalV4 += len(state.V4)
				totalV6 += len(state.V6)
			}
		}
		backend := "policies"
		if !anyApplied {
			backend = "none"
		}
		appliedStr := ""
		if !lastApplied.IsZero() {
			appliedStr = lastApplied.Format(time.RFC3339)
		}
		s.jsonOK(w, map[string]interface{}{
			"backend":    backend,
			"applied_at": appliedStr,
			"v4":         totalV4,
			"v6":         totalV6,
			"policies":   len(cfg.Routing.Policies),
		})
		return
	}

	snapshot := s.router.Snapshot()
	resp := map[string]interface{}{
		"backend":    snapshot.Backend,
		"applied_at": snapshot.AppliedAt,
		"v4":         snapshot.V4,
		"v6":         snapshot.V6,
	}
	s.jsonOK(w, resp)
}

// handleExportDownload serves exported prefix files for download.
func (s *Server) handleExportDownload(w http.ResponseWriter, r *http.Request) {
	policy := r.URL.Query().Get("policy")
	typ := r.URL.Query().Get("type")
	if policy == "" || (typ != "ipv4" && typ != "ipv6") {
		s.jsonError(w, http.StatusBadRequest, "policy and type (ipv4|ipv6) query params required")
		return
	}

	cfg := s.cfgWatcher.Current().Config
	baseDir := cfg.Export.Dir
	if baseDir == "" {
		baseDir = "/var/lib/d2ip/out"
	}

	filename := cfg.Export.IPv4File
	if typ == "ipv6" {
		filename = cfg.Export.IPv6File
	}
	if filename == "" {
		filename = typ + ".txt"
	}

	path := baseDir + "/" + policy + "/" + filename
	data, err := os.ReadFile(path)
	if err != nil {
		s.jsonError(w, http.StatusNotFound, "export file not found: "+err.Error())
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s_%s.txt", policy, typ))
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

// handleConfigExport returns the current effective config as JSON download.
func (s *Server) handleConfigExport(w http.ResponseWriter, r *http.Request) {
	snapshot := s.cfgWatcher.Current()
	overrides, _ := s.kvStore.GetAll(r.Context())

	resp := map[string]interface{}{
		"config":    structToMap(snapshot.Config),
		"overrides": overrides,
	}

	data, err := json.MarshalIndent(resp, "", "  ")
	if err != nil {
		s.jsonError(w, http.StatusInternalServerError, "failed to marshal config: "+err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", "attachment; filename=d2ip-config.json")
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

// handleConfigImport applies a JSON config object as KV overrides.
func (s *Server) handleConfigImport(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Overrides map[string]string `json:"overrides"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		s.jsonError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	if s.kvStore == nil {
		s.jsonError(w, http.StatusInternalServerError, "kvStore not initialized")
		return
	}

	for key, value := range payload.Overrides {
		if err := s.kvStore.Set(r.Context(), key, value); err != nil {
			log.Error().Err(err).Str("key", key).Msg("api: failed to import config override")
			s.jsonError(w, http.StatusInternalServerError, "failed to set "+key+": "+err.Error())
			return
		}
	}

	if err := s.reloadConfig(r.Context()); err != nil {
		log.Error().Err(err).Msg("api: config reload failed after import")
		s.jsonError(w, http.StatusInternalServerError, "config reload failed: "+err.Error())
		return
	}

	s.jsonOK(w, map[string]string{"status": "ok", "message": "config imported"})
}
