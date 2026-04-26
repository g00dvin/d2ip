package routing

import (
	"context"
	"net/netip"
	"strings"
	"testing"

	"github.com/goodvin/d2ip/internal/config"
)

func TestBuildPlan(t *testing.T) {
	current := []netip.Prefix{
		netip.MustParsePrefix("1.2.3.0/24"),
		netip.MustParsePrefix("4.5.6.0/24"),
	}
	desired := []netip.Prefix{
		netip.MustParsePrefix("4.5.6.0/24"),
		netip.MustParsePrefix("7.8.9.0/24"),
	}

	plan := buildPlan(current, desired, FamilyV4)
	if len(plan.Add) != 1 || plan.Add[0].String() != "7.8.9.0/24" {
		t.Errorf("Add = %v, want [7.8.9.0/24]", plan.Add)
	}
	if len(plan.Remove) != 1 || plan.Remove[0].String() != "1.2.3.0/24" {
		t.Errorf("Remove = %v, want [1.2.3.0/24]", plan.Remove)
	}
	if plan.Family != FamilyV4 {
		t.Errorf("Family = %v, want v4", plan.Family)
	}
}

func TestBuildPlan_NoChange(t *testing.T) {
	current := []netip.Prefix{netip.MustParsePrefix("1.2.3.0/24")}
	desired := []netip.Prefix{netip.MustParsePrefix("1.2.3.0/24")}

	plan := buildPlan(current, desired, FamilyV4)
	if !plan.Empty() {
		t.Errorf("expected empty plan, got Add=%v Remove=%v", plan.Add, plan.Remove)
	}
}

func TestBuildPlan_EmptyCurrent(t *testing.T) {
	desired := []netip.Prefix{netip.MustParsePrefix("1.2.3.0/24")}

	plan := buildPlan(nil, desired, FamilyV6)
	if len(plan.Add) != 1 {
		t.Errorf("Add = %v, want 1", plan.Add)
	}
	if plan.Family != FamilyV6 {
		t.Errorf("Family = %v, want v6", plan.Family)
	}
}

func TestDiffString(t *testing.T) {
	plan := Plan{
		Family: FamilyV4,
		Add:    []netip.Prefix{netip.MustParsePrefix("1.2.3.0/24")},
		Remove: []netip.Prefix{netip.MustParsePrefix("9.9.9.0/24")},
	}
	diff := diffString(plan)
	if !strings.Contains(diff, "+ 1.2.3.0/24") {
		t.Errorf("missing add marker: %q", diff)
	}
	if !strings.Contains(diff, "- 9.9.9.0/24") {
		t.Errorf("missing remove marker: %q", diff)
	}
}

func TestDiffString_NoChanges(t *testing.T) {
	diff := diffString(Plan{Family: FamilyV4})
	if !strings.Contains(diff, "(no changes)") {
		t.Errorf("expected '(no changes)', got %q", diff)
	}
}

func TestNewIProute2PolicyRouter(t *testing.T) {
	r := newIProute2PolicyRouter(t.TempDir())
	if r == nil {
		t.Fatal("expected non-nil router")
	}
}

func TestIProute2PolicyRouter_Caps(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping: requires ip command")
	}
	r := newIProute2PolicyRouter(t.TempDir())
	ctx := context.Background()
	// Table 254 (main) always exists
	err := r.Caps(ctx, config.PolicyConfig{TableID: 254})
	if err != nil {
		t.Fatalf("Caps: %v", err)
	}
}

func TestIProute2PolicyRouter_DryRunPolicy(t *testing.T) {
	dir := t.TempDir()
	r := newIProute2PolicyRouter(dir)
	ctx := context.Background()

	v4 := []netip.Prefix{netip.MustParsePrefix("1.2.3.0/24")}
	v6 := []netip.Prefix{netip.MustParsePrefix("2001:db8::/32")}

	v4Plan, v6Plan, v4Diff, v6Diff, err := r.DryRunPolicy(ctx, config.PolicyConfig{Name: "test"}, v4, v6)
	if err != nil {
		t.Fatalf("DryRunPolicy: %v", err)
	}
	if len(v4Plan.Add) != 1 {
		t.Errorf("v4Plan.Add = %v, want 1", v4Plan.Add)
	}
	if len(v6Plan.Add) != 1 {
		t.Errorf("v6Plan.Add = %v, want 1", v6Plan.Add)
	}
	if v4Diff == "" || v6Diff == "" {
		t.Errorf("diffs should not be empty")
	}
}

