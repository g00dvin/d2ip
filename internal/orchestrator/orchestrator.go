// Package orchestrator wires the d2ip pipeline together and enforces
// single-flight execution semantics: only one pipeline run can be active at
// a time. Concurrent triggers receive a Busy error with the in-flight run ID.
//
// The pipeline owns context cancellation, fan-out/fan-in between resolver
// and cache writer, and run history accounting in the runs table.
package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"sync"
	"sync/atomic"
	"time"

	"github.com/goodvin/d2ip/internal/aggregator"
	"github.com/goodvin/d2ip/internal/cache"
	"github.com/goodvin/d2ip/internal/config"
	"github.com/goodvin/d2ip/internal/domainlist"
	"github.com/goodvin/d2ip/internal/exporter"
	"github.com/goodvin/d2ip/internal/metrics"
	"github.com/goodvin/d2ip/internal/resolver"
	"github.com/goodvin/d2ip/internal/routing"
	"github.com/goodvin/d2ip/internal/source"
	"github.com/rs/zerolog/log"
)

// PipelineRequest configures a single pipeline execution.
type PipelineRequest struct {
	DryRun       bool // stop before route apply
	ForceResolve bool // ignore cache TTL, re-resolve everything
	SkipRouting  bool // stop after export
}

// PipelineReport is the outcome of a completed (or failed) pipeline run.
type PipelineReport struct {
	RunID    int64
	Domains  int
	Stale    int
	Resolved int
	Failed   int
	IPv4Out  int
	IPv6Out  int
	// Export   exporter.ExportReport  // TODO: Iteration 3
	// RoutingPlan *routing.Plan        // TODO: Iteration 5
	Duration time.Duration
}

// RunStatus describes the current or last-completed pipeline run.
type RunStatus struct {
	Running bool
	RunID   int64
	Started time.Time
	Report  *PipelineReport // nil if still running
}

// ErrBusy is returned when a second Run() is attempted while one is in flight.
var ErrBusy = errors.New("orchestrator: pipeline already running")

// Orchestrator composes the agents and executes the pipeline.
type Orchestrator struct {
	// Agent dependencies (injected by cmd/d2ip).
	source     source.DLCStore
	domainlist domainlist.ListProvider
	resolver   resolver.Resolver
	cache      cache.Cache
	aggregator *aggregator.Aggregator
	exporter   *exporter.FileExporter
	router     routing.Router
	config     func() config.Config // returns current config snapshot

	// Single-flight enforcement.
	mu      sync.Mutex
	running atomic.Bool
	current RunStatus
	cancelFn context.CancelFunc

	// Run history (last 10).
	history   []PipelineReport
	historyMu sync.Mutex
}

// New creates an Orchestrator with injected agent dependencies.
func New(
	src source.DLCStore,
	dl domainlist.ListProvider,
	res resolver.Resolver,
	cch cache.Cache,
	agg *aggregator.Aggregator,
	exp *exporter.FileExporter,
	rtr routing.Router,
	cfgGetter func() config.Config,
) *Orchestrator {
	return &Orchestrator{
		source:     src,
		domainlist: dl,
		resolver:   res,
		cache:      cch,
		aggregator: agg,
		exporter:   exp,
		router:     rtr,
		config:     cfgGetter,
	}
}

