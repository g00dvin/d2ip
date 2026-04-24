package routing

import (
	"context"
	"testing"

	"github.com/goodvin/d2ip/internal/config"
)

func TestParseIPRouteShow_V4(t *testing.T) {
	raw := `default via 192.168.1.1 dev eth0
1.2.3.0/24 dev tun0 proto static
4.5.6.0/24 dev tun0 proto static scope link
blackhole 7.7.7.0/24
`
	got, err := parseIPRouteShow(raw, FamilyV4)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 prefixes, got %d: %v", len(got), got)
	}
	if got[0].String() != "1.2.3.0/24" || got[1].String() != "4.5.6.0/24" {
		t.Errorf("unexpected: %v", got)
	}
}

func TestParseIPRouteShow_V6FiltersV4(t *testing.T) {
	raw := `2001:db8::/32 dev tun0 proto static
1.2.3.0/24 dev tun0 proto static
`
	got, err := parseIPRouteShow(raw, FamilyV6)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].String() != "2001:db8::/32" {
		t.Errorf("got %v", got)
	}
}

func TestIPFam(t *testing.T) {
	if ipFam(FamilyV4) != "-4" {
		t.Error("v4")
	}
	if ipFam(FamilyV6) != "-6" {
		t.Error("v6")
	}
}

func TestListRoutes_NonexistentTable(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping: requires ip command")
	}
	r := &iproute2Router{
		cfg: config.PolicyConfig{TableID: 9999},
	}
	prefixes, err := r.listRoutes(context.Background(), FamilyV4)
	if err != nil {
		t.Fatalf("expected nil error for nonexistent table, got: %v", err)
	}
	if prefixes != nil {
		t.Fatalf("expected nil slice for nonexistent table, got: %v", prefixes)
	}
}
