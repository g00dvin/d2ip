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
	"github.com/goodvin/d2ip/internal/events"
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
	RunID    int64          `json:"run_id"`
	Domains  int            `json:"domains"`
	Stale    int            `json:"stale"`
	Resolved int            `json:"resolved"`
	Failed   int            `json:"failed"`
	IPv4Out  int            `json:"ipv4_out"`
	IPv6Out  int            `json:"ipv6_out"`
	Duration time.Duration  `json:"duration"`
	Policies []PolicyReport `json:"policies,omitempty"`
}

// PolicyReport is the outcome of a single policy run.
type PolicyReport struct {
	Name     string `json:"name"`
	Domains  int    `json:"domains"`
	Resolved int    `json:"resolved"`
	Failed   int    `json:"failed"`
	IPv4Out  int    `json:"ipv4_out"`
	IPv6Out  int    `json:"ipv6_out"`
	Duration int64  `json:"duration_ms"`
}

// RunStatus describes the current or last-completed pipeline run.
type RunStatus struct {
	Running bool            `json:"running"`
	RunID   int64           `json:"run_id"`
	Started time.Time       `json:"started"`
	Report  *PipelineReport `json:"report"` // nil if still running
}

// ErrBusy is returned when a second Run() is attempted while one is in flight.
var ErrBusy = errors.New("orchestrator: pipeline already running")

// ErrNotRunning is returned when Cancel() is called but no pipeline is active.
var ErrNotRunning = errors.New("orchestrator: no pipeline running")

// Orchestrator composes the agents and executes the pipeline.
type Orchestrator struct {
	// Agent dependencies (injected by cmd/d2ip).
	source     source.DLCStore
	domainlist domainlist.ListProvider
	resolver   resolver.Resolver
	cache      cache.Cache
	aggregator *aggregator.Aggregator
	exporter   *exporter.FileExporter
	policyExp  *exporter.PolicyExporter
	router     routing.Router
	policyRtr  routing.PolicyRouter
	config     func() config.Config // returns current config snapshot

	// Single-flight enforcement.
	mu       sync.Mutex
	running  atomic.Bool
	current  RunStatus
	cancelFn context.CancelFunc

	// Run history (last 10).
	history   []PipelineReport
	historyMu sync.Mutex

	// Event bus for pipeline notifications.
	eventBus *events.Bus
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
	eventBus *events.Bus,
	policyExp *exporter.PolicyExporter,
	policyRtr routing.PolicyRouter,
) *Orchestrator {
	return &Orchestrator{
		source:     src,
		domainlist: dl,
		resolver:   res,
		cache:      cch,
		aggregator: agg,
		exporter:   exp,
		policyExp:  policyExp,
		router:     rtr,
		policyRtr:  policyRtr,
		config:     cfgGetter,
		eventBus:   eventBus,
	}
}

func (o *Orchestrator) emit(eventType string, data any) {
	if o.eventBus == nil {
		return
	}
	o.eventBus.Publish("pipeline", events.Event{
		Topic: "pipeline",
		Type:  eventType,
		Data:  data,
	})
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
	o.emit("pipeline.start", map[string]any{"run_id": runID})
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
			if report.RunID != 0 {
				o.emit("pipeline.failed", map[string]any{
					"run_id": runID,
					"error":  "pipeline failed",
				})
			}
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

	// Update report to reflect only resolvable domain count
	report.Domains = len(resolvable)

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
		o.emit("pipeline.progress", map[string]any{
			"run_id":   runID,
			"step":     "resolver",
			"resolved": report.Resolved,
			"failed":   report.Failed,
			"total":    len(toResolve),
		})

		log.Info().
			Int("valid", report.Resolved).
			Int("failed", report.Failed).
			Int("total", len(results)).
			Msg("orchestrator: resolution summary")

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
		log.Info().
			Int("ipv4", len(ipv4Addrs)).
			Int("ipv6", len(ipv6Addrs)).
			Str("level", string(cfg.Aggregation.Level)).
			Msg("orchestrator: aggregating addresses")
		ipv4Prefixes = o.aggregator.AggregateV4(ipv4Addrs, aggLevel, cfg.Aggregation.V4MaxPrefix)
		ipv6Prefixes = o.aggregator.AggregateV6(ipv6Addrs, aggLevel, cfg.Aggregation.V6MaxPrefix)
	} else {
		log.Info().
			Int("ipv4", len(ipv4Addrs)).
			Int("ipv6", len(ipv6Addrs)).
			Msg("orchestrator: skipping aggregation, converting to /32 and /128")
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

	// Step 8+9: Export and route — per-policy or legacy single-policy
	if len(cfg.Routing.Policies) > 0 && o.policyRtr != nil && o.policyExp != nil {
		for _, policy := range cfg.Routing.Policies {
			if !policy.Enabled {
				continue
			}
			start := time.Now()
			policyReport, err := o.runPolicy(ctx, policy, resolvable, ipv4Addrs, ipv6Addrs, aggLevel, cfg)
			if err != nil {
				// Log error but continue with other policies
				log.Error().Err(err).Str("policy", policy.Name).Msg("orchestrator: policy run failed")
			}
			policyReport.Duration = time.Since(start).Milliseconds()
			report.Policies = append(report.Policies, policyReport)
		}
	} else {
		// Legacy single-policy path
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

		// Step 9: Routing — legacy single-policy path is not available with new config structure.
		// Routing requires at least one policy in routing.policies.
		log.Info().Msg("orchestrator: legacy routing not configured, skipping")
	}

	report.Duration = time.Since(started)
	log.Info().Int64("run_id", runID).Dur("duration", report.Duration).Msg("orchestrator: pipeline completed")

	o.emit("pipeline.complete", map[string]any{
		"run_id":   runID,
		"domains":  report.Domains,
		"resolved": report.Resolved,
		"failed":   report.Failed,
		"ipv4_out": report.IPv4Out,
		"ipv6_out": report.IPv6Out,
		"duration": report.Duration.Seconds(),
	})

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

func (o *Orchestrator) runPolicy(ctx context.Context, policy config.PolicyConfig, allDomains []string, allIPv4, allIPv6 []netip.Addr, aggLevel aggregator.Aggressiveness, cfg config.Config) (PolicyReport, error) {
	// For now, use all IPs (proper domain filtering will be added later)
	v4Out := o.aggregator.AggregateV4(allIPv4, aggLevel, cfg.Aggregation.V4MaxPrefix)
	v6Out := o.aggregator.AggregateV6(allIPv6, aggLevel, cfg.Aggregation.V6MaxPrefix)

	expReport, err := o.policyExp.WritePolicy(ctx, policy, v4Out, v6Out)
	if err != nil {
		return PolicyReport{}, err
	}

	if !policy.DryRun {
		if err := o.policyRtr.ApplyPolicy(ctx, policy, v4Out, v6Out); err != nil {
			return PolicyReport{}, err
		}
	}

	return PolicyReport{
		Name:    policy.Name,
		Domains: len(allDomains),
		IPv4Out: expReport.IPv4Count,
		IPv6Out: expReport.IPv6Count,
	}, nil
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
		return ErrNotRunning
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.cancelFn != nil {
		o.cancelFn()
		return nil
	}
	return ErrNotRunning
}

// History returns the last 10 pipeline runs.
func (o *Orchestrator) History() []PipelineReport {
	o.historyMu.Lock()
	defer o.historyMu.Unlock()
	out := make([]PipelineReport, len(o.history))
	copy(out, o.history)
	return out
}