func TestIProute2PolicyRouter_DryRunPolicy_WithState(t *testing.T) {
	dir := t.TempDir()
	r := newIProute2PolicyRouter(dir)
	ctx := context.Background()

	// Pre-populate state
	state := RouterState{
		Backend: string(config.BackendIProute2),
		V4:      []netip.Prefix{netip.MustParsePrefix("1.2.3.0/24")},
	}
	if err := savePolicyState(dir, "test", state); err != nil {
		t.Fatal(err)
	}

	// Same desired as current — should yield empty plan
	v4 := []netip.Prefix{netip.MustParsePrefix("1.2.3.0/24")}
	v4Plan, _, v4Diff, _, err := r.DryRunPolicy(ctx, config.PolicyConfig{Name: "test"}, v4, nil)
	if err != nil {
		t.Fatalf("DryRunPolicy: %v", err)
	}
	if !v4Plan.Empty() {
		t.Errorf("expected empty plan, got %v", v4Plan)
	}
	if !strings.Contains(v4Diff, "(no changes)") {
		t.Errorf("expected '(no changes)' diff, got %q", v4Diff)
	}
}

func TestIProute2PolicyRouter_SnapshotPolicy(t *testing.T) {
	dir := t.TempDir()
	r := newIProute2PolicyRouter(dir)

	state := r.SnapshotPolicy("missing")
	if state.Backend != "" {
		t.Errorf("expected empty backend, got %q", state.Backend)
	}

	want := RouterState{Backend: string(config.BackendIProute2), V4: []netip.Prefix{netip.MustParsePrefix("1.2.3.0/24")}}
	if err := savePolicyState(dir, "existing", want); err != nil {
		t.Fatal(err)
	}
	state = r.SnapshotPolicy("existing")
	if state.Backend != want.Backend {
		t.Errorf("backend = %q, want %q", state.Backend, want.Backend)
	}
}

func TestIProute2PolicyRouter_RollbackPolicy(t *testing.T) {
	dir := t.TempDir()
	r := newIProute2PolicyRouter(dir)
	ctx := context.Background()

	// Without state, returns nil (loadState returns zero state for missing file)
	if err := r.RollbackPolicy(ctx, "missing"); err != nil {
		t.Fatalf("RollbackPolicy(missing): %v", err)
	}

	// With state, should return nil (implementation is limited)
	state := RouterState{Backend: string(config.BackendIProute2), V4: []netip.Prefix{netip.MustParsePrefix("1.2.3.0/24")}}
	if err := savePolicyState(dir, "test", state); err != nil {
		t.Fatal(err)
	}
	if err := r.RollbackPolicy(ctx, "test"); err != nil {
		t.Fatalf("RollbackPolicy: %v", err)
	}
}

func TestNewNFTPolicyRouter(t *testing.T) {
	r := newNFTPolicyRouter(t.TempDir())
	if r == nil {
		t.Fatal("expected non-nil router")
	}
}

func TestNFTPolicyRouter_DryRunPolicy(t *testing.T) {
	dir := t.TempDir()
	r := newNFTPolicyRouter(dir)
	ctx := context.Background()

	v4 := []netip.Prefix{netip.MustParsePrefix("1.2.3.0/24")}
	v6 := []netip.Prefix{netip.MustParsePrefix("2001:db8::/32")}

	v4Plan, v6Plan, v4Diff, v6Diff, err := r.DryRunPolicy(ctx, config.PolicyConfig{Name: "test"}, v4, v6)
	if err != nil {
		t.Fatalf("DryRunPolicy: %v", err)
	}
	if len(v4Plan.Add) != 1 {
		t.Errorf("v4Plan.Add = %v, want 1", v4Plan.Add)
	}
	if len(v6Plan.Add) != 1 {
		t.Errorf("v6Plan.Add = %v, want 1", v6Plan.Add)
	}
	if v4Diff == "" || v6Diff == "" {
		t.Errorf("diffs should not be empty")
	}
}

