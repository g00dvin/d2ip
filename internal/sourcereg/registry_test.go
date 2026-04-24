package sourcereg_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/goodvin/d2ip/internal/cache"
	"github.com/goodvin/d2ip/internal/sourcereg"
	_ "github.com/goodvin/d2ip/internal/sourcereg/providers/plaintext"
	_ "github.com/goodvin/d2ip/internal/sourcereg/providers/v2flygeosite"
)

func newTestDB(t *testing.T) *cache.SQLiteCache {
	t.Helper()
	c, err := cache.Open(context.Background(), ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Close() })
	return c
}

func TestNewDBRegistry(t *testing.T) {
	db := newTestDB(t)
	reg, err := sourcereg.NewDBRegistry(db.DB())
	if err != nil {
		t.Fatal(err)
	}
	if reg == nil {
		t.Fatal("expected non-nil registry")
	}
	_ = reg.Close()
}

func TestAddAndListSources(t *testing.T) {
	db := newTestDB(t)
	reg, err := sourcereg.NewDBRegistry(db.DB())
	if err != nil {
		t.Fatal(err)
	}
	defer reg.Close()
	ctx := context.Background()

	// Add plaintext domain source
	tmpDir := t.TempDir()
	domainFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(domainFile, []byte("example.com\ngoogle.com\n"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := reg.AddSource(ctx, sourcereg.SourceConfig{
		ID:       "src1",
		Provider: sourcereg.TypePlaintext,
		Prefix:   "corp",
		Enabled:  true,
		Config: map[string]any{
			"type": "domains",
			"file": domainFile,
		},
	}); err != nil {
		t.Fatal(err)
	}

	sources := reg.ListSources()
	if len(sources) != 1 {
		t.Fatalf("expected 1 source, got %d", len(sources))
	}
	if sources[0].ID != "src1" {
		t.Errorf("ID = %q", sources[0].ID)
	}
	if sources[0].Prefix != "corp" {
		t.Errorf("Prefix = %q", sources[0].Prefix)
	}
}

func TestPrefixUniqueness(t *testing.T) {
	db := newTestDB(t)
	reg, err := sourcereg.NewDBRegistry(db.DB())
	if err != nil {
		t.Fatal(err)
	}
	defer reg.Close()
	ctx := context.Background()

	if err := reg.AddSource(ctx, sourcereg.SourceConfig{
		ID:       "src1",
		Provider: sourcereg.TypePlaintext,
		Prefix:   "geoip",
		Enabled:  true,
		Config:   map[string]any{"type": "domains", "file": "/tmp/x.txt"},
	}); err != nil {
		t.Fatal(err)
	}

	// Same prefix should fail due to UNIQUE constraint
	err = reg.AddSource(ctx, sourcereg.SourceConfig{
		ID:       "src2",
		Provider: sourcereg.TypePlaintext,
		Prefix:   "geoip",
		Enabled:  true,
		Config:   map[string]any{"type": "domains", "file": "/tmp/y.txt"},
	})
	if err == nil {
		t.Fatal("expected error for duplicate prefix")
	}
}

func TestLoadAllAndCategories(t *testing.T) {
	db := newTestDB(t)
	reg, err := sourcereg.NewDBRegistry(db.DB())
	if err != nil {
		t.Fatal(err)
	}
	defer reg.Close()
	ctx := context.Background()

	tmpDir := t.TempDir()
	domainFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(domainFile, []byte("example.com\ngoogle.com\n"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := reg.AddSource(ctx, sourcereg.SourceConfig{
		ID:       "src1",
		Provider: sourcereg.TypePlaintext,
		Prefix:   "corp",
		Enabled:  true,
		Config:   map[string]any{"type": "domains", "file": domainFile},
	}); err != nil {
		t.Fatal(err)
	}

	if err := reg.LoadAll(ctx); err != nil {
		t.Fatal(err)
	}

	cats := reg.ListCategories()
	if len(cats) != 1 {
		t.Fatalf("expected 1 category, got %d", len(cats))
	}
	if cats[0].Name != "corp:default" {
		t.Errorf("Name = %q", cats[0].Name)
	}
	if cats[0].Type != sourcereg.CategoryDomain {
		t.Errorf("Type = %q", cats[0].Type)
	}
	if cats[0].Count != 2 {
		t.Errorf("Count = %d", cats[0].Count)
	}
}

func TestGetDomains(t *testing.T) {
	db := newTestDB(t)
	reg, err := sourcereg.NewDBRegistry(db.DB())
	if err != nil {
		t.Fatal(err)
	}
	defer reg.Close()
	ctx := context.Background()

	tmpDir := t.TempDir()
	domainFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(domainFile, []byte("example.com\ngoogle.com\n"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := reg.AddSource(ctx, sourcereg.SourceConfig{
		ID:       "src1",
		Provider: sourcereg.TypePlaintext,
		Prefix:   "corp",
		Enabled:  true,
		Config:   map[string]any{"type": "domains", "file": domainFile},
	}); err != nil {
		t.Fatal(err)
	}
	if err := reg.LoadAll(ctx); err != nil {
		t.Fatal(err)
	}

	domains, err := reg.GetDomains("corp:default")
	if err != nil {
		t.Fatal(err)
	}
	if len(domains) != 2 {
		t.Fatalf("expected 2 domains, got %d", len(domains))
	}
}

func TestGetPrefixes(t *testing.T) {
	db := newTestDB(t)
	reg, err := sourcereg.NewDBRegistry(db.DB())
	if err != nil {
		t.Fatal(err)
	}
	defer reg.Close()
	ctx := context.Background()

	tmpDir := t.TempDir()
	ipFile := filepath.Join(tmpDir, "ips.txt")
	if err := os.WriteFile(ipFile, []byte("192.168.1.0/24\n10.0.0.1\n"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := reg.AddSource(ctx, sourcereg.SourceConfig{
		ID:       "src1",
		Provider: sourcereg.TypePlaintext,
		Prefix:   "corp",
		Enabled:  true,
		Config:   map[string]any{"type": "ips", "file": ipFile},
	}); err != nil {
		t.Fatal(err)
	}
	if err := reg.LoadAll(ctx); err != nil {
		t.Fatal(err)
	}

	prefixes, err := reg.GetPrefixes("corp:default")
	if err != nil {
		t.Fatal(err)
	}
	if len(prefixes) != 2 {
		t.Fatalf("expected 2 prefixes, got %d", len(prefixes))
	}
}

func TestResolveCategory(t *testing.T) {
	db := newTestDB(t)
	reg, err := sourcereg.NewDBRegistry(db.DB())
	if err != nil {
		t.Fatal(err)
	}
	defer reg.Close()
	ctx := context.Background()

	if err := reg.AddSource(ctx, sourcereg.SourceConfig{
		ID:       "src1",
		Provider: sourcereg.TypePlaintext,
		Prefix:   "corp",
		Enabled:  true,
		Config:   map[string]any{"type": "domains", "file": "/tmp/x.txt"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := reg.LoadAll(ctx); err != nil {
		t.Fatal(err)
	}

	srcID, catType, found := reg.ResolveCategory("corp:default")
	if !found {
		t.Fatal("expected to find category")
	}
	if srcID != "src1" {
		t.Errorf("sourceID = %q", srcID)
	}
	if catType != string(sourcereg.CategoryDomain) {
		t.Errorf("catType = %q", catType)
	}
}

func TestRemoveSource(t *testing.T) {
	db := newTestDB(t)
	reg, err := sourcereg.NewDBRegistry(db.DB())
	if err != nil {
		t.Fatal(err)
	}
	defer reg.Close()
	ctx := context.Background()

	if err := reg.AddSource(ctx, sourcereg.SourceConfig{
		ID:       "src1",
		Provider: sourcereg.TypePlaintext,
		Prefix:   "corp",
		Enabled:  true,
		Config:   map[string]any{"type": "domains", "file": "/tmp/x.txt"},
	}); err != nil {
		t.Fatal(err)
	}

	if err := reg.RemoveSource(ctx, "src1"); err != nil {
		t.Fatal(err)
	}

	if _, ok := reg.GetSource("src1"); ok {
		t.Error("expected source to be removed")
	}
}
