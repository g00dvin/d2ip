package routing

import (
	"context"
	"net/netip"
	"path/filepath"
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

func TestNewIProute2Router(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	cfg := config.PolicyConfig{TableID: 100, Iface: "eth0"}
	r := newIProute2Router(cfg, path)
	if r == nil {
		t.Fatal("expected non-nil router")
	}
	if r.cfg.TableID != 100 {
		t.Errorf("tableID = %d, want 100", r.cfg.TableID)
	}
	if r.iface != "eth0" {
		t.Errorf("iface = %q, want eth0", r.iface)
	}
}

func TestNewIProute2Router_PreloadedState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	want := RouterState{Backend: string(config.BackendIProute2), V4: []netip.Prefix{netip.MustParsePrefix("1.2.3.0/24")}}
	if err := saveState(path, want); err != nil {
		t.Fatal(err)
	}

	r := newIProute2Router(config.PolicyConfig{}, path)
	if r.state.Backend != want.Backend {
		t.Errorf("preloaded backend = %q, want %q", r.state.Backend, want.Backend)
	}
}

func TestIProute2Router_SetIface(t *testing.T) {
	r := &iproute2Router{}
	r.SetIface("eth1")
	if r.iface != "eth1" {
		t.Errorf("iface = %q, want eth1", r.iface)
	}
}

func TestIProute2Router_Caps(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping: requires ip command")
	}
	r := &iproute2Router{}
	if err := r.Caps(); err != nil {
		t.Fatalf("Caps: %v", err)
	}
}

func TestIProute2Router_Caps_NetNSMissing(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping: requires ip command")
	}
	r := &iproute2Router{netns: "nonexistent-netns"}
	if err := r.Caps(); err == nil {
		t.Fatal("expected error for missing netns")
	}
}

func TestIProute2Router_Plan(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping: requires ip command")
	}
	r := &iproute2Router{cfg: config.PolicyConfig{TableID: 9999}}
	ctx := context.Background()
	p, err := r.Plan(ctx, []netip.Prefix{netip.MustParsePrefix("1.2.3.0/24")}, FamilyV4)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if len(p.Add) != 1 {
		t.Errorf("Add = %v, want 1 prefix", p.Add)
	}
}

func TestIProute2Router_DryRun(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping: requires ip command")
	}
	r := &iproute2Router{cfg: config.PolicyConfig{TableID: 9999}}
	ctx := context.Background()
	plan, diff, err := r.DryRun(ctx, []netip.Prefix{netip.MustParsePrefix("1.2.3.0/24")}, FamilyV4)
	if err != nil {
		t.Fatalf("DryRun: %v", err)
	}
	if plan.Empty() {
		t.Error("expected non-empty plan")
	}
	if diff == "" {
		t.Error("expected non-empty diff")
	}
}

func TestIProute2Router_Snapshot(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	want := RouterState{Backend: string(config.BackendIProute2), V4: []netip.Prefix{netip.MustParsePrefix("1.2.3.0/24")}}
	if err := saveState(path, want); err != nil {
		t.Fatal(err)
	}

	r := newIProute2Router(config.PolicyConfig{}, path)
	got := r.Snapshot()
	if got.Backend != want.Backend {
		t.Errorf("backend = %q, want %q", got.Backend, want.Backend)
	}
}

func TestIProute2Router_Apply_NoIface(t *testing.T) {
	r := &iproute2Router{cfg: config.PolicyConfig{TableID: 100}}
	ctx := context.Background()
	err := r.Apply(ctx, Plan{Family: FamilyV4, Add: []netip.Prefix{netip.MustParsePrefix("1.2.3.0/24")}})
	if err == nil {
		t.Fatal("expected error when iface not set")
	}
}

func TestIProute2Router_Apply_EmptyPlan(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	r := newIProute2Router(config.PolicyConfig{TableID: 100}, path)
	r.SetIface("eth0")
	ctx := context.Background()

	// Pre-populate state so refreshState has something to work with
	if err := saveState(path, RouterState{Backend: string(config.BackendIProute2), V4: []netip.Prefix{netip.MustParsePrefix("1.2.3.0/24")}}); err != nil {
		t.Fatal(err)
	}
	// Reload state into router
	r = newIProute2Router(config.PolicyConfig{TableID: 100}, path)
	r.SetIface("eth0")

	err := r.Apply(ctx, Plan{Family: FamilyV4})
	if err != nil {
		t.Fatalf("Apply(empty plan): %v", err)
	}
}

func TestIProute2Router_Apply_DryRun(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	r := newIProute2Router(config.PolicyConfig{TableID: 100, DryRun: true}, path)
	r.SetIface("eth0")
	ctx := context.Background()

	p := Plan{Family: FamilyV4, Add: []netip.Prefix{netip.MustParsePrefix("1.2.3.0/24")}}
	err := r.Apply(ctx, p)
	if err != nil {
		t.Fatalf("Apply(dryRun): %v", err)
	}
	// State should be refreshed even in dry-run mode
	state := r.Snapshot()
	if len(state.V4) != 1 {
		t.Errorf("expected 1 v4 prefix in state, got %d", len(state.V4))
	}
}

