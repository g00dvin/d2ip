package source

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ASNSource fetches announced prefixes for ASNs from RIPE RIS.
type ASNSource struct {
	cacheDir string
	client   *http.Client
}

// NewASNSource creates a new ASN prefix fetcher.
func NewASNSource(cacheDir string) *ASNSource {
	return &ASNSource{
		cacheDir: cacheDir,
		client:   &http.Client{Timeout: 30 * time.Second},
	}
}

// FetchPrefixes returns all announced prefixes for the given ASN.
func (s *ASNSource) FetchPrefixes(ctx context.Context, asn string) ([]netip.Prefix, error) {
	asn = strings.TrimPrefix(asn, "AS")
	cachePath := filepath.Join(s.cacheDir, fmt.Sprintf("asn_%s.json", asn))

	// Try cache first (if less than 24h old)
	if info, err := os.Stat(cachePath); err == nil {
		if time.Since(info.ModTime()) < 24*time.Hour {
			if data, err := os.ReadFile(cachePath); err == nil {
				return parseASNResponse(data)
			}
		}
	}

	url := fmt.Sprintf("https://stat.ripe.net/data/announced-prefixes/data.json?resource=AS%s", asn)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch ASN %s: %w", asn, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("RIPE API returned %d for AS%s", resp.StatusCode, asn)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Save cache
	os.MkdirAll(s.cacheDir, 0755)
	os.WriteFile(cachePath, data, 0644)

	return parseASNResponse(data)
}

func parseASNResponse(data []byte) ([]netip.Prefix, error) {
	var result struct {
		Data struct {
			Prefixes []struct {
				Prefix string `json:"prefix"`
			} `json:"prefixes"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}

	var prefixes []netip.Prefix
	for _, p := range result.Data.Prefixes {
		prefix, err := netip.ParsePrefix(p.Prefix)
		if err != nil {
			continue // skip invalid
		}
		prefixes = append(prefixes, prefix)
	}
	return prefixes, nil
}
