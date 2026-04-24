package plaintext

import (
	"bufio"
	"context"
	"fmt"
	"net/netip"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/goodvin/d2ip/internal/sourcereg"
)

// Config is the provider-specific configuration.
type Config struct {
	Type string `mapstructure:"type"` // "domains" or "ips"
	File string `mapstructure:"file"`
}

// Provider implements a plaintext file source.
type Provider struct {
	mu       sync.RWMutex
	id       string
	prefix   string
	config   Config
	domains  []string
	prefixes []netip.Prefix
	loadedAt *time.Time
	lastErr  string
}

// New creates a new plaintext provider.
func New(id string, prefix string, cfg map[string]any) (*Provider, error) {
	var c Config
	if t, ok := cfg["type"].(string); ok {
		c.Type = t
	}
	if f, ok := cfg["file"].(string); ok {
		c.File = f
	}
	if c.Type == "" {
		c.Type = "domains"
	}
	if c.Type != "domains" && c.Type != "ips" {
		return nil, fmt.Errorf("plaintext: type must be 'domains' or 'ips', got %q", c.Type)
	}
	if c.File == "" {
		return nil, fmt.Errorf("plaintext: file is required")
	}
	return &Provider{
		id:     id,
		prefix: prefix,
		config: c,
	}, nil
}

func (p *Provider) ID() string             { return p.id }
func (p *Provider) Prefix() string         { return p.prefix }
func (p *Provider) Provider() sourcereg.SourceType { return sourcereg.TypePlaintext }
func (p *Provider) IsDomainSource() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.config.Type == "domains"
}
func (p *Provider) IsPrefixSource() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.config.Type == "ips"
}

func (p *Provider) AsDomainSource() sourcereg.DomainSource {
	if p.IsDomainSource() {
		return p
	}
	return nil
}

func (p *Provider) AsPrefixSource() sourcereg.PrefixSource {
	if p.IsPrefixSource() {
		return p
	}
	return nil
}

func (p *Provider) Categories() []string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return []string{p.prefix + ":default"}
}

func (p *Provider) Info() sourcereg.SourceInfo {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return sourcereg.SourceInfo{
		ID:          p.id,
		Provider:    string(sourcereg.TypePlaintext),
		Prefix:      p.prefix,
		Enabled:     true,
		Categories:  []string{p.prefix + ":default"},
		LastFetched: p.loadedAt,
		LastError:   p.lastErr,
	}
}

// Load reads the plaintext file and parses entries.
func (p *Provider) Load(_ context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	f, err := os.Open(p.config.File)
	if err != nil {
		p.lastErr = err.Error()
		p.loadedAt = nil
		return fmt.Errorf("plaintext: open %q: %w", p.config.File, err)
	}
	defer f.Close()

	p.domains = nil
	p.prefixes = nil
	p.lastErr = ""
	p.loadedAt = nil

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if p.config.Type == "domains" {
			p.domains = append(p.domains, line)
		} else {
			prefix, err := netip.ParsePrefix(line)
			if err != nil {
				// Try parsing as single IP and convert to /32 or /128
				addr, err2 := netip.ParseAddr(line)
				if err2 == nil {
					if addr.Is4() {
						prefix = netip.PrefixFrom(addr, 32)
					} else {
						prefix = netip.PrefixFrom(addr, 128)
					}
				} else {
					continue // skip invalid
				}
			}
			p.prefixes = append(p.prefixes, prefix)
		}
	}
	if err := scanner.Err(); err != nil {
		p.lastErr = err.Error()
		p.loadedAt = nil
		return fmt.Errorf("plaintext: scan %q: %w", p.config.File, err)
	}

	now := time.Now()
	p.loadedAt = &now
	return nil
}

// Close is a no-op for plaintext provider.
func (p *Provider) Close() error { return nil }

// GetDomains returns domains for the given category (only "prefix:default").
func (p *Provider) GetDomains(category string) ([]string, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	expected := p.prefix + ":default"
	if category != expected {
		return nil, fmt.Errorf("plaintext: unknown category %q (expected %q)", category, expected)
	}
	if p.domains == nil {
		return nil, fmt.Errorf("plaintext: not loaded")
	}
	out := make([]string, len(p.domains))
	copy(out, p.domains)
	return out, nil
}

// GetPrefixes returns prefixes for the given category (only "prefix:default").
func (p *Provider) GetPrefixes(category string) ([]netip.Prefix, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	expected := p.prefix + ":default"
	if category != expected {
		return nil, fmt.Errorf("plaintext: unknown category %q (expected %q)", category, expected)
	}
	if p.prefixes == nil {
		return nil, fmt.Errorf("plaintext: not loaded")
	}
	out := make([]netip.Prefix, len(p.prefixes))
	copy(out, p.prefixes)
	return out, nil
}

func init() {
	sourcereg.RegisterFactory(sourcereg.TypePlaintext, func(id, prefix string, cfg map[string]any) (sourcereg.Source, error) {
		return New(id, prefix, cfg)
	})
}
