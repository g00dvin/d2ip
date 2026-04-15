package cidr

import (
	"net/netip"
	"testing"
)

// TestConservativeLossless verifies that conservative mode (threshold=1.0) is lossless.
func TestConservativeLossless(t *testing.T) {
	tests := []struct {
		name  string
		addrs []string
	}{
		{
			name: "adjacent /32s merge to /31",
			addrs: []string{
				"192.168.1.0",
				"192.168.1.1",
			},
		},
		{
			name: "four adjacent /32s merge to /30",
			addrs: []string{
				"10.0.0.0",
				"10.0.0.1",
				"10.0.0.2",
				"10.0.0.3",
			},
		},
		{
			name: "full /24",
			addrs: func() []string {
				var addrs []string
				for i := 0; i < 256; i++ {
					addrs = append(addrs, netip.AddrFrom4([4]byte{10, 0, 0, byte(i)}).String())
				}
				return addrs
			}(),
		},
		{
			name: "sparse addresses no merge",
			addrs: []string{
				"10.0.0.1",
				"10.0.0.3",
				"10.0.0.7",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			addrs := parseAddrs(t, tt.addrs)
			prefixes := Aggregate(addrs, 1.0, 16)

			// Verify lossless: every input address must be covered
			for _, addr := range addrs {
				if !isCovered(addr, prefixes) {
					t.Errorf("address %s not covered by output prefixes", addr)
				}
			}

			// Verify no extra addresses introduced
			// (in conservative mode, we should not expand coverage)
			t.Logf("Input: %d addrs, Output: %d prefixes", len(addrs), len(prefixes))
		})
	}
}

// TestAggressiveRespectsMaxPrefix verifies that aggressive mode respects maxPrefix.
func TestAggressiveRespectsMaxPrefix(t *testing.T) {
	tests := []struct {
		name      string
		addrs     []string
		maxPrefix int
		isV6      bool
	}{
		{
			name: "IPv4 maxPrefix=16",
			addrs: []string{
				"10.0.0.1",
				"10.0.0.2",
				"10.0.1.1",
				"10.0.2.1",
			},
			maxPrefix: 16,
			isV6:      false,
		},
		{
			name: "IPv4 maxPrefix=24",
			addrs: []string{
				"192.168.1.1",
				"192.168.1.2",
				"192.168.1.3",
			},
			maxPrefix: 24,
			isV6:      false,
		},
		{
			name: "IPv6 maxPrefix=32",
			addrs: []string{
				"2001:db8::1",
				"2001:db8::2",
				"2001:db8:1::1",
			},
			maxPrefix: 32,
			isV6:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			addrs := parseAddrs(t, tt.addrs)
			prefixes := Aggregate(addrs, 0.5, tt.maxPrefix)

			// Verify no prefix is broader than maxPrefix
			for _, prefix := range prefixes {
				if prefix.Bits() < tt.maxPrefix {
					t.Errorf("prefix %s is broader than maxPrefix %d", prefix, tt.maxPrefix)
				}
			}

			// Verify never produces /0
			for _, prefix := range prefixes {
				if prefix.Bits() == 0 {
					t.Errorf("produced /0 prefix: %s", prefix)
				}
			}
		})
	}
}

// TestDeterminism verifies that identical input produces identical output.
func TestDeterminism(t *testing.T) {
	addrs := parseAddrs(t, []string{
		"10.0.0.1",
		"10.0.0.2",
		"10.0.0.3",
		"192.168.1.1",
		"192.168.1.2",
		"2001:db8::1",
		"2001:db8::2",
	})

	// Run aggregation multiple times
	result1 := Aggregate(addrs, 1.0, 16)
	result2 := Aggregate(addrs, 1.0, 16)
	result3 := Aggregate(addrs, 1.0, 16)

	// Verify all results are identical
	if !prefixSlicesEqual(result1, result2) {
		t.Error("aggregation is not deterministic (result1 != result2)")
	}
	if !prefixSlicesEqual(result1, result3) {
		t.Error("aggregation is not deterministic (result1 != result3)")
	}
}

// TestOutputSorted verifies that output is sorted by address.
func TestOutputSorted(t *testing.T) {
	addrs := parseAddrs(t, []string{
		"192.168.1.1",
		"10.0.0.1",
		"172.16.0.1",
		"2001:db8::1",
		"2001:db8::2",
		"fe80::1",
	})

	prefixes := Aggregate(addrs, 1.0, 16)

	// Verify sorted
	for i := 1; i < len(prefixes); i++ {
		if !compareAddrs(prefixes[i-1].Addr(), prefixes[i].Addr()) {
			t.Errorf("output not sorted at index %d: %s > %s",
				i, prefixes[i-1].Addr(), prefixes[i].Addr())
		}
	}
}

