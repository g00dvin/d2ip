package routing

import (
	"context"
	"fmt"
	"os/exec"

	"github.com/goodvin/d2ip/internal/config"
	"github.com/rs/zerolog/log"
)

// Validator performs lazy validation of routing backends.
// Only checks backends actually referenced by enabled policies.
type Validator struct {
	health map[config.RoutingBackend]bool // Layer 2 health results
}

// NewValidator creates a new Validator with empty health state.
func NewValidator() *Validator {
	return &Validator{
		health: make(map[config.RoutingBackend]bool),
	}
}

// IsHealthy reports whether a backend passed Layer 2 health check.
// If the backend was never checked, it returns true (optimistic default).
func (v *Validator) IsHealthy(backend config.RoutingBackend) bool {
	if healthy, ok := v.health[backend]; ok {
		return healthy
	}
	return true // unchecked backends assumed healthy
}

// Validate performs Layer 1 (binary) and Layer 2 (health) checks
// only for backends referenced by enabled policies.
// Returns nil if all needed backends are healthy; returns error if any check fails.
// Warnings are logged internally.
func (v *Validator) Validate(ctx context.Context, policies []config.PolicyConfig) error {
	needed := make(map[config.RoutingBackend]bool)
	for _, pol := range policies {
		if pol.Enabled {
			needed[pol.Backend] = true
		}
	}

	if len(needed) == 0 {
		log.Info().Msg("routing: no enabled policies, skipping backend validation")
		return nil
	}

	var firstErr error
	for backend := range needed {
		if err := v.validateBackend(ctx, backend); err != nil {
			log.Warn().Err(err).Str("backend", string(backend)).Msg("routing: backend validation failed")
			if firstErr == nil {
				firstErr = err
			}
		}
	}

	return firstErr
}

func (v *Validator) validateBackend(ctx context.Context, backend config.RoutingBackend) error {
	switch backend {
	case config.BackendNFTables:
		return v.validateNftables(ctx)
	case config.BackendIProute2:
		return v.validateIProute2(ctx)
	default:
		return fmt.Errorf("unsupported backend: %s", backend)
	}
}

func (v *Validator) validateNftables(ctx context.Context) error {
	// Layer 1: binary check
	if _, err := exec.LookPath("nft"); err != nil {
		v.health[config.BackendNFTables] = false
		return fmt.Errorf("nftables binary not found in PATH: %w", err)
	}
	log.Info().Str("backend", "nftables").Msg("routing: binary found")

	// Layer 2: kernel health check
	cmd := exec.CommandContext(ctx, "nft", "list", "tables")
	if err := cmd.Run(); err != nil {
		v.health[config.BackendNFTables] = false
		return fmt.Errorf("nftables kernel subsystem unresponsive: %w", err)
	}
	v.health[config.BackendNFTables] = true
	log.Info().Str("backend", "nftables").Msg("routing: health check passed")
	return nil
}

func (v *Validator) validateIProute2(ctx context.Context) error {
	// Layer 1: binary check
	if _, err := exec.LookPath("ip"); err != nil {
		v.health[config.BackendIProute2] = false
		return fmt.Errorf("iproute2 binary not found in PATH: %w", err)
	}
	log.Info().Str("backend", "iproute2").Msg("routing: binary found")

	// Layer 2: kernel health check
	cmd := exec.CommandContext(ctx, "ip", "route", "show")
	if err := cmd.Run(); err != nil {
		v.health[config.BackendIProute2] = false
		return fmt.Errorf("iproute2 kernel subsystem unresponsive: %w", err)
	}
	v.health[config.BackendIProute2] = true
	log.Info().Str("backend", "iproute2").Msg("routing: health check passed")
	return nil
}
