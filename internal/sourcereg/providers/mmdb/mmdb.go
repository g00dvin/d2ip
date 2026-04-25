package mmdb

import (
	"context"
	"fmt"
	"net/netip"
	"sync"
	"time"

	"github.com/goodvin/d2ip/internal/sourcereg"
	"github.com/oschwald/maxminddb-golang"
)

// Config is the provider-specific configuration.
type Config struct {
	File      string   `mapstructure:"file"`
	URL       string   `mapstructure:"url"`
	Countries []string `mapstructure:"countries"`
}

// mmdbReader is the subset of maxminddb.Reader we use.
type mmdbReader interface {
	Networks(skipAliasedNetworks bool) (*maxminddb.Networks, error)
	Close() error
}

// Provider implements a MaxMind MMDB source.
type Provider struct {
	mu       sync.RWMutex
	id       string
	prefix   string
	config   Config
	prefixes map[string][]netip.Prefix
	loadedAt *time.Time
	lastErr  string
	reader   mmdbReader
}

// New creates a new MMDB provider.
func New(id string, prefix string, cfg map[string]any) (*Provider, error) {
	var c Config
	if f, ok := cfg["file"].(string); ok {
		c.File = f
	}
	if u, ok := cfg["url"].(string); ok {
		c.URL = u
	}
	if countries, ok := cfg["countries"].([]any); ok {
		for _, co := range countries {
			if s, ok := co.(string); ok {
				c.Countries = append(c.Countries, s)
			}
		}
	}
	if c.File == "" && c.URL == "" {
		return nil, fmt.Errorf("mmdb: either file or url is required")
	}
	return &Provider{
		id:     id,
		prefix: prefix,
		config: c,
	}, nil
}

func (p *Provider) ID() string             { return p.id }
func (p *Provider) Prefix() string         { return p.prefix }
func (p *Provider) Provider() sourcereg.SourceType { return sourcereg.TypeMMDB }
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
		Provider:    string(sourcereg.TypeMMDB),
		Prefix:      p.prefix,
		Enabled:     true,
		Categories:  p.Categories(),
		LastFetched: p.loadedAt,
		LastError:   p.lastErr,
	}
}

// Load reads the MMDB file and groups prefixes by country.
func (p *Provider) Load(ctx context.Context) error {
	return fmt.Errorf("not implemented")
}

// GetPrefixes returns prefixes for the given category.
func (p *Provider) GetPrefixes(category string) ([]netip.Prefix, error) {
	return nil, fmt.Errorf("not implemented")
}

// Close closes the MMDB reader.
func (p *Provider) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.reader != nil {
		return p.reader.Close()
	}
	return nil
}
