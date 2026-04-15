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
	cfg := config.RoutingConfig{
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
	r := &nftRouter{cfg: config.RoutingConfig{NFTTable: "inet d2ip", NFTSetV4: "d2ip_v4", NFTSetV6: "d2ip_v6"}}
	if s := r.buildScript(Plan{Family: FamilyV4}); s != "" {
		t.Errorf("want empty, got %q", s)
	}
}

func TestTableArgs_Default(t *testing.T) {
	r := &nftRouter{cfg: config.RoutingConfig{NFTTable: ""}}
	fam, name := r.tableArgs()
	if fam != "inet" || name != "d2ip" {
		t.Errorf("tableArgs = %q %q", fam, name)
	}
}
