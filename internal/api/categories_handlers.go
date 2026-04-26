package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/goodvin/d2ip/internal/config"
	"github.com/rs/zerolog/log"
)

// handleCategoriesList returns configured and available geosite categories.
func (s *Server) handleCategoriesList(w http.ResponseWriter, r *http.Request) {
	if s.registry == nil {
		s.jsonOK(w, map[string]interface{}{
			"configured": []any{},
			"available":  []any{},
		})
		return
	}

	cats := s.registry.ListCategories()
	var available []string
	for _, c := range cats {
		available = append(available, c.Name)
	}

	snapshot := s.cfgWatcher.Current()
	cfg := snapshot.Config.Clone()
	type catInfo struct {
		Code         string `json:"code"`
		DomainCount  int    `json:"domain_count"`
	}
	var configured []catInfo
	for _, c := range cfg.Categories {
		domains, err := s.registry.GetDomains(c.Code)
		count := 0
		if err == nil {
			count = len(domains)
		}
		configured = append(configured, catInfo{
			Code:        c.Code,
			DomainCount: count,
		})
	}

	s.jsonOK(w, map[string]interface{}{
		"configured": configured,
		"available":  available,
	})
}

// handleCategoryDomains returns domains for a specific category.
func (s *Server) handleCategoryDomains(w http.ResponseWriter, r *http.Request) {
	code := chi.URLParam(r, "code")
	if code == "" {
		s.jsonError(w, http.StatusBadRequest, "category code is required")
		return
	}
	if s.registry == nil {
		s.jsonError(w, http.StatusServiceUnavailable, "registry unavailable")
		return
	}

	domains, err := s.registry.GetDomains(code)
	if err != nil {
		// Try prefixes
		prefixes, err2 := s.registry.GetPrefixes(code)
		if err2 == nil {
			// Return prefixes as "domains" for display
			prefixStrs := make([]string, len(prefixes))
			for i, p := range prefixes {
				prefixStrs[i] = p.String()
			}
			s.jsonOK(w, map[string]interface{}{
				"code":     code,
				"domains":  prefixStrs,
				"page":     1,
				"per_page": len(prefixes),
				"total":    len(prefixes),
				"has_more": false,
			})
			return
		}
		s.jsonError(w, http.StatusNotFound, "category not found: "+code)
		return
	}

	// Pagination
	page := 1
	perPage := 100
	if p := r.URL.Query().Get("page"); p != "" {
		if _, err := fmt.Sscanf(p, "%d", &page); err == nil && page < 1 {
			page = 1
		}
	}
	if pp := r.URL.Query().Get("per_page"); pp != "" {
		var n int
		if _, err := fmt.Sscanf(pp, "%d", &n); err == nil && n > 0 && n <= 500 {
			perPage = n
		}
	}

	start := (page - 1) * perPage
	end := start + perPage
	var pageDomains []string
	if start < len(domains) {
		if end > len(domains) {
			pageDomains = domains[start:]
		} else {
			pageDomains = domains[start:end]
		}
	}

	s.jsonOK(w, map[string]interface{}{
		"code":     code,
		"domains":  pageDomains,
		"page":     page,
		"per_page": perPage,
		"total":    len(domains),
		"has_more": end < len(domains),
	})
}

// handleCategoriesAdd adds a new category to the config.
func (s *Server) handleCategoriesAdd(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Code  string   `json:"code"`
		Attrs []string `json:"attrs,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if req.Code == "" {
		s.jsonError(w, http.StatusBadRequest, "code is required")
		return
	}

	// Normalize: if code has no prefix (no colon), add "geosite:" prefix.
	// If code already has a colon (e.g. "ipverse:cn"), use it as-is.
	code := req.Code
	if !strings.Contains(code, ":") {
		code = "geosite:" + code
	}

	snapshot := s.cfgWatcher.Current()
	cfg := snapshot.Config.Clone()

	// Check for duplicate (case-insensitive)
	for _, cat := range cfg.Categories {
		if strings.EqualFold(cat.Code, code) {
			s.jsonError(w, http.StatusConflict, "category already exists: "+code)
			return
		}
	}

	cfg.Categories = append(cfg.Categories, config.CategoryConfig{
		Code:  code,
		Attrs: req.Attrs,
	})

	if err := s.cfgWatcher.Publish(cfg); err != nil {
		log.Error().Err(err).Str("code", req.Code).Msg("api: failed to add category")
		s.jsonError(w, http.StatusInternalServerError, "failed to update config: "+err.Error())
		return
	}

	s.jsonOK(w, map[string]string{"status": "ok"})
}

// handleCategoriesDelete removes a category from the config.
func (s *Server) handleCategoriesDelete(w http.ResponseWriter, r *http.Request) {
	code := chi.URLParam(r, "code")
	if code == "" {
		s.jsonError(w, http.StatusBadRequest, "code is required")
		return
	}

	// Normalize: if code has no prefix (no colon), add "geosite:" prefix.
	// If code already has a colon (e.g. "ipverse:cn"), use it as-is.
	if !strings.Contains(code, ":") {
		code = "geosite:" + code
	}

	snapshot := s.cfgWatcher.Current()
	cfg := snapshot.Config.Clone()

	found := false
	for i, cat := range cfg.Categories {
		if strings.EqualFold(cat.Code, code) {
			cfg.Categories = append(cfg.Categories[:i], cfg.Categories[i+1:]...)
			found = true
			break
		}
	}

	if !found {
		s.jsonError(w, http.StatusNotFound, "category not found: "+code)
		return
	}

	if err := s.cfgWatcher.Publish(cfg); err != nil {
		log.Error().Err(err).Str("code", code).Msg("api: failed to delete category")
		s.jsonError(w, http.StatusInternalServerError, "failed to update config: "+err.Error())
		return
	}

	s.jsonOK(w, map[string]string{"status": "ok"})
}
