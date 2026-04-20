package api

import "net/http"

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
		s.jsonError(w, http.StatusConflict, err.Error())
		return
	}
	s.jsonOK(w, map[string]string{"status": "cancelled"})
}
