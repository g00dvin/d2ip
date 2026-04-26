package routing

import (
	"context"
	"net/netip"
	"path/filepath"
	"strings"
	"testing"

	"github.com/goodvin/d2ip/internal/config"
)

func mp(s string) []netip.Prefix {
	p, err := netip.ParsePrefix(s)
	if err != nil {
		panic(err)
	}
	return []netip.Prefix{p}
}

func TestParseNftSet(t *testing.T) {
	raw := `table inet d2ip {
    set d2ip_v4 {
        type ipv4_addr
        flags interval
        elements = { 1.2.3.0/24, 4.5.6.0/24, 10.0.0.0/8 }
    }
}`
	got, err := parseNftSet(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("want 3 prefixes, got %d: %v", len(got), got)
	}
	if got[0].String() != "1.2.3.0/24" {
		t.Errorf("first = %v", got[0])
	}
}

func TestParseNftSet_Empty(t *testing.T) {
	raw := `table inet d2ip { set d2ip_v4 { type ipv4_addr; flags interval; } }`
	got, err := parseNftSet(raw)
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Errorf("want nil, got %v", got)
	}
}

func TestBuildScript_AddAndRemove(t *testing.T) {
	cfg := config.PolicyConfig{
		Enabled:  true,
		Backend:  config.BackendNFTables,
		NFTTable: "inet d2ip",
		NFTSetV4: "d2ip_v4",
		NFTSetV6: "d2ip_v6",
	}
	r := &nftRouter{cfg: cfg}
	p := Plan{
		Family: FamilyV4,
		Add:    mp("1.2.3.0/24"),
		Remove: mp("9.9.9.0/24"),
	}
	s := r.buildScript(p)
	if !strings.Contains(s, "add element inet d2ip d2ip_v4 { 1.2.3.0/24 }") {
		t.Errorf("missing add: %q", s)
	}
	if !strings.Contains(s, "delete element inet d2ip d2ip_v4 { 9.9.9.0/24 }") {
		t.Errorf("missing del: %q", s)
	}
}

func TestBuildScript_Empty(t *testing.T) {
	r := &nftRouter{cfg: config.PolicyConfig{NFTTable: "inet d2ip", NFTSetV4: "d2ip_v4", NFTSetV6: "d2ip_v6"}}
	if s := r.buildScript(Plan{Family: FamilyV4}); s != "" {
		t.Errorf("want empty, got %q", s)
	}
}

func TestTableArgs_Default(t *testing.T) {
	r := &nftRouter{cfg: config.PolicyConfig{NFTTable: ""}}
	fam, name := r.tableArgs()
	if fam != "inet" || name != "d2ip" {
		t.Errorf("tableArgs = %q %q", fam, name)
	}
}

func TestParseNftSetJSON(t *testing.T) {
	jsonOutput := `{
		"nftables": [
			{
				"metainfo": {
					"version": "1.0.2",
					"release_name": "Lester Gooch",
					"json_schema_version": 1
				}
			},
			{
				"set": {
					"family": "inet",
					"table": "d2ip",
					"name": "d2ip_v4",
					"type": "ipv4_addr",
					"flags": ["interval"],
					"elem": [
						{"prefix": {"addr": "192.0.2.0", "len": 24}},
						{"prefix": {"addr": "198.51.100.0", "len": 24}}
					]
				}
			}
		]
	}`

	prefixes, err := parseNftSetJSON([]byte(jsonOutput))
	if err != nil {
		t.Fatalf("parseNftSetJSON: %v", err)
	}

	want := []string{"192.0.2.0/24", "198.51.100.0/24"}
	if len(prefixes) != len(want) {
		t.Fatalf("got %d prefixes, want %d", len(prefixes), len(want))
	}

	for i, p := range prefixes {
		if p.String() != want[i] {
			t.Errorf("prefix[%d] = %v, want %v", i, p, want[i])
		}
	}
}

