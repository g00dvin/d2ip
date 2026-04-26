package mmdb

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"os"
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

func TestInfo(t *testing.T) {
	p, err := New("mmdb-test", "mmdb", map[string]any{
		"file": "/tmp/test.mmdb",
	})
	require.NoError(t, err)

	p.prefixes = map[string][]netip.Prefix{
		"ru": {netip.MustParsePrefix("1.2.3.0/24")},
	}
	now := time.Now()
	p.loadedAt = &now
	p.lastErr = "some error"

	info := p.Info()
	assert.Equal(t, "mmdb-test", info.ID)
	assert.Equal(t, "mmdb", info.Prefix)
	assert.Equal(t, string(sourcereg.TypeMMDB), info.Provider)
	assert.True(t, info.Enabled)
	assert.Equal(t, "some error", info.LastError)
	assert.NotNil(t, info.LastFetched)
	assert.Contains(t, info.Categories, "mmdb:ru")
}

func TestExtractCountry_EdgeCases(t *testing.T) {
	tests := []struct {
		name   string
		record interface{}
		want   string
	}{
		{"nil", nil, ""},
		{"string", "not a map", ""},
		{"no country", map[string]interface{}{"other": "value"}, ""},
		{"no iso_code", map[string]interface{}{"country": map[string]interface{}{"name": "Russia"}}, ""},
		{"non-string iso_code", map[string]interface{}{"country": map[string]interface{}{"iso_code": 123}}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractCountry(tt.record)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestNetipPrefixFromIPNet_Nil(t *testing.T) {
	prefix, ok := netipPrefixFromIPNet(nil)
	assert.False(t, ok)
	assert.Equal(t, netip.Prefix{}, prefix)
}

func TestNetipPrefixFromIPNet_InvalidIP(t *testing.T) {
	_, ipNet, _ := net.ParseCIDR("192.168.1.0/24")
	ipNet.IP = net.IP{1, 2, 3} // invalid length
	prefix, ok := netipPrefixFromIPNet(ipNet)
	assert.False(t, ok)
	assert.Equal(t, netip.Prefix{}, prefix)
}

func TestClose_NoReader(t *testing.T) {
	p, err := New("mmdb-test", "mmdb", map[string]any{
		"file": "/tmp/test.mmdb",
	})
	require.NoError(t, err)
	require.NoError(t, p.Close())
}

func TestDownloadToTemp_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("mmdb data"))
	}))
	defer srv.Close()

	path, err := downloadToTemp(context.Background(), srv.URL)
	require.NoError(t, err)
	defer os.Remove(path)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "mmdb data", string(data))
}

func TestDownloadToTemp_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	_, err := downloadToTemp(context.Background(), srv.URL)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "HTTP 404")
}

func TestLoad_BadURLDownload(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("not a valid mmdb file"))
	}))
	defer srv.Close()

	p, err := New("mmdb-test", "mmdb", map[string]any{
		"url": srv.URL,
	})
	require.NoError(t, err)

	err = p.Load(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mmdb: open")
}

func TestLoadFromIterator_SkipsInvalidPrefix(t *testing.T) {
	p, err := New("mmdb-test", "mmdb", map[string]any{
		"file": "/tmp/test.mmdb",
	})
	require.NoError(t, err)

	_, ipNet, _ := net.ParseCIDR("1.2.3.0/24")

	iter := &mockNetworkIterator{
		networks: []struct {
			ipNet  *net.IPNet
			record map[string]interface{}
		}{
			{nil, map[string]interface{}{"country": map[string]interface{}{"iso_code": "ru"}}},
			{&net.IPNet{IP: net.ParseIP("0.0.0.0"), Mask: net.CIDRMask(0, 0)}, map[string]interface{}{"country": map[string]interface{}{"iso_code": "us"}}},
			{ipNet, map[string]interface{}{"country": map[string]interface{}{"iso_code": ""}}},
		},
	}

	require.NoError(t, p.loadFromIterator(iter))
	assert.Empty(t, p.Categories())
}

func TestNew_CountriesWithInvalidElement(t *testing.T) {
	p, err := New("mmdb-test", "mmdb", map[string]any{
		"file":      "/tmp/test.mmdb",
		"countries": []any{"ru", 123, "us"},
	})
	require.NoError(t, err)
	assert.Equal(t, []string{"ru", "us"}, p.config.Countries)
}

func TestNew_CountriesNotSlice(t *testing.T) {
	p, err := New("mmdb-test", "mmdb", map[string]any{
		"file":      "/tmp/test.mmdb",
		"countries": "not-a-slice",
	})
	require.NoError(t, err)
	assert.Empty(t, p.config.Countries)
}

func TestLoad_ValidFile(t *testing.T) {
	p, err := New("mmdb-test", "mmdb", map[string]any{
		"file": "testdata/GeoIP2-Country-Test.mmdb",
	})
	require.NoError(t, err)

	err = p.Load(context.Background())
	require.NoError(t, err)

	cats := p.Categories()
	assert.NotEmpty(t, cats)

	// Get prefixes for a known country
	for _, cat := range cats {
		prefixes, err := p.GetPrefixes(cat)
		require.NoError(t, err)
		assert.NotEmpty(t, prefixes)
	}
}

func TestLoad_ValidURL(t *testing.T) {
	data, err := os.ReadFile("testdata/GeoIP2-Country-Test.mmdb")
	require.NoError(t, err)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
	}))
	defer srv.Close()

	p, err := New("mmdb-test", "mmdb", map[string]any{
		"url": srv.URL,
	})
	require.NoError(t, err)

	err = p.Load(context.Background())
	require.NoError(t, err)

	cats := p.Categories()
	assert.NotEmpty(t, cats)
}

func TestGetPrefixes_WrongPrefix(t *testing.T) {
	p, err := New("mmdb-test", "mmdb", map[string]any{
		"file": "/tmp/test.mmdb",
	})
	require.NoError(t, err)

	p.prefixes = map[string][]netip.Prefix{
		"ru": {netip.MustParsePrefix("1.2.3.0/24")},
	}
	now := time.Now()
	p.loadedAt = &now

	_, err = p.GetPrefixes("other:ru")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown category")
}
