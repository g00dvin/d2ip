package orchestrator

import (
	"context"
	"errors"
	"net/netip"
	"testing"
	"time"

	"github.com/goodvin/d2ip/internal/aggregator"
	"github.com/goodvin/d2ip/internal/cache"
	"github.com/goodvin/d2ip/internal/config"
	"github.com/goodvin/d2ip/internal/events"
	"github.com/goodvin/d2ip/internal/exporter"
	"github.com/goodvin/d2ip/internal/resolver"
	"github.com/goodvin/d2ip/internal/routing"
	"github.com/goodvin/d2ip/internal/sourcereg"
	"go.uber.org/goleak"
)

// TestMain verifies no goroutines leak from any test in this package.
func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

// TestOrchestrator_NoGoroutineLeak is a placeholder test that verifies
// the goleak infrastructure is in place. Full integration testing with mocks
// requires complex setup with all agent interfaces properly implemented.
//
// The orchestrator itself doesn't spawn background goroutines in New(),
// so structural leaks are unlikely. The real leak risk is in Run(), which
// would require full mock setup of all 7 agents.
func TestOrchestrator_NoGoroutineLeak(t *testing.T) {
	defer goleak.VerifyNone(t)

	// Placeholder test - orchestrator construction is simple and doesn't
	// spawn goroutines. Full Run() testing would require:
	// - source.DLCStore mock
	// - domainlist.ListProvider mock
	// - resolver.Resolver mock
	// - cache.Cache mock
	// - aggregator.Aggregator mock
	// - exporter.FileExporter mock
	// - routing.Router mock
	// - config.Config function
	//
	// These mocks are non-trivial and better suited for integration tests.
	// This test mainly ensures goleak is enabled for future work.

	t.Log("goleak enabled for orchestrator package")
}

// mockRegistry is a stub sourcereg.Registry for tests.
type mockRegistry struct{}

func (m *mockRegistry) LoadAll(ctx context.Context) error            { return nil }
func (m *mockRegistry) ListSources() []sourcereg.SourceInfo          { return nil }
func (m *mockRegistry) GetSource(id string) (sourcereg.Source, bool) { return nil, false }
func (m *mockRegistry) ListCategories() []sourcereg.CategoryInfo     { return nil }
func (m *mockRegistry) GetDomains(category string) ([]string, error) {
	return []string{"example.com", "test.com"}, nil
}
func (m *mockRegistry) GetPrefixes(category string) ([]netip.Prefix, error) { return nil, nil }
func (m *mockRegistry) ResolveCategory(category string) (string, string, bool) {
	return "test", "domain", true
}
func (m *mockRegistry) AddSource(ctx context.Context, cfg sourcereg.SourceConfig) error { return nil }
func (m *mockRegistry) RemoveSource(ctx context.Context, id string) error               { return nil }
func (m *mockRegistry) Close() error                                                    { return nil }

// slowMockRegistry delays LoadAll so that single-flight and cancel tests
// have time to observe the running state.
type slowMockRegistry struct {
	mockRegistry
	delay time.Duration
}

