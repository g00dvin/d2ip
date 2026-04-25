package domainlist

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/goodvin/d2ip/internal/domainlist/dlcpb"
	"github.com/rs/zerolog/log"
	"google.golang.org/protobuf/proto"
)

// Provider is the concrete implementation of ListProvider.
type Provider struct {
	categories map[string]*dlcpb.GeoSite // Keyed by lowercase country_code
}

// NewProvider creates an empty domain list provider.
func NewProvider() *Provider {
	return &Provider{
		categories: make(map[string]*dlcpb.GeoSite),
	}
}

// Load parses the dlc.dat file and builds the in-memory category map.
func (p *Provider) Load(dlcPath string) error {
	data, err := os.ReadFile(dlcPath)
	if err != nil {
		return fmt.Errorf("domainlist: read file: %w", err)
	}

	var list dlcpb.GeoSiteList
	if err := proto.Unmarshal(data, &list); err != nil {
		return fmt.Errorf("domainlist: unmarshal protobuf: %w", err)
	}

	// Build category map keyed by lowercase country_code.
	p.categories = make(map[string]*dlcpb.GeoSite, len(list.Entry))
	for _, site := range list.Entry {
		key := strings.ToLower(site.CountryCode)
		p.categories[key] = site
	}

	log.Info().
		Int("categories", len(p.categories)).
		Str("path", dlcPath).
		Msg("domainlist: loaded")

	return nil
}

// Categories returns all discovered category codes in sorted order.
func (p *Provider) Categories() []string {
	codes := make([]string, 0, len(p.categories))
	for code := range p.categories {
		codes = append(codes, code)
	}
	sort.Strings(codes)
	return codes
}

// Select extracts rules matching the given selectors.
func (p *Provider) Select(selectors []CategorySelector) ([]Rule, error) {
	var allRules []Rule

	for _, sel := range selectors {
		// Normalize category code: "geosite:ru" → "ru"
		code := strings.ToLower(sel.Code)
		code = strings.TrimPrefix(code, "geosite:")

		site, ok := p.categories[code]
		if !ok {
			return nil, fmt.Errorf("domainlist: unknown category %q", sel.Code)
		}

		// Extract domains matching attribute filter.
		for _, domain := range site.Domain {
			if !matchesAttrs(domain, sel.Attrs) {
				continue
			}

			rule := convertDomain(domain, code)
			allRules = append(allRules, rule)
		}
	}

	// Deduplicate by (Type, Value).
	allRules = deduplicate(allRules)

	log.Debug().
		Int("selectors", len(selectors)).
		Int("rules", len(allRules)).
		Msg("domainlist: selected")

	return allRules, nil
}

// matchesAttrs checks if a domain has all required attributes (AND semantics).
func matchesAttrs(domain *dlcpb.Domain, requiredAttrs []string) bool {
	if len(requiredAttrs) == 0 {
		return true // No filter → pass all
	}

	// Build attribute map for this domain.
	attrs := make(map[string]any)
	for _, attr := range domain.Attribute {
		switch v := attr.TypedValue.(type) {
		case *dlcpb.Domain_Attribute_BoolValue:
			attrs[attr.Key] = v.BoolValue
		case *dlcpb.Domain_Attribute_IntValue:
			attrs[attr.Key] = v.IntValue
		}
	}

	// Check all required attrs are present and truthy.
	for _, key := range requiredAttrs {
		val, ok := attrs[key]
		if !ok {
			return false
		}

		// Truthy check: bool=true or int!=0
		switch v := val.(type) {
		case bool:
			if !v {
				return false
			}
		case int64:
			if v == 0 {
				return false
			}
		default:
			return false
		}
	}

	return true
}

// convertDomain transforms a protobuf Domain into a Rule.
func convertDomain(domain *dlcpb.Domain, category string) Rule {
	var ruleType RuleType
	switch domain.Type {
	case dlcpb.Domain_Full:
		ruleType = RuleFull
	case dlcpb.Domain_RootDomain:
		ruleType = RuleRootDomain
	case dlcpb.Domain_Plain:
		ruleType = RulePlain
	case dlcpb.Domain_Regex:
		ruleType = RuleRegex
	default:
		ruleType = RuleFull // Fallback
	}

	// Build attribute map.
	attrs := make(map[string]any)
	for _, attr := range domain.Attribute {
		switch v := attr.TypedValue.(type) {
		case *dlcpb.Domain_Attribute_BoolValue:
			attrs[attr.Key] = v.BoolValue
		case *dlcpb.Domain_Attribute_IntValue:
			attrs[attr.Key] = v.IntValue
		}
	}

	rule := Rule{
		Type:  ruleType,
		Value: domain.Value,
		Attrs: attrs,
		Cat:   category,
	}

	// Normalize Full and RootDomain values.
	if ruleType == RuleFull || ruleType == RuleRootDomain {
		rule.Value = normalizeDomain(rule.Value)
	}

	return rule
}

// deduplicate removes duplicate rules based on (Type, Value).
func deduplicate(rules []Rule) []Rule {
	seen := make(map[string]bool)
	var unique []Rule

	for _, rule := range rules {
		key := fmt.Sprintf("%d:%s", rule.Type, rule.Value)
		if seen[key] {
			continue
		}
		seen[key] = true
		unique = append(unique, rule)
	}

	return unique
}
