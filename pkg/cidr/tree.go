package cidr

import (
	"net/netip"
)

// radixNode represents a node in the radix tree for CIDR aggregation.
// Each node represents a potential prefix at a given bit depth.
type radixNode struct {
	// left and right children (bit 0 and bit 1 at this depth)
	left  *radixNode
	right *radixNode
	// isLeaf marks that this node represents an actual input address
	isLeaf bool
	// prefix is the network prefix this node represents
	prefix netip.Prefix
}

// radixTree builds a binary radix tree for IPv4 or IPv6 addresses.
// It's used to perform bottom-up aggregation with configurable threshold.
type radixTree struct {
	root *radixNode
	// bits is 32 for IPv4, 128 for IPv6
	bits int
	// isV6 tracks if this is an IPv6 tree
	isV6 bool
}

// newRadixTree creates a new radix tree for the given IP family.
func newRadixTree(isV6 bool) *radixTree {
	bits := 32
	if isV6 {
		bits = 128
	}
	return &radixTree{
		root: &radixNode{},
		bits: bits,
		isV6: isV6,
	}
}

// insert adds an address as a /32 (IPv4) or /128 (IPv6) leaf to the tree.
func (t *radixTree) insert(addr netip.Addr) {
	if !addr.IsValid() {
		return
	}

	// Convert to /32 or /128 prefix
	prefixLen := 32
	if addr.Is6() {
		prefixLen = 128
	}
	prefix := netip.PrefixFrom(addr, prefixLen)

	// Navigate to the correct leaf position
	node := t.root
	bytes := addr.As16()

	// For IPv4, As16() stores the address in bytes 12-15
	// For IPv6, it's in bytes 0-15
	byteOffset := 0
	if addr.Is4() {
		byteOffset = 12
	}

	for depth := 0; depth < prefixLen; depth++ {
		// Extract bit at current depth
		byteIdx := byteOffset + (depth / 8)
		bitIdx := 7 - (depth % 8)
		bit := (bytes[byteIdx] >> bitIdx) & 1

		if bit == 0 {
			if node.left == nil {
				node.left = &radixNode{}
			}
			node = node.left
		} else {
			if node.right == nil {
				node.right = &radixNode{}
			}
			node = node.right
		}
	}

	node.isLeaf = true
	node.prefix = prefix
}

// aggregate walks the tree bottom-up and merges subtrees based on threshold.
// threshold: 1.0 = lossless (both children must be complete), 0.5 = aggressive
// maxPrefix: never produce prefixes broader than this (e.g., /16 for IPv4)
func (t *radixTree) aggregate(threshold float64, maxPrefix int) []netip.Prefix {
	prefixes := t.collectPrefixes(t.root, 0, [16]byte{}, threshold, maxPrefix)
	return prefixes
}

// collectPrefixes recursively walks the tree and collects aggregated prefixes.
// Returns the list of prefixes for this subtree (already aggregated).
func (t *radixTree) collectPrefixes(
	node *radixNode,
	depth int,
	addrBytes [16]byte,
	threshold float64,
	maxPrefix int,
) []netip.Prefix {
	if node == nil {
		return nil
	}

	// If this is a leaf node, return its prefix
	if node.isLeaf {
		return []netip.Prefix{node.prefix}
	}

	// Recursively collect from children
	// For left child: bit at depth is 0 (already the case since we start with zeros)
	leftBytes := addrBytes
	leftPrefixes := t.collectPrefixes(node.left, depth+1, leftBytes, threshold, maxPrefix)

	// For right child: set bit at depth to 1
	rightBytes := addrBytes
	if depth < t.bits {
		// For IPv4, bits start at byte 12 (As16 representation)
		// For IPv6, bits start at byte 0
		byteOffset := 0
		if !t.isV6 {
			byteOffset = 12
		}
		byteIdx := byteOffset + (depth / 8)
		bitIdx := 7 - (depth % 8)
		rightBytes[byteIdx] |= (1 << bitIdx)
	}
	rightPrefixes := t.collectPrefixes(node.right, depth+1, rightBytes, threshold, maxPrefix)

	// Count total leaves
	leftCount := countLeavesRecursive(node.left)
	rightCount := countLeavesRecursive(node.right)
	totalCount := leftCount + rightCount

	if totalCount == 0 {
		return nil
	}

	// Check if we should merge at this level
	// We can only merge if both children exist and have content
	if leftCount == 0 || rightCount == 0 {
		// Can't merge with only one child, return child prefixes
		result := append(leftPrefixes, rightPrefixes...)
		return result
	}

	// Maximum possible leaves in a subtree at this depth
	maxPossible := 1 << (t.bits - depth)
	ratio := float64(totalCount) / float64(maxPossible)

	// Don't merge if:
	// 1. We're at depth 0 (would create /0 prefix)
	// 2. We're shallower than maxPrefix (would be broader than allowed)
	// 3. The ratio doesn't meet threshold
	canMerge := depth > 0 && depth >= maxPrefix && ratio >= threshold

	if canMerge {
		// Merge into a single prefix at this depth
		var addr netip.Addr
		if t.isV6 {
			addr = netip.AddrFrom16(addrBytes)
		} else {
			// Extract IPv4 from the last 4 bytes of As16 representation
			var v4Bytes [4]byte
			copy(v4Bytes[:], addrBytes[12:16])
			addr = netip.AddrFrom4(v4Bytes)
		}

		prefix := netip.PrefixFrom(addr, depth)
		// Normalize the prefix to ensure the address bits beyond prefix length are zero
		prefix = prefix.Masked()
		return []netip.Prefix{prefix}
	}

	// Can't merge, return combined children prefixes
	result := append(leftPrefixes, rightPrefixes...)
	return result
}

// countLeavesRecursive counts the number of leaf nodes in a subtree.
func countLeavesRecursive(node *radixNode) int {
	if node == nil {
		return 0
	}
	if node.isLeaf {
		return 1
	}
	return countLeavesRecursive(node.left) + countLeavesRecursive(node.right)
}
