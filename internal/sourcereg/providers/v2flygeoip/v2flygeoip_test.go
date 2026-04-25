package v2flygeoip

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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
