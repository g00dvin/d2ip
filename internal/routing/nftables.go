package routing

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/netip"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/goodvin/d2ip/internal/config"
)

// nftRouter is the nftables backend. It owns `table inet d2ip` with sets
// d2ip_v4 and d2ip_v6 (both interval sets). All mutations are performed via
// `nft -f -` for single-transaction atomicity.
type nftRouter struct {
	cfg   config.RoutingConfig
	mu    sync.Mutex // process-wide serialization of Apply/Rollback
	state RouterState
}

func newNFTRouter(cfg config.RoutingConfig) *nftRouter {
	r := &nftRouter{cfg: cfg}
	// best-effort: pre-load state so Snapshot() reports truth before first Apply
	if s, err := loadState(cfg.StatePath); err == nil {
		r.state = s
	}
	return r
}

// Caps verifies nft is on PATH. The CAP_NET_ADMIN check is left to Apply
// (which will fail with EPERM clearly from nft itself).
func (r *nftRouter) Caps() error {
	if _, err := exec.LookPath("nft"); err != nil {
		return fmt.Errorf("%w: nft not found: %v", ErrNoCapability, err)
	}
	return nil
}

// Plan reads the current set contents from the kernel and computes the diff.
func (r *nftRouter) Plan(ctx context.Context, desired []netip.Prefix, f Family) (Plan, error) {
	desired = filterByFamily(desired, f)
	current, err := r.listSet(ctx, f)
	if err != nil {
		return Plan{}, err
	}
	return computePlan(current, desired, f), nil
}

// Apply executes the plan as a single nft transaction. Idempotent: an empty
// plan issues zero commands.
func (r *nftRouter) Apply(ctx context.Context, p Plan) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if err := r.ensureTable(ctx); err != nil {
		return err
	}
	if p.Empty() {
		return r.refreshState(p)
	}
	script := r.buildScript(p)
	if err := r.runScript(ctx, script); err != nil {
		return fmt.Errorf("routing/nft: apply: %w", err)
	}
	return r.refreshState(p)
}

// refreshState reconciles in-memory state and persists it to disk.
func (r *nftRouter) refreshState(p Plan) error {
	s := r.state
	s.Backend = string(config.BackendNFTables)
	s.AppliedAt = time.Now().UTC()

	apply := func(list *[]netip.Prefix) {
		cur := dedup(*list)
		curSet := make(map[netip.Prefix]struct{}, len(cur))
		for _, x := range cur {
			curSet[x] = struct{}{}
		}
		for _, x := range p.Remove {
			delete(curSet, x)
		}
		for _, x := range p.Add {
			curSet[x] = struct{}{}
		}
		out := make([]netip.Prefix, 0, len(curSet))
		for x := range curSet {
			out = append(out, x)
		}
		sortPrefixes(out)
		*list = out
	}

	if p.Family == FamilyV4 {
		apply(&s.V4)
	} else {
		apply(&s.V6)
	}
	if err := saveState(r.cfg.StatePath, s); err != nil {
		return err
	}
	r.state = s
	return nil
}

// Snapshot returns the in-memory state (last known good).
func (r *nftRouter) Snapshot() RouterState {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.state
}

// Rollback removes every prefix listed in the state file (scoped — we never
// enumerate-and-flush the kernel set because user-added entries may coexist).
func (r *nftRouter) Rollback(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	s := r.state
	if len(s.V4) == 0 && len(s.V6) == 0 {
		return nil
	}
	if err := r.ensureTable(ctx); err != nil {
		return err
	}
	var sb strings.Builder
	r.writeRemove(&sb, FamilyV4, s.V4)
	r.writeRemove(&sb, FamilyV6, s.V6)
	if sb.Len() == 0 {
		return nil
	}
	if err := r.runScript(ctx, sb.String()); err != nil {
		return fmt.Errorf("routing/nft: rollback: %w", err)
	}
	r.state = RouterState{Backend: string(config.BackendNFTables), AppliedAt: time.Now().UTC()}
	return saveState(r.cfg.StatePath, r.state)
}

// DryRun returns the plan and a human-readable diff without executing.
func (r *nftRouter) DryRun(ctx context.Context, desired []netip.Prefix, f Family) (Plan, string, error) {
	p, err := r.Plan(ctx, desired, f)
	if err != nil {
		return Plan{}, "", err
	}
	return p, renderDiff(p), nil
}

// --- nft plumbing ---------------------------------------------------------

func (r *nftRouter) tableArgs() (family, name string) {
	// cfg.NFTTable is e.g. "inet d2ip"
	parts := strings.Fields(r.cfg.NFTTable)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return "inet", "d2ip"
}

