package routing

import (
	"context"
	"net/netip"

	"github.com/goodvin/d2ip/internal/config"
)

// PolicyRouter manages routes for multiple independent policies.
type PolicyRouter interface {
	Caps(ctx context.Context, policy config.PolicyConfig) error
	ApplyPolicy(ctx context.Context, policy config.PolicyConfig, v4, v6 []netip.Prefix) error
	DryRunPolicy(ctx context.Context, policy config.PolicyConfig, v4, v6 []netip.Prefix) (v4Plan, v6Plan Plan, v4Diff, v6Diff string, err error)
	RollbackPolicy(ctx context.Context, policyName string) error
	SnapshotPolicy(policyName string) RouterState
}

// buildPlan computes the difference between current and desired prefixes.
func buildPlan(current, desired []netip.Prefix, family Family) Plan {
	currentSet := make(map[string]struct{})
	for _, p := range current {
		currentSet[p.String()] = struct{}{}
	}
	desiredSet := make(map[string]struct{})
	for _, p := range desired {
		desiredSet[p.String()] = struct{}{}
	}

	var plan Plan
	plan.Family = family

	for _, p := range desired {
		if _, ok := currentSet[p.String()]; !ok {
			plan.Add = append(plan.Add, p)
		}
	}
	for _, p := range current {
		if _, ok := desiredSet[p.String()]; !ok {
			plan.Remove = append(plan.Remove, p)
		}
	}
	return plan
}

// diffString returns a human-readable diff of a plan.
func diffString(p Plan) string {
	var out string
	for _, r := range p.Remove {
		out += "- " + r.String() + "\n"
	}
	for _, a := range p.Add {
		out += "+ " + a.String() + "\n"
	}
	if out == "" {
		out = "(no changes)\n"
	}
	return out
}
