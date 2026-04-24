package v2flygeosite

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/goodvin/d2ip/internal/domainlist"
	"github.com/goodvin/d2ip/internal/source"
	"github.com/goodvin/d2ip/internal/sourcereg"
)

// Config is the provider-specific configuration.
type Config struct {
	URL             string `mapstructure:"url"`
	CachePath       string `mapstructure:"cache_path"`
	RefreshInterval string `mapstructure:"refresh_interval"` // parsed as duration
	HTTPTimeout     string `mapstructure:"http_timeout"`     // parsed as duration
}

// Provider implements a v2fly domain-list-community source.
type Provider struct {
	mu       sync.RWMutex
	id       string
	prefix   string
	config   Config
	store    source.DLCStore
	dl       *domainlist.Provider
	loadedAt *time.Time
	lastErr  string
}

// New creates a new v2fly geosite provider.
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
		c.URL = "https://github.com/v2fly/domain-list-community/releases/latest/download/dlc.dat"
	}
	if c.CachePath == "" {
		c.CachePath = "/var/lib/d2ip/dlc.dat"
	}
	if c.RefreshInterval == "" {
		c.RefreshInterval = "24h"
	}
	if c.HTTPTimeout == "" {
		c.HTTPTimeout = "30s"
	}

	// Parse durations (validated in New, used in Load)
	if _, err := time.ParseDuration(c.RefreshInterval); err != nil {
		return nil, fmt.Errorf("v2flygeosite: invalid refresh_interval %q: %w", c.RefreshInterval, err)
	}
	httpTimeout, err := time.ParseDuration(c.HTTPTimeout)
	if err != nil {
		return nil, fmt.Errorf("v2flygeosite: invalid http_timeout %q: %w", c.HTTPTimeout, err)
	}

	store, err := source.NewHTTPStore(c.URL, c.CachePath, httpTimeout)
	if err != nil {
		return nil, fmt.Errorf("v2flygeosite: create HTTPStore: %w", err)
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
func (p *Provider) Provider() sourcereg.SourceType { return sourcereg.TypeV2flyGeosite }
func (p *Provider) IsDomainSource() bool   { return true }
func (p *Provider) IsPrefixSource() bool   { return false }

func (p *Provider) AsDomainSource() sourcereg.DomainSource { return p }
func (p *Provider) AsPrefixSource() sourcereg.PrefixSource  { return nil }

func (p *Provider) Categories() []string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.dl == nil {
		return nil
	}
	cats := p.dl.Categories()
	out := make([]string, len(cats))
	for i, c := range cats {
		out[i] = p.prefix + ":" + c
	}
	return out
}

func (p *Provider) Info() sourcereg.SourceInfo {
	p.mu.RLock()
	dl := p.dl
	lastErr := p.lastErr
	loadedAt := p.loadedAt
	p.mu.RUnlock()

	info := sourcereg.SourceInfo{
		ID:        p.id,
		Provider:  string(sourcereg.TypeV2flyGeosite),
		Prefix:    p.prefix,
		Enabled:   true,
		LastError: lastErr,
	}

	var cats []string
	if dl != nil {
		rawCats := dl.Categories()
		cats = make([]string, len(rawCats))
		for i, c := range rawCats {
			cats[i] = p.prefix + ":" + c
		}
	}
	info.Categories = cats

	if loadedAt != nil {
		t := *loadedAt
		info.LastFetched = &t
	}
	return info
}

// Load fetches the dlc.dat file and parses it.
func (p *Provider) Load(ctx context.Context) error {
	refreshInterval, _ := time.ParseDuration(p.config.RefreshInterval)
	path, _, err := p.store.Get(ctx, refreshInterval)
	if err != nil {
		p.mu.Lock()
		p.lastErr = err.Error()
		p.loadedAt = nil
		p.mu.Unlock()
		return fmt.Errorf("v2flygeosite: fetch failed: %w", err)
	}

	// Load into temporary provider to avoid race
	tempDL := domainlist.NewProvider()
	if err := tempDL.Load(path); err != nil {
		p.mu.Lock()
		p.lastErr = err.Error()
		p.loadedAt = nil
		p.mu.Unlock()
		return fmt.Errorf("v2flygeosite: load domainlist: %w", err)
	}

	p.mu.Lock()
	p.dl = tempDL
	p.lastErr = ""
	now := time.Now()
	p.loadedAt = &now
	p.mu.Unlock()
	return nil
}

// Close is a no-op for this provider (HTTPStore has no Close).
func (p *Provider) Close() error { return nil }

// GetDomains returns domains for a prefixed category like "geosite:ru".
func (p *Provider) GetDomains(category string) ([]string, error) {
	p.mu.RLock()
	dl := p.dl
	p.mu.RUnlock()

	if dl == nil {
		return nil, fmt.Errorf("v2flygeosite: provider not loaded")
	}

	expectedPrefix := p.prefix + ":"
	if !strings.HasPrefix(category, expectedPrefix) {
		return nil, fmt.Errorf("v2flygeosite: unknown category %q (prefix %q)", category, p.prefix)
	}
	code := strings.TrimPrefix(category, expectedPrefix)

	// Also handle "geosite:" prefix in the code itself for backward compat
	code = strings.TrimPrefix(code, "geosite:")

	rules, err := dl.Select([]domainlist.CategorySelector{{Code: code}})
	if err != nil {
		return nil, fmt.Errorf("v2flygeosite: select %q: %w", code, err)
	}

	domains := make([]string, 0, len(rules))
	for _, r := range rules {
		if r.Type.IsResolvable() && r.Value != "" {
			domains = append(domains, r.Value)
		}
	}
	return domains, nil
}

func init() {
	sourcereg.RegisterFactory(sourcereg.TypeV2flyGeosite, func(id, prefix string, cfg map[string]any) (sourcereg.Source, error) {
		return New(id, prefix, cfg)
	})
}
