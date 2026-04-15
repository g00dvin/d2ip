package routing

import (
	"context"
	"net/netip"
	"reflect"
	"strings"
	"testing"

	"github.com/goodvin/d2ip/internal/config"
)

func mustPrefix(t *testing.T, s string) netip.Prefix {
	t.Helper()
	p, err := netip.ParsePrefix(s)
	if err != nil {
		t.Fatalf("ParsePrefix(%q): %v", s, err)
	}
	return p
}

func TestComputePlan_SetDifference(t *testing.T) {
	cur := []netip.Prefix{
		mustPrefix(t, "1.2.3.0/24"),
		mustPrefix(t, "10.0.0.0/8"),
	}
	des := []netip.Prefix{
		mustPrefix(t, "10.0.0.0/8"),
		mustPrefix(t, "4.5.6.0/24"),
	}
	p := computePlan(cur, des, FamilyV4)
	wantAdd := []netip.Prefix{mustPrefix(t, "4.5.6.0/24")}
	wantRem := []netip.Prefix{mustPrefix(t, "1.2.3.0/24")}
	if !reflect.DeepEqual(p.Add, wantAdd) {
		t.Errorf("Add = %v, want %v", p.Add, wantAdd)
	}
	if !reflect.DeepEqual(p.Remove, wantRem) {
		t.Errorf("Remove = %v, want %v", p.Remove, wantRem)
	}
}

func TestComputePlan_IdempotentNoChange(t *testing.T) {
	cur := []netip.Prefix{mustPrefix(t, "1.2.3.0/24"), mustPrefix(t, "4.5.6.0/24")}
	des := []netip.Prefix{mustPrefix(t, "4.5.6.0/24"), mustPrefix(t, "1.2.3.0/24")}
	p := computePlan(cur, des, FamilyV4)
	if !p.Empty() {
		t.Errorf("expected empty plan, got Add=%v Remove=%v", p.Add, p.Remove)
	}
}

func TestComputePlan_DedupInputs(t *testing.T) {
	cur := []netip.Prefix{mustPrefix(t, "1.2.3.0/24"), mustPrefix(t, "1.2.3.0/24")}
	des := []netip.Prefix{mustPrefix(t, "1.2.3.0/24"), mustPrefix(t, "4.5.6.0/24"), mustPrefix(t, "4.5.6.0/24")}
	p := computePlan(cur, des, FamilyV4)
	if len(p.Add) != 1 || p.Add[0].String() != "4.5.6.0/24" {
		t.Errorf("Add = %v, want [4.5.6.0/24]", p.Add)
	}
	if len(p.Remove) != 0 {
		t.Errorf("Remove = %v, want []", p.Remove)
	}
}

func TestFilterByFamily(t *testing.T) {
	in := []netip.Prefix{
		mustPrefix(t, "1.2.3.0/24"),
		mustPrefix(t, "2001:db8::/32"),
	}
	v4 := filterByFamily(in, FamilyV4)
	v6 := filterByFamily(in, FamilyV6)
	if len(v4) != 1 || !v4[0].Addr().Is4() {
		t.Errorf("v4 filter = %v", v4)
	}
	if len(v6) != 1 || v6[0].Addr().Is4() {
		t.Errorf("v6 filter = %v", v6)
	}
}

func TestRenderDiff(t *testing.T) {
	p := Plan{
		Family: FamilyV4,
		Add:    []netip.Prefix{mustPrefix(t, "1.2.3.0/24")},
		Remove: []netip.Prefix{mustPrefix(t, "9.9.9.0/24")},
	}
	d := renderDiff(p)
	if !strings.Contains(d, "+ 1.2.3.0/24") || !strings.Contains(d, "- 9.9.9.0/24") {
		t.Errorf("diff missing markers: %q", d)
	}
}

func TestRenderDiff_Empty(t *testing.T) {
	if got := renderDiff(Plan{Family: FamilyV4}); !strings.Contains(got, "no changes") {
		t.Errorf("expected 'no changes', got %q", got)
	}
}

func TestNoopRouter_DisabledShortCircuits(t *testing.T) {
	r, err := New(config.RoutingConfig{Enabled: false})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := r.Caps(); err != nil {
		t.Errorf("Caps: %v", err)
	}
	p, _, err := r.DryRun(ctx, []netip.Prefix{mustPrefix(t, "1.2.3.0/24")}, FamilyV4)
	if err != nil {
		t.Fatal(err)
	}
	if !p.Empty() {
		t.Errorf("expected empty plan, got %v", p)
	}
	if err := r.Apply(ctx, Plan{}); err != nil {
		t.Errorf("Apply: %v", err)
	}
	if err := r.Rollback(ctx); err != nil {
		t.Errorf("Rollback: %v", err)
	}
	if s := r.Snapshot(); s.Backend != "none" {
		t.Errorf("Snapshot backend = %q, want none", s.Backend)
	}
}

func TestNew_UnknownBackend(t *testing.T) {
	_, err := New(config.RoutingConfig{Enabled: true, Backend: "bogus"})
	if err == nil {
		t.Fatal("expected error for unknown backend")
	}
}