// Run executes the full pipeline: fetch → parse → resolve → cache → aggregate → export → route.
// Returns ErrBusy if another run is already in progress.
func (o *Orchestrator) Run(ctx context.Context, req PipelineRequest) (PipelineReport, error) {
	// Single-flight: acquire the run slot.
	if !o.running.CompareAndSwap(false, true) {
		o.mu.Lock()
		runID := o.current.RunID
		o.mu.Unlock()
		return PipelineReport{}, fmt.Errorf("%w: run_id=%d", ErrBusy, runID)
	}
	defer o.running.Store(false)

	runID := time.Now().UnixNano() // temporary; in Iteration 4 we'll use DB-assigned id
	started := time.Now()

	ctx, cancel := context.WithCancel(ctx)
	o.mu.Lock()
	o.cancelFn = cancel
	o.mu.Unlock()
	defer cancel()

	report := PipelineReport{
		RunID: runID,
	}

	// Track pipeline failure via defer if we don't reach the success path
	pipelineSucceeded := false
	defer func() {
		if !pipelineSucceeded {
			metrics.PipelineRunsTotal.WithLabelValues("failed").Inc()
			o.historyMu.Lock()
			if len(o.history) >= 10 {
				o.history = o.history[1:]
			}
			o.history = append(o.history, report)
			o.historyMu.Unlock()
		}
	}()

	o.mu.Lock()
	o.current = RunStatus{
		Running: true,
		RunID:   runID,
		Started: started,
		Report:  nil,
	}
	o.mu.Unlock()

	log.Info().Int64("run_id", runID).Msg("orchestrator: pipeline started")

	// Step 1: Get config snapshot
	cfg := o.config()

	// Step 2: Source - fetch dlc.dat
	log.Info().Msg("orchestrator: fetching dlc.dat")
	stepStart := time.Now()
	dlcPath, _, err := o.source.Get(ctx, cfg.Source.RefreshInterval)
	metrics.PipelineStepDuration.WithLabelValues("source").Observe(time.Since(stepStart).Seconds())
	if err != nil {
		log.Error().Err(err).Msg("orchestrator: source fetch failed")
		return report, fmt.Errorf("source fetch: %w", err)
	}

	// Check context after I/O
	select {
	case <-ctx.Done():
		log.Warn().Int64("run_id", runID).Msg("orchestrator: pipeline canceled")
		return report, ctx.Err()
	default:
	}

	// Step 3: Domain - parse and select categories
	log.Info().Msg("orchestrator: loading domain list")
	stepStart = time.Now()
	if err := o.domainlist.Load(dlcPath); err != nil {
		log.Error().Err(err).Msg("orchestrator: domain list load failed")
		return report, fmt.Errorf("domainlist load: %w", err)
	}

	selectors := make([]domainlist.CategorySelector, len(cfg.Categories))
	for i, cat := range cfg.Categories {
		selectors[i] = domainlist.CategorySelector{
			Code:  cat.Code,
			Attrs: cat.Attrs,
		}
	}

	log.Info().Msg("orchestrator: selecting categories")
	rules, err := o.domainlist.Select(selectors)
	metrics.PipelineStepDuration.WithLabelValues("domainlist").Observe(time.Since(stepStart).Seconds())
	if err != nil {
		log.Error().Err(err).Msg("orchestrator: category selection failed")
		return report, fmt.Errorf("domainlist select: %w", err)
	}
	report.Domains = len(rules)
	if len(rules) == 0 {
		log.Warn().Msg("orchestrator: no categories configured; add entries to config.categories to resolve domains")
	}

	// Filter to only resolvable rules (Full and RootDomain)
	resolvable := make([]string, 0, len(rules))
	for _, rule := range rules {
		if rule.Type.IsResolvable() {
			resolvable = append(resolvable, rule.Value)
		}
	}

	log.Info().
		Int("total", len(rules)).
		Int("resolvable", len(resolvable)).
		Msg("orchestrator: filtered resolvable domains")

	// Check context
	select {
	case <-ctx.Done():
		log.Warn().Int64("run_id", runID).Msg("orchestrator: pipeline canceled")
		return report, ctx.Err()
	default:
	}

	// Step 4: Cache - check what needs refresh
	log.Info().Msg("orchestrator: checking cache freshness")
	stepStart = time.Now()
	stale, err := o.cache.NeedsRefresh(ctx, resolvable, cfg.Cache.TTL, cfg.Cache.FailedTTL)
	metrics.PipelineStepDuration.WithLabelValues("cache").Observe(time.Since(stepStart).Seconds())
	if err != nil {
		log.Error().Err(err).Msg("orchestrator: cache check failed")
		return report, fmt.Errorf("cache check: %w", err)
	}
	report.Stale = len(stale)

	// Step 5: Resolver - resolve stale domains
	if len(stale) > 0 || req.ForceResolve {
		toResolve := resolvable
		if !req.ForceResolve {
			toResolve = stale
		}

		log.Info().
			Int("count", len(toResolve)).
			Bool("force", req.ForceResolve).
			Msg("orchestrator: resolving domains")

		stepStart = time.Now()
		resultsCh := o.resolver.ResolveBatch(ctx, toResolve)
		results := make([]cache.ResolveResult, 0, len(toResolve))

		for res := range resultsCh {
			// Convert resolver.ResolveResult to cache.ResolveResult
			cacheStatus := cache.StatusValid
			if res.Status == resolver.StatusFailed {
				cacheStatus = cache.StatusFailed
			} else if res.Status == resolver.StatusNXDomain {
				cacheStatus = cache.StatusNXDomain
			}

			results = append(results, cache.ResolveResult{
				Domain:     res.Domain,
				IPv4:       res.IPv4,
				IPv6:       res.IPv6,
				Status:     cacheStatus,
				ResolvedAt: res.ResolvedAt,
				Err:        res.Err,
			})

			if res.Status == resolver.StatusValid {
				report.Resolved++
			} else {
				report.Failed++
			}
		}
		metrics.PipelineStepDuration.WithLabelValues("resolver").Observe(time.Since(stepStart).Seconds())

		// Check context after resolution
		select {
		case <-ctx.Done():
			log.Warn().Int64("run_id", runID).Msg("orchestrator: pipeline canceled")
			return report, ctx.Err()
		default:
		}

		// Step 6: Cache - upsert batch results
		log.Info().Int("count", len(results)).Msg("orchestrator: upserting results to cache")
		if err := o.cache.UpsertBatch(ctx, results); err != nil {
			log.Error().Err(err).Msg("orchestrator: cache upsert failed")
			return report, fmt.Errorf("cache upsert: %w", err)
		}
	} else if len(resolvable) == 0 {
		log.Info().Msg("orchestrator: no resolvable domains selected, skipping resolution")
	} else {
		log.Info().Msg("orchestrator: all domains cached, skipping resolution")
	}

	// Check context
	select {
	case <-ctx.Done():
		log.Warn().Int64("run_id", runID).Msg("orchestrator: pipeline canceled")
		return report, ctx.Err()
	default:
	}

	// Step 7: Aggregator - get snapshot and aggregate
	log.Info().Msg("orchestrator: fetching cache snapshot")
	stepStart = time.Now()
	ipv4Addrs, ipv6Addrs, err := o.cache.Snapshot(ctx)
	if err != nil {
		log.Error().Err(err).Msg("orchestrator: cache snapshot failed")
		return report, fmt.Errorf("cache snapshot: %w", err)
	}

	log.Info().
		Int("ipv4", len(ipv4Addrs)).
		Int("ipv6", len(ipv6Addrs)).
		Msg("orchestrator: aggregating addresses")

	// Convert aggregation level
	aggLevel := aggregator.AggConservative
	switch cfg.Aggregation.Level {
	case config.AggOff:
		aggLevel = aggregator.AggOff
	case config.AggConservative:
		aggLevel = aggregator.AggConservative
	case config.AggBalanced:
		aggLevel = aggregator.AggBalanced
	case config.AggAggressive:
		aggLevel = aggregator.AggAggressive
	}

	var ipv4Prefixes, ipv6Prefixes []netip.Prefix
	if cfg.Aggregation.Enabled {
		ipv4Prefixes = o.aggregator.AggregateV4(ipv4Addrs, aggLevel, cfg.Aggregation.V4MaxPrefix)
		ipv6Prefixes = o.aggregator.AggregateV6(ipv6Addrs, aggLevel, cfg.Aggregation.V6MaxPrefix)
	} else {
		// No aggregation: convert each address to /32 or /128
		ipv4Prefixes = make([]netip.Prefix, len(ipv4Addrs))
		for i, addr := range ipv4Addrs {
			ipv4Prefixes[i] = netip.PrefixFrom(addr, 32)
		}
		ipv6Prefixes = make([]netip.Prefix, len(ipv6Addrs))
		for i, addr := range ipv6Addrs {
			ipv6Prefixes[i] = netip.PrefixFrom(addr, 128)
		}
	}
	metrics.PipelineStepDuration.WithLabelValues("aggregator").Observe(time.Since(stepStart).Seconds())

	report.IPv4Out = len(ipv4Prefixes)
	report.IPv6Out = len(ipv6Prefixes)

	// Check context
	select {
	case <-ctx.Done():
		log.Warn().Int64("run_id", runID).Msg("orchestrator: pipeline canceled")
		return report, ctx.Err()
	default:
	}

	// Step 8: Exporter - write files
	if !req.DryRun {
		log.Info().Msg("orchestrator: exporting prefix files")
		stepStart = time.Now()
		exportReport, err := o.exporter.Write(ctx, ipv4Prefixes, ipv6Prefixes)
		metrics.PipelineStepDuration.WithLabelValues("exporter").Observe(time.Since(stepStart).Seconds())
		if err != nil {
			log.Error().Err(err).Msg("orchestrator: export failed")
			return report, fmt.Errorf("export: %w", err)
		}

		log.Info().
			Str("ipv4_path", exportReport.IPv4Path).
			Str("ipv6_path", exportReport.IPv6Path).
			Bool("unchanged", exportReport.Unchanged).
			Msg("orchestrator: export completed")
	} else {
		log.Info().Msg("orchestrator: dry run, skipping export")
	}

	// Check context
	select {
	case <-ctx.Done():
		log.Warn().Int64("run_id", runID).Msg("orchestrator: pipeline canceled")
		return report, ctx.Err()
	default:
	}

	// Step 9: Routing - apply to kernel (if enabled and not skipped)
	if !req.SkipRouting && cfg.Routing.Enabled {
		log.Info().Msg("orchestrator: applying routing rules")
		stepStart = time.Now()

		// Check capabilities first
		if err := o.router.Caps(); err != nil {
			log.Error().Err(err).Msg("orchestrator: routing capability check failed")
			return report, fmt.Errorf("routing caps: %w", err)
		}

		// Plan IPv4
		planV4, err := o.router.Plan(ctx, ipv4Prefixes, routing.FamilyV4)
		if err != nil {
			log.Error().Err(err).Msg("orchestrator: routing plan v4 failed")
			return report, fmt.Errorf("routing plan v4: %w", err)
		}

		// Plan IPv6
		planV6, err := o.router.Plan(ctx, ipv6Prefixes, routing.FamilyV6)
		if err != nil {
			log.Error().Err(err).Msg("orchestrator: routing plan v6 failed")
			return report, fmt.Errorf("routing plan v6: %w", err)
		}

		log.Info().
			Int("v4_add", len(planV4.Add)).
			Int("v4_remove", len(planV4.Remove)).
			Int("v6_add", len(planV6.Add)).
			Int("v6_remove", len(planV6.Remove)).
			Msg("orchestrator: routing plan computed")

		// Apply plans (skip if dry-run or config dry_run)
		if !req.DryRun && !cfg.Routing.DryRun {
			if err := o.router.Apply(ctx, planV4); err != nil {
				log.Error().Err(err).Msg("orchestrator: routing apply v4 failed")
				return report, fmt.Errorf("routing apply v4: %w", err)
			}

			if err := o.router.Apply(ctx, planV6); err != nil {
				log.Error().Err(err).Msg("orchestrator: routing apply v6 failed")
				return report, fmt.Errorf("routing apply v6: %w", err)
			}

			log.Info().Msg("orchestrator: routing applied successfully")
		} else {
			log.Info().Msg("orchestrator: dry run, skipping routing apply")
		}
		metrics.PipelineStepDuration.WithLabelValues("routing").Observe(time.Since(stepStart).Seconds())
	} else {
		log.Info().Msg("orchestrator: routing skipped (disabled or SkipRouting=true)")
	}

	report.Duration = time.Since(started)
	log.Info().Int64("run_id", runID).Dur("duration", report.Duration).Msg("orchestrator: pipeline completed")

	// Update metrics for successful pipeline run
	pipelineSucceeded = true
	metrics.PipelineRunsTotal.WithLabelValues("success").Inc()
	metrics.PipelineLastSuccess.Set(float64(time.Now().Unix()))

	o.mu.Lock()
	o.current.Running = false
	o.current.Report = &report
	o.mu.Unlock()

	o.historyMu.Lock()
	if len(o.history) >= 10 {
		o.history = o.history[1:]
	}
	o.history = append(o.history, report)
	o.historyMu.Unlock()

	return report, nil
}

// Status returns the current or last-completed run status.
func (o *Orchestrator) Status() RunStatus {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.current
}

// Cancel aborts the current run (if any) by canceling its context.
func (o *Orchestrator) Cancel() error {
	if !o.running.Load() {
		return errors.New("no pipeline running")
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.cancelFn != nil {
		o.cancelFn()
		return nil
	}
	return errors.New("no cancel function available")
}

// History returns the last 10 pipeline runs.
func (o *Orchestrator) History() []PipelineReport {
	o.historyMu.Lock()
	defer o.historyMu.Unlock()
	out := make([]PipelineReport, len(o.history))
	copy(out, o.history)
	return out
}
