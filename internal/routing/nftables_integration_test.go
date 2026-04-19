//go:build routing_integration

package routing

import (
	"context"
	"net/netip"
	"testing"

	"github.com/goodvin/d2ip/internal/config"
)

// TestNftablesBackend_Integration_RealKernel tests nftables backend with real kernel operations.
// Requires CAP_NET_ADMIN (run with sudo) and isolated network namespace.
func TestNftablesBackend_Integration_RealKernel(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	nsName := "d2ip-test-nft"
	cleanup := createNetns(t, nsName)
	defer cleanup()

	// Check if nftables is available
	if !checkNftablesAvailable(t, nsName) {
		t.Skip("Skipping integration test: nftables not available")
	}

	// Create nftables backend
	cfg := config.RoutingConfig{
		Enabled:   true,
		Backend:   "nftables",
		NFTTable:  "inet d2ip",
		NFTSetV4:  "d2ip_v4",
		NFTSetV6:  "d2ip_v6",
		StatePath: "/tmp/d2ip-test-nft-state.json",
		DryRun:    false,
	}

	backend, err := New(cfg)
	if err != nil {
		t.Fatalf("Failed to create nftables backend: %v", err)
	}

	// Check capabilities
	if err := backend.Caps(); err != nil {
		t.Skipf("Capabilities check failed: %v", err)
	}

	ctx := context.Background()

	// Test 1: Apply IPv4 prefixes
	t.Run("Apply_IPv4_Prefixes", func(t *testing.T) {
		v4Prefixes := []netip.Prefix{
			netip.MustParsePrefix("192.0.2.0/24"),
			netip.MustParsePrefix("198.51.100.0/24"),
		}

		plan, err := backend.Plan(ctx, v4Prefixes, FamilyV4)
		if err != nil {
			t.Fatalf("Plan failed: %v", err)
		}

		if len(plan.Add) != 2 || len(plan.Remove) != 0 {
			t.Fatalf("Expected Add=2, Remove=0, got Add=%d, Remove=%d", len(plan.Add), len(plan.Remove))
		}

		if err := backend.Apply(ctx, plan); err != nil {
			t.Fatalf("Apply failed: %v", err)
		}

		t.Logf("Applied IPv4 prefixes: %v", v4Prefixes)
	})

	// Test 2: Verify sets created (would need to run nft in netns - simplified for now)
	t.Run("Verify_Sets_Created", func(t *testing.T) {
		snapshot := backend.Snapshot()
		if len(snapshot.V4) != 2 {
			t.Errorf("Expected 2 IPv4 prefixes in snapshot, got %d", len(snapshot.V4))
		}
		if snapshot.Backend != "nftables" {
			t.Errorf("Expected backend=nftables, got %s", snapshot.Backend)
		}
		t.Logf("Snapshot: %+v", snapshot)
	})

	// Test 3: Idempotence - second apply should be no-op
	t.Run("Idempotence_Second_Apply", func(t *testing.T) {
		v4Prefixes := []netip.Prefix{
			netip.MustParsePrefix("192.0.2.0/24"),
			netip.MustParsePrefix("198.51.100.0/24"),
		}

		plan, err := backend.Plan(ctx, v4Prefixes, FamilyV4)
		if err != nil {
			t.Fatalf("Plan failed: %v", err)
		}

		if len(plan.Add) != 0 || len(plan.Remove) != 0 {
			t.Errorf("Expected no-op plan (Add=0, Remove=0), got Add=%d, Remove=%d", len(plan.Add), len(plan.Remove))
		}

		t.Logf("Second apply is no-op (idempotent): %+v", plan)
	})

	// Test 4: Update with new prefixes
	t.Run("Update_Prefixes", func(t *testing.T) {
		v4Prefixes := []netip.Prefix{
			netip.MustParsePrefix("192.0.2.0/24"),   // Keep
			netip.MustParsePrefix("203.0.113.0/24"), // Add (new)
			// Remove: 198.51.100.0/24
		}

		plan, err := backend.Plan(ctx, v4Prefixes, FamilyV4)
		if err != nil {
			t.Fatalf("Plan failed: %v", err)
		}

		if len(plan.Add) != 1 || len(plan.Remove) != 1 {
			t.Errorf("Expected Add=1, Remove=1, got Add=%d, Remove=%d", len(plan.Add), len(plan.Remove))
		}

		if err := backend.Apply(ctx, plan); err != nil {
			t.Fatalf("Apply failed: %v", err)
		}

		t.Logf("Updated prefixes: %+v", plan)
	})

	// Test 5: Rollback
	t.Run("Rollback", func(t *testing.T) {
		if err := backend.Rollback(ctx); err != nil {
			t.Fatalf("Rollback failed: %v", err)
		}

		snapshot := backend.Snapshot()
		if len(snapshot.V4) != 0 && len(snapshot.V6) != 0 {
			t.Errorf("Expected empty snapshot after rollback, got V4=%d, V6=%d", len(snapshot.V4), len(snapshot.V6))
		}

		t.Logf("Rollback successful, snapshot cleared")
	})

	// Test 6: Dry-run
	t.Run("DryRun", func(t *testing.T) {
		v4Prefixes := []netip.Prefix{
			netip.MustParsePrefix("10.0.0.0/8"),
		}

		plan, diff, err := backend.DryRun(ctx, v4Prefixes, FamilyV4)
		if err != nil {
			t.Fatalf("DryRun failed: %v", err)
		}

		if diff == "" {
			t.Error("Expected non-empty diff output")
		}

		t.Logf("Dry-run plan: %+v\nDiff:\n%s", plan, diff)

		// Verify dry-run didn't actually apply
		snapshot := backend.Snapshot()
		if len(snapshot.V4) != 0 {
			t.Errorf("Dry-run should not modify state, but snapshot has %d prefixes", len(snapshot.V4))
		}
	})
}