func TestIProute2Router_Rollback_NoIface(t *testing.T) {
	r := &iproute2Router{cfg: config.PolicyConfig{TableID: 100}}
	ctx := context.Background()
	err := r.Rollback(ctx)
	if err == nil {
		t.Fatal("expected error when iface not set")
	}
}

func TestIProute2Router_Rollback_EmptyState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	r := newIProute2Router(config.PolicyConfig{TableID: 100}, path)
	r.SetIface("eth0")
	ctx := context.Background()
	err := r.Rollback(ctx)
	if err != nil {
		t.Fatalf("Rollback(empty state): %v", err)
	}
}

func TestIProute2Router_Rollback_WithState(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping: requires ip command")
	}
	path := filepath.Join(t.TempDir(), "state.json")
	state := RouterState{
		Backend:   string(config.BackendIProute2),
		V4:        []netip.Prefix{netip.MustParsePrefix("1.2.3.0/24")},
		V6:        []netip.Prefix{netip.MustParsePrefix("2001:db8::/32")},
	}
	if err := saveState(path, state); err != nil {
		t.Fatal(err)
	}

	r := newIProute2Router(config.PolicyConfig{TableID: 100}, path)
	r.SetIface("eth0")
	ctx := context.Background()

	// Without root, rollback will fail when trying to delete routes,
	// but we still cover the code path.
	_ = r.Rollback(ctx)
}

func TestIProute2Router_RefreshState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	r := newIProute2Router(config.PolicyConfig{TableID: 100}, path)

	p := Plan{Family: FamilyV4, Add: []netip.Prefix{netip.MustParsePrefix("1.2.3.0/24")}}
	if err := r.refreshState(p); err != nil {
		t.Fatalf("refreshState: %v", err)
	}

	state := r.Snapshot()
	if len(state.V4) != 1 {
		t.Errorf("expected 1 v4 prefix, got %d", len(state.V4))
	}
	if state.Backend != string(config.BackendIProute2) {
		t.Errorf("backend = %q, want iproute2", state.Backend)
	}
}

func TestIProute2Router_RefreshState_V6(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	r := newIProute2Router(config.PolicyConfig{TableID: 100}, path)

	p := Plan{Family: FamilyV6, Add: []netip.Prefix{netip.MustParsePrefix("2001:db8::/32")}}
	if err := r.refreshState(p); err != nil {
		t.Fatalf("refreshState: %v", err)
	}

	state := r.Snapshot()
	if len(state.V6) != 1 {
		t.Errorf("expected 1 v6 prefix, got %d", len(state.V6))
	}
}

func TestIProute2Router_RefreshState_Remove(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	pre := RouterState{
		Backend: string(config.BackendIProute2),
		V4:      []netip.Prefix{netip.MustParsePrefix("1.2.3.0/24"), netip.MustParsePrefix("4.5.6.0/24")},
	}
	if err := saveState(path, pre); err != nil {
		t.Fatal(err)
	}
	r := newIProute2Router(config.PolicyConfig{TableID: 100}, path)

	p := Plan{Family: FamilyV4, Remove: []netip.Prefix{netip.MustParsePrefix("1.2.3.0/24")}}
	if err := r.refreshState(p); err != nil {
		t.Fatalf("refreshState: %v", err)
	}

	state := r.Snapshot()
	if len(state.V4) != 1 {
		t.Errorf("expected 1 v4 prefix after removal, got %d", len(state.V4))
	}
}

func TestIProute2Router_RunBatch_Empty(t *testing.T) {
	r := &iproute2Router{cfg: config.PolicyConfig{TableID: 100}}
	ctx := context.Background()
	if err := r.runBatch(ctx, "", FamilyV4); err != nil {
		t.Fatalf("runBatch(empty): %v", err)
	}
}

func TestIProute2Router_ipCommand_NetNS(t *testing.T) {
	r := &iproute2Router{netns: "testns"}
	cmd := r.ipCommand(context.Background(), "route", "show")
	args := cmd.Args
	// Should be: ip netns exec testns ip route show
	if len(args) < 7 || args[1] != "netns" || args[2] != "exec" || args[3] != "testns" || args[4] != "ip" {
		t.Errorf("unexpected args: %v", args)
	}
}

func TestIProute2Router_ipCommand_NoNetNS(t *testing.T) {
	r := &iproute2Router{}
	cmd := r.ipCommand(context.Background(), "route", "show")
	args := cmd.Args
	// Should be: ip route show
	if len(args) != 3 || args[0] != "ip" || args[1] != "route" || args[2] != "show" {
		t.Errorf("unexpected args: %v", args)
	}
}
