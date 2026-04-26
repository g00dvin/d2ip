package domainlist

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/goodvin/d2ip/internal/domainlist/dlcpb"
	"google.golang.org/protobuf/proto"
)

func generateTestDLC(t *testing.T) []byte {
	t.Helper()
	list := &dlcpb.GeoSiteList{
		Entry: []*dlcpb.GeoSite{
			{
				CountryCode: "RU",
				Domain: []*dlcpb.Domain{
					{Type: dlcpb.Domain_Full, Value: "yandex.ru"},
					{Type: dlcpb.Domain_RootDomain, Value: "vk.com"},
				},
			},
			{
				CountryCode: "GOOGLE",
				Domain: []*dlcpb.Domain{
					{Type: dlcpb.Domain_Full, Value: "google.com",
						Attribute: []*dlcpb.Domain_Attribute{
							{Key: "@ads", TypedValue: &dlcpb.Domain_Attribute_BoolValue{BoolValue: true}},
						},
					},
					{Type: dlcpb.Domain_RootDomain, Value: "youtube.com"},
					{Type: dlcpb.Domain_Plain, Value: "goog"},
					{Type: dlcpb.Domain_Regex, Value: "^goog.*$"},
				},
			},
		},
	}
	data, err := proto.Marshal(list)
	if err != nil {
		t.Fatalf("marshal test dlc: %v", err)
	}
	return data
}

func writeTempDLC(t *testing.T, data []byte) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.dlc")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write temp dlc: %v", err)
	}
	return path
}

func TestProvider_Load(t *testing.T) {
	data := generateTestDLC(t)
	path := writeTempDLC(t, data)

	p := NewProvider()
	if err := p.Load(path); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	cats := p.Categories()
	if len(cats) != 2 {
		t.Errorf("Categories() len = %d, want 2", len(cats))
	}
}

func TestProvider_Categories(t *testing.T) {
	data := generateTestDLC(t)
	path := writeTempDLC(t, data)

	p := NewProvider()
	if err := p.Load(path); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	cats := p.Categories()
	want := []string{"google", "ru"}
	if len(cats) != len(want) {
		t.Fatalf("Categories() = %v, want %v", cats, want)
	}
	for i, c := range want {
		if cats[i] != c {
			t.Errorf("Categories()[%d] = %q, want %q", i, cats[i], c)
		}
	}
}

func TestProvider_Select_Basic(t *testing.T) {
	data := generateTestDLC(t)
	path := writeTempDLC(t, data)

	p := NewProvider()
	if err := p.Load(path); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	rules, err := p.Select([]CategorySelector{{Code: "google"}})
	if err != nil {
		t.Fatalf("Select() error = %v", err)
	}

	if len(rules) != 4 {
		t.Fatalf("Select() returned %d rules, want 4", len(rules))
	}

	want := []struct {
		typ   RuleType
		value string
	}{
		{RuleFull, "google.com"},
		{RuleRootDomain, "youtube.com"},
		{RulePlain, "goog"},
		{RuleRegex, "^goog.*$"},
	}

	for i, w := range want {
		if rules[i].Type != w.typ {
			t.Errorf("rule[%d].Type = %v, want %v", i, rules[i].Type, w.typ)
		}
		if rules[i].Value != w.value {
			t.Errorf("rule[%d].Value = %q, want %q", i, rules[i].Value, w.value)
		}
	}
}

func TestProvider_Select_WithAttrs(t *testing.T) {
	data := generateTestDLC(t)
	path := writeTempDLC(t, data)

	p := NewProvider()
	if err := p.Load(path); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	rules, err := p.Select([]CategorySelector{{Code: "google", Attrs: []string{"@ads"}}})
	if err != nil {
		t.Fatalf("Select() error = %v", err)
	}

	if len(rules) != 1 {
		t.Fatalf("Select() returned %d rules, want 1", len(rules))
	}

	if rules[0].Value != "google.com" {
		t.Errorf("rule[0].Value = %q, want %q", rules[0].Value, "google.com")
	}
	if rules[0].Type != RuleFull {
		t.Errorf("rule[0].Type = %v, want %v", rules[0].Type, RuleFull)
	}
}

func TestProvider_Select_UnknownCategory(t *testing.T) {
	data := generateTestDLC(t)
	path := writeTempDLC(t, data)

	p := NewProvider()
	if err := p.Load(path); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	_, err := p.Select([]CategorySelector{{Code: "unknown"}})
	if err == nil {
		t.Fatal("Select() expected error for unknown category, got nil")
	}
}

func TestProvider_Select_Deduplication(t *testing.T) {
	data := generateTestDLC(t)
	path := writeTempDLC(t, data)

	p := NewProvider()
	if err := p.Load(path); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	rules, err := p.Select([]CategorySelector{
		{Code: "google"},
		{Code: "google"},
	})
	if err != nil {
		t.Fatalf("Select() error = %v", err)
	}

	if len(rules) != 4 {
		t.Errorf("Select() returned %d rules, want 4 (deduplicated)", len(rules))
	}
}

func TestProvider_Select_Empty(t *testing.T) {
	data := generateTestDLC(t)
	path := writeTempDLC(t, data)

	p := NewProvider()
	if err := p.Load(path); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	rules, err := p.Select([]CategorySelector{})
	if err != nil {
		t.Fatalf("Select() error = %v", err)
	}

	if len(rules) != 0 {
		t.Errorf("Select() returned %d rules, want 0", len(rules))
	}
}

func TestConvertDomain_Normalization(t *testing.T) {
	domain := &dlcpb.Domain{
		Type:  dlcpb.Domain_Full,
		Value: "Example.COM",
	}
	rule := convertDomain(domain, "test")
	if rule.Value != "example.com" {
		t.Errorf("convertDomain() Value = %q, want %q", rule.Value, "example.com")
	}

	domain = &dlcpb.Domain{
		Type:  dlcpb.Domain_RootDomain,
		Value: "YouTube.COM",
	}
	rule = convertDomain(domain, "test")
	if rule.Value != "youtube.com" {
		t.Errorf("convertDomain() Value = %q, want %q", rule.Value, "youtube.com")
	}
}

func TestDeduplicate(t *testing.T) {
	rules := []Rule{
		{Type: RuleFull, Value: "example.com"},
		{Type: RuleFull, Value: "example.com"},
		{Type: RuleRootDomain, Value: "example.com"},
		{Type: RulePlain, Value: "example"},
	}

	result := deduplicate(rules)
	if len(result) != 3 {
		t.Errorf("deduplicate() returned %d rules, want 3", len(result))
	}

	// Verify unique rules kept.
	seen := make(map[string]bool)
	for _, r := range result {
		key := r.Type.String() + ":" + r.Value
		if seen[key] {
			t.Errorf("duplicate rule found: %v", r)
		}
		seen[key] = true
	}
}
