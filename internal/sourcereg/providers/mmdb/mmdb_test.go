package mmdb

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/goodvin/d2ip/internal/sourcereg"
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
