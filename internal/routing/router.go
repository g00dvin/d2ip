package routing

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"sort"
	"strings"

	"github.com/goodvin/d2ip/internal/config"
)

// ErrDisabled is returned (or short-circuited) when routing.enabled=false.
var ErrDisabled = errors.New("routing: disabled")

// ErrNoCapability is returned when Caps() self-check fails.
var ErrNoCapability = errors.New("routing: missing CAP_NET_ADMIN or required binary")

// Router is the high-level contract. See docs/agents/07-routing.md.
type Router interface {
	Caps() error
	Plan(ctx context.Context, desired []netip.Prefix, f Family) (Plan, error)
	Apply(ctx context.Context, p Plan) error
	Snapshot() RouterState
	Rollback(ctx context.Context) error
	DryRun(ctx context.Context, desired []netip.Prefix, f Family) (Plan, string, error)
}

// New constructs a Router for the given config. If routing is disabled a
// noopRouter is returned whose methods are inexpensive and side-effect-free.
func New(cfg config.RoutingConfig) (Router, error) {
	if !cfg.Enabled || cfg.Backend == config.BackendNone {
		return &noopRouter{}, nil
	}
	switch cfg.Backend {
	case config.BackendNFTables:
		return newNFTRouter(cfg), nil
	case config.BackendIProute2:
		return newIProute2Router(cfg), nil
	default:
		return nil, fmt.Errorf("routing: unknown backend %q", cfg.Backend)
	}
}

// computePlan returns the minimal deterministic set difference between
// current and desired prefix lists. Duplicates in inputs are collapsed.
func computePlan(current, desired []netip.Prefix, f Family) Plan {
	cur := dedup(current)
	des := dedup(desired)
	curSet := make(map[netip.Prefix]struct{}, len(cur))
	for _, p := range cur {
		curSet[p] = struct{}{}
	}
	desSet := make(map[netip.Prefix]struct{}, len(des))
	for _, p := range des {
		desSet[p] = struct{}{}
	}

	var add, remove []netip.Prefix
	for _, p := range des {
		if _, ok := curSet[p]; !ok {
			add = append(add, p)
		}
	}
	for _, p := range cur {
		if _, ok := desSet[p]; !ok {
			remove = append(remove, p)
		}
	}
	sortPrefixes(add)
	sortPrefixes(remove)
	return Plan{Family: f, Add: add, Remove: remove}
}

// dedup returns a de-duplicated slice in sorted order.
func dedup(in []netip.Prefix) []netip.Prefix {
	if len(in) == 0 {
		return nil
	}
	seen := make(map[netip.Prefix]struct{}, len(in))
	out := make([]netip.Prefix, 0, len(in))
	for _, p := range in {
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		out = append(out, p)
	}
	sortPrefixes(out)
	return out
}

func sortPrefixes(s []netip.Prefix) {
	sort.Slice(s, func(i, j int) bool {
		if c := s[i].Addr().Compare(s[j].Addr()); c != 0 {
			return c < 0
		}
		return s[i].Bits() < s[j].Bits()
	})
}

// filterByFamily returns only prefixes matching f.
func filterByFamily(in []netip.Prefix, f Family) []netip.Prefix {
	out := make([]netip.Prefix, 0, len(in))
	for _, p := range in {
		if f == FamilyV4 && p.Addr().Is4() {
			out = append(out, p)
		} else if f == FamilyV6 && !p.Addr().Is4() {
			out = append(out, p)
		}
	}
	return out
}

// renderDiff returns a unified-style "+ prefix" / "- prefix" human diff.
func renderDiff(p Plan) string {
	if p.Empty() {
		return "(no changes)\n"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# family=%s add=%d remove=%d\n", p.Family, len(p.Add), len(p.Remove))
	for _, x := range p.Remove {
		fmt.Fprintf(&b, "- %s\n", x)
	}
	for _, x := range p.Add {
		fmt.Fprintf(&b, "+ %s\n", x)
	}
	return b.String()
}

// noopRouter is returned when routing.enabled=false. All methods are
// safe no-ops — no syscalls, no exec, no state file.
type noopRouter struct{}

func (*noopRouter) Caps() error { return nil }
func (*noopRouter) Plan(_ context.Context, _ []netip.Prefix, f Family) (Plan, error) {
	return Plan{Family: f}, nil
}
func (*noopRouter) Apply(_ context.Context, _ Plan) error { return nil }
func (*noopRouter) Snapshot() RouterState                 { return RouterState{Backend: "none"} }
func (*noopRouter) Rollback(_ context.Context) error      { return nil }
func (*noopRouter) DryRun(_ context.Context, _ []netip.Prefix, f Family) (Plan, string, error) {
	return Plan{Family: f}, "(routing disabled)\n", nil
}
