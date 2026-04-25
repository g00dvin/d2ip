package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/goodvin/d2ip/internal/config"
	"github.com/rs/zerolog/log"
)

func (s *Server) handlePoliciesList(w http.ResponseWriter, r *http.Request) {
	cfg := s.cfgWatcher.Current()
	policies := cfg.Config.Routing.Policies
	if policies == nil {
		policies = []config.PolicyConfig{}
	}
	log.Debug().Int("count", len(policies)).Msg("api: listing policies")
	s.jsonOK(w, map[string]interface{}{"policies": policies})
}

func (s *Server) handlePolicyGet(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	cfg := s.cfgWatcher.Current()
	for _, p := range cfg.Config.Routing.Policies {
		if p.Name == name {
			log.Debug().Str("name", name).Msg("api: got policy")
			s.jsonOK(w, p)
			return
		}
	}
	log.Warn().Str("name", name).Msg("api: policy not found")
	s.jsonError(w, http.StatusNotFound, "policy not found: "+name)
}

// persistPolicies writes the current routing policies to kvStore for durability.
func (s *Server) persistPolicies(ctx context.Context, cfg config.Config) error {
	if s.kvStore == nil {
		return nil
	}
	data, err := json.Marshal(cfg.Routing.Policies)
	if err != nil {
		return fmt.Errorf("marshal policies: %w", err)
	}
	return s.kvStore.Set(ctx, "routing.policies", string(data))
}

func (s *Server) handlePolicyCreate(w http.ResponseWriter, r *http.Request) {
	var policy config.PolicyConfig
	if err := json.NewDecoder(r.Body).Decode(&policy); err != nil {
		log.Warn().Err(err).Msg("api: invalid policy JSON")
		s.jsonError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	snapshot := s.cfgWatcher.Current()
	cfg := snapshot.Config.Clone()

	// Check for duplicate name
	for _, p := range cfg.Routing.Policies {
		if strings.EqualFold(p.Name, policy.Name) {
			log.Warn().Str("name", policy.Name).Msg("api: policy already exists")
			s.jsonError(w, http.StatusConflict, "policy already exists: "+policy.Name)
			return
		}
	}

	cfg.Routing.Policies = append(cfg.Routing.Policies, policy)

	if err := s.cfgWatcher.Publish(cfg); err != nil {
		log.Error().Err(err).Str("name", policy.Name).Msg("api: failed to create policy")
		s.jsonError(w, http.StatusInternalServerError, "failed to update config: "+err.Error())
		return
	}
	if err := s.persistPolicies(r.Context(), cfg); err != nil {
		log.Error().Err(err).Str("name", policy.Name).Msg("api: failed to persist policies to KV")
	}

	log.Info().Str("name", policy.Name).Str("backend", string(policy.Backend)).Int("categories", len(policy.Categories)).Msg("api: policy created")
	s.jsonOK(w, map[string]string{"status": "ok"})
}

func (s *Server) handlePolicyUpdate(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	var policy config.PolicyConfig
	if err := json.NewDecoder(r.Body).Decode(&policy); err != nil {
		log.Warn().Err(err).Msg("api: invalid policy JSON")
		s.jsonError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if policy.Name != name {
		log.Warn().Str("url_name", name).Str("body_name", policy.Name).Msg("api: policy name mismatch")
		s.jsonError(w, http.StatusBadRequest, "name mismatch")
		return
	}

	snapshot := s.cfgWatcher.Current()
	cfg := snapshot.Config.Clone()

	found := false
	for i, p := range cfg.Routing.Policies {
		if p.Name == name {
			cfg.Routing.Policies[i] = policy
			found = true
			break
		}
	}

	if !found {
		log.Warn().Str("name", name).Msg("api: policy not found for update")
		s.jsonError(w, http.StatusNotFound, "policy not found: "+name)
		return
	}

	if err := s.cfgWatcher.Publish(cfg); err != nil {
		log.Error().Err(err).Str("name", name).Msg("api: failed to update policy")
		s.jsonError(w, http.StatusInternalServerError, "failed to update config: "+err.Error())
		return
	}
	if err := s.persistPolicies(r.Context(), cfg); err != nil {
		log.Error().Err(err).Str("name", name).Msg("api: failed to persist policies to KV")
	}

	log.Info().Str("name", name).Str("backend", string(policy.Backend)).Int("categories", len(policy.Categories)).Msg("api: policy updated")
	s.jsonOK(w, map[string]string{"status": "ok"})
}

func (s *Server) handlePolicyDelete(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")

	snapshot := s.cfgWatcher.Current()
	cfg := snapshot.Config.Clone()

	found := false
	for i, p := range cfg.Routing.Policies {
		if p.Name == name {
			cfg.Routing.Policies = append(cfg.Routing.Policies[:i], cfg.Routing.Policies[i+1:]...)
			found = true
			break
		}
	}

	if !found {
		log.Warn().Str("name", name).Msg("api: policy not found for delete")
		s.jsonError(w, http.StatusNotFound, "policy not found: "+name)
		return
	}

	if err := s.cfgWatcher.Publish(cfg); err != nil {
		log.Error().Err(err).Str("name", name).Msg("api: failed to delete policy")
		s.jsonError(w, http.StatusInternalServerError, "failed to update config: "+err.Error())
		return
	}
	if err := s.persistPolicies(r.Context(), cfg); err != nil {
		log.Error().Err(err).Str("name", name).Msg("api: failed to persist policies to KV")
	}

	log.Info().Str("name", name).Msg("api: policy deleted")
	s.jsonOK(w, map[string]string{"status": "ok"})
}
