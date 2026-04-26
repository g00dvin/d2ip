package v2flygeosite

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/goodvin/d2ip/internal/domainlist/dlcpb"
	"github.com/goodvin/d2ip/internal/sourcereg"
	"google.golang.org/protobuf/proto"
)

func TestProviderNewDefaults(t *testing.T) {
	p, err := New("id1", "geosite", map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	if p.ID() != "id1" {
		t.Errorf("ID = %q", p.ID())
	}
	if p.Prefix() != "geosite" {
		t.Errorf("Prefix = %q", p.Prefix())
	}
	if p.Provider() != sourcereg.TypeV2flyGeosite {
		t.Errorf("Provider = %q", p.Provider())
	}
	if !p.IsDomainSource() {
		t.Error("expected IsDomainSource = true")
	}
	if p.IsPrefixSource() {
		t.Error("expected IsPrefixSource = false")
	}
}

func TestProviderNewInvalidDuration(t *testing.T) {
	_, err := New("id1", "geosite", map[string]any{"refresh_interval": "invalid"})
	if err == nil {
		t.Fatal("expected error for invalid duration")
	}
}

func TestProviderInfo(t *testing.T) {
	p, _ := New("id1", "geosite", map[string]any{})
	info := p.Info()
	if info.ID != "id1" {
		t.Errorf("Info.ID = %q", info.ID)
	}
	if info.Provider != string(sourcereg.TypeV2flyGeosite) {
		t.Errorf("Info.Provider = %q", info.Provider)
	}
	if info.Prefix != "geosite" {
		t.Errorf("Info.Prefix = %q", info.Prefix)
	}
}

func TestProviderInfo_AfterLoad(t *testing.T) {
	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "dlc.dat")

	list := &dlcpb.GeoSiteList{
		Entry: []*dlcpb.GeoSite{
			{
				CountryCode: "ru",
				Domain: []*dlcpb.Domain{
					{Type: dlcpb.Domain_Full, Value: "example.com"},
				},
			},
		},
	}
	data, err := proto.Marshal(list)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(cachePath, data, 0644))

	p, err := New("id1", "geosite", map[string]any{
		"cache_path":       cachePath,
		"refresh_interval": "0s",
	})
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, p.Load(ctx))

	info := p.Info()
	assert.Equal(t, "id1", info.ID)
	assert.NotNil(t, info.LastFetched)
	assert.Len(t, info.Categories, 1)
	assert.Equal(t, "geosite:ru", info.Categories[0])
}

func TestProviderCategoriesBeforeLoad(t *testing.T) {
	p, _ := New("id1", "geosite", map[string]any{})
	cats := p.Categories()
	if cats != nil {
		t.Errorf("expected nil before Load, got %v", cats)
	}
}

func TestProviderCategoriesAfterLoadEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "dlc.dat")

	list := &dlcpb.GeoSiteList{Entry: []*dlcpb.GeoSite{}}
	data, err := proto.Marshal(list)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(cachePath, data, 0644))

	p, err := New("id1", "geosite", map[string]any{
		"cache_path":       cachePath,
		"refresh_interval": "0s",
	})
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, p.Load(ctx))

	cats := p.Categories()
	assert.Empty(t, cats)
}

func TestProviderAsDomainSource(t *testing.T) {
	p, err := New("id1", "geosite", map[string]any{})
	require.NoError(t, err)
	assert.Equal(t, p, p.AsDomainSource())
}

func TestProviderAsPrefixSource(t *testing.T) {
	p, err := New("id1", "geosite", map[string]any{})
	require.NoError(t, err)
	assert.Nil(t, p.AsPrefixSource())
}

func TestProviderGetDomainsUnknownCategory(t *testing.T) {
	p, _ := New("id1", "geosite", map[string]any{})
	_, err := p.GetDomains("otherprefix:ru")
	if err == nil {
		t.Fatal("expected error for unknown category prefix")
	}
}

func TestProviderGetDomains_NotLoaded(t *testing.T) {
	p, err := New("id1", "geosite", map[string]any{})
	require.NoError(t, err)
	_, err = p.GetDomains("geosite:ru")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not loaded")
}

func TestProviderGetDomains_UnknownCategoryAfterLoad(t *testing.T) {
	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "dlc.dat")

	list := &dlcpb.GeoSiteList{
		Entry: []*dlcpb.GeoSite{
			{
				CountryCode: "ru",
				Domain: []*dlcpb.Domain{
					{Type: dlcpb.Domain_Full, Value: "example.com"},
				},
			},
		},
	}
	data, err := proto.Marshal(list)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(cachePath, data, 0644))

	p, err := New("id1", "geosite", map[string]any{
		"cache_path":       cachePath,
		"refresh_interval": "0s",
	})
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, p.Load(ctx))

	_, err = p.GetDomains("geosite:unknown")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown category")
}

func TestProviderLoadAndGetDomains(t *testing.T) {
	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "dlc.dat")

	list := &dlcpb.GeoSiteList{
		Entry: []*dlcpb.GeoSite{
			{
				CountryCode: "ru",
				Domain: []*dlcpb.Domain{
					{Type: dlcpb.Domain_Full, Value: "example.com"},
					{Type: dlcpb.Domain_RootDomain, Value: "example.org"},
					{Type: dlcpb.Domain_Plain, Value: "keyword"},
					{Type: dlcpb.Domain_Regex, Value: ".*"},
				},
			},
		},
	}
	data, err := proto.Marshal(list)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(cachePath, data, 0644))

	p, err := New("id1", "geosite", map[string]any{
		"cache_path":       cachePath,
		"refresh_interval": "0s",
	})
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, p.Load(ctx))

	domains, err := p.GetDomains("geosite:ru")
	require.NoError(t, err)
	require.Len(t, domains, 2)
	assert.Equal(t, "example.com", domains[0])
	assert.Equal(t, "example.org", domains[1])
}

func TestProviderLoad_StoreError(t *testing.T) {
	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "nonexistent.dat")

	p, err := New("id1", "geosite", map[string]any{
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

func TestProviderLoad_ParseError(t *testing.T) {
	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "dlc.dat")
	require.NoError(t, os.WriteFile(cachePath, []byte("invalid protobuf"), 0644))

	p, err := New("id1", "geosite", map[string]any{
		"cache_path":       cachePath,
		"refresh_interval": "0s",
	})
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err = p.Load(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "load domainlist")
}

func TestProviderClose(t *testing.T) {
	p, err := New("id1", "geosite", map[string]any{})
	require.NoError(t, err)
	assert.NoError(t, p.Close())
}

func TestFactory_Registered(t *testing.T) {
	factory := sourcereg.GetFactory(sourcereg.TypeV2flyGeosite)
	require.NotNil(t, factory, "v2flygeosite factory should be registered")

	src, err := factory("geosite-f", "geosite", map[string]any{})
	require.NoError(t, err)
	assert.Equal(t, "geosite-f", src.ID())
	assert.Equal(t, sourcereg.TypeV2flyGeosite, src.Provider())
}
