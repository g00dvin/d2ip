package v2flygeoip

import (
	"net/netip"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/goodvin/d2ip/internal/domainlist/dlcpb"
	"github.com/goodvin/d2ip/internal/sourcereg"
)

func TestNew_ValidConfig(t *testing.T) {
	p, err := New("geoip-1", "geoip", map[string]any{})
	require.NoError(t, err)
	assert.Equal(t, "geoip-1", p.ID())
	assert.Equal(t, "geoip", p.Prefix())
	assert.Equal(t, sourcereg.TypeV2flyGeoIP, p.Provider())
	assert.True(t, p.IsPrefixSource())
	assert.False(t, p.IsDomainSource())
	assert.Nil(t, p.AsDomainSource())
	require.NotNil(t, p.AsPrefixSource())
}

func TestLoad_FromData(t *testing.T) {
	p, err := New("geoip-test", "geoip", map[string]any{})
	require.NoError(t, err)

	// Build a GeoIPList with test data
	list := &dlcpb.GeoIPList{
		Entry: []*dlcpb.GeoIP{
			{
				CountryCode: "ru",
				Cidr: []*dlcpb.CIDR{
					{Ip: []byte{1, 2, 3, 0}, Prefix: 24},
					{Ip: []byte{0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}, Prefix: 32},
				},
			},
			{
				CountryCode: "us",
				Cidr: []*dlcpb.CIDR{
					{Ip: []byte{5, 6, 7, 8}, Prefix: 32},
				},
			},
		},
	}

	data, err := marshalGeoIPList(list)
	require.NoError(t, err)

	require.NoError(t, p.loadFromData(data))

	cats := p.Categories()
	require.Len(t, cats, 2)
	assert.Contains(t, cats, "geoip:ru")
	assert.Contains(t, cats, "geoip:us")

	ruPrefixes, err := p.GetPrefixes("geoip:ru")
	require.NoError(t, err)
	require.Len(t, ruPrefixes, 2)
	assert.Contains(t, ruPrefixes, netip.MustParsePrefix("1.2.3.0/24"))
	assert.Contains(t, ruPrefixes, netip.MustParsePrefix("2001:db8::/32"))

	usPrefixes, err := p.GetPrefixes("geoip:us")
	require.NoError(t, err)
	require.Len(t, usPrefixes, 1)
	assert.Equal(t, "5.6.7.8/32", usPrefixes[0].String())
}

func TestGetPrefixes_NotLoaded(t *testing.T) {
	p, err := New("geoip-test", "geoip", map[string]any{})
	require.NoError(t, err)

	_, err = p.GetPrefixes("geoip:ru")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not loaded")
}

func TestGetPrefixes_UnknownCategory(t *testing.T) {
	p, err := New("geoip-test", "geoip", map[string]any{})
	require.NoError(t, err)

	list := &dlcpb.GeoIPList{
		Entry: []*dlcpb.GeoIP{
			{CountryCode: "ru", Cidr: []*dlcpb.CIDR{{Ip: []byte{10, 0, 0, 0}, Prefix: 8}}},
		},
	}
	data, err := marshalGeoIPList(list)
	require.NoError(t, err)
	require.NoError(t, p.loadFromData(data))

	_, err = p.GetPrefixes("geoip:xx")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown country")
}

func TestFactory_Registered(t *testing.T) {
	factory := sourcereg.GetFactory(sourcereg.TypeV2flyGeoIP)
	require.NotNil(t, factory, "v2flygeoip factory should be registered")

	src, err := factory("geoip-f", "geoip", map[string]any{})
	require.NoError(t, err)
	assert.Equal(t, "geoip-f", src.ID())
	assert.Equal(t, sourcereg.TypeV2flyGeoIP, src.Provider())
}

func TestLoad_Concurrent(t *testing.T) {
	p, err := New("geoip-test", "geoip", map[string]any{})
	require.NoError(t, err)

	list := &dlcpb.GeoIPList{
		Entry: []*dlcpb.GeoIP{
			{CountryCode: "ru", Cidr: []*dlcpb.CIDR{{Ip: []byte{10, 0, 0, 0}, Prefix: 8}}},
		},
	}
	data, err := marshalGeoIPList(list)
	require.NoError(t, err)
	require.NoError(t, p.loadFromData(data))

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = p.GetPrefixes("geoip:ru")
		}()
	}
	wg.Wait()

	prefixes, err := p.GetPrefixes("geoip:ru")
	require.NoError(t, err)
	assert.Len(t, prefixes, 1)
}
