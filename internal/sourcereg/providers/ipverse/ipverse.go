package ipverse

import (
	"context"
	"fmt"
	"net/netip"
	"sync"
	"time"

	"github.com/goodvin/d2ip/internal/sourcereg"
)

// Config is the provider-specific configuration.
type Config struct {
	BaseURL   string   `mapstructure:"base_url"`
	Countries []string `mapstructure:"countries"`
}

// Provider implements an IPverse country IP blocks source.
type Provider struct {
	mu       sync.RWMutex
	id       string
	prefix   string
	config   Config
	prefixes map[string][]netip.Prefix
	loadedAt *time.Time
	lastErr  string
}

// New creates a new IPverse provider.
func New(id string, prefix string, cfg map[string]any) (*Provider, error) {
	var c Config
	if u, ok := cfg["base_url"].(string); ok {
		c.BaseURL = u
	}
	if countries, ok := cfg["countries"].([]any); ok {
		for _, co := range countries {
			if s, ok := co.(string); ok {
				c.Countries = append(c.Countries, s)
			}
		}
	}
	if c.BaseURL == "" {
		c.BaseURL = "https://ipverse.net/ipblocks/data/countries/{country}.zone"
	}
	if len(c.Countries) == 0 {
		return nil, fmt.Errorf("ipverse: countries is required")
	}
	return &Provider{
		id:     id,
		prefix: prefix,
		config: c,
	}, nil
}

func (p *Provider) ID() string             { return p.id }
func (p *Provider) Prefix() string         { return p.prefix }
func (p *Provider) Provider() sourcereg.SourceType { return sourcereg.TypeIPverse }
func (p *Provider) IsDomainSource() bool   { return false }
func (p *Provider) IsPrefixSource() bool   { return true }
func (p *Provider) AsDomainSource() sourcereg.DomainSource { return nil }
func (p *Provider) AsPrefixSource() sourcereg.PrefixSource { return p }

func (p *Provider) Categories() []string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	cats := make([]string, len(p.config.Countries))
	for i, c := range p.config.Countries {
		cats[i] = p.prefix + ":" + c
	}
	return cats
}

func (p *Provider) Info() sourcereg.SourceInfo {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return sourcereg.SourceInfo{
		ID:          p.id,
		Provider:    string(sourcereg.TypeIPverse),
		Prefix:      p.prefix,
		Enabled:     true,
		Categories:  p.Categories(),
		LastFetched: p.loadedAt,
		LastError:   p.lastErr,
	}
}

// Load fetches IP block lists for all configured countries.
func (p *Provider) Load(ctx context.Context) error {
	return fmt.Errorf("not implemented")
}

// GetPrefixes returns prefixes for the given category.
func (p *Provider) GetPrefixes(category string) ([]netip.Prefix, error) {
	return nil, fmt.Errorf("not implemented")
}

// Close is a no-op for IPverse provider.
func (p *Provider) Close() error { return nil }
