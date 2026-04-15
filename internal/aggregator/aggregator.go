// Package aggregator implements IP address aggregation into CIDR prefixes
// using the pkg/cidr radix tree algorithm.
package aggregator

import (
	"net/netip"

	"github.com/goodvin/d2ip/pkg/cidr"
)

// Aggressiveness defines the aggregation level.
type Aggressiveness uint8

const (
	AggOff          Aggressiveness = iota // No aggregation, /32 or /128 per address
	AggConservative                       // Lossless aggregation
	AggBalanced                           // Balanced (may introduce some addresses)
	AggAggressive                         // Aggressive (may introduce many addresses)
)

// Aggregator performs CIDR aggregation.
type Aggregator struct{}

// New creates a new Aggregator.
func New() *Aggregator {
	return &Aggregator{}
}

// AggregateV4 aggregates IPv4 addresses into CIDR prefixes.
func (a *Aggregator) AggregateV4(in []netip.Addr, level Aggressiveness, maxPrefix int) []netip.Prefix {
	threshold := levelToThreshold(level)
	return cidr.Aggregate(in, threshold, maxPrefix)
}

// AggregateV6 aggregates IPv6 addresses into CIDR prefixes.
func (a *Aggregator) AggregateV6(in []netip.Addr, level Aggressiveness, maxPrefix int) []netip.Prefix {
	threshold := levelToThreshold(level)
	return cidr.Aggregate(in, threshold, maxPrefix)
}

// levelToThreshold converts Aggressiveness level to a threshold value for pkg/cidr.
func levelToThreshold(level Aggressiveness) float64 {
	switch level {
	case AggOff:
		return -1.0 // Special value: no aggregation
	case AggConservative:
		return 1.0 // Lossless: only merge when 100% of subtree present
	case AggBalanced:
		return 0.75 // Balanced: merge when 75% of subtree present
	case AggAggressive:
		return 0.5 // Aggressive: merge when 50% of subtree present
	default:
		return 1.0 // Default to conservative
	}
}
