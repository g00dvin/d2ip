package v2flygeoip

import (
	"context"
	"net/netip"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

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

func TestInfo_BeforeLoad(t *testing.T) {
	p, err := New("geoip-info", "geoip", map[string]any{})
	require.NoError(t, err)

	info := p.Info()
	assert.Equal(t, "geoip-info", info.ID)
	assert.Equal(t, string(sourcereg.TypeV2flyGeoIP), info.Provider)
	assert.Equal(t, "geoip", info.Prefix)
	assert.True(t, info.Enabled)
	assert.Nil(t, info.LastFetched)
	assert.Empty(t, info.Categories)
	assert.Empty(t, info.LastError)
}

func TestInfo_AfterLoad(t *testing.T) {
	p, err := New("geoip-info", "geoip", map[string]any{})
	require.NoError(t, err)

	list := &dlcpb.GeoIPList{
		Entry: []*dlcpb.GeoIP{
			{CountryCode: "ru", Cidr: []*dlcpb.CIDR{{Ip: []byte{10, 0, 0, 0}, Prefix: 8}}},
		},
	}
	data, err := marshalGeoIPList(list)
	require.NoError(t, err)
	require.NoError(t, p.loadFromData(data))

	info := p.Info()
	assert.Equal(t, "geoip-info", info.ID)
	assert.NotNil(t, info.LastFetched)
	assert.Len(t, info.Categories, 1)
	assert.Equal(t, "geoip:ru", info.Categories[0])
	assert.Empty(t, info.LastError)
}

func TestLoad_Success(t *testing.T) {
	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "geoip.dat")

	list := &dlcpb.GeoIPList{
		Entry: []*dlcpb.GeoIP{
			{CountryCode: "ru", Cidr: []*dlcpb.CIDR{{Ip: []byte{10, 0, 0, 0}, Prefix: 8}}},
		},
	}
	data, err := marshalGeoIPList(list)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(cachePath, data, 0644))

	p, err := New("geoip-load", "geoip", map[string]any{
		"cache_path":       cachePath,
		"refresh_interval": "0s",
	})
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	require.NoError(t, p.Load(ctx))

	cats := p.Categories()
	require.Len(t, cats, 1)
	assert.Equal(t, "geoip:ru", cats[0])

	prefixes, err := p.GetPrefixes("geoip:ru")
	require.NoError(t, err)
	require.Len(t, prefixes, 1)
}

func TestLoad_StoreError(t *testing.T) {
	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "nonexistent.dat")

	p, err := New("geoip-load", "geoip", map[string]any{
		"cache_path":       cachePath,
		"refresh_interval": "0s",
	})
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = p.Load(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "fetch failed")
}

func TestLoad_ReadFileError(t *testing.T) {
	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "geoipdir")
	require.NoError(t, os.Mkdir(cachePath, 0755))

	p, err := New("geoip-load", "geoip", map[string]any{
		"cache_path":       cachePath,
		"refresh_interval": "0s",
	})
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = p.Load(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "read file")
}

func TestLoad_UnmarshalError(t *testing.T) {
	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "geoip.dat")
	require.NoError(t, os.WriteFile(cachePath, []byte("not protobuf"), 0644))

	p, err := New("geoip-load", "geoip", map[string]any{
		"cache_path":       cachePath,
		"refresh_interval": "0s",
	})
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = p.Load(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unmarshal")
}

func TestClose(t *testing.T) {
	p, err := New("geoip-close", "geoip", map[string]any{})
	require.NoError(t, err)
	assert.NoError(t, p.Close())
}

func TestCidrToPrefix_Nil(t *testing.T) {
	prefix, ok := cidrToPrefix(nil)
	assert.False(t, ok)
	assert.Equal(t, netip.Prefix{}, prefix)
}

func TestCidrToPrefix_EmptyIP(t *testing.T) {
	prefix, ok := cidrToPrefix(&dlcpb.CIDR{Ip: []byte{}, Prefix: 24})
	assert.False(t, ok)
	assert.Equal(t, netip.Prefix{}, prefix)
}

func TestCidrToPrefix_InvalidIP(t *testing.T) {
	prefix, ok := cidrToPrefix(&dlcpb.CIDR{Ip: []byte{1, 2, 3}, Prefix: 24})
	assert.False(t, ok)
	assert.Equal(t, netip.Prefix{}, prefix)
}
