package mmdb

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"os"
	"strings"
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
	Networks(options ...maxminddb.NetworksOption) *maxminddb.Networks
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

// networkIterator abstracts maxminddb network iteration for testability.
type networkIterator interface {
	Next() bool
	Network() *net.IPNet
	Record() interface{}
}

// loadFromIterator extracts prefixes from a network iterator.
func (p *Provider) loadFromIterator(iter networkIterator) error {
	p.prefixes = make(map[string][]netip.Prefix)
	p.lastErr = ""
	p.loadedAt = nil

	whitelist := make(map[string]bool)
	for _, c := range p.config.Countries {
		whitelist[c] = true
	}

	for iter.Next() {
		ipNet := iter.Network()
		prefix, ok := netipPrefixFromIPNet(ipNet)
		if !ok {
			continue
		}

		country := extractCountry(iter.Record())
		if country == "" {
			continue
		}
		if len(whitelist) > 0 && !whitelist[country] {
			continue
		}
		p.prefixes[country] = append(p.prefixes[country], prefix)
	}

	now := time.Now()
	p.loadedAt = &now
	return nil
}

func extractCountry(record interface{}) string {
	m, ok := record.(map[string]interface{})
	if !ok {
		return ""
	}
	country, ok := m["country"].(map[string]interface{})
	if !ok {
		return ""
	}
	iso, ok := country["iso_code"].(string)
	if !ok {
		return ""
	}
	return iso
}

func netipPrefixFromIPNet(ipNet *net.IPNet) (netip.Prefix, bool) {
	if ipNet == nil {
		return netip.Prefix{}, false
	}
	ones, bits := ipNet.Mask.Size()
	if ones == 0 && bits == 0 {
		return netip.Prefix{}, false
	}
	addr, ok := netip.AddrFromSlice(ipNet.IP)
	if !ok {
		return netip.Prefix{}, false
	}
	return netip.PrefixFrom(addr, ones), true
}

// Load opens the MMDB file and extracts prefixes by country.
func (p *Provider) Load(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.prefixes = make(map[string][]netip.Prefix)
	p.lastErr = ""
	p.loadedAt = nil

	path := p.config.File
	if path == "" && p.config.URL != "" {
		tmpPath, err := downloadToTemp(ctx, p.config.URL)
		if err != nil {
			p.lastErr = err.Error()
			return fmt.Errorf("mmdb: download: %w", err)
		}
		defer os.Remove(tmpPath)
		path = tmpPath
	}

	reader, err := maxminddb.Open(path)
	if err != nil {
		p.lastErr = err.Error()
		return fmt.Errorf("mmdb: open %q: %w", path, err)
	}

	if p.reader != nil {
		_ = p.reader.Close()
	}
	p.reader = reader

	whitelist := make(map[string]bool)
	for _, c := range p.config.Countries {
		whitelist[c] = true
	}

	networks := reader.Networks(maxminddb.SkipAliasedNetworks)

	var record struct {
		Country struct {
			ISOCode string `maxminddb:"iso_code"`
		} `maxminddb:"country"`
	}

	for networks.Next() {
		ipNet, err := networks.Network(&record)
		if err != nil {
			continue
		}
		prefix, ok := netipPrefixFromIPNet(ipNet)
		if !ok {
			continue
		}
		country := record.Country.ISOCode
		if country == "" {
			continue
		}
		if len(whitelist) > 0 && !whitelist[country] {
			continue
		}
		p.prefixes[country] = append(p.prefixes[country], prefix)
	}
	if err := networks.Err(); err != nil {
		p.lastErr = err.Error()
		return fmt.Errorf("mmdb: iterate: %w", err)
	}

	now := time.Now()
	p.loadedAt = &now
	return nil
}

func downloadToTemp(ctx context.Context, url string) (string, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	f, err := os.CreateTemp("", "mmdb-*.mmdb")
	if err != nil {
		return "", err
	}
	defer f.Close()

	if _, err := io.Copy(f, resp.Body); err != nil {
		os.Remove(f.Name())
		return "", err
	}
	return f.Name(), nil
}

// GetPrefixes returns prefixes for the given category (e.g., "mmdb:ru").
func (p *Provider) GetPrefixes(category string) ([]netip.Prefix, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.loadedAt == nil {
		return nil, fmt.Errorf("mmdb: not loaded")
	}

	expectedPrefix := p.prefix + ":"
	if !strings.HasPrefix(category, expectedPrefix) {
		return nil, fmt.Errorf("mmdb: unknown category %q", category)
	}
	country := strings.TrimPrefix(category, expectedPrefix)
	prefixes, ok := p.prefixes[country]
	if !ok {
		return nil, fmt.Errorf("mmdb: unknown country %q", country)
	}
	out := make([]netip.Prefix, len(prefixes))
	copy(out, prefixes)
	return out, nil
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

func init() {
	sourcereg.RegisterFactory(sourcereg.TypeMMDB, func(id, prefix string, cfg map[string]any) (sourcereg.Source, error) {
		return New(id, prefix, cfg)
	})
}
