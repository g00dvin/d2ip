package routing

import (
	"context"
	"fmt"
	"net/netip"

	"github.com/goodvin/d2ip/internal/config"
)

// CompositeRouter implements PolicyRouter by dispatching to backend-specific routers.
type CompositeRouter struct {
	iproute2 *iproute2PolicyRouter
	nftables *nftPolicyRouter
	stateDir string
}

// NewCompositeRouter creates a new CompositeRouter.
func NewCompositeRouter(cfg config.RoutingConfig) *CompositeRouter {
	return &CompositeRouter{
		iproute2: newIProute2PolicyRouter(cfg.StateDir),
		nftables: newNFTPolicyRouter(cfg.StateDir),
		stateDir: cfg.StateDir,
	}
}

func (c *CompositeRouter) Caps(ctx context.Context, policy config.PolicyConfig) error {
	switch policy.Backend {
	case config.BackendIProute2:
		return c.iproute2.Caps(ctx, policy)
	case config.BackendNFTables:
		return c.nftables.Caps(ctx, policy)
	default:
		return fmt.Errorf("unsupported backend: %s", policy.Backend)
	}
}

func (c *CompositeRouter) ApplyPolicy(ctx context.Context, policy config.PolicyConfig, v4, v6 []netip.Prefix) error {
	switch policy.Backend {
	case config.BackendIProute2:
		return c.iproute2.ApplyPolicy(ctx, policy, v4, v6)
	case config.BackendNFTables:
		return c.nftables.ApplyPolicy(ctx, policy, v4, v6)
	default:
		return fmt.Errorf("unsupported backend: %s", policy.Backend)
	}
}

func (c *CompositeRouter) DryRunPolicy(ctx context.Context, policy config.PolicyConfig, v4, v6 []netip.Prefix) (Plan, Plan, string, string, error) {
	switch policy.Backend {
	case config.BackendIProute2:
		return c.iproute2.DryRunPolicy(ctx, policy, v4, v6)
	case config.BackendNFTables:
		return c.nftables.DryRunPolicy(ctx, policy, v4, v6)
	default:
		return Plan{}, Plan{}, "", "", fmt.Errorf("unsupported backend: %s", policy.Backend)
	}
}

func (c *CompositeRouter) RollbackPolicy(ctx context.Context, policyName string) error {
	state, err := loadPolicyState(c.stateDir, policyName)
	if err != nil {
		return err
	}
	switch config.RoutingBackend(state.Backend) {
	case config.BackendIProute2:
		return c.iproute2.RollbackPolicy(ctx, policyName)
	case config.BackendNFTables:
		return c.nftables.RollbackPolicy(ctx, policyName)
	default:
		return fmt.Errorf("unknown backend in state: %s", state.Backend)
	}
}

func (c *CompositeRouter) SnapshotPolicy(policyName string) RouterState {
	state, _ := loadPolicyState(c.stateDir, policyName)
	return state
}
