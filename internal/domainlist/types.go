// Package domainlist parses dlc.dat (domain-list-community protobuf) and
// provides category selection with attribute filtering.
package domainlist

import (
	"fmt"
)

// RuleType classifies the domain matching strategy.
type RuleType uint8

const (
	RuleFull       RuleType = iota // Full domain match (e.g., "example.com")
	RuleRootDomain                 // Suffix match (e.g., "*.example.com")
	RulePlain                      // Keyword match — UNRESOLVABLE via DNS
	RuleRegex                      // Regex pattern — UNRESOLVABLE via DNS
)

func (t RuleType) String() string {
	switch t {
	case RuleFull:
		return "full"
	case RuleRootDomain:
		return "root"
	case RulePlain:
		return "plain"
	case RuleRegex:
		return "regex"
	default:
		return fmt.Sprintf("unknown(%d)", t)
	}
}

// IsResolvable returns true if this rule type can be resolved via DNS.
func (t RuleType) IsResolvable() bool {
	return t == RuleFull || t == RuleRootDomain
}

// Rule represents a normalized domain rule from dlc.dat.
type Rule struct {
	Type  RuleType       // Matching strategy
	Value string         // Normalized domain (lowercase + punycode for Full/RootDomain)
	Attrs map[string]any // Attributes from protobuf (key → bool|int64)
	Cat   string         // Origin category code (for diagnostics)
}

// CategorySelector specifies which categories to extract and how to filter them.
type CategorySelector struct {
	Code  string   // Category code (e.g., "geosite:ru" or "ru")
	Attrs []string // Attribute filter (AND semantics: all must be present and truthy)
}

// ListProvider parses and queries dlc.dat.
type ListProvider interface {
	// Load parses the dlc.dat file at the given path.
	Load(dlcPath string) error

	// Select extracts rules matching the given selectors.
	Select(sel []CategorySelector) ([]Rule, error)

	// Categories returns all discovered category codes (for UI/diagnostics).
	Categories() []string
}
