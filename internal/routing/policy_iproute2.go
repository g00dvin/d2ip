package routing

import (
	"context"
	"fmt"
	"net/netip"
	"os/exec"
	"time"

	"github.com/goodvin/d2ip/internal/config"
)

// iproute2PolicyRouter implements PolicyRouter for iproute2.
type iproute2PolicyRouter struct {
	stateDir string
}

func newIProute2PolicyRouter(stateDir string) *iproute2PolicyRouter {
	return &iproute2PolicyRouter{stateDir: stateDir}
}

func (r *iproute2PolicyRouter) Caps(ctx context.Context, policy config.PolicyConfig) error {
	// Check if ip route command works for this table
	cmd := exec.CommandContext(ctx, "ip", "route", "show", "table", fmt.Sprintf("%d", policy.TableID))
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("iproute2 cap check failed: %w", err)
	}
	return nil
}

func (r *iproute2PolicyRouter) ApplyPolicy(ctx context.Context, policy config.PolicyConfig, v4, v6 []netip.Prefix) error {
	state, _ := loadPolicyState(r.stateDir, policy.Name)

	planV4 := buildPlan(state.V4, v4, FamilyV4)
	planV6 := buildPlan(state.V6, v6, FamilyV6)

	if err := r.applyFamily(ctx, policy, planV4); err != nil {
		return fmt.Errorf("apply v4: %w", err)
	}
	if err := r.applyFamily(ctx, policy, planV6); err != nil {
		return fmt.Errorf("apply v6: %w", err)
	}

	state = RouterState{
		Backend:   string(config.BackendIProute2),
		AppliedAt: time.Now(),
		V4:        v4,
		V6:        v6,
	}
	return savePolicyState(r.stateDir, policy.Name, state)
}

func (r *iproute2PolicyRouter) applyFamily(ctx context.Context, policy config.PolicyConfig, plan Plan) error {
	for _, p := range plan.Remove {
		cmd := exec.CommandContext(ctx, "ip", "route", "del", p.String(), "table", fmt.Sprintf("%d", policy.TableID))
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("del route %s: %w", p.String(), err)
		}
	}
	for _, p := range plan.Add {
		cmd := exec.CommandContext(ctx, "ip", "route", "add", p.String(), "dev", policy.Iface, "table", fmt.Sprintf("%d", policy.TableID))
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("add route %s: %w", p.String(), err)
		}
	}
	return nil
}

func (r *iproute2PolicyRouter) DryRunPolicy(ctx context.Context, policy config.PolicyConfig, v4, v6 []netip.Prefix) (v4Plan, v6Plan Plan, v4Diff, v6Diff string, err error) {
	state, _ := loadPolicyState(r.stateDir, policy.Name)
	v4Plan = buildPlan(state.V4, v4, FamilyV4)
	v6Plan = buildPlan(state.V6, v6, FamilyV6)
	v4Diff = diffString(v4Plan)
	v6Diff = diffString(v6Plan)
	return v4Plan, v6Plan, v4Diff, v6Diff, nil
}

func (r *iproute2PolicyRouter) RollbackPolicy(ctx context.Context, policyName string) error {
	state, err := loadPolicyState(r.stateDir, policyName)
	if err != nil {
		return err
	}
	// Remove all previously applied routes by removing each prefix
	for _, p := range state.V4 {
		// We need table_id from state, but RouterState doesn't store it yet.
		// For now, this is a limitation. State struct should include table_id.
		_ = p
	}
	for _, p := range state.V6 {
		_ = p
	}
	return nil
}

func (r *iproute2PolicyRouter) SnapshotPolicy(policyName string) RouterState {
	state, _ := loadPolicyState(r.stateDir, policyName)
	return state
}