func (r *nftRouter) setName(f Family) string {
	if f == FamilyV6 {
		return r.cfg.NFTSetV6
	}
	return r.cfg.NFTSetV4
}

// ensureTable creates (or leaves intact) the table + sets. Idempotent.
func (r *nftRouter) ensureTable(ctx context.Context) error {
	fam, name := r.tableArgs()
	script := fmt.Sprintf(`add table %s %s
add set %s %s %s { type ipv4_addr; flags interval; }
add set %s %s %s { type ipv6_addr; flags interval; }
`, fam, name, fam, name, r.cfg.NFTSetV4, fam, name, r.cfg.NFTSetV6)
	return r.runScript(ctx, script)
}

// buildScript constructs a single-transaction nft script for Plan p.
func (r *nftRouter) buildScript(p Plan) string {
	var sb strings.Builder
	r.writeRemove(&sb, p.Family, p.Remove)
	r.writeAdd(&sb, p.Family, p.Add)
	return sb.String()
}

func (r *nftRouter) writeAdd(sb *strings.Builder, f Family, prefixes []netip.Prefix) {
	if len(prefixes) == 0 {
		return
	}
	fam, name := r.tableArgs()
	fmt.Fprintf(sb, "add element %s %s %s { %s }\n", fam, name, r.setName(f), joinPrefixes(prefixes))
}

func (r *nftRouter) writeRemove(sb *strings.Builder, f Family, prefixes []netip.Prefix) {
	if len(prefixes) == 0 {
		return
	}
	fam, name := r.tableArgs()
	fmt.Fprintf(sb, "delete element %s %s %s { %s }\n", fam, name, r.setName(f), joinPrefixes(prefixes))
}

func joinPrefixes(ps []netip.Prefix) string {
	parts := make([]string, len(ps))
	for i, p := range ps {
		parts[i] = p.String()
	}
	return strings.Join(parts, ", ")
}

// listSet returns the elements currently in the set for family f by parsing
// `nft --json list set ...` — we use plain text to keep deps to stdlib.
func (r *nftRouter) listSet(ctx context.Context, f Family) ([]netip.Prefix, error) {
	fam, name := r.tableArgs()
	cmd := exec.CommandContext(ctx, "nft", "list", "set", fam, name, r.setName(f))
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		// Missing table/set is not a hard error — treat as empty (first run).
		if strings.Contains(errb.String(), "No such file") ||
			strings.Contains(errb.String(), "does not exist") {
			return nil, nil
		}
		return nil, fmt.Errorf("routing/nft: list set: %w: %s", err, errb.String())
	}
	return parseNftSet(out.String())
}

// parseNftSet extracts "elements = { 1.2.3.0/24, 4.5.6.7, ... }" from nft output.
func parseNftSet(text string) ([]netip.Prefix, error) {
	i := strings.Index(text, "elements")
	if i < 0 {
		return nil, nil
	}
	open := strings.Index(text[i:], "{")
	close := strings.Index(text[i:], "}")
	if open < 0 || close < 0 || close < open {
		return nil, nil
	}
	body := text[i+open+1 : i+close]
	var out []netip.Prefix
	for _, raw := range strings.Split(body, ",") {
		s := strings.TrimSpace(raw)
		if s == "" {
			continue
		}
		// Strip trailing comments nft may emit.
		if j := strings.IndexAny(s, " \t"); j >= 0 {
			s = s[:j]
		}
		p, err := parsePrefixLoose(s)
		if err != nil {
			return nil, fmt.Errorf("routing/nft: parse element %q: %w", s, err)
		}
		out = append(out, p)
	}
	return out, nil
}

// parsePrefixLoose accepts either "1.2.3.4" (host) or "1.2.3.0/24" (prefix).
func parsePrefixLoose(s string) (netip.Prefix, error) {
	if strings.Contains(s, "/") {
		return netip.ParsePrefix(s)
	}
	a, err := netip.ParseAddr(s)
	if err != nil {
		return netip.Prefix{}, err
	}
	bits := 32
	if a.Is6() {
		bits = 128
	}
	return a.Prefix(bits)
}

// runScript feeds script into `nft -f -` (optionally `nft -c -f -` for dry).
func (r *nftRouter) runScript(ctx context.Context, script string) error {
	if strings.TrimSpace(script) == "" {
		return nil
	}
	args := []string{"-f", "-"}
	if r.cfg.DryRun {
		args = append([]string{"-c"}, args...)
	}
	cmd := exec.CommandContext(ctx, "nft", args...)
	cmd.Stdin = strings.NewReader(script)
	var errb bytes.Buffer
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		return errors.New(strings.TrimSpace(errb.String() + " | " + err.Error()))
	}
	return nil
}
