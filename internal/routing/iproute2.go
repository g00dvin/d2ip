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

type iproute2Router struct {
	cfg       config.PolicyConfig
	statePath string
	iface     string
	netns     string
	mu        sync.Mutex
	state     RouterState
}

//nolint:unused
func newIProute2Router(cfg config.PolicyConfig, statePath string) *iproute2Router {
	r := &iproute2Router{cfg: cfg, statePath: statePath, iface: cfg.Iface}
	if s, err := loadState(statePath); err == nil {
		r.state = s
	}
	return r
}

func (r *iproute2Router) ipCommand(ctx context.Context, extraArgs ...string) *exec.Cmd {
	args := extraArgs
	if r.netns != "" {
		args = append([]string{"netns", "exec", r.netns, "ip"}, args...)
		return exec.CommandContext(ctx, "ip", args...)
	}
	return exec.CommandContext(ctx, "ip", args...)
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
	if r.netns != "" {
		if err := exec.Command("ip", "netns", "pids", r.netns).Run(); err != nil {
			return fmt.Errorf("%w: netns %q not found: %v", ErrNoCapability, r.netns, err)
		}
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
	for _, pr := range p.Remove {
		fmt.Fprintf(&sb, "route del %s dev %s table %d proto static\n", pr, r.iface, r.cfg.TableID)
	}
	for _, pr := range p.Add {
		fmt.Fprintf(&sb, "route add %s dev %s table %d proto static\n", pr, r.iface, r.cfg.TableID)
	}
	if r.cfg.DryRun {
		return r.refreshState(p)
	}
	if err := r.runBatch(ctx, sb.String(), p.Family); err != nil {
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
	var v4Sb, v6Sb strings.Builder
	for _, p := range r.state.V4 {
		fmt.Fprintf(&v4Sb, "route del %s dev %s table %d proto static\n", p, r.iface, r.cfg.TableID)
	}
	for _, p := range r.state.V6 {
		fmt.Fprintf(&v6Sb, "route del %s dev %s table %d proto static\n", p, r.iface, r.cfg.TableID)
	}
	if v4Sb.Len() == 0 && v6Sb.Len() == 0 {
		return nil
	}
	if v4Sb.Len() > 0 {
		if err := r.runBatch(ctx, v4Sb.String(), FamilyV4); err != nil {
			return fmt.Errorf("routing/iproute2: rollback v4: %w", err)
		}
	}
	if v6Sb.Len() > 0 {
		if err := r.runBatch(ctx, v6Sb.String(), FamilyV6); err != nil {
			return fmt.Errorf("routing/iproute2: rollback v6: %w", err)
		}
	}
	r.state = RouterState{Backend: string(config.BackendIProute2), AppliedAt: time.Now().UTC()}
	return saveState(r.statePath, r.state)
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
	if err := saveState(r.statePath, s); err != nil {
		return err
	}
	r.state = s
	return nil
}

func (r *iproute2Router) listRoutes(ctx context.Context, f Family) ([]netip.Prefix, error) {
	cmd := r.ipCommand(ctx, ipFam(f), "route", "show", "table", fmt.Sprint(r.cfg.TableID))
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

func (r *iproute2Router) runBatch(ctx context.Context, batch string, fam Family) error {
	if strings.TrimSpace(batch) == "" {
		return nil
	}
	args := []string{ipFam(fam), "-batch", "-"}
	cmd := r.ipCommand(ctx, args...)
	cmd.Stdin = strings.NewReader(batch)
	var errb bytes.Buffer
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		return errors.New(strings.TrimSpace(errb.String()) + "\nCommand failed " + cmd.String() + "\n | " + err.Error())
	}
	return nil
}
