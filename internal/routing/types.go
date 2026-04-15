// Package routing implements the HIGH RISK host route/firewall mutation agent.
//
// Only this package mutates external kernel state. Every change is planned,
// owned (objects carry the "d2ip" prefix), and reversible via state file.
package routing

import (
	"net/netip"
	"time"
)

// Family distinguishes IPv4 and IPv6 address families.
type Family uint8

// Family values.
const (
	FamilyV4 Family = iota
	FamilyV6
)

// String returns "v4" or "v6".
func (f Family) String() string {
	if f == FamilyV6 {
		return "v6"
	}
	return "v4"
}

// Plan is the computed set difference between current and desired prefixes
// for a single address family.
type Plan struct {
	Family Family
	Add    []netip.Prefix
	Remove []netip.Prefix
}

// Empty reports whether this plan has no changes.
func (p Plan) Empty() bool { return len(p.Add) == 0 && len(p.Remove) == 0 }

// RouterState is the snapshot persisted to disk after a successful apply.
// It is the authoritative record of what d2ip has installed in the kernel.
type RouterState struct {
	Backend   string         `json:"backend"`
	AppliedAt time.Time      `json:"applied_at"`
	V4        []netip.Prefix `json:"v4_prefixes"`
	V6        []netip.Prefix `json:"v6_prefixes"`
}
