package source

import (
	"fmt"
	"net/netip"
)

// GeoIPSource resolves country codes to IP prefixes using MaxMind MMDB.
type GeoIPSource struct {
	mmdbPath string
}

// NewGeoIPSource creates a new GeoIP source.
func NewGeoIPSource(mmdbPath string) *GeoIPSource {
	return &GeoIPSource{mmdbPath: mmdbPath}
}

// CountryPrefixes returns all IP prefixes for the given country code.
// NOTE: This is a stub. Full implementation requires MMDB parsing or
// using a library like github.com/oschwald/geoip2-golang.
func (s *GeoIPSource) CountryPrefixes(countryCode string) ([]netip.Prefix, error) {
	return nil, fmt.Errorf("geoip source not yet implemented")
}
