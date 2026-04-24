package routing

import (
	"net/netip"
	"testing"
	"time"
)

func TestPolicyState(t *testing.T) {
	dir := t.TempDir()
	state := RouterState{
		Backend:   "iproute2",
		AppliedAt: time.Now(),
		V4:        []netip.Prefix{netip.MustParsePrefix("1.2.3.0/24")},
	}

	if err := savePolicyState(dir, "streaming", state); err != nil {
		t.Fatal(err)
	}

	loaded, err := loadPolicyState(dir, "streaming")
	if err != nil {
		t.Fatal(err)
	}

	if loaded.Backend != state.Backend {
		t.Fatalf("backend mismatch: %s vs %s", loaded.Backend, state.Backend)
	}
	if len(loaded.V4) != 1 {
		t.Fatalf("expected 1 v4 prefix, got %d", len(loaded.V4))
	}
}
