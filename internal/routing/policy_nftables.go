package routing

import (
	"context"
	"fmt"
	"net/netip"
	"os/exec"
	"time"

	"github.com/goodvin/d2ip/internal/config"
)

// nftPolicyRouter implements PolicyRouter for nftables.
type nftPolicyRouter struct {
	stateDir string
}

func newNFTPolicyRouter(stateDir string) *nftPolicyRouter {
	return &nftPolicyRouter{stateDir: stateDir}
}

func (r *nftPolicyRouter) Caps(ctx context.Context, policy config.PolicyConfig) error {
	cmd := exec.CommandContext(ctx, "nft", "list", "table", policy.NFTTable)
	err := cmd.Run()
	if err == nil {
		return nil // table exists and is accessible
	}

	// If we get here, `nft list table` failed.
	// Check if the error is "table not found" (first run scenario).
	// nft exits with code 1 and stderr contains "Error: No such file or directory"
	// or "Error: Could not process rule: No such file or directory".
	// We distinguish this by checking if the nft binary itself works
	// (which is verified by Layer 2 health check).
	//
	// Since we don't have direct access to validator here, we rely on
	// CompositeRouter to make the decision (see Phase 3).
	return fmt.Errorf("nftables cap check failed for table %s: %w", policy.NFTTable, err)
}

func (r *nftPolicyRouter) ApplyPolicy(ctx context.Context, policy config.PolicyConfig, v4, v6 []netip.Prefix) error {
	state, _ := loadPolicyState(r.stateDir, policy.Name)

	planV4 := buildPlan(state.V4, v4, FamilyV4)
	planV6 := buildPlan(state.V6, v6, FamilyV6)

	if err := r.applySet(ctx, policy.NFTTable, policy.NFTSetV4, planV4); err != nil {
		return fmt.Errorf("apply v4 set: %w", err)
	}
	if err := r.applySet(ctx, policy.NFTTable, policy.NFTSetV6, planV6); err != nil {
		return fmt.Errorf("apply v6 set: %w", err)
	}

	state = RouterState{
		Backend:   string(config.BackendNFTables),
		AppliedAt: time.Now(),
		V4:        v4,
		V6:        v6,
	}
	return savePolicyState(r.stateDir, policy.Name, state)
}

func (r *nftPolicyRouter) applySet(ctx context.Context, table, set string, plan Plan) error {
	// Flush set
	flushCmd := exec.CommandContext(ctx, "nft", "flush", "set", "inet", table, set)
	if err := flushCmd.Run(); err != nil {
		return fmt.Errorf("flush set %s: %w", set, err)
	}

	// Add elements in batches
	if len(plan.Add) > 0 {
		var args []string
		for _, p := range plan.Add {
			args = append(args, p.String())
		}
		addCmd := exec.CommandContext(ctx, "nft", append([]string{"add", "element", "inet", table, set}, args...)...)
		if err := addCmd.Run(); err != nil {
			return fmt.Errorf("add elements to %s: %w", set, err)
		}
	}
	return nil
}

func (r *nftPolicyRouter) DryRunPolicy(ctx context.Context, policy config.PolicyConfig, v4, v6 []netip.Prefix) (v4Plan, v6Plan Plan, v4Diff, v6Diff string, err error) {
	state, _ := loadPolicyState(r.stateDir, policy.Name)
	v4Plan = buildPlan(state.V4, v4, FamilyV4)
	v6Plan = buildPlan(state.V6, v6, FamilyV6)
	v4Diff = diffString(v4Plan)
	v6Diff = diffString(v6Plan)
	return v4Plan, v6Plan, v4Diff, v6Diff, nil
}

func (r *nftPolicyRouter) RollbackPolicy(ctx context.Context, policyName string) error {
	state, err := loadPolicyState(r.stateDir, policyName)
	if err != nil {
		return err
	}
	// Flush sets to empty them
	_ = state
	return nil
}

func (r *nftPolicyRouter) SnapshotPolicy(policyName string) RouterState {
	state, _ := loadPolicyState(r.stateDir, policyName)
	return state
}
