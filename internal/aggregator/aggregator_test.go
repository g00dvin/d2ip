package aggregator

import (
	"net/netip"
	"testing"
)

func TestAggregateV4_Off(t *testing.T) {
	addrs := []netip.Addr{
		netip.MustParseAddr("10.0.0.1"),
		netip.MustParseAddr("10.0.0.2"),
	}
	a := New()
	got := a.AggregateV4(addrs, AggOff, 32)
	if len(got) != 2 {
		t.Fatalf("expected 2 prefixes, got %d", len(got))
	}
	want := []netip.Prefix{
		netip.MustParsePrefix("10.0.0.1/32"),
		netip.MustParsePrefix("10.0.0.2/32"),
	}
	if !prefixesEqual(got, want) {
		t.Errorf("AggregateV4(_, AggOff, 32) = %v, want %v", got, want)
	}
}

func TestAggregateV4_Conservative(t *testing.T) {
	addrs := make([]netip.Addr, 256)
	for i := 0; i < 256; i++ {
		addrs[i] = netip.AddrFrom4([4]byte{10, 0, 0, byte(i)})
	}
	a := New()
	got := a.AggregateV4(addrs, AggConservative, 16)
	if len(got) != 1 {
		t.Fatalf("expected 1 prefix, got %d", len(got))
	}
	want := netip.MustParsePrefix("10.0.0.0/24")
	if got[0] != want {
		t.Errorf("AggregateV4(_, AggConservative, 16) = %v, want %v", got[0], want)
	}
}

func TestAggregateV4_Balanced(t *testing.T) {
	addrs := make([]netip.Addr, 512)
	for i := 0; i < 256; i++ {
		addrs[i] = netip.AddrFrom4([4]byte{10, 0, 0, byte(i)})
	}
	for i := 0; i < 256; i++ {
		addrs[256+i] = netip.AddrFrom4([4]byte{10, 0, 1, byte(i)})
	}
	a := New()
	got := a.AggregateV4(addrs, AggBalanced, 16)
	if len(got) != 1 {
		t.Fatalf("expected 1 prefix, got %d", len(got))
	}
	want := netip.MustParsePrefix("10.0.0.0/23")
	if got[0] != want {
		t.Errorf("AggregateV4(_, AggBalanced, 16) = %v, want %v", got[0], want)
	}
}

func TestAggregateV4_Aggressive(t *testing.T) {
	addrs := make([]netip.Addr, 128)
	for i := 0; i < 128; i++ {
		addrs[i] = netip.AddrFrom4([4]byte{10, 0, 0, byte(i * 2)})
	}
	a := New()
	gotAggressive := a.AggregateV4(addrs, AggAggressive, 16)
	gotBalanced := a.AggregateV4(addrs, AggBalanced, 16)
	if len(gotAggressive) >= len(gotBalanced) {
		t.Errorf("expected aggressive to aggregate more than balanced, got %d vs %d prefixes",
			len(gotAggressive), len(gotBalanced))
	}
	if len(gotAggressive) != 1 {
		t.Errorf("expected 1 prefix with aggressive, got %d: %v", len(gotAggressive), gotAggressive)
	}
	want := netip.MustParsePrefix("10.0.0.0/24")
	if len(gotAggressive) > 0 && gotAggressive[0] != want {
		t.Errorf("AggregateV4(_, AggAggressive, 16)[0] = %v, want %v", gotAggressive[0], want)
	}
}

func TestAggregateV4_MaxPrefix(t *testing.T) {
	addrs := make([]netip.Addr, 256)
	for i := 0; i < 256; i++ {
		addrs[i] = netip.AddrFrom4([4]byte{10, 0, 0, byte(i)})
	}
	a := New()
	got := a.AggregateV4(addrs, AggBalanced, 24)
	if len(got) != 1 {
		t.Fatalf("expected 1 prefix, got %d", len(got))
	}
	want := netip.MustParsePrefix("10.0.0.0/24")
	if got[0] != want {
		t.Errorf("AggregateV4(_, AggBalanced, 24) = %v, want %v", got[0], want)
	}
}

func TestAggregateV6(t *testing.T) {
	addrs := []netip.Addr{
		netip.MustParseAddr("2001:db8::0"),
		netip.MustParseAddr("2001:db8::1"),
	}
	a := New()
	got := a.AggregateV6(addrs, AggConservative, 32)
	if len(got) != 1 {
		t.Fatalf("expected 1 prefix, got %d", len(got))
	}
	want := netip.MustParsePrefix("2001:db8::/127")
	if got[0] != want {
		t.Errorf("AggregateV6(_, AggConservative, 32) = %v, want %v", got[0], want)
	}
}

func TestAggregate_Empty(t *testing.T) {
	a := New()

	gotV4 := a.AggregateV4(nil, AggBalanced, 16)
	if len(gotV4) != 0 {
		t.Errorf("AggregateV4(nil, AggBalanced, 16) = %v, want empty", gotV4)
	}
	gotV4Empty := a.AggregateV4([]netip.Addr{}, AggBalanced, 16)
	if len(gotV4Empty) != 0 {
		t.Errorf("AggregateV4([], AggBalanced, 16) = %v, want empty", gotV4Empty)
	}

	gotV6 := a.AggregateV6(nil, AggBalanced, 32)
	if len(gotV6) != 0 {
		t.Errorf("AggregateV6(nil, AggBalanced, 32) = %v, want empty", gotV6)
	}
	gotV6Empty := a.AggregateV6([]netip.Addr{}, AggBalanced, 32)
	if len(gotV6Empty) != 0 {
		t.Errorf("AggregateV6([], AggBalanced, 32) = %v, want empty", gotV6Empty)
	}
}

func TestLevelToThreshold(t *testing.T) {
	tests := []struct {
		name  string
		level Aggressiveness
		want  float64
	}{
		{"Off", AggOff, -1.0},
		{"Conservative", AggConservative, 1.0},
		{"Balanced", AggBalanced, 0.75},
		{"Aggressive", AggAggressive, 0.5},
		{"Unknown", Aggressiveness(99), 1.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := levelToThreshold(tt.level)
			if got != tt.want {
				t.Errorf("levelToThreshold(%v) = %v, want %v", tt.level, got, tt.want)
			}
		})
	}
}

func prefixesEqual(a, b []netip.Prefix) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
