package routing

import (
	"bufio"
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

// iproute2Router is the fallback backend using `ip route` on a custom table.
// It refuses to operate without an interface (cfg.Iface).
type iproute2Router struct {
	cfg   config.RoutingConfig
	iface string
	mu    sync.Mutex
	state RouterState
}

func newIProute2Router(cfg config.RoutingConfig) *iproute2Router {
	r := &iproute2Router{cfg: cfg, iface: cfg.Iface}
	if s, err := loadState(cfg.StatePath); err == nil {
		r.state = s
	}
	return r
}

// SetIface allows orchestration code to inject the egress interface name.
// Without it, Apply returns an error (routes with no dev/via are unsafe).
func (r *iproute2Router) SetIface(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.iface = name
}

func (r *iproute2Router) Caps() error {
	if _, err := exec.LookPath("ip"); err != nil {
		return fmt.Errorf("%w: ip not found: %v", ErrNoCapability, err)
	}
	return nil
}

func (r *iproute2Router) Plan(ctx context.Context, desired []netip.Prefix, f Family) (Plan, error) {
	desired = filterByFamily(desired, f)
	current, err := r.listRoutes(ctx, f)
	if err != nil {
		return Plan{}, err
	}
	return computePlan(current, desired, f), nil
}

func (r *iproute2Router) Apply(ctx context.Context, p Plan) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.iface == "" {
		return errors.New("routing/iproute2: no iface configured; refusing to apply")
	}
	if p.Empty() {
		return r.refreshState(p)
	}

	// Execute as a batch via `ip -batch -` for atomic-ish application.
	var sb strings.Builder
	fam := ipFam(p.Family)
	for _, pr := range p.Remove {
		fmt.Fprintf(&sb, "%s route del %s dev %s table %d proto static\n", fam, pr, r.iface, r.cfg.TableID)
	}
	for _, pr := range p.Add {
		fmt.Fprintf(&sb, "%s route add %s dev %s table %d proto static\n", fam, pr, r.iface, r.cfg.TableID)
	}
	if r.cfg.DryRun {
		return r.refreshState(p)
	}
	if err := r.runBatch(ctx, sb.String()); err != nil {
		return fmt.Errorf("routing/iproute2: apply: %w", err)
	}
	return r.refreshState(p)
}

func (r *iproute2Router) Snapshot() RouterState {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.state
}

func (r *iproute2Router) Rollback(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.iface == "" {
		return errors.New("routing/iproute2: no iface configured; cannot rollback")
	}
	var sb strings.Builder
	for _, p := range r.state.V4 {
		fmt.Fprintf(&sb, "-4 route del %s dev %s table %d proto static\n", p, r.iface, r.cfg.TableID)
	}
	for _, p := range r.state.V6 {
		fmt.Fprintf(&sb, "-6 route del %s dev %s table %d proto static\n", p, r.iface, r.cfg.TableID)
	}
	if sb.Len() == 0 {
		return nil
	}
	if err := r.runBatch(ctx, sb.String()); err != nil {
		return fmt.Errorf("routing/iproute2: rollback: %w", err)
	}
	r.state = RouterState{Backend: string(config.BackendIProute2), AppliedAt: time.Now().UTC()}
	return saveState(r.cfg.StatePath, r.state)
}

func (r *iproute2Router) DryRun(ctx context.Context, desired []netip.Prefix, f Family) (Plan, string, error) {
	p, err := r.Plan(ctx, desired, f)
	if err != nil {
		return Plan{}, "", err
	}
	return p, renderDiff(p), nil
}

// --- plumbing -------------------------------------------------------------

func ipFam(f Family) string {
	if f == FamilyV6 {
		return "-6"
	}
	return "-4"
}

func (r *iproute2Router) refreshState(p Plan) error {
	s := r.state
	s.Backend = string(config.BackendIProute2)
	s.AppliedAt = time.Now().UTC()

	apply := func(list *[]netip.Prefix) {
		set := make(map[netip.Prefix]struct{})
		for _, x := range *list {
			set[x] = struct{}{}
		}
		for _, x := range p.Remove {
			delete(set, x)
		}
		for _, x := range p.Add {
			set[x] = struct{}{}
		}
		out := make([]netip.Prefix, 0, len(set))
		for x := range set {
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

func (r *iproute2Router) listRoutes(ctx context.Context, f Family) ([]netip.Prefix, error) {
	cmd := exec.CommandContext(ctx, "ip", ipFam(f), "route", "show", "table", fmt.Sprint(r.cfg.TableID))
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		stderr := errb.String()
		if strings.Contains(stderr, "does not exist") || strings.Contains(stderr, "invalid argument") {
			return nil, nil
		}
		return nil, fmt.Errorf("routing/iproute2: list: %w: %s", err, stderr)
	}
	return parseIPRouteShow(out.String(), f)
}

// parseIPRouteShow extracts the leading prefix token from each line of
// `ip route show table N` output.
func parseIPRouteShow(text string, f Family) ([]netip.Prefix, error) {
	var out []netip.Prefix
	sc := bufio.NewScanner(strings.NewReader(text))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		tok := fields[0]
		if tok == "default" {
			continue // we never own `default`
		}
		p, err := parsePrefixLoose(tok)
		if err != nil {
			continue // skip non-prefix noise (blackhole etc.)
		}
		if f == FamilyV4 && !p.Addr().Is4() {
			continue
		}
		if f == FamilyV6 && p.Addr().Is4() {
			continue
		}
		out = append(out, p)
	}
	return out, sc.Err()
}

func (r *iproute2Router) runBatch(ctx context.Context, batch string) error {
	if strings.TrimSpace(batch) == "" {
		return nil
	}
	cmd := exec.CommandContext(ctx, "ip", "-batch", "-")
	cmd.Stdin = strings.NewReader(batch)
	var errb bytes.Buffer
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		return errors.New(strings.TrimSpace(errb.String() + " | " + err.Error()))
	}
	return nil
}
