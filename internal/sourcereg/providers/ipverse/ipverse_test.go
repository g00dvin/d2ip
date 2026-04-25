package ipverse

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/goodvin/d2ip/internal/sourcereg"
)

func TestNew_ValidConfig(t *testing.T) {
	p, err := New("ipverse-1", "ipverse", map[string]any{
		"countries": []any{"ru", "us"},
	})
	require.NoError(t, err)
	assert.Equal(t, "ipverse-1", p.ID())
	assert.Equal(t, "ipverse", p.Prefix())
	assert.Equal(t, sourcereg.TypeIPverse, p.Provider())
	assert.True(t, p.IsPrefixSource())
	assert.False(t, p.IsDomainSource())
	assert.Nil(t, p.AsDomainSource())
	require.NotNil(t, p.AsPrefixSource())
}

func TestLoad_HappyPath(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		country := strings.TrimPrefix(r.URL.Path, "/")
		w.Header().Set("Content-Type", "text/plain")
		switch country {
		case "ru.zone":
			fmt.Fprintln(w, "1.2.3.0/24")
			fmt.Fprintln(w, "# comment")
			fmt.Fprintln(w, "")
			fmt.Fprintln(w, "2001:db8::/32")
		case "us.zone":
			fmt.Fprintln(w, "5.6.7.8")
			fmt.Fprintln(w, "bad-line")
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	p, err := New("ipverse-test", "ipverse", map[string]any{
		"base_url":  server.URL + "/{country}.zone",
		"countries": []any{"ru", "us"},
	})
	require.NoError(t, err)

	ctx := context.Background()
	require.NoError(t, p.Load(ctx))

	cats := p.Categories()
	require.Len(t, cats, 2)
	assert.Contains(t, cats, "ipverse:ru")
	assert.Contains(t, cats, "ipverse:us")

	ruPrefixes, err := p.GetPrefixes("ipverse:ru")
	require.NoError(t, err)
	require.Len(t, ruPrefixes, 2)

	usPrefixes, err := p.GetPrefixes("ipverse:us")
	require.NoError(t, err)
	require.Len(t, usPrefixes, 1)
	assert.Equal(t, "5.6.7.8/32", usPrefixes[0].String())
}

func TestLoad_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	p, err := New("ipverse-test", "ipverse", map[string]any{
		"base_url":  server.URL + "/{country}.zone",
		"countries": []any{"ru"},
	})
	require.NoError(t, err)

	ctx := context.Background()
	err = p.Load(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "HTTP 404")

	info := p.Info()
	assert.NotEmpty(t, info.LastError)
	assert.Nil(t, info.LastFetched)
}

func TestNew_EmptyCountries(t *testing.T) {
	_, err := New("ipverse-test", "ipverse", map[string]any{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "countries is required")
}

func TestGetPrefixes_NotLoaded(t *testing.T) {
	p, err := New("ipverse-test", "ipverse", map[string]any{
		"countries": []any{"ru"},
	})
	require.NoError(t, err)

	_, err = p.GetPrefixes("ipverse:ru")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not loaded")
}

func TestGetPrefixes_UnknownCategory(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "10.0.0.0/8")
	}))
	defer server.Close()

	p, err := New("ipverse-test", "ipverse", map[string]any{
		"base_url":  server.URL + "/{country}.zone",
		"countries": []any{"ru"},
	})
	require.NoError(t, err)

	ctx := context.Background()
	require.NoError(t, p.Load(ctx))

	_, err = p.GetPrefixes("ipverse:xx")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown country")
}

func TestFactory_Registered(t *testing.T) {
	factory := sourcereg.GetFactory(sourcereg.TypeIPverse)
	require.NotNil(t, factory, "ipverse factory should be registered")

	src, err := factory("ipverse-f", "ipverse", map[string]any{
		"countries": []any{"ru"},
	})
	require.NoError(t, err)
	assert.Equal(t, "ipverse-f", src.ID())
	assert.Equal(t, sourcereg.TypeIPverse, src.Provider())
}

func TestLoad_Concurrent(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		fmt.Fprintln(w, "10.0.0.0/8")
	}))
	defer server.Close()

	p, err := New("ipverse-test", "ipverse", map[string]any{
		"base_url":  server.URL + "/{country}.zone",
		"countries": []any{"ru"},
	})
	require.NoError(t, err)

	ctx := context.Background()
	require.NoError(t, p.Load(ctx))

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = p.GetPrefixes("ipverse:ru")
		}()
	}
	wg.Wait()

	// Should not panic or data race
	prefixes, err := p.GetPrefixes("ipverse:ru")
	require.NoError(t, err)
	assert.Len(t, prefixes, 1)
}
