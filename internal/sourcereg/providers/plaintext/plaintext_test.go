package plaintext

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/goodvin/d2ip/internal/sourcereg"
)

func TestProviderNew(t *testing.T) {
	tests := []struct {
		name    string
		cfg     map[string]any
		wantErr bool
	}{
		{
			name: "valid domains",
			cfg:  map[string]any{"type": "domains", "file": "/tmp/test.txt"},
		},
		{
			name: "valid ips",
			cfg:  map[string]any{"type": "ips", "file": "/tmp/test.txt"},
		},
		{
			name:    "missing file",
			cfg:     map[string]any{"type": "domains"},
			wantErr: true,
		},
		{
			name:    "invalid type",
			cfg:     map[string]any{"type": "invalid", "file": "/tmp/test.txt"},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := New("id1", "corp", tt.cfg)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if p.ID() != "id1" {
				t.Errorf("ID = %q, want id1", p.ID())
			}
			if p.Prefix() != "corp" {
				t.Errorf("Prefix = %q, want corp", p.Prefix())
			}
		})
	}
}

func TestProviderLoadDomains(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "domains.txt")
	data := `# Comment line
example.com
  sub.example.com  
# Another comment
google.com

`
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}

	p, err := New("id1", "corp", map[string]any{"type": "domains", "file": path})
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Load(context.Background()); err != nil {
		t.Fatal(err)
	}

	domains, err := p.GetDomains("corp:default")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"example.com", "sub.example.com", "google.com"}
	if len(domains) != len(want) {
		t.Fatalf("got %d domains, want %d", len(domains), len(want))
	}
	for i, d := range want {
		if domains[i] != d {
			t.Errorf("domains[%d] = %q, want %q", i, domains[i], d)
		}
	}
}

func TestProviderLoadIPs(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "ips.txt")
	data := `192.168.1.0/24
10.0.0.1
2001:db8::/32
# comment
invalid-line
`
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}

	p, err := New("id1", "corp", map[string]any{"type": "ips", "file": path})
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Load(context.Background()); err != nil {
		t.Fatal(err)
	}

	prefixes, err := p.GetPrefixes("corp:default")
	if err != nil {
		t.Fatal(err)
	}
	if len(prefixes) != 3 {
		t.Fatalf("got %d prefixes, want 3", len(prefixes))
	}
	if prefixes[0].String() != "192.168.1.0/24" {
		t.Errorf("prefix[0] = %q", prefixes[0].String())
	}
	if prefixes[1].String() != "10.0.0.1/32" {
		t.Errorf("prefix[1] = %q", prefixes[1].String())
	}
	if prefixes[2].String() != "2001:db8::/32" {
		t.Errorf("prefix[2] = %q", prefixes[2].String())
	}
}

func TestProviderCategories(t *testing.T) {
	p, _ := New("id1", "streaming", map[string]any{"type": "domains", "file": "/tmp/x.txt"})
	cats := p.Categories()
	if len(cats) != 1 || cats[0] != "streaming:default" {
		t.Errorf("Categories = %v, want [streaming:default]", cats)
	}
}

func TestProviderInfo(t *testing.T) {
	p, _ := New("id1", "streaming", map[string]any{"type": "domains", "file": "/tmp/x.txt"})
	info := p.Info()
	if info.ID != "id1" {
		t.Errorf("Info.ID = %q", info.ID)
	}
	if info.Provider != string(sourcereg.TypePlaintext) {
		t.Errorf("Info.Provider = %q", info.Provider)
	}
	if info.Prefix != "streaming" {
		t.Errorf("Info.Prefix = %q", info.Prefix)
	}
}
