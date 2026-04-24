package v2flygeosite

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/goodvin/d2ip/internal/sourcereg"
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

// TestProviderLoadAndGetDomains requires a real dlc.dat file.
// Use a minimal test fixture or skip if not available.
func TestProviderLoadAndGetDomains(t *testing.T) {
	// Create a minimal dlc.dat for testing.
	// Since we don't have a protobuf builder easily, we test Load failure.
	p, _ := New("id1", "geosite", map[string]any{
		"url":       "file:///nonexistent",
		"cache_path": filepath.Join(t.TempDir(), "dlc.dat"),
	})

	err := p.Load(context.Background())
	if err == nil {
		t.Fatal("expected error for nonexistent file URL")
	}
}