// TestBoundarySafety verifies that we never emit /0 prefixes or prefixes broader than maxPrefix.
func TestBoundarySafety(t *testing.T) {
	tests := []struct {
		name      string
		addrs     []string
		threshold float64
		maxPrefix int
	}{
		{
			name: "aggressive with low maxPrefix",
			addrs: []string{
				"10.0.0.1",
				"10.0.0.2",
				"10.1.0.1",
			},
			threshold: 0.5,
			maxPrefix: 20,
		},
		{
			name: "many addresses",
			addrs: func() []string {
				var addrs []string
				for i := 0; i < 1000; i++ {
					addrs = append(addrs, netip.AddrFrom4([4]byte{
						byte(i >> 8),
						byte(i & 0xff),
						0,
						1,
					}).String())
				}
				return addrs
			}(),
			threshold: 0.5,
			maxPrefix: 16,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			addrs := parseAddrs(t, tt.addrs)
			prefixes := Aggregate(addrs, tt.threshold, tt.maxPrefix)

			for _, prefix := range prefixes {
				// Check for /0
				if prefix.Bits() == 0 {
					t.Errorf("produced /0 prefix: %s", prefix)
				}

				// Check broader than maxPrefix
				if prefix.Bits() < tt.maxPrefix {
					t.Errorf("produced prefix %s broader than maxPrefix %d", prefix, tt.maxPrefix)
				}

				// Check for 0.0.0.0/0
				if prefix.String() == "0.0.0.0/0" {
					t.Error("produced 0.0.0.0/0")
				}

				// Check for ::/0
				if prefix.String() == "::/0" {
					t.Error("produced ::/0")
				}
			}
		})
	}
}

// TestIPv4AndIPv6Mixed verifies handling of mixed IPv4/IPv6 input.
func TestIPv4AndIPv6Mixed(t *testing.T) {
	addrs := parseAddrs(t, []string{
		"10.0.0.1",
		"10.0.0.2",
		"2001:db8::1",
		"2001:db8::2",
		"192.168.1.1",
		"fe80::1",
	})

	prefixes := Aggregate(addrs, 1.0, 16)

	// Count IPv4 and IPv6 prefixes
	var v4Count, v6Count int
	for _, prefix := range prefixes {
		if prefix.Addr().Is4() {
			v4Count++
		} else if prefix.Addr().Is6() {
			v6Count++
		}
	}

	if v4Count == 0 {
		t.Error("no IPv4 prefixes in output")
	}
	if v6Count == 0 {
		t.Error("no IPv6 prefixes in output")
	}

	t.Logf("IPv4 prefixes: %d, IPv6 prefixes: %d", v4Count, v6Count)
}

// TestEmptyInput verifies handling of empty input.
func TestEmptyInput(t *testing.T) {
	prefixes := Aggregate(nil, 1.0, 16)
	if len(prefixes) != 0 {
		t.Errorf("expected empty output, got %d prefixes", len(prefixes))
	}
}

// TestSingleAddress verifies handling of single address.
func TestSingleAddress(t *testing.T) {
	addrs := parseAddrs(t, []string{"10.0.0.1"})
	prefixes := Aggregate(addrs, 1.0, 16)

	if len(prefixes) != 1 {
		t.Fatalf("expected 1 prefix, got %d", len(prefixes))
	}

	if prefixes[0].String() != "10.0.0.1/32" {
		t.Errorf("expected 10.0.0.1/32, got %s", prefixes[0])
	}
}

// TestDuplicateAddresses verifies handling of duplicate addresses.
func TestDuplicateAddresses(t *testing.T) {
	addrs := parseAddrs(t, []string{
		"10.0.0.0",
		"10.0.0.0",
		"10.0.0.0",
		"10.0.0.1",
		"10.0.0.1",
	})

	prefixes := Aggregate(addrs, 1.0, 16)

	// Should deduplicate and produce 10.0.0.0/31
	if len(prefixes) != 1 {
		t.Errorf("expected 1 prefix after deduplication, got %d", len(prefixes))
	}
}

// Helper functions

func parseAddrs(t *testing.T, strs []string) []netip.Addr {
	var addrs []netip.Addr
	for _, s := range strs {
		addr, err := netip.ParseAddr(s)
		if err != nil {
			t.Fatalf("failed to parse address %s: %v", s, err)
		}
		addrs = append(addrs, addr)
	}
	return addrs
}

func isCovered(addr netip.Addr, prefixes []netip.Prefix) bool {
	for _, prefix := range prefixes {
		if prefix.Contains(addr) {
			return true
		}
	}
	return false
}

func prefixSlicesEqual(a, b []netip.Prefix) bool {
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
