package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"

	"github.com/go-chi/chi/v5"
	"github.com/goodvin/d2ip/internal/config"
	"github.com/goodvin/d2ip/internal/domainlist"
)

// categoryInfo represents a category with its domain count.
type categoryInfo struct {
	Code        string   `json:"code"`
	Attrs       []string `json:"attrs,omitempty"`
	DomainCount int      `json:"domain_count"`
}

// handleCategoriesList returns configured and available geosite categories.
func (s *Server) handleCategoriesList(w http.ResponseWriter, r *http.Request) {
	snapshot := s.cfgWatcher.Current()

	// Build configured categories map
	configuredSet := make(map[string]categoryInfo)
	for _, cat := range snapshot.Config.Categories {
		configuredSet[cat.Code] = categoryInfo{
			Code:  cat.Code,
			Attrs: cat.Attrs,
		}
	}

	// Enrich with domain counts from provider
	if s.dlProvider != nil {
		for code := range configuredSet {
			rules, err := s.dlProvider.Select([]domainlist.CategorySelector{{Code: code}})
			if err == nil {
				info := configuredSet[code]
				info.DomainCount = len(rules)
				configuredSet[code] = info
			}
		}
	}

	// Build configured slice (sorted)
	configured := make([]categoryInfo, 0, len(configuredSet))
	for _, c := range configuredSet {
		configured = append(configured, c)
	}
	sort.Slice(configured, func(i, j int) bool {
		return configured[i].Code < configured[j].Code
	})

	// Build available slice (all provider categories minus configured)
	available := []string{}
	if s.dlProvider != nil {
		allCats := s.dlProvider.Categories()
		for _, code := range allCats {
			if _, isConfigured := configuredSet[code]; !isConfigured {
				available = append(available, code)
			}
		}
	}
	sort.Strings(available)

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

	if s.dlProvider == nil {
		s.jsonError(w, http.StatusServiceUnavailable, "domain list provider unavailable")
		return
	}

	rules, err := s.dlProvider.Select([]domainlist.CategorySelector{{Code: code}})
	if err != nil {
		s.jsonError(w, http.StatusNotFound, "category not found: "+code)
		return
	}

	// Extract domain values from rules
	domains := make([]string, 0, len(rules))
	for _, rule := range rules {
		if rule.Value != "" {
			domains = append(domains, rule.Value)
		}
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
	if start >= len(domains) {
		domains = []string{}
	} else if end > len(domains) {
		domains = domains[start:]
	} else {
		domains = domains[start:end]
	}

	s.jsonOK(w, map[string]interface{}{
		"code":     code,
		"domains":  domains,
		"page":     page,
		"per_page": perPage,
		"total":    len(rules),
		"has_more": end < len(rules),
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

	snapshot := s.cfgWatcher.Current()
	cfg := snapshot.Config.Clone()

	// Check for duplicate
	for _, cat := range cfg.Categories {
		if cat.Code == req.Code {
			s.jsonError(w, http.StatusConflict, "category already exists: "+req.Code)
			return
		}
	}

	cfg.Categories = append(cfg.Categories, config.CategoryConfig{
		Code:  req.Code,
		Attrs: req.Attrs,
	})

	if err := s.cfgWatcher.Publish(cfg); err != nil {
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

	snapshot := s.cfgWatcher.Current()
	cfg := snapshot.Config.Clone()

	found := false
	for i, cat := range cfg.Categories {
		if cat.Code == code {
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
		s.jsonError(w, http.StatusInternalServerError, "failed to update config: "+err.Error())
		return
	}

	s.jsonOK(w, map[string]string{"status": "ok"})
}
