package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/goodvin/d2ip/internal/config"
	"github.com/rs/zerolog/log"
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

// handleSettingsPut updates a config override via KVStore.
func (s *Server) handleSettingsPut(w http.ResponseWriter, r *http.Request) {
	var overrides map[string]string
	if err := json.NewDecoder(r.Body).Decode(&overrides); err != nil {
		s.jsonError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	if s.kvStore == nil {
		s.jsonError(w, http.StatusInternalServerError, "kvStore not initialized")
		return
	}

	for key, value := range overrides {
		if err := s.kvStore.Set(r.Context(), key, value); err != nil {
			log.Error().Err(err).Str("key", key).Msg("api: failed to set kv override")
			s.jsonError(w, http.StatusInternalServerError, "failed to set "+key+": "+err.Error())
			return
		}
	}

	if err := s.reloadConfig(r.Context()); err != nil {
		log.Error().Err(err).Msg("api: config reload failed")
		s.jsonError(w, http.StatusInternalServerError, "config reload failed: "+err.Error())
		return
	}

	s.jsonOK(w, map[string]string{"status": "ok"})
}

// handleSettingsDelete removes a config override.
func (s *Server) handleSettingsDelete(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "key")
	if key == "" {
		s.jsonError(w, http.StatusBadRequest, "key is required")
		return
	}

	if s.kvStore == nil {
		s.jsonError(w, http.StatusInternalServerError, "kvStore not initialized")
		return
	}

	if err := s.kvStore.Delete(r.Context(), key); err != nil {
		s.jsonError(w, http.StatusInternalServerError, "failed to delete "+key+": "+err.Error())
		return
	}

	if err := s.reloadConfig(r.Context()); err != nil {
		s.jsonError(w, http.StatusInternalServerError, "config reload failed: "+err.Error())
		return
	}

	s.jsonOK(w, map[string]string{"status": "ok"})
}

// reloadConfig fetches current config, applies KV overrides, and publishes.
func (s *Server) reloadConfig(ctx context.Context) error {
	if s.kvStore == nil {
		return errors.New("kvStore not initialized")
	}

	overrides, err := s.kvStore.GetAll(ctx)
	if err != nil {
		return err
	}

	// Start from current config (preserves YAML/ENV values), then apply KV overrides.
	snapshot := s.cfgWatcher.Current()
	cfg := snapshot.Config.Clone()

	if err := config.ApplyOverrides(&cfg, overrides); err != nil {
		return err
	}

	return s.cfgWatcher.Publish(cfg)
}