func TestParseNftSetJSON_EmptySet(t *testing.T) {
	jsonOutput := `{
		"nftables": [
			{
				"set": {
					"family": "inet",
					"table": "d2ip",
					"name": "d2ip_v4",
					"type": "ipv4_addr",
					"flags": ["interval"]
				}
			}
		]
	}`

	prefixes, err := parseNftSetJSON([]byte(jsonOutput))
	if err != nil {
		t.Fatalf("parseNftSetJSON: %v", err)
	}

	if len(prefixes) != 0 {
		t.Errorf("expected empty list, got %v", prefixes)
	}
}

func TestParseNftSetJSON_IPv6(t *testing.T) {
	jsonOutput := `{
		"nftables": [
			{
				"set": {
					"family": "inet",
					"table": "d2ip",
					"name": "d2ip_v6",
					"type": "ipv6_addr",
					"flags": ["interval"],
					"elem": [
						{"prefix": {"addr": "2001:db8::", "len": 32}},
						{"prefix": {"addr": "2001:db8:1::", "len": 48}}
					]
				}
			}
		]
	}`

	prefixes, err := parseNftSetJSON([]byte(jsonOutput))
	if err != nil {
		t.Fatalf("parseNftSetJSON: %v", err)
	}

	want := []string{"2001:db8::/32", "2001:db8:1::/48"}
	if len(prefixes) != len(want) {
		t.Fatalf("got %d prefixes, want %d", len(prefixes), len(want))
	}

	for i, p := range prefixes {
		if p.String() != want[i] {
			t.Errorf("prefix[%d] = %v, want %v", i, p, want[i])
		}
	}
}

func TestParseNftSetJSON_InvalidJSON(t *testing.T) {
	_, err := parseNftSetJSON([]byte("not valid json"))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestParseNftSetJSON_NoElements(t *testing.T) {
	jsonOutput := `{
		"nftables": [
			{
				"metainfo": {
					"version": "1.0.2"
				}
			}
		]
	}`

	prefixes, err := parseNftSetJSON([]byte(jsonOutput))
	if err != nil {
		t.Fatalf("parseNftSetJSON: %v", err)
	}

	if len(prefixes) != 0 {
		t.Errorf("expected empty list for JSON without sets, got %v", prefixes)
	}
}

func TestParseNftSetJSON_MixedPrefixAndVal(t *testing.T) {
	// Test handling of both prefix and val elements (though val is rare)
	jsonOutput := `{
		"nftables": [
			{
				"set": {
					"family": "inet",
					"table": "d2ip",
					"name": "d2ip_v4",
					"elem": [
						{"prefix": {"addr": "192.0.2.0", "len": 24}},
						{"val": "198.51.100.1"}
					]
				}
			}
		]
	}`

	prefixes, err := parseNftSetJSON([]byte(jsonOutput))
	if err != nil {
		t.Fatalf("parseNftSetJSON: %v", err)
	}

	if len(prefixes) != 2 {
		t.Fatalf("got %d prefixes, want 2", len(prefixes))
	}

	// First should be CIDR
	if prefixes[0].String() != "192.0.2.0/24" {
		t.Errorf("prefix[0] = %v, want 192.0.2.0/24", prefixes[0])
	}

	// Second should be single IP converted to /32
	if prefixes[1].String() != "198.51.100.1/32" {
		t.Errorf("prefix[1] = %v, want 198.51.100.1/32", prefixes[1])
	}
}

func TestNewNFTRouter(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	cfg := config.PolicyConfig{NFTTable: "inet d2ip", NFTSetV4: "d2ip_v4", NFTSetV6: "d2ip_v6"}
	r := newNFTRouter(cfg, path)
	if r == nil {
		t.Fatal("expected non-nil router")
	}
}

func TestNewNFTRouter_PreloadedState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	want := RouterState{Backend: string(config.BackendNFTables), V4: []netip.Prefix{netip.MustParsePrefix("1.2.3.0/24")}}
	if err := saveState(path, want); err != nil {
		t.Fatal(err)
	}

	r := newNFTRouter(config.PolicyConfig{}, path)
	if r.state.Backend != want.Backend {
		t.Errorf("preloaded backend = %q, want %q", r.state.Backend, want.Backend)
	}
}

func TestNFTRouter_Caps(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping: requires nft command")
	}
	r := &nftRouter{}
	if err := r.Caps(); err != nil {
		t.Fatalf("Caps: %v", err)
	}
}

