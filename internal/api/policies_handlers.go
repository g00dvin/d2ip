package api

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/goodvin/d2ip/internal/config"
)

func (s *Server) handlePoliciesList(w http.ResponseWriter, r *http.Request) {
	cfg := s.cfgWatcher.Current()
	s.jsonOK(w, map[string]interface{}{"policies": cfg.Routing.Policies})
}

func (s *Server) handlePolicyGet(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	cfg := s.cfgWatcher.Current()
	for _, p := range cfg.Routing.Policies {
		if p.Name == name {
			s.jsonOK(w, p)
			return
		}
	}
	s.jsonError(w, http.StatusNotFound, "policy not found: "+name)
}

func (s *Server) handlePolicyCreate(w http.ResponseWriter, r *http.Request) {
	var policy config.PolicyConfig
	if err := json.NewDecoder(r.Body).Decode(&policy); err != nil {
		s.jsonError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	// TODO: persist to config / kv store
	s.jsonOK(w, map[string]string{"status": "ok", "message": "policy creation requires config persistence — not yet implemented"})
}

func (s *Server) handlePolicyUpdate(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	var policy config.PolicyConfig
	if err := json.NewDecoder(r.Body).Decode(&policy); err != nil {
		s.jsonError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if policy.Name != name {
		s.jsonError(w, http.StatusBadRequest, "name mismatch")
		return
	}
	// TODO: persist
	s.jsonOK(w, map[string]string{"status": "ok", "message": "policy update requires config persistence — not yet implemented"})
}

func (s *Server) handlePolicyDelete(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	_ = name
	// TODO: remove from config
	s.jsonOK(w, map[string]string{"status": "ok", "message": "policy deletion requires config persistence — not yet implemented"})
}
