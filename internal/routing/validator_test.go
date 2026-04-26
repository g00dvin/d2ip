package routing

import (
	"context"
	"os"
	"testing"

	"github.com/goodvin/d2ip/internal/config"
)

func TestValidator_IsHealthy_Unchecked(t *testing.T) {
	v := NewValidator()
	if !v.IsHealthy(config.BackendNFTables) {
		t.Error("expected unchecked backend to be healthy")
	}
	if !v.IsHealthy(config.BackendIProute2) {
		t.Error("expected unchecked backend to be healthy")
	}
}

func TestValidator_Validate_NoPolicies(t *testing.T) {
	v := NewValidator()
	ctx := context.Background()
	if err := v.Validate(ctx, nil); err != nil {
		t.Fatalf("expected nil error for no policies, got %v", err)
	}
	if err := v.Validate(ctx, []config.PolicyConfig{}); err != nil {
		t.Fatalf("expected nil error for empty policies, got %v", err)
	}
}

func TestValidator_Validate_DisabledPolicy(t *testing.T) {
	v := NewValidator()
	ctx := context.Background()
	policies := []config.PolicyConfig{
		{Enabled: false, Backend: config.BackendNFTables},
	}
	if err := v.Validate(ctx, policies); err != nil {
		t.Fatalf("expected nil error for disabled policy, got %v", err)
	}
}

func TestValidator_Validate_NFTables_MissingBinary(t *testing.T) {
	origPath := os.Getenv("PATH")
	os.Setenv("PATH", "/nonexistent")
	defer os.Setenv("PATH", origPath)

	v := NewValidator()
	ctx := context.Background()
	policies := []config.PolicyConfig{
		{Enabled: true, Backend: config.BackendNFTables},
	}
	err := v.Validate(ctx, policies)
	if err == nil {
		t.Fatal("expected error when nft binary is missing")
	}
	if v.IsHealthy(config.BackendNFTables) {
		t.Error("expected nftables to be unhealthy after failed validation")
	}
}

func TestValidator_Validate_IProute2_MissingBinary(t *testing.T) {
	origPath := os.Getenv("PATH")
	os.Setenv("PATH", "/nonexistent")
	defer os.Setenv("PATH", origPath)

	v := NewValidator()
	ctx := context.Background()
	policies := []config.PolicyConfig{
		{Enabled: true, Backend: config.BackendIProute2},
	}
	err := v.Validate(ctx, policies)
	if err == nil {
		t.Fatal("expected error when ip binary is missing")
	}
	if v.IsHealthy(config.BackendIProute2) {
		t.Error("expected iproute2 to be unhealthy after failed validation")
	}
}

func TestValidator_Validate_MultiplePolicies_SameBackend(t *testing.T) {
	origPath := os.Getenv("PATH")
	os.Setenv("PATH", "/nonexistent")
	defer os.Setenv("PATH", origPath)

	v := NewValidator()
	ctx := context.Background()
	policies := []config.PolicyConfig{
		{Enabled: true, Backend: config.BackendNFTables, Name: "policy-a"},
		{Enabled: true, Backend: config.BackendNFTables, Name: "policy-b"},
	}
	err := v.Validate(ctx, policies)
	if err == nil {
		t.Fatal("expected error when nft binary is missing")
	}
	// Should still be only one error, not two
}

func TestValidator_Validate_UnsupportedBackend(t *testing.T) {
	v := NewValidator()
	ctx := context.Background()
	policies := []config.PolicyConfig{
		{Enabled: true, Backend: "unknown"},
	}
	err := v.Validate(ctx, policies)
	if err == nil {
		t.Fatal("expected error for unsupported backend")
	}
}
