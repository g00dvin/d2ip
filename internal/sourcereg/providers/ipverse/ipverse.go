package ipverse

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/netip"
	"strings"
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
	p.mu.Lock()
	defer p.mu.Unlock()

	p.prefixes = make(map[string][]netip.Prefix)
	p.lastErr = ""
	p.loadedAt = nil

	client := &http.Client{Timeout: 30 * time.Second}

	for _, country := range p.config.Countries {
		url := strings.ReplaceAll(p.config.BaseURL, "{country}", country)
		prefixes, err := fetchCountry(ctx, client, url)
		if err != nil {
			p.lastErr = err.Error()
			return fmt.Errorf("ipverse: fetch %s: %w", country, err)
		}
		p.prefixes[country] = prefixes
	}

	now := time.Now()
	p.loadedAt = &now
	return nil
}

func fetchCountry(ctx context.Context, client *http.Client, url string) ([]netip.Prefix, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return parsePrefixes(resp.Body)
}

func parsePrefixes(r io.Reader) ([]netip.Prefix, error) {
	var prefixes []netip.Prefix
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		prefix, err := netip.ParsePrefix(line)
		if err != nil {
			addr, err2 := netip.ParseAddr(line)
			if err2 == nil {
				if addr.Is4() {
					prefix = netip.PrefixFrom(addr, 32)
				} else {
					prefix = netip.PrefixFrom(addr, 128)
				}
			} else {
				continue // skip invalid lines
			}
		}
		prefixes = append(prefixes, prefix)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return prefixes, nil
}

// GetPrefixes returns prefixes for the given category (e.g., "ipverse:ru").
func (p *Provider) GetPrefixes(category string) ([]netip.Prefix, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	expectedPrefix := p.prefix + ":"
	if !strings.HasPrefix(category, expectedPrefix) {
		return nil, fmt.Errorf("ipverse: unknown category %q", category)
	}
	country := strings.TrimPrefix(category, expectedPrefix)
	prefixes, ok := p.prefixes[country]
	if !ok {
		return nil, fmt.Errorf("ipverse: unknown country %q", country)
	}
	if p.loadedAt == nil {
		return nil, fmt.Errorf("ipverse: not loaded")
	}
	out := make([]netip.Prefix, len(prefixes))
	copy(out, prefixes)
	return out, nil
}

// Close is a no-op for IPverse provider.
func (p *Provider) Close() error { return nil }
