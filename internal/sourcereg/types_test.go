package sourcereg

import (
	"context"
	"net/netip"
)

// Compile-time interface checks.
var (
	_ Source   = (*mockSource)(nil)
	_ Registry = (*mockRegistry)(nil)
)

type mockSource struct{}

func (m *mockSource) ID() string                           { return "" }
func (m *mockSource) Prefix() string                       { return "" }
func (m *mockSource) Provider() SourceType                 { return "" }
func (m *mockSource) Load(ctx context.Context) error       { return nil }
func (m *mockSource) Close() error                         { return nil }
func (m *mockSource) Categories() []string                 { return nil }
func (m *mockSource) Info() SourceInfo                     { return SourceInfo{} }
func (m *mockSource) IsDomainSource() bool                 { return false }
func (m *mockSource) IsPrefixSource() bool                 { return false }
func (m *mockSource) AsDomainSource() DomainSource         { return nil }
func (m *mockSource) AsPrefixSource() PrefixSource         { return nil }

type mockRegistry struct{}

func (m *mockRegistry) AddSource(ctx context.Context, cfg SourceConfig) error       { return nil }
func (m *mockRegistry) RemoveSource(ctx context.Context, id string) error           { return nil }
func (m *mockRegistry) LoadAll(ctx context.Context) error                           { return nil }
func (m *mockRegistry) Close() error                                                { return nil }
func (m *mockRegistry) ListSources() []SourceInfo                                   { return nil }
func (m *mockRegistry) GetSource(id string) (Source, bool)                          { return nil, false }
func (m *mockRegistry) ListCategories() []CategoryInfo                              { return nil }
func (m *mockRegistry) GetDomains(category string) ([]string, error)                { return nil, nil }
func (m *mockRegistry) GetPrefixes(category string) ([]netip.Prefix, error)         { return nil, nil }
func (m *mockRegistry) ResolveCategory(category string) (string, string, bool)      { return "", "", false }
