package mmdb

import (
	"context"
	"net"
	"net/netip"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/goodvin/d2ip/internal/sourcereg"
	"github.com/oschwald/maxminddb-golang"
)

func TestNew_ValidConfig(t *testing.T) {
	p, err := New("mmdb-1", "mmdb", map[string]any{
		"file": "/tmp/GeoLite2-Country.mmdb",
	})
	require.NoError(t, err)
	assert.Equal(t, "mmdb-1", p.ID())
	assert.Equal(t, "mmdb", p.Prefix())
	assert.Equal(t, sourcereg.TypeMMDB, p.Provider())
	assert.True(t, p.IsPrefixSource())
	assert.False(t, p.IsDomainSource())
	assert.Nil(t, p.AsDomainSource())
	require.NotNil(t, p.AsPrefixSource())
}

func TestNew_URLConfig(t *testing.T) {
	p, err := New("mmdb-2", "mmdb", map[string]any{
		"url": "https://example.com/GeoLite2-Country.mmdb",
	})
	require.NoError(t, err)
	assert.Equal(t, "mmdb-2", p.ID())
}

func TestNew_MissingSource(t *testing.T) {
	_, err := New("mmdb-3", "mmdb", map[string]any{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "file or url")
}

// mockNetworkIterator simulates maxminddb network iteration.
type mockNetworkIterator struct {
	networks []struct {
		ipNet  *net.IPNet
		record map[string]interface{}
	}
	idx int
}

func (m *mockNetworkIterator) Next() bool {
	if m.idx < len(m.networks) {
		m.idx++
		return true
	}
	return false
}

func (m *mockNetworkIterator) Network() *net.IPNet {
	return m.networks[m.idx-1].ipNet
}

func (m *mockNetworkIterator) Record() interface{} {
	return m.networks[m.idx-1].record
}

func TestLoad_MockIterator(t *testing.T) {
	p, err := New("mmdb-test", "mmdb", map[string]any{
		"file": "/tmp/test.mmdb",
	})
	require.NoError(t, err)

	_, ipNet1, _ := net.ParseCIDR("1.2.3.0/24")
	_, ipNet2, _ := net.ParseCIDR("5.6.7.0/24")
	_, ipNet3, _ := net.ParseCIDR("2001:db8::/32")

	iter := &mockNetworkIterator{
		networks: []struct {
			ipNet  *net.IPNet
			record map[string]interface{}
		}{
			{ipNet1, map[string]interface{}{"country": map[string]interface{}{"iso_code": "ru"}}},
			{ipNet2, map[string]interface{}{"country": map[string]interface{}{"iso_code": "us"}}},
			{ipNet3, map[string]interface{}{"country": map[string]interface{}{"iso_code": "ru"}}},
		},
	}

	require.NoError(t, p.loadFromIterator(iter))

	cats := p.Categories()
	require.Len(t, cats, 2)
	assert.Contains(t, cats, "mmdb:ru")
	assert.Contains(t, cats, "mmdb:us")

	ruPrefixes, err := p.GetPrefixes("mmdb:ru")
	require.NoError(t, err)
	require.Len(t, ruPrefixes, 2)
	assert.Contains(t, ruPrefixes, netip.MustParsePrefix("1.2.3.0/24"))
	assert.Contains(t, ruPrefixes, netip.MustParsePrefix("2001:db8::/32"))

	usPrefixes, err := p.GetPrefixes("mmdb:us")
	require.NoError(t, err)
	require.Len(t, usPrefixes, 1)
}

func TestLoad_FileNotFound(t *testing.T) {
	p, err := New("mmdb-test", "mmdb", map[string]any{
		"file": "/nonexistent/path/test.mmdb",
	})
	require.NoError(t, err)

	ctx := context.Background()
	err = p.Load(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "open")
}

func TestGetPrefixes_NotLoaded(t *testing.T) {
	p, err := New("mmdb-test", "mmdb", map[string]any{
		"file": "/tmp/test.mmdb",
	})
	require.NoError(t, err)

	_, err = p.GetPrefixes("mmdb:ru")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not loaded")
}

func TestGetPrefixes_UnknownCategory(t *testing.T) {
	p, err := New("mmdb-test", "mmdb", map[string]any{
		"file": "/tmp/test.mmdb",
	})
	require.NoError(t, err)
	p.prefixes = map[string][]netip.Prefix{"ru": {netip.MustParsePrefix("1.2.3.0/24")}}
	now := time.Now()
	p.loadedAt = &now

	_, err = p.GetPrefixes("mmdb:xx")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown country")
}

func TestFactory_Registered(t *testing.T) {
	factory := sourcereg.GetFactory(sourcereg.TypeMMDB)
	require.NotNil(t, factory, "mmdb factory should be registered")

	src, err := factory("mmdb-f", "mmdb", map[string]any{
		"file": "/tmp/test.mmdb",
	})
	require.NoError(t, err)
	assert.Equal(t, "mmdb-f", src.ID())
	assert.Equal(t, sourcereg.TypeMMDB, src.Provider())
}

type mockReaderWithClose struct {
	closed bool
}

func (m *mockReaderWithClose) Networks(_ ...maxminddb.NetworksOption) *maxminddb.Networks {
	return nil
}

func (m *mockReaderWithClose) Close() error {
	m.closed = true
	return nil
}

func TestClose(t *testing.T) {
	p, err := New("mmdb-test", "mmdb", map[string]any{
		"file": "/tmp/test.mmdb",
	})
	require.NoError(t, err)

	mock := &mockReaderWithClose{}
	p.reader = mock

	require.NoError(t, p.Close())
	assert.True(t, mock.closed)
}

func TestLoad_Concurrent(t *testing.T) {
	p, err := New("mmdb-test", "mmdb", map[string]any{
		"file": "/tmp/test.mmdb",
	})
	require.NoError(t, err)

	p.prefixes = map[string][]netip.Prefix{
		"ru": {netip.MustParsePrefix("10.0.0.0/8")},
	}
	now := time.Now()
	p.loadedAt = &now

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = p.GetPrefixes("mmdb:ru")
		}()
	}
	wg.Wait()

	prefixes, err := p.GetPrefixes("mmdb:ru")
	require.NoError(t, err)
	assert.Len(t, prefixes, 1)
}

func TestLoad_CountriesWhitelist(t *testing.T) {
	p, err := New("mmdb-test", "mmdb", map[string]any{
		"file":      "/tmp/test.mmdb",
		"countries": []any{"ru"},
	})
	require.NoError(t, err)

	_, ipNet1, _ := net.ParseCIDR("1.2.3.0/24")
	_, ipNet2, _ := net.ParseCIDR("5.6.7.0/24")

	iter := &mockNetworkIterator{
		networks: []struct {
			ipNet  *net.IPNet
			record map[string]interface{}
		}{
			{ipNet1, map[string]interface{}{"country": map[string]interface{}{"iso_code": "ru"}}},
			{ipNet2, map[string]interface{}{"country": map[string]interface{}{"iso_code": "us"}}},
		},
	}

	require.NoError(t, p.loadFromIterator(iter))

	cats := p.Categories()
	require.Len(t, cats, 1)
	assert.Contains(t, cats, "mmdb:ru")
	assert.NotContains(t, cats, "mmdb:us")
}
