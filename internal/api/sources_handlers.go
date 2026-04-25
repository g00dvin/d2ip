package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/goodvin/d2ip/internal/sourcereg"
	"github.com/rs/zerolog/log"
)

// handleSourcesList returns all configured sources.
func (s *Server) handleSourcesList(w http.ResponseWriter, r *http.Request) {
	if s.registry == nil {
		s.jsonOK(w, map[string]interface{}{"sources": []any{}})
		return
	}
	sources := s.registry.ListSources()
	s.jsonOK(w, map[string]interface{}{"sources": sources})
}

// handleSourceGet returns a single source.
func (s *Server) handleSourceGet(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		s.jsonError(w, http.StatusBadRequest, "source id is required")
		return
	}
	if s.registry == nil {
		s.jsonError(w, http.StatusServiceUnavailable, "registry unavailable")
		return
	}
	src, ok := s.registry.GetSource(id)
	if !ok {
		s.jsonError(w, http.StatusNotFound, "source not found")
		return
	}
	s.jsonOK(w, src.Info())
}

// handleSourceCreate creates a new source.
func (s *Server) handleSourceCreate(w http.ResponseWriter, r *http.Request) {
	if s.registry == nil {
		s.jsonError(w, http.StatusServiceUnavailable, "registry unavailable")
		return
	}
	var cfg sourcereg.SourceConfig
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		s.jsonError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if cfg.ID == "" {
		s.jsonError(w, http.StatusBadRequest, "id is required")
		return
	}
	if cfg.Prefix == "" {
		s.jsonError(w, http.StatusBadRequest, "prefix is required")
		return
	}
	if cfg.Provider == "" {
		s.jsonError(w, http.StatusBadRequest, "provider is required")
		return
	}

	if err := s.registry.AddSource(r.Context(), cfg); err != nil {
		log.Error().Err(err).Str("id", cfg.ID).Msg("api: failed to add source")
		s.jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.jsonOK(w, map[string]string{"status": "ok"})
}

// handleSourceUpdate updates an existing source.
func (s *Server) handleSourceUpdate(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		s.jsonError(w, http.StatusBadRequest, "source id is required")
		return
	}
	if s.registry == nil {
		s.jsonError(w, http.StatusServiceUnavailable, "registry unavailable")
		return
	}
	var cfg sourcereg.SourceConfig
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		s.jsonError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	cfg.ID = id // ensure ID matches URL

	if err := s.registry.AddSource(r.Context(), cfg); err != nil {
		log.Error().Err(err).Str("id", id).Msg("api: failed to update source")
		s.jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.jsonOK(w, map[string]string{"status": "ok"})
}

// handleSourceDelete deletes a source.
func (s *Server) handleSourceDelete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		s.jsonError(w, http.StatusBadRequest, "source id is required")
		return
	}
	if s.registry == nil {
		s.jsonError(w, http.StatusServiceUnavailable, "registry unavailable")
		return
	}
	if err := s.registry.RemoveSource(r.Context(), id); err != nil {
		log.Error().Err(err).Str("id", id).Msg("api: failed to delete source")
		s.jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.jsonOK(w, map[string]string{"status": "ok"})
}

// handleSourceRefresh triggers a manual reload of a source.
func (s *Server) handleSourceRefresh(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		s.jsonError(w, http.StatusBadRequest, "source id is required")
		return
	}
	if s.registry == nil {
		s.jsonError(w, http.StatusServiceUnavailable, "registry unavailable")
		return
	}
	src, ok := s.registry.GetSource(id)
	if !ok {
		s.jsonError(w, http.StatusNotFound, "source not found")
		return
	}
	if err := src.Load(r.Context()); err != nil {
		log.Error().Err(err).Str("id", id).Msg("api: source refresh failed")
		s.jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.jsonOK(w, map[string]interface{}{
		"status": "ok",
		"info":   src.Info(),
	})
}

// handleSourceFetchLegacy triggers a full reload of all sources (legacy endpoint).
func (s *Server) handleSourceFetchLegacy(w http.ResponseWriter, r *http.Request) {
	if s.registry == nil {
		s.jsonError(w, http.StatusServiceUnavailable, "registry unavailable")
		return
	}
	if err := s.registry.LoadAll(r.Context()); err != nil {
		s.jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.jsonOK(w, map[string]string{"status": "ok"})
}

// handleSourceUpload accepts a plaintext file upload and returns the saved path.
func (s *Server) handleSourceUpload(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		s.jsonError(w, http.StatusBadRequest, fmt.Sprintf("parse form: %v", err))
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		s.jsonError(w, http.StatusBadRequest, fmt.Sprintf("get file: %v", err))
		return
	}
	defer file.Close()

	if !strings.HasSuffix(strings.ToLower(header.Filename), ".txt") {
		s.jsonError(w, http.StatusBadRequest, "only .txt files allowed")
		return
	}

	dir := "/tmp/d2ip-uploads"
	if err := os.MkdirAll(dir, 0755); err != nil {
		s.jsonError(w, http.StatusInternalServerError, fmt.Sprintf("mkdir: %v", err))
		return
	}

	path := filepath.Join(dir, fmt.Sprintf("%s.txt", uuid.NewString()))
	f, err := os.Create(path)
	if err != nil {
		s.jsonError(w, http.StatusInternalServerError, fmt.Sprintf("create file: %v", err))
		return
	}
	defer f.Close()

	if _, err := io.Copy(f, file); err != nil {
		os.Remove(path)
		s.jsonError(w, http.StatusInternalServerError, fmt.Sprintf("copy: %v", err))
		return
	}

	s.jsonOK(w, map[string]string{"path": path})
}
