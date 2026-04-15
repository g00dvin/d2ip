package routing

import (
	"net/netip"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadState_Missing(t *testing.T) {
	s, err := loadState(filepath.Join(t.TempDir(), "nope.json"))
	if err != nil {
		t.Fatal(err)
	}
	if s.Backend != "" || len(s.V4) != 0 {
		t.Errorf("expected zero-value state, got %+v", s)
	}
}

func TestSaveLoadState_RoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "state.json")
	v4, _ := netip.ParsePrefix("1.2.3.0/24")
	want := RouterState{
		Backend:   "nftables",
		AppliedAt: time.Now().UTC().Truncate(time.Second),
		V4:        []netip.Prefix{v4},
	}
	if err := saveState(path, want); err != nil {
		t.Fatal(err)
	}
	got, err := loadState(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Backend != want.Backend || len(got.V4) != 1 || got.V4[0] != v4 {
		t.Errorf("got %+v, want %+v", got, want)
	}
}
