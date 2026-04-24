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
	"github.com/goodvin/d2ip/internal/events"
	"github.com/goodvin/d2ip/internal/exporter"
	"github.com/goodvin/d2ip/internal/metrics"
	"github.com/goodvin/d2ip/internal/resolver"
	"github.com/goodvin/d2ip/internal/routing"
	"github.com/goodvin/d2ip/internal/sourcereg"
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
	RunID     int64          `json:"run_id"`
	Domains   int            `json:"domains"`
	Stale     int            `json:"stale"`
	Resolved  int            `json:"resolved"`
	CacheHits int            `json:"cache_hits"`
	Failed    int            `json:"failed"`
	IPv4Out   int            `json:"ipv4_out"`
	IPv6Out   int            `json:"ipv6_out"`
	Duration  time.Duration  `json:"duration"`
	Policies  []PolicyReport `json:"policies,omitempty"`
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
	registry   sourcereg.Registry
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

	// Sequential run ID counter (starts at 1).
	runCounter atomic.Int64

	// Event bus for pipeline notifications.
	eventBus *events.Bus
}

// New creates an Orchestrator with injected agent dependencies.
func New(
	registry sourcereg.Registry,
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
		registry:   registry,
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

	runID := o.runCounter.Add(1)
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

	// Step 1-3: Load all sources
	log.Info().Msg("orchestrator: loading sources")
	stepStart := time.Now()
	if err := o.registry.LoadAll(ctx); err != nil {
		log.Error().Err(err).Msg("orchestrator: source load failed")
		return report, fmt.Errorf("source load: %w", err)
	}
	metrics.PipelineStepDuration.WithLabelValues("source").Observe(time.Since(stepStart).Seconds())

	// Check context after I/O
	select {
	case <-ctx.Done():
		log.Warn().Int64("run_id", runID).Msg("orchestrator: pipeline canceled")
		return report, ctx.Err()
	default:
	}

	// Collect all domain categories across all policies
	cfg := o.config()
	allDomainCats, _ := o.categorizePolicyCategories(cfg)

	// Collect all domains that need resolution
	domainSet := make(map[string]struct{})
	for _, cat := range allDomainCats {
		domains, err := o.registry.GetDomains(cat)
		if err != nil {
			log.Warn().Err(err).Str("category", cat).Msg("orchestrator: failed to get domains")
			continue
		}
		for _, d := range domains {
			domainSet[d] = struct{}{}
		}
	}
	resolvable := make([]string, 0, len(domainSet))
	for d := range domainSet {
		resolvable = append(resolvable, d)
	}
	report.Domains = len(resolvable)

	log.Info().
		Int("domains", len(resolvable)).
		Msg("orchestrator: collected resolvable domains")

	// Check context
	select {
	case <-ctx.Done():
		log.Warn().Int64("run_id", runID).Msg("orchestrator: pipeline canceled")
		return report, ctx.Err()
	default:
	}

	// Compute global aggregation level for policies that don't override it
	aggLevel := aggregator.AggBalanced
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

	// Step 4-9: Per-policy execution
	if len(cfg.Routing.Policies) > 0 && o.policyRtr != nil && o.policyExp != nil {
		for _, policy := range cfg.Routing.Policies {
			if !policy.Enabled {
				continue
			}
			policyStart := time.Now()
			policyReport, staleCount, cacheHits, err := o.runPolicy(ctx, policy, req, aggLevel, cfg)
			if err != nil {
				// Log error but continue with other policies
				log.Error().Err(err).Str("policy", policy.Name).Msg("orchestrator: policy run failed")
				report.Policies = append(report.Policies, PolicyReport{
					Name:     policy.Name,
					Duration: time.Since(policyStart).Milliseconds(),
				})
				continue
			}
			policyReport.Duration = time.Since(policyStart).Milliseconds()
			report.Policies = append(report.Policies, policyReport)

			// Accumulate global report counts
			report.Resolved += policyReport.Resolved
			report.Failed += policyReport.Failed
			report.IPv4Out += policyReport.IPv4Out
			report.IPv6Out += policyReport.IPv6Out
			report.Stale += staleCount
			report.CacheHits += cacheHits
		}
	} else {
		log.Info().Msg("orchestrator: no policies configured, skipping export and routing")
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

func (o *Orchestrator) runPolicy(ctx context.Context, policy config.PolicyConfig, req PipelineRequest, aggLevel aggregator.Aggressiveness, cfg config.Config) (PolicyReport, int, int, error) {
	start := time.Now()

	// Separate policy categories into domain and prefix categories
	var domainCats, prefixCats []string
	for _, cat := range policy.Categories {
		_, catType, found := o.registry.ResolveCategory(cat)
		if !found {
			log.Warn().Str("category", cat).Msg("orchestrator: unknown category in policy")
			continue
		}
		if catType == "domain" {
			domainCats = append(domainCats, cat)
		} else {
			prefixCats = append(prefixCats, cat)
		}
	}

	// Collect domains for this policy
	domainSet := make(map[string]struct{})
	for _, cat := range domainCats {
		domains, err := o.registry.GetDomains(cat)
		if err != nil {
			log.Warn().Err(err).Str("category", cat).Msg("orchestrator: failed to get domains for policy")
			continue
		}
		for _, d := range domains {
			domainSet[d] = struct{}{}
		}
	}
	policyDomains := make([]string, 0, len(domainSet))
	for d := range domainSet {
		policyDomains = append(policyDomains, d)
	}

	// Resolve policy domains (use cache)
	var ipv4Addrs, ipv6Addrs []netip.Addr
	var resolvedCount, failedCount int
	staleCount := 0
	cacheHitCount := 0
	if len(policyDomains) > 0 {
		var toResolve []string
		if req.ForceResolve {
			toResolve = policyDomains
			staleCount = len(policyDomains)
		} else {
			stale, err := o.cache.NeedsRefresh(ctx, policyDomains, cfg.Cache.TTL, cfg.Cache.FailedTTL)
			if err != nil {
				return PolicyReport{}, 0, 0, fmt.Errorf("cache check: %w", err)
			}
			toResolve = stale
			staleCount = len(stale)
			cacheHitCount = len(policyDomains) - len(stale)
		}

		if len(toResolve) > 0 {
			log.Info().
				Int("count", len(toResolve)).
				Bool("force", req.ForceResolve).
				Str("policy", policy.Name).
				Msg("orchestrator: resolving policy domains")

			resultsCh := o.resolver.ResolveBatch(ctx, toResolve)
			var results []cache.ResolveResult
			for res := range resultsCh {
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
					resolvedCount++
				} else {
					failedCount++
				}
			}
		if err := o.cache.UpsertBatch(ctx, results); err != nil {
			return PolicyReport{}, 0, 0, fmt.Errorf("cache upsert: %w", err)
		}

			log.Info().
				Int("valid", resolvedCount).
				Int("failed", failedCount).
				Int("total", len(results)).
				Str("policy", policy.Name).
				Msg("orchestrator: policy resolution summary")
		}

		var err error
		ipv4Addrs, ipv6Addrs, err = o.cache.SnapshotForDomains(ctx, policyDomains)
		if err != nil {
			return PolicyReport{}, 0, 0, fmt.Errorf("cache snapshot: %w", err)
		}
	}

	// Collect prefixes from IP-direct sources
	var ipv4Prefixes, ipv6Prefixes []netip.Prefix
	for _, cat := range prefixCats {
		prefixes, err := o.registry.GetPrefixes(cat)
		if err != nil {
			log.Warn().Err(err).Str("category", cat).Msg("orchestrator: failed to get prefixes for policy")
			continue
		}
		for _, p := range prefixes {
			if p.Addr().Is4() {
				ipv4Prefixes = append(ipv4Prefixes, p)
			} else {
				ipv6Prefixes = append(ipv6Prefixes, p)
			}
		}
	}

	// Convert resolved addresses to prefixes
	for _, addr := range ipv4Addrs {
		ipv4Prefixes = append(ipv4Prefixes, netip.PrefixFrom(addr, 32))
	}
	for _, addr := range ipv6Addrs {
		ipv6Prefixes = append(ipv6Prefixes, netip.PrefixFrom(addr, 128))
	}

	// Aggregate per policy
	var v4Out, v6Out []netip.Prefix
	if policy.Aggregation != nil && policy.Aggregation.Enabled {
		level := aggregator.AggBalanced
		switch policy.Aggregation.Level {
		case config.AggOff:
			level = aggregator.AggOff
		case config.AggConservative:
			level = aggregator.AggConservative
		case config.AggBalanced:
			level = aggregator.AggBalanced
		case config.AggAggressive:
			level = aggregator.AggAggressive
		}
		v4Out = o.aggregator.AggregateV4(ipv4Prefixes, level, policy.Aggregation.V4MaxPrefix)
		v6Out = o.aggregator.AggregateV6(ipv6Prefixes, level, policy.Aggregation.V6MaxPrefix)
	} else if cfg.Aggregation.Enabled {
		v4Out = o.aggregator.AggregateV4(ipv4Prefixes, aggLevel, cfg.Aggregation.V4MaxPrefix)
		v6Out = o.aggregator.AggregateV6(ipv6Prefixes, aggLevel, cfg.Aggregation.V6MaxPrefix)
	} else {
		v4Out = ipv4Prefixes
		v6Out = ipv6Prefixes
	}

	// Dry run: compute outputs but skip export and routing
	if req.DryRun || policy.DryRun {
		log.Info().Str("policy", policy.Name).Msg("orchestrator: dry run, skipping policy export and routing")
		return PolicyReport{
			Name:     policy.Name,
			Domains:  len(policyDomains),
			Resolved: resolvedCount,
			Failed:   failedCount,
			IPv4Out:  len(v4Out),
			IPv6Out:  len(v6Out),
			Duration: time.Since(start).Milliseconds(),
		}, staleCount, cacheHitCount, nil
	}

	expReport, err := o.policyExp.WritePolicy(ctx, policy, v4Out, v6Out)
	if err != nil {
		return PolicyReport{}, 0, 0, err
	}

	if !req.SkipRouting {
		if err := o.policyRtr.ApplyPolicy(ctx, policy, v4Out, v6Out); err != nil {
			return PolicyReport{}, 0, 0, err
		}
	} else {
		log.Info().Str("policy", policy.Name).Msg("orchestrator: skip routing requested, skipping policy routing")
	}

	return PolicyReport{
		Name:     policy.Name,
		Domains:  len(policyDomains),
		Resolved: resolvedCount,
		Failed:   failedCount,
		IPv4Out:  expReport.IPv4Count,
		IPv6Out:  expReport.IPv6Count,
		Duration: time.Since(start).Milliseconds(),
	}, staleCount, cacheHitCount, nil
}

func (o *Orchestrator) categorizePolicyCategories(cfg config.Config) (domainCats, prefixCats []string) {
	catSet := make(map[string]struct{})
	for _, pol := range cfg.Routing.Policies {
		if !pol.Enabled {
			continue
		}
		for _, cat := range pol.Categories {
			catSet[cat] = struct{}{}
		}
	}
	for cat := range catSet {
		_, catType, found := o.registry.ResolveCategory(cat)
		if !found {
			continue
		}
		if catType == "domain" {
			domainCats = append(domainCats, cat)
		} else {
			prefixCats = append(prefixCats, cat)
		}
	}
	return
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