func (m *slowMockRegistry) LoadAll(ctx context.Context) error {
	select {
	case <-time.After(m.delay):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// mockResolver is a stub resolver.Resolver for tests.
type mockResolver struct{}

func (m *mockResolver) ResolveBatch(ctx context.Context, domains []string) <-chan resolver.ResolveResult {
	ch := make(chan resolver.ResolveResult, len(domains))
	go func() {
		defer close(ch)
		for _, d := range domains {
			ch <- resolver.ResolveResult{
				Domain: d,
				IPv4:   []netip.Addr{netip.MustParseAddr("1.2.3.4")},
				Status: resolver.StatusValid,
			}
		}
	}()
	return ch
}
func (m *mockResolver) Close() error { return nil }

// mockCache is a stub cache.Cache for tests.
type mockCache struct{}

func (m *mockCache) NeedsRefresh(ctx context.Context, domains []string, ttl, failedTTL time.Duration) ([]string, error) {
	return domains, nil
}
func (m *mockCache) UpsertBatch(ctx context.Context, results []cache.ResolveResult) error { return nil }
func (m *mockCache) Snapshot(ctx context.Context) ([]netip.Addr, []netip.Addr, error) {
	return nil, nil, nil
}
func (m *mockCache) SnapshotForDomains(ctx context.Context, domains []string) ([]netip.Addr, []netip.Addr, error) {
	return []netip.Addr{netip.MustParseAddr("1.2.3.4")}, nil, nil
}
func (m *mockCache) Stats(ctx context.Context) (cache.Stats, error) { return cache.Stats{}, nil }
func (m *mockCache) Vacuum(ctx context.Context, olderThan time.Duration) (int64, error) {
	return 0, nil
}
func (m *mockCache) Close() error { return nil }

// mockRouter is a stub routing.Router for tests.
type mockRouter struct{}

func (m *mockRouter) Caps() error { return nil }
func (m *mockRouter) Plan(ctx context.Context, desired []netip.Prefix, f routing.Family) (routing.Plan, error) {
	return routing.Plan{}, nil
}
func (m *mockRouter) Apply(ctx context.Context, p routing.Plan) error { return nil }
func (m *mockRouter) Snapshot() routing.RouterState                   { return routing.RouterState{} }
func (m *mockRouter) Rollback(ctx context.Context) error              { return nil }
func (m *mockRouter) DryRun(ctx context.Context, desired []netip.Prefix, f routing.Family) (routing.Plan, string, error) {
	return routing.Plan{}, "", nil
}

// mockPolicyRouter is a stub routing.PolicyRouter for tests.
type mockPolicyRouter struct{}

func (m *mockPolicyRouter) Caps(ctx context.Context, policy config.PolicyConfig) error { return nil }
func (m *mockPolicyRouter) ApplyPolicy(ctx context.Context, policy config.PolicyConfig, v4, v6 []netip.Prefix) error {
	return nil
}
func (m *mockPolicyRouter) DryRunPolicy(ctx context.Context, policy config.PolicyConfig, v4, v6 []netip.Prefix) (routing.Plan, routing.Plan, string, string, error) {
	return routing.Plan{}, routing.Plan{}, "", "", nil
}
func (m *mockPolicyRouter) RollbackPolicy(ctx context.Context, policyName string) error { return nil }
func (m *mockPolicyRouter) SnapshotPolicy(policyName string) routing.RouterState {
	return routing.RouterState{}
}

func setupOrchestrator(t *testing.T) *Orchestrator {
	t.Helper()
	reg := &mockRegistry{}
	res := &mockResolver{}
	cch := &mockCache{}
	agg := aggregator.New()
	tmp := t.TempDir()
	exp, _ := exporter.New(tmp)
	policyExp := exporter.NewPolicyExporter(tmp)
	rtr := &mockRouter{}
	bus := events.NewBus()
	cfg := config.Defaults()
	cfg.Routing.Policies = []config.PolicyConfig{
		{
			Name:       "test-policy",
			Enabled:    true,
			Categories: []string{"geosite:test"},
			Backend:    config.BackendNFTables,
			NFTTable:   "inet d2ip",
			NFTSetV4:   "test_v4",
			NFTSetV6:   "test_v6",
			Aggregation: &config.AggregationConfig{
				Enabled:     true,
				Level:       config.AggBalanced,
				V4MaxPrefix: 16,
				V6MaxPrefix: 32,
			},
		},
	}

	return New(reg, res, cch, agg, exp, rtr, func() config.Config { return cfg }, bus, policyExp, &mockPolicyRouter{})
}

func TestOrchestrator_Run_DryRun(t *testing.T) {
	t.Parallel()
	o := setupOrchestrator(t)
	ctx := context.Background()
	req := PipelineRequest{DryRun: true}

	report, err := o.Run(ctx, req)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if report.RunID == 0 {
		t.Fatal("expected RunID > 0")
	}
	if len(report.Policies) == 0 {
		t.Fatal("expected policies in report")
	}
}

func TestOrchestrator_Run_SingleFlight(t *testing.T) {
	tmp := t.TempDir()
	exp, _ := exporter.New(tmp)
	policyExp := exporter.NewPolicyExporter(tmp)
	bus := events.NewBus()
	cfg := config.Defaults()
	cfg.Routing.Policies = []config.PolicyConfig{
		{
			Name:       "test-policy",
			Enabled:    true,
			Categories: []string{"geosite:test"},
			Backend:    config.BackendNFTables,
			NFTTable:   "inet d2ip",
			NFTSetV4:   "test_v4",
			NFTSetV6:   "test_v6",
		},
	}

	o := New(&slowMockRegistry{delay: 200 * time.Millisecond}, &mockResolver{}, &mockCache{}, aggregator.New(), exp, &mockRouter{}, func() config.Config { return cfg }, bus, policyExp, &mockPolicyRouter{})

	ctx := context.Background()
	req := PipelineRequest{}

	done := make(chan struct{})
	var firstErr error
	go func() {
		defer close(done)
		_, firstErr = o.Run(ctx, req)
	}()

	time.Sleep(20 * time.Millisecond)

	_, err := o.Run(ctx, req)
	if !errors.Is(err, ErrBusy) {
		t.Fatalf("expected ErrBusy, got: %v", err)
	}

	<-done
	if firstErr != nil {
		t.Fatalf("first Run returned unexpected error: %v", firstErr)
	}
}

func TestOrchestrator_Cancel(t *testing.T) {
	tmp := t.TempDir()
	exp, _ := exporter.New(tmp)
	policyExp := exporter.NewPolicyExporter(tmp)
	bus := events.NewBus()
	cfg := config.Defaults()
	cfg.Routing.Policies = []config.PolicyConfig{
		{
			Name:       "test-policy",
			Enabled:    true,
			Categories: []string{"geosite:test"},
			Backend:    config.BackendNFTables,
			NFTTable:   "inet d2ip",
			NFTSetV4:   "test_v4",
			NFTSetV6:   "test_v6",
		},
	}

	o := New(&slowMockRegistry{delay: 200 * time.Millisecond}, &mockResolver{}, &mockCache{}, aggregator.New(), exp, &mockRouter{}, func() config.Config { return cfg }, bus, policyExp, &mockPolicyRouter{})

	ctx := context.Background()
	req := PipelineRequest{}

	done := make(chan struct{})
	var report PipelineReport
	var runErr error
	go func() {
		defer close(done)
		report, runErr = o.Run(ctx, req)
	}()

	time.Sleep(20 * time.Millisecond)

	if err := o.Cancel(); err != nil {
		t.Fatalf("Cancel returned error: %v", err)
	}

	<-done

	if runErr == nil {
		t.Fatal("expected Run to return error after cancel")
	}
	if !errors.Is(runErr, context.Canceled) {
		t.Fatalf("expected context.Canceled, got: %v", runErr)
	}
	if report.RunID == 0 {
		t.Fatal("expected RunID > 0 even for canceled run")
	}
}

func TestOrchestrator_Status(t *testing.T) {
	t.Parallel()
	o := setupOrchestrator(t)
	ctx := context.Background()
	req := PipelineRequest{DryRun: true}

	report, err := o.Run(ctx, req)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	status := o.Status()
	if status.Running {
		t.Fatal("expected Running to be false")
	}
	if status.RunID != report.RunID {
		t.Fatalf("expected RunID %d, got %d", report.RunID, status.RunID)
	}
	if status.Report == nil {
		t.Fatal("expected Report to be non-nil")
	}
	if status.Report.RunID != report.RunID {
		t.Fatalf("expected Report.RunID %d, got %d", report.RunID, status.Report.RunID)
	}
}

func TestOrchestrator_History(t *testing.T) {
	t.Parallel()
	o := setupOrchestrator(t)
	ctx := context.Background()
	req := PipelineRequest{DryRun: true}

	report, err := o.Run(ctx, req)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	history := o.History()
	if len(history) != 1 {
		t.Fatalf("expected history length 1, got %d", len(history))
	}
	if history[0].RunID != report.RunID {
		t.Fatalf("expected history[0].RunID %d, got %d", report.RunID, history[0].RunID)
	}
}

func TestOrchestrator_Run_NoPolicies(t *testing.T) {
	t.Parallel()
	reg := &mockRegistry{}
	res := &mockResolver{}
	cch := &mockCache{}
	agg := aggregator.New()
	tmp := t.TempDir()
	exp, _ := exporter.New(tmp)
	policyExp := exporter.NewPolicyExporter(tmp)
	rtr := &mockRouter{}
	bus := events.NewBus()
	cfg := config.Defaults()
	cfg.Routing.Policies = nil

	o := New(reg, res, cch, agg, exp, rtr, func() config.Config { return cfg }, bus, policyExp, &mockPolicyRouter{})

	ctx := context.Background()
	req := PipelineRequest{}

	report, err := o.Run(ctx, req)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if report.RunID == 0 {
		t.Fatal("expected RunID > 0")
	}
	if len(report.Policies) != 0 {
		t.Fatalf("expected no policies, got %d", len(report.Policies))
	}
}