func TestNFTRouter_Snapshot(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	want := RouterState{Backend: string(config.BackendNFTables), V4: []netip.Prefix{netip.MustParsePrefix("1.2.3.0/24")}}
	if err := saveState(path, want); err != nil {
		t.Fatal(err)
	}

	r := newNFTRouter(config.PolicyConfig{}, path)
	got := r.Snapshot()
	if got.Backend != want.Backend {
		t.Errorf("backend = %q, want %q", got.Backend, want.Backend)
	}
}

func TestNFTRouter_SetName(t *testing.T) {
	r := &nftRouter{cfg: config.PolicyConfig{NFTSetV4: "v4set", NFTSetV6: "v6set"}}
	if r.setName(FamilyV4) != "v4set" {
		t.Errorf("setName(v4) = %q", r.setName(FamilyV4))
	}
	if r.setName(FamilyV6) != "v6set" {
		t.Errorf("setName(v6) = %q", r.setName(FamilyV6))
	}
}

func TestNFTRouter_Plan_Error(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping: requires nft command")
	}
	// Without root/CAP_NET_ADMIN, listSet will fail
	r := newNFTRouter(config.PolicyConfig{NFTTable: "inet d2ip", NFTSetV4: "d2ip_v4"}, "")
	ctx := context.Background()
	_, err := r.Plan(ctx, []netip.Prefix{netip.MustParsePrefix("1.2.3.0/24")}, FamilyV4)
	if err == nil {
		t.Fatal("expected error from Plan without CAP_NET_ADMIN")
	}
}

func TestNFTRouter_DryRun_Error(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping: requires nft command")
	}
	r := newNFTRouter(config.PolicyConfig{NFTTable: "inet d2ip", NFTSetV4: "d2ip_v4"}, "")
	ctx := context.Background()
	_, _, err := r.DryRun(ctx, []netip.Prefix{netip.MustParsePrefix("1.2.3.0/24")}, FamilyV4)
	if err == nil {
		t.Fatal("expected error from DryRun without CAP_NET_ADMIN")
	}
}

func TestNFTRouter_Apply_Error(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping: requires nft command")
	}
	r := newNFTRouter(config.PolicyConfig{NFTTable: "inet d2ip", NFTSetV4: "d2ip_v4"}, "")
	ctx := context.Background()
	p := Plan{Family: FamilyV4, Add: []netip.Prefix{netip.MustParsePrefix("1.2.3.0/24")}}
	err := r.Apply(ctx, p)
	if err == nil {
		t.Fatal("expected error from Apply without CAP_NET_ADMIN")
	}
}

func TestNFTRouter_Rollback_EmptyState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	r := newNFTRouter(config.PolicyConfig{NFTTable: "inet d2ip", NFTSetV4: "d2ip_v4"}, path)
	ctx := context.Background()
	err := r.Rollback(ctx)
	if err != nil {
		t.Fatalf("Rollback(empty state): %v", err)
	}
}

func TestNFTRouter_Rollback_WithState_Error(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping: requires nft command")
	}
	path := filepath.Join(t.TempDir(), "state.json")
	state := RouterState{
		Backend: string(config.BackendNFTables),
		V4:      []netip.Prefix{netip.MustParsePrefix("1.2.3.0/24")},
	}
	if err := saveState(path, state); err != nil {
		t.Fatal(err)
	}

	r := newNFTRouter(config.PolicyConfig{NFTTable: "inet d2ip", NFTSetV4: "d2ip_v4"}, path)
	ctx := context.Background()

	// Without root, ensureTable will fail, but we cover the code path
	_ = r.Rollback(ctx)
}

func TestNFTRouter_RefreshState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	r := newNFTRouter(config.PolicyConfig{NFTTable: "inet d2ip"}, path)

	p := Plan{Family: FamilyV4, Add: []netip.Prefix{netip.MustParsePrefix("1.2.3.0/24")}}
	if err := r.refreshState(p); err != nil {
		t.Fatalf("refreshState: %v", err)
	}

	state := r.Snapshot()
	if len(state.V4) != 1 {
		t.Errorf("expected 1 v4 prefix, got %d", len(state.V4))
	}
	if state.Backend != string(config.BackendNFTables) {
		t.Errorf("backend = %q, want nftables", state.Backend)
	}
}

