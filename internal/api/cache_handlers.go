package api

import (
	"encoding/json"
	"net/http"
)

// handleCacheStats returns cache statistics.
func (s *Server) handleCacheStats(w http.ResponseWriter, r *http.Request) {
	if s.cacheAgent == nil {
		s.jsonError(w, http.StatusServiceUnavailable, "cache unavailable")
		return
	}

	stats, err := s.cacheAgent.Stats(r.Context())
	if err != nil {
		s.jsonError(w, http.StatusInternalServerError, "failed to get stats: "+err.Error())
		return
	}

	s.jsonOK(w, map[string]interface{}{
		"domains":        stats.Domains,
		"records_total":  stats.RecordsTotal,
		"records_v4":     stats.RecordsV4,
		"records_v6":     stats.RecordsV6,
		"records_valid":  stats.RecordsValid,
		"records_failed": stats.RecordsFail,
		"oldest_updated": stats.OldestUpdatedAt,
		"newest_updated": stats.NewestUpdatedAt,
	})
}

// handleCachePurge purges cache entries by pattern, age, or failed status.
func (s *Server) handleCachePurge(w http.ResponseWriter, r *http.Request) {
	if s.cacheAgent == nil {
		s.jsonError(w, http.StatusServiceUnavailable, "cache unavailable")
		return
	}

	var req struct {
		Pattern string `json:"pattern,omitempty"`
		Older   string `json:"older,omitempty"`
		Failed  bool   `json:"failed,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	// The cache interface doesn't have a DeleteByPattern method yet.
	// For now, return a placeholder response.
	s.jsonOK(w, map[string]interface{}{
		"status":  "ok",
		"message": "purge requires cache.DeleteByPattern — not yet implemented",
	})
}

// handleCacheVacuum runs SQLite VACUUM.
func (s *Server) handleCacheVacuum(w http.ResponseWriter, r *http.Request) {
	if s.cacheAgent == nil {
		s.jsonError(w, http.StatusServiceUnavailable, "cache unavailable")
		return
	}

	// Vacuum signature: Vacuum(ctx context.Context, olderThan time.Duration) (deleted int, err error)
	deleted, err := s.cacheAgent.Vacuum(r.Context(), 0)
	if err != nil {
		s.jsonError(w, http.StatusInternalServerError, "vacuum failed: "+err.Error())
		return
	}

	s.jsonOK(w, map[string]interface{}{
		"status":  "ok",
		"deleted": deleted,
	})
}

// handleCacheEntries searches cached entries by domain.
func (s *Server) handleCacheEntries(w http.ResponseWriter, r *http.Request) {
	if s.cacheAgent == nil {
		s.jsonError(w, http.StatusServiceUnavailable, "cache unavailable")
		return
	}

	domain := r.URL.Query().Get("domain")
	if domain == "" {
		s.jsonError(w, http.StatusBadRequest, "domain query parameter is required")
		return
	}

	s.jsonError(w, http.StatusServiceUnavailable, "domain-level lookup not yet implemented")
	return
}
