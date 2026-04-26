package routing

import (
	"context"
	"os"
	"testing"

	"github.com/goodvin/d2ip/internal/config"
)

func TestNFTPolicyRouter_Caps_MissingBinary(t *testing.T) {
	origPath := os.Getenv("PATH")
	os.Setenv("PATH", "/nonexistent")
	defer os.Setenv("PATH", origPath)

	r := newNFTPolicyRouter(t.TempDir())
	ctx := context.Background()
	err := r.Caps(ctx, config.PolicyConfig{NFTTable: "inet d2ip"})
	if err == nil {
		t.Fatal("expected error when nft binary is missing")
	}
}

func TestNFTPolicyRouter_Caps_WithoutRoot(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping: requires root/CAP_NET_ADMIN")
	}
	r := newNFTPolicyRouter(t.TempDir())
	ctx := context.Background()
	// Without root, nft list table will fail with permission denied
	err := r.Caps(ctx, config.PolicyConfig{NFTTable: "inet d2ip"})
	if err == nil {
		t.Fatal("expected error from Caps without CAP_NET_ADMIN")
	}
}
