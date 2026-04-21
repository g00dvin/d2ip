package api

import (
	"net/http"

	"github.com/goodvin/d2ip/internal/orchestrator"
)

// handlePipelineHistory returns the last 10 pipeline runs.
func (s *Server) handlePipelineHistory(w http.ResponseWriter, r *http.Request) {
	if s.orch == nil {
		s.jsonOK(w, map[string]interface{}{
			"history": []interface{}{},
		})
		return
	}
	history := s.orch.History()
	s.jsonOK(w, map[string]interface{}{
		"history": history,
	})
}

// handlePipelineCancel requests cancellation of the running pipeline.
func (s *Server) handlePipelineCancel(w http.ResponseWriter, r *http.Request) {
	if s.orch == nil {
		s.jsonError(w, http.StatusServiceUnavailable, "orchestrator unavailable")
		return
	}
	if err := s.orch.Cancel(); err != nil {
		// Pipeline already finished — treat as success (idempotent).
		if err == orchestrator.ErrNotRunning {
			s.jsonOK(w, map[string]string{"status": "not running"})
			return
		}
		s.jsonError(w, http.StatusConflict, err.Error())
		return
	}
	s.jsonOK(w, map[string]string{"status": "cancelled"})
}