func TestNFTPolicyRouter_SnapshotPolicy(t *testing.T) {
	dir := t.TempDir()
	r := newNFTPolicyRouter(dir)

	state := r.SnapshotPolicy("missing")
	if state.Backend != "" {
		t.Errorf("expected empty backend, got %q", state.Backend)
	}

	want := RouterState{Backend: string(config.BackendNFTables), V4: []netip.Prefix{netip.MustParsePrefix("1.2.3.0/24")}}
	if err := savePolicyState(dir, "existing", want); err != nil {
		t.Fatal(err)
	}
	state = r.SnapshotPolicy("existing")
	if state.Backend != want.Backend {
		t.Errorf("backend = %q, want %q", state.Backend, want.Backend)
	}
}

func TestNFTPolicyRouter_RollbackPolicy(t *testing.T) {
	dir := t.TempDir()
	r := newNFTPolicyRouter(dir)
	ctx := context.Background()

	// Without state, returns nil (loadState returns zero state for missing file)
	if err := r.RollbackPolicy(ctx, "missing"); err != nil {
		t.Fatalf("RollbackPolicy(missing): %v", err)
	}

	state := RouterState{Backend: string(config.BackendNFTables), V4: []netip.Prefix{netip.MustParsePrefix("1.2.3.0/24")}}
	if err := savePolicyState(dir, "test", state); err != nil {
		t.Fatal(err)
	}
	if err := r.RollbackPolicy(ctx, "test"); err != nil {
		t.Fatalf("RollbackPolicy: %v", err)
	}
}

func TestIProute2PolicyRouter_ApplyPolicy(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping: requires ip command")
	}
	dir := t.TempDir()
	r := newIProute2PolicyRouter(dir)
	ctx := context.Background()

	policy := config.PolicyConfig{
		Name:    "test",
		TableID: 100,
		Iface:   "lo",
	}
	v4 := []netip.Prefix{netip.MustParsePrefix("1.2.3.0/24")}
	v6 := []netip.Prefix{netip.MustParsePrefix("2001:db8::/32")}

	// Without root, applyFamily will fail, but we cover the code path
	err := r.ApplyPolicy(ctx, policy, v4, v6)
	if err == nil {
		t.Fatal("expected error from ApplyPolicy without root")
	}
}

func TestNFTPolicyRouter_ApplyPolicy(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping: requires nft command")
	}
	dir := t.TempDir()
	r := newNFTPolicyRouter(dir)
	ctx := context.Background()

	policy := config.PolicyConfig{
		Name:     "test",
		NFTTable: "d2ip",
		NFTSetV4: "d2ip_v4",
		NFTSetV6: "d2ip_v6",
	}
	v4 := []netip.Prefix{netip.MustParsePrefix("1.2.3.0/24")}
	v6 := []netip.Prefix{netip.MustParsePrefix("2001:db8::/32")}

	// Without root/CAP_NET_ADMIN, applySet will fail, but we cover the code path
	err := r.ApplyPolicy(ctx, policy, v4, v6)
	if err == nil {
		t.Fatal("expected error from ApplyPolicy without CAP_NET_ADMIN")
	}
}

func TestNFTPolicyRouter_Caps(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping: requires nft command")
	}
	r := newNFTPolicyRouter(t.TempDir())
	ctx := context.Background()
	// Without root, nft list table fails even for existing tables
	err := r.Caps(ctx, config.PolicyConfig{NFTTable: "inet d2ip"})
	if err == nil {
		t.Fatal("expected error from Caps without CAP_NET_ADMIN")
	}
}
