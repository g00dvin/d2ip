package cidr

import (
	"net/netip"
	"sort"
)

// Aggregate takes a list of IP addresses and aggregates them into CIDR prefixes.
//
// Parameters:
//   - addrs: input IP addresses (can be mixed IPv4/IPv6, will be separated)
//   - threshold: aggregation threshold (1.0 = conservative/lossless, 0.5 = aggressive)
//     - 1.0: only merge when 100% of subtree is present (lossless)
//     - 0.75: merge when 75% of subtree is present (balanced)
//     - 0.5: merge when 50% of subtree is present (aggressive)
//   - maxPrefix: maximum prefix length to aggregate to (never broader than this)
//     - IPv4: typically 16 (never broader than /16)
//     - IPv6: typically 32 (never broader than /32)
//
// Returns:
//   - Sorted list of aggregated CIDR prefixes
//
// Guarantees:
//   - Deterministic: identical input produces identical output
//   - Sorted: output is sorted by prefix.Addr().As16() ascending
//   - Boundary safe: never emits 0.0.0.0/0, ::/0, or prefixes broader than maxPrefix
//   - Lossless when threshold=1.0: all input addresses are covered, no extras
func Aggregate(addrs []netip.Addr, threshold float64, maxPrefix int) []netip.Prefix {
	// Separate IPv4 and IPv6 addresses
	var v4addrs, v6addrs []netip.Addr
	for _, addr := range addrs {
		if !addr.IsValid() {
			continue
		}
		if addr.Is4() {
			v4addrs = append(v4addrs, addr)
		} else if addr.Is6() {
			v6addrs = append(v6addrs, addr)
		}
	}

	var result []netip.Prefix

	// Aggregate IPv4
	if len(v4addrs) > 0 {
		v4Prefixes := aggregateFamily(v4addrs, false, threshold, maxPrefix)
		result = append(result, v4Prefixes...)
	}

	// Aggregate IPv6
	if len(v6addrs) > 0 {
		v6Prefixes := aggregateFamily(v6addrs, true, threshold, maxPrefix)
		result = append(result, v6Prefixes...)
	}

	// Sort by address bytes for determinism
	sort.Slice(result, func(i, j int) bool {
		return compareAddrs(result[i].Addr(), result[j].Addr())
	})

	return result
}

// aggregateFamily aggregates addresses of a single IP family.
func aggregateFamily(addrs []netip.Addr, isV6 bool, threshold float64, maxPrefix int) []netip.Prefix {
	// Clamp maxPrefix to valid range
	if isV6 {
		if maxPrefix < 0 {
			maxPrefix = 0
		}
		if maxPrefix > 128 {
			maxPrefix = 128
		}
	} else {
		if maxPrefix < 0 {
			maxPrefix = 0
		}
		if maxPrefix > 32 {
			maxPrefix = 32
		}
	}

	// Deduplicate addresses
	addrSet := make(map[netip.Addr]struct{})
	for _, addr := range addrs {
		addrSet[addr] = struct{}{}
	}

	// Convert to sorted slice for deterministic tree insertion
	uniqueAddrs := make([]netip.Addr, 0, len(addrSet))
	for addr := range addrSet {
		uniqueAddrs = append(uniqueAddrs, addr)
	}
	sort.Slice(uniqueAddrs, func(i, j int) bool {
		return compareAddrs(uniqueAddrs[i], uniqueAddrs[j])
	})

	// Build radix tree with deterministic insertion order
	tree := newRadixTree(isV6)
	for _, addr := range uniqueAddrs {
		tree.insert(addr)
	}

	// Aggregate
	prefixes := tree.aggregate(threshold, maxPrefix)

	return prefixes
}

// compareAddrs compares two addresses by their byte representation.
// Returns true if a < b.
func compareAddrs(a, b netip.Addr) bool {
	aBytes := a.As16()
	bBytes := b.As16()

	for i := 0; i < 16; i++ {
		if aBytes[i] < bBytes[i] {
			return true
		}
		if aBytes[i] > bBytes[i] {
			return false
		}
	}
	return false
}
