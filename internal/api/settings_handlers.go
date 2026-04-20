package api

import (
	"encoding/json"
	"net/http"

	"github.com/goodvin/d2ip/internal/config"
)

// settingsResponse is the JSON response for GET /api/settings.
type settingsResponse struct {
	Config    map[string]interface{} `json:"config"`
	Defaults  map[string]interface{} `json:"defaults"`
	Overrides map[string]string      `json:"overrides"`
}

// handleSettingsGet returns the current config, defaults, and KV overrides.
func (s *Server) handleSettingsGet(w http.ResponseWriter, r *http.Request) {
	snapshot := s.cfgWatcher.Current()
	defaults := config.Defaults()

	overrides, err := s.kvStore.GetAll(r.Context())
	if err != nil {
		s.jsonError(w, http.StatusInternalServerError, "failed to read overrides: "+err.Error())
		return
	}

	resp := settingsResponse{
		Config:    structToMap(snapshot.Config),
		Defaults:  structToMap(defaults),
		Overrides: overrides,
	}
	s.jsonOK(w, resp)
}

// structToMap converts a struct to a map via JSON round-trip.
func structToMap(v interface{}) map[string]interface{} {
	data, err := json.Marshal(v)
	if err != nil {
		return map[string]interface{}{}
	}
	var m map[string]interface{}
	_ = json.Unmarshal(data, &m)
	return m
}
