package api

import (
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
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

	type catInfo struct {
		Code        string `json:"code"`
		DomainCount int    `json:"domain_count"`
	}
	var configured []catInfo

	if s.cfgWatcher != nil {
		snapshot := s.cfgWatcher.Current()
		cfg := snapshot.Config.Clone()
		seen := make(map[string]struct{})
		for _, pol := range cfg.Routing.Policies {
			if !pol.Enabled {
				continue
			}
			for _, cat := range pol.Categories {
				if _, ok := seen[cat]; ok {
					continue
				}
				seen[cat] = struct{}{}
				domains, err := s.registry.GetDomains(cat)
				count := 0
				if err == nil {
					count = len(domains)
				}
				configured = append(configured, catInfo{
					Code:        cat,
					DomainCount: count,
				})
			}
		}
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
// Deprecated: categories are now managed via routing policies.
func (s *Server) handleCategoriesAdd(w http.ResponseWriter, r *http.Request) {
	s.jsonError(w, http.StatusNotFound, "POST /api/categories is no longer supported; categories are managed via routing policies")
}

// handleCategoriesDelete removes a category from the config.
// Deprecated: categories are now managed via routing policies.
func (s *Server) handleCategoriesDelete(w http.ResponseWriter, r *http.Request) {
	s.jsonError(w, http.StatusNotFound, "DELETE /api/categories/{code} is no longer supported; categories are managed via routing policies")
}
