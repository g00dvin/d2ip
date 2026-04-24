package sourcereg

import (
	"context"
	"net/netip"
	"time"
)

// SourceType identifies the provider implementation.
type SourceType string

const (
	TypeV2flyGeosite SourceType = "v2flygeosite"
	TypePlaintext    SourceType = "plaintext"
	TypeIPverse      SourceType = "ipverse"
	TypeV2flyGeoIP   SourceType = "v2flygeoip"
	TypeMMDB         SourceType = "mmdb"
)

// CategoryType distinguishes domain-based from prefix-based categories.
type CategoryType string

const (
	CategoryDomain CategoryType = "domain"
	CategoryPrefix CategoryType = "prefix"
)

// SourceConfig is the user-facing configuration for a source.
type SourceConfig struct {
	ID       string         `mapstructure:"id" json:"id" yaml:"id"`
	Provider SourceType     `mapstructure:"provider" json:"provider" yaml:"provider"`
	Prefix   string         `mapstructure:"prefix" json:"prefix" yaml:"prefix"`
	Enabled  bool           `mapstructure:"enabled" json:"enabled" yaml:"enabled"`
	Config   map[string]any `mapstructure:"config" json:"config" yaml:"config"`
}

// CategoryInfo describes a category exposed by a source.
type CategoryInfo struct {
	Name     string       `json:"name"`      // Full prefixed name, e.g. "geosite:ru"
	SourceID string       `json:"source_id"`
	Type     CategoryType `json:"type"`      // "domain" or "prefix"
	Count    int          `json:"count"`     // Number of entries (domains or prefixes)
}

// SourceInfo describes a source for the UI.
type SourceInfo struct {
	ID          string     `json:"id"`
	Provider    string     `json:"provider"`
	Prefix      string     `json:"prefix"`
	Enabled     bool       `json:"enabled"`
	Categories  []string   `json:"categories"`
	LastFetched *time.Time `json:"last_fetched,omitempty"`
	LastError   string     `json:"last_error,omitempty"`
}

// DomainSource provides domains that need DNS resolution.
// Implementations must be safe for concurrent use.
type DomainSource interface {
	Load(ctx context.Context) error
	Close() error
	GetDomains(category string) ([]string, error)
	Categories() []string
	Info() SourceInfo
}

// PrefixSource provides IP prefixes directly (no DNS resolution needed).
// Implementations must be safe for concurrent use.
type PrefixSource interface {
	Load(ctx context.Context) error
	Close() error
	GetPrefixes(category string) ([]netip.Prefix, error)
	Categories() []string
	Info() SourceInfo
}

// Source is the union interface implemented by all providers.
// Implementations must be safe for concurrent use.
type Source interface {
	ID() string
	Prefix() string
	Provider() SourceType
	Load(ctx context.Context) error
	Close() error
	Categories() []string
	Info() SourceInfo
	// IsDomainSource returns true if this source provides resolvable domains.
	IsDomainSource() bool
	// IsPrefixSource returns true if this source provides IP prefixes directly.
	IsPrefixSource() bool
	// AsDomainSource returns the DomainSource interface, or nil.
	AsDomainSource() DomainSource
	// AsPrefixSource returns the PrefixSource interface, or nil.
	AsPrefixSource() PrefixSource
}

// Registry manages the lifecycle of all configured sources.
// Implementations must be safe for concurrent use.
type Registry interface {
	// LoadAll loads/reloads all enabled sources.
	LoadAll(ctx context.Context) error
	// Close releases all resources held by the registry and its sources.
	Close() error
	// ListSources returns metadata for all configured sources.
	ListSources() []SourceInfo
	// GetSource returns a source by ID.
	GetSource(id string) (Source, bool)
	// ListCategories returns all categories across all loaded sources.
	ListCategories() []CategoryInfo
	// GetDomains collects domains for the given category from its source.
	//
	// Category strings are globally unique because source prefixes are unique.
	// The registry finds the owning source by matching the category against all
	// source categories. If no source owns the category, an error is returned.
	GetDomains(category string) ([]string, error)
	// GetPrefixes collects prefixes for the given category from its source.
	//
	// Category strings are globally unique because source prefixes are unique.
	// The registry finds the owning source by matching the category against all
	// source categories. If no source owns the category, an error is returned.
	GetPrefixes(category string) ([]netip.Prefix, error)
	// ResolveCategory finds which source owns a category and its type.
	//
	// Category strings are globally unique because source prefixes are unique.
	// The registry finds the owning source by matching the category against all
	// source categories. If no source owns the category, found is false.
	ResolveCategory(category string) (sourceID string, catType string, found bool)
}
