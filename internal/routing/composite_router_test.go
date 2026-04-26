package routing

import (
	"context"
	"net/netip"
	"testing"

	"github.com/goodvin/d2ip/internal/config"
)

func TestNewCompositeRouter(t *testing.T) {
	cr := NewCompositeRouter(config.RoutingConfig{StateDir: t.TempDir()})
	if cr == nil {
		t.Fatal("expected non-nil CompositeRouter")
	}
}

func TestCompositeRouter_Caps_IProute2(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping: requires ip command")
	}
	cr := NewCompositeRouter(config.RoutingConfig{StateDir: t.TempDir()})
	ctx := context.Background()
	// Table 254 (main) always exists
	err := cr.Caps(ctx, config.PolicyConfig{Backend: config.BackendIProute2, TableID: 254})
	if err != nil {
		t.Fatalf("Caps(iproute2): %v", err)
	}
}

func TestCompositeRouter_Caps_Unsupported(t *testing.T) {
	cr := NewCompositeRouter(config.RoutingConfig{StateDir: t.TempDir()})
	ctx := context.Background()
	err := cr.Caps(ctx, config.PolicyConfig{Backend: "unknown"})
	if err == nil {
		t.Fatal("expected error for unsupported backend")
	}
}

func TestCompositeRouter_SetValidator(t *testing.T) {
	cr := NewCompositeRouter(config.RoutingConfig{StateDir: t.TempDir()})
	v := NewValidator()
	cr.SetValidator(v)
	if cr.validator != v {
		t.Error("expected validator to be set")
	}
}

func TestCompositeRouter_Caps_NFTables_FirstRun(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping: requires nft command")
	}
	cr := NewCompositeRouter(config.RoutingConfig{StateDir: t.TempDir()})
	ctx := context.Background()

	// Pretend nftables Layer 2 is healthy
	v := NewValidator()
	v.health[config.BackendNFTables] = true
	cr.SetValidator(v)

	// Use a table that does not exist — Caps should return nil (first run)
	err := cr.Caps(ctx, config.PolicyConfig{Backend: config.BackendNFTables, NFTTable: "inet nonexistent_d2ip_test_table"})
	if err != nil {
		t.Fatalf("expected nil for first run (missing table with healthy backend), got %v", err)
	}
}

func TestCompositeRouter_Caps_NFTables_Unhealthy(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping: requires nft command")
	}
	cr := NewCompositeRouter(config.RoutingConfig{StateDir: t.TempDir()})
	ctx := context.Background()

	// Pretend nftables Layer 2 is unhealthy
	v := NewValidator()
	v.health[config.BackendNFTables] = false
	cr.SetValidator(v)

	// Use a table that does not exist — Caps should return error because backend is unhealthy
	err := cr.Caps(ctx, config.PolicyConfig{Backend: config.BackendNFTables, NFTTable: "inet nonexistent_d2ip_test_table"})
	if err == nil {
		t.Fatal("expected error when backend is unhealthy")
	}
}

func TestCompositeRouter_Caps_NFTables_NoValidator(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping: requires nft command")
	}
	cr := NewCompositeRouter(config.RoutingConfig{StateDir: t.TempDir()})
	ctx := context.Background()

	// No validator set — Caps should return error for missing table
	err := cr.Caps(ctx, config.PolicyConfig{Backend: config.BackendNFTables, NFTTable: "inet nonexistent_d2ip_test_table"})
	if err == nil {
		t.Fatal("expected error when no validator is set")
	}
}

