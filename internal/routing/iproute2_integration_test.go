//go:build routing_integration

package routing

import (
	"context"
	"net/netip"
	"testing"

	"github.com/goodvin/d2ip/internal/config"
)

// TestIproute2Backend_Integration_RealKernel tests iproute2 backend with real kernel operations.
// Requires CAP_NET_ADMIN (run with sudo) and isolated network namespace.
func TestIproute2Backend_Integration_RealKernel(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	nsName := "d2ip-test-ip"
	cleanup := createNetns(t, nsName)
	defer cleanup()

	// Create dummy interface in namespace
	ifname := "dummy0"
	createDummyInterface(t, nsName, ifname)

	// Create iproute2 backend
	cfg := config.RoutingConfig{
		Enabled:   true,
		Backend:   "iproute2",
		Iface:     ifname,
		TableID:   100,
		StatePath: "/tmp/d2ip-test-ip-state.json",
		DryRun:    false,
	}

	backend, err := New(cfg)
	if err != nil {
		t.Fatalf("Failed to create iproute2 backend: %v", err)
	}

	// Check capabilities
	if err := backend.Caps(); err != nil {
		t.Skipf("Capabilities check failed: %v", err)
	}

	ctx := context.Background()

	// Test 1: Apply IPv4 routes
	t.Run("Apply_IPv4_Routes", func(t *testing.T) {
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

		t.Logf("Applied IPv4 routes: %v", v4Prefixes)
	})

	// Test 2: Verify routes created
	t.Run("Verify_Routes_Created", func(t *testing.T) {
		snapshot := backend.Snapshot()
		if len(snapshot.V4) != 2 {
			t.Errorf("Expected 2 IPv4 routes in snapshot, got %d", len(snapshot.V4))
		}
		if snapshot.Backend != "iproute2" {
			t.Errorf("Expected backend=iproute2, got %s", snapshot.Backend)
		}
		t.Logf("Snapshot: %+v", snapshot)

		// Verify routes in kernel (simplified - would need to run ip route in netns)
		routes, err := getIpRoutes(t, nsName, cfg.TableID)
		if err != nil {
			t.Logf("Warning: failed to get routes: %v", err)
		} else {
			t.Logf("Kernel routes in table %d: %v", cfg.TableID, routes)
			if len(routes) < 2 {
				t.Errorf("Expected at least 2 routes, got %d", len(routes))
			}
		}
	})

	// Test 3: Idempotence
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
			t.Errorf("Expected no-op plan, got Add=%d, Remove=%d", len(plan.Add), len(plan.Remove))
		}

		t.Logf("Second apply is no-op (idempotent)")
	})

	// Test 4: Update routes
	t.Run("Update_Routes", func(t *testing.T) {
		v4Prefixes := []netip.Prefix{
			netip.MustParsePrefix("192.0.2.0/24"),   // Keep
			netip.MustParsePrefix("203.0.113.0/24"), // Add
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

		t.Logf("Updated routes")
	})

	// Test 5: IPv6 routes
	t.Run("Apply_IPv6_Routes", func(t *testing.T) {
		v6Prefixes := []netip.Prefix{
			netip.MustParsePrefix("2001:db8::/32"),
			netip.MustParsePrefix("2001:db8:1::/48"),
		}

		plan, err := backend.Plan(ctx, v6Prefixes, FamilyV6)
		if err != nil {
			t.Fatalf("Plan failed: %v", err)
		}

		if len(plan.Add) != 2 {
			t.Fatalf("Expected Add=2, got %d", len(plan.Add))
		}

		if err := backend.Apply(ctx, plan); err != nil {
			t.Fatalf("Apply failed: %v", err)
		}

		snapshot := backend.Snapshot()
		if len(snapshot.V6) != 2 {
			t.Errorf("Expected 2 IPv6 routes, got %d", len(snapshot.V6))
		}

		t.Logf("Applied IPv6 routes: %v", v6Prefixes)
	})

	// Test 6: Rollback
	t.Run("Rollback", func(t *testing.T) {
		if err := backend.Rollback(ctx); err != nil {
			t.Fatalf("Rollback failed: %v", err)
		}

		snapshot := backend.Snapshot()
		if len(snapshot.V4) != 0 || len(snapshot.V6) != 0 {
			t.Errorf("Expected empty snapshot after rollback, got V4=%d, V6=%d", len(snapshot.V4), len(snapshot.V6))
		}

		// Verify routes removed from kernel
		routes, err := getIpRoutes(t, nsName, cfg.TableID)
		if err == nil && len(routes) > 0 {
			t.Errorf("Expected no routes after rollback, got %d routes", len(routes))
		}

		t.Logf("Rollback successful")
	})

	// Test 7: Dry-run
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

		// Verify dry-run didn't modify state
		snapshot := backend.Snapshot()
		if len(snapshot.V4) != 0 {
			t.Errorf("Dry-run should not modify state, but snapshot has %d routes", len(snapshot.V4))
		}
	})
}
