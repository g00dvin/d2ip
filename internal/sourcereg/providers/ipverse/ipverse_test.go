package ipverse

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
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