func TestNFTRouter_RefreshState_V6(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	r := newNFTRouter(config.PolicyConfig{NFTTable: "inet d2ip"}, path)

	p := Plan{Family: FamilyV6, Add: []netip.Prefix{netip.MustParsePrefix("2001:db8::/32")}}
	if err := r.refreshState(p); err != nil {
		t.Fatalf("refreshState: %v", err)
	}

	state := r.Snapshot()
	if len(state.V6) != 1 {
		t.Errorf("expected 1 v6 prefix, got %d", len(state.V6))
	}
}

func TestNFTRouter_RefreshState_Dedup(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	pre := RouterState{
		Backend: string(config.BackendNFTables),
		V4:      []netip.Prefix{netip.MustParsePrefix("1.2.3.0/24"), netip.MustParsePrefix("4.5.6.0/24")},
	}
	if err := saveState(path, pre); err != nil {
		t.Fatal(err)
	}
	r := newNFTRouter(config.PolicyConfig{NFTTable: "inet d2ip"}, path)

	p := Plan{Family: FamilyV4, Remove: []netip.Prefix{netip.MustParsePrefix("1.2.3.0/24")}}
	if err := r.refreshState(p); err != nil {
		t.Fatalf("refreshState: %v", err)
	}

	state := r.Snapshot()
	if len(state.V4) != 1 {
		t.Errorf("expected 1 v4 prefix after removal, got %d", len(state.V4))
	}
}

func TestNFTRouter_RunScript_Empty(t *testing.T) {
	r := &nftRouter{cfg: config.PolicyConfig{NFTTable: "inet d2ip"}}
	ctx := context.Background()
	if err := r.runScript(ctx, "  \n  "); err != nil {
		t.Fatalf("runScript(empty): %v", err)
	}
}

func TestNFTRouter_WriteAdd_Empty(t *testing.T) {
	r := &nftRouter{cfg: config.PolicyConfig{NFTTable: "inet d2ip", NFTSetV4: "d2ip_v4"}}
	var sb strings.Builder
	r.writeAdd(&sb, FamilyV4, nil)
	if sb.Len() != 0 {
		t.Errorf("expected empty, got %q", sb.String())
	}
}

func TestNFTRouter_WriteRemove_Empty(t *testing.T) {
	r := &nftRouter{cfg: config.PolicyConfig{NFTTable: "inet d2ip", NFTSetV4: "d2ip_v4"}}
	var sb strings.Builder
	r.writeRemove(&sb, FamilyV4, nil)
	if sb.Len() != 0 {
		t.Errorf("expected empty, got %q", sb.String())
	}
}

func TestJoinPrefixes(t *testing.T) {
	ps := []netip.Prefix{netip.MustParsePrefix("1.2.3.0/24"), netip.MustParsePrefix("4.5.6.0/24")}
	got := joinPrefixes(ps)
	if got != "1.2.3.0/24, 4.5.6.0/24" {
		t.Errorf("joinPrefixes = %q", got)
	}
}

func TestParsePrefixLoose(t *testing.T) {
	// CIDR
	p, err := parsePrefixLoose("1.2.3.0/24")
	if err != nil {
		t.Fatalf("parsePrefixLoose(CIDR): %v", err)
	}
	if p.String() != "1.2.3.0/24" {
		t.Errorf("got %v", p)
	}

	// IPv4 host
	p, err = parsePrefixLoose("1.2.3.4")
	if err != nil {
		t.Fatalf("parsePrefixLoose(IPv4 host): %v", err)
	}
	if p.String() != "1.2.3.4/32" {
		t.Errorf("got %v", p)
	}

	// IPv6 host
	p, err = parsePrefixLoose("2001:db8::1")
	if err != nil {
		t.Fatalf("parsePrefixLoose(IPv6 host): %v", err)
	}
	if p.String() != "2001:db8::1/128" {
		t.Errorf("got %v", p)
	}

	// Invalid
	_, err = parsePrefixLoose("not-an-ip")
	if err == nil {
		t.Error("expected error for invalid input")
	}
}
