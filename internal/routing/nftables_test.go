package routing

import (
	"net/netip"
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