func TestCompositeRouter_DryRunPolicy_IProute2(t *testing.T) {
	dir := t.TempDir()
	cr := NewCompositeRouter(config.RoutingConfig{StateDir: dir})
	ctx := context.Background()

	v4 := []netip.Prefix{netip.MustParsePrefix("1.2.3.0/24")}
	v6 := []netip.Prefix{netip.MustParsePrefix("2001:db8::/32")}

	v4Plan, v6Plan, v4Diff, v6Diff, err := cr.DryRunPolicy(ctx, config.PolicyConfig{
		Backend: config.BackendIProute2,
		Name:    "test-policy",
	}, v4, v6)
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

func TestCompositeRouter_DryRunPolicy_NFTables(t *testing.T) {
	dir := t.TempDir()
	cr := NewCompositeRouter(config.RoutingConfig{StateDir: dir})
	ctx := context.Background()

	v4 := []netip.Prefix{netip.MustParsePrefix("1.2.3.0/24")}
	v6 := []netip.Prefix{netip.MustParsePrefix("2001:db8::/32")}

	v4Plan, v6Plan, v4Diff, v6Diff, err := cr.DryRunPolicy(ctx, config.PolicyConfig{
		Backend: config.BackendNFTables,
		Name:    "test-policy",
	}, v4, v6)
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

func TestCompositeRouter_DryRunPolicy_Unsupported(t *testing.T) {
	cr := NewCompositeRouter(config.RoutingConfig{StateDir: t.TempDir()})
	ctx := context.Background()
	_, _, _, _, err := cr.DryRunPolicy(ctx, config.PolicyConfig{Backend: "unknown"}, nil, nil)
	if err == nil {
		t.Fatal("expected error for unsupported backend")
	}
}

func TestCompositeRouter_SnapshotPolicy(t *testing.T) {
	dir := t.TempDir()
	cr := NewCompositeRouter(config.RoutingConfig{StateDir: dir})

	// No state file yet — should return zero value
	state := cr.SnapshotPolicy("missing")
	if state.Backend != "" {
		t.Errorf("expected empty backend, got %q", state.Backend)
	}

	// Save state and verify snapshot loads it
	want := RouterState{Backend: string(config.BackendIProute2)}
	if err := savePolicyState(dir, "existing", want); err != nil {
		t.Fatal(err)
	}
	state = cr.SnapshotPolicy("existing")
	if state.Backend != want.Backend {
		t.Errorf("backend = %q, want %q", state.Backend, want.Backend)
	}
}

func TestCompositeRouter_RollbackPolicy_IProute2(t *testing.T) {
	dir := t.TempDir()
	cr := NewCompositeRouter(config.RoutingConfig{StateDir: dir})
	ctx := context.Background()

	// Save state so RollbackPolicy can load it
	want := RouterState{Backend: string(config.BackendIProute2)}
	if err := savePolicyState(dir, "test", want); err != nil {
		t.Fatal(err)
	}

	err := cr.RollbackPolicy(ctx, "test")
	if err != nil {
		t.Fatalf("RollbackPolicy: %v", err)
	}
}

func TestCompositeRouter_RollbackPolicy_NFTables(t *testing.T) {
	dir := t.TempDir()
	cr := NewCompositeRouter(config.RoutingConfig{StateDir: dir})
	ctx := context.Background()

	want := RouterState{Backend: string(config.BackendNFTables)}
	if err := savePolicyState(dir, "test", want); err != nil {
		t.Fatal(err)
	}

	err := cr.RollbackPolicy(ctx, "test")
	if err != nil {
		t.Fatalf("RollbackPolicy: %v", err)
	}
}

func TestCompositeRouter_RollbackPolicy_MissingState(t *testing.T) {
	cr := NewCompositeRouter(config.RoutingConfig{StateDir: t.TempDir()})
	ctx := context.Background()

	err := cr.RollbackPolicy(ctx, "missing")
	if err == nil {
		t.Fatal("expected error for missing state")
	}
}

func TestCompositeRouter_RollbackPolicy_UnknownBackend(t *testing.T) {
	dir := t.TempDir()
	cr := NewCompositeRouter(config.RoutingConfig{StateDir: dir})
	ctx := context.Background()

	want := RouterState{Backend: "unknown"}
	if err := savePolicyState(dir, "test", want); err != nil {
		t.Fatal(err)
	}

	err := cr.RollbackPolicy(ctx, "test")
	if err == nil {
		t.Fatal("expected error for unknown backend")
	}
}

func TestCompositeRouter_ApplyPolicy_Unsupported(t *testing.T) {
	cr := NewCompositeRouter(config.RoutingConfig{StateDir: t.TempDir()})
	ctx := context.Background()

	err := cr.ApplyPolicy(ctx, config.PolicyConfig{Backend: "unknown"}, nil, nil)
	if err == nil {
		t.Fatal("expected error for unsupported backend")
	}
}

func TestCompositeRouter_ApplyPolicy_IProute2(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping: requires ip command")
	}
	cr := NewCompositeRouter(config.RoutingConfig{StateDir: t.TempDir()})
	ctx := context.Background()

	v4 := []netip.Prefix{netip.MustParsePrefix("1.2.3.0/24")}
	v6 := []netip.Prefix{netip.MustParsePrefix("2001:db8::/32")}

	// Without root, ApplyPolicy will fail, but we cover the dispatch path
	err := cr.ApplyPolicy(ctx, config.PolicyConfig{
		Backend: config.BackendIProute2,
		Name:    "test",
		TableID: 100,
		Iface:   "lo",
	}, v4, v6)
	if err == nil {
		t.Fatal("expected error from ApplyPolicy without root")
	}
}

func TestCompositeRouter_ApplyPolicy_NFTables(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping: requires nft command")
	}
	cr := NewCompositeRouter(config.RoutingConfig{StateDir: t.TempDir()})
	ctx := context.Background()

	v4 := []netip.Prefix{netip.MustParsePrefix("1.2.3.0/24")}
	v6 := []netip.Prefix{netip.MustParsePrefix("2001:db8::/32")}

	// Without root/CAP_NET_ADMIN, ApplyPolicy will fail, but we cover the dispatch path
	err := cr.ApplyPolicy(ctx, config.PolicyConfig{
		Backend:  config.BackendNFTables,
		Name:     "test",
		NFTTable: "d2ip",
		NFTSetV4: "d2ip_v4",
		NFTSetV6: "d2ip_v6",
	}, v4, v6)
	if err == nil {
		t.Fatal("expected error from ApplyPolicy without CAP_NET_ADMIN")
	}
}
