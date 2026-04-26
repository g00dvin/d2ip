package routing

import (
	"context"
	"fmt"
	"net/netip"

	"github.com/goodvin/d2ip/internal/config"
	"github.com/rs/zerolog/log"
)

// CompositeRouter implements PolicyRouter by dispatching to backend-specific routers.
type CompositeRouter struct {
	iproute2 *iproute2PolicyRouter
	nftables *nftPolicyRouter
	stateDir string
	validator *Validator
}

// NewCompositeRouter creates a new CompositeRouter.
func NewCompositeRouter(cfg config.RoutingConfig) *CompositeRouter {
	return &CompositeRouter{
		iproute2: newIProute2PolicyRouter(cfg.StateDir),
		nftables: newNFTPolicyRouter(cfg.StateDir),
		stateDir: cfg.StateDir,
	}
}

// SetValidator attaches a Validator to the CompositeRouter for backend health checks.
func (c *CompositeRouter) SetValidator(v *Validator) {
	c.validator = v
}

func (c *CompositeRouter) Caps(ctx context.Context, policy config.PolicyConfig) error {
	switch policy.Backend {
	case config.BackendIProute2:
		return c.iproute2.Caps(ctx, policy)
	case config.BackendNFTables:
		err := c.nftables.Caps(ctx, policy)
		if err != nil && c.validator != nil && c.validator.IsHealthy(config.BackendNFTables) {
			// Layer 2 passed but table-specific check failed → table missing, proceed
			log.Warn().Str("table", policy.NFTTable).Msg("routing: nftables table not found, will create on first run")
			return nil
		}
		return err
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
