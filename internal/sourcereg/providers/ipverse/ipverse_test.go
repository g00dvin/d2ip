package ipverse

import (
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
