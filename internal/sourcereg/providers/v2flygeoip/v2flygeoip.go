package v2flygeoip

import (
	"context"
	"fmt"
	"net/netip"
	"sync"
	"time"

	"github.com/goodvin/d2ip/internal/sourcereg"
	"github.com/goodvin/d2ip/internal/source"
)

// Config is the provider-specific configuration.
type Config struct {
	URL             string `mapstructure:"url"`
	CachePath       string `mapstructure:"cache_path"`
	RefreshInterval string `mapstructure:"refresh_interval"`
	HTTPTimeout     string `mapstructure:"http_timeout"`
}

// Provider implements a v2fly geoip source.
type Provider struct {
	mu       sync.RWMutex
	id       string
	prefix   string
	config   Config
	store    source.DLCStore
	prefixes map[string][]netip.Prefix
	loadedAt *time.Time
	lastErr  string
}

// New creates a new v2fly geoip provider.
func New(id string, prefix string, cfg map[string]any) (*Provider, error) {
	var c Config
	if u, ok := cfg["url"].(string); ok {
		c.URL = u
	}
	if cp, ok := cfg["cache_path"].(string); ok {
		c.CachePath = cp
	}
	if ri, ok := cfg["refresh_interval"].(string); ok {
		c.RefreshInterval = ri
	}
	if ht, ok := cfg["http_timeout"].(string); ok {
		c.HTTPTimeout = ht
	}
	if c.URL == "" {
		c.URL = "https://github.com/v2fly/geoip/releases/latest/download/geoip.dat"
	}
	if c.CachePath == "" {
		c.CachePath = "/var/lib/d2ip/geoip.dat"
	}
	if c.RefreshInterval == "" {
		c.RefreshInterval = "24h"
	}
	if c.HTTPTimeout == "" {
		c.HTTPTimeout = "30s"
	}

	if _, err := time.ParseDuration(c.RefreshInterval); err != nil {
		return nil, fmt.Errorf("v2flygeoip: invalid refresh_interval %q: %w", c.RefreshInterval, err)
	}
	httpTimeout, err := time.ParseDuration(c.HTTPTimeout)
	if err != nil {
		return nil, fmt.Errorf("v2flygeoip: invalid http_timeout %q: %w", c.HTTPTimeout, err)
	}

	store, err := source.NewHTTPStore(c.URL, c.CachePath, httpTimeout)
	if err != nil {
		return nil, fmt.Errorf("v2flygeoip: create HTTPStore: %w", err)
	}

	return &Provider{
		id:     id,
		prefix: prefix,
		config: c,
		store:  store,
	}, nil
}

func (p *Provider) ID() string             { return p.id }
func (p *Provider) Prefix() string         { return p.prefix }
func (p *Provider) Provider() sourcereg.SourceType { return sourcereg.TypeV2flyGeoIP }
func (p *Provider) IsDomainSource() bool   { return false }
func (p *Provider) IsPrefixSource() bool   { return true }
func (p *Provider) AsDomainSource() sourcereg.DomainSource { return nil }
func (p *Provider) AsPrefixSource() sourcereg.PrefixSource { return p }

func (p *Provider) Categories() []string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	cats := make([]string, 0, len(p.prefixes))
	for country := range p.prefixes {
		cats = append(cats, p.prefix+":"+country)
	}
	return cats
}

func (p *Provider) Info() sourcereg.SourceInfo {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return sourcereg.SourceInfo{
		ID:          p.id,
		Provider:    string(sourcereg.TypeV2flyGeoIP),
		Prefix:      p.prefix,
		Enabled:     true,
		Categories:  p.Categories(),
		LastFetched: p.loadedAt,
		LastError:   p.lastErr,
	}
}

// Load fetches the geoip.dat file and parses it.
func (p *Provider) Load(ctx context.Context) error {
	return fmt.Errorf("not implemented")
}

// GetPrefixes returns prefixes for the given category.
func (p *Provider) GetPrefixes(category string) ([]netip.Prefix, error) {
	return nil, fmt.Errorf("not implemented")
}

// Close is a no-op for this provider.
func (p *Provider) Close() error { return nil }
