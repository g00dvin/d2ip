package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/goodvin/d2ip/internal/aggregator"
	"github.com/goodvin/d2ip/internal/api"
	"github.com/goodvin/d2ip/internal/cache"
	"github.com/goodvin/d2ip/internal/config"
	"github.com/goodvin/d2ip/internal/domainlist"
	"github.com/goodvin/d2ip/internal/events"
	"github.com/goodvin/d2ip/internal/exporter"
	"github.com/goodvin/d2ip/internal/logging"
	"github.com/goodvin/d2ip/internal/metrics"
	"github.com/goodvin/d2ip/internal/orchestrator"
	"github.com/goodvin/d2ip/internal/resolver"
	"github.com/goodvin/d2ip/internal/routing"
	"github.com/goodvin/d2ip/internal/scheduler"
	"github.com/goodvin/d2ip/internal/sourcereg"
	_ "github.com/goodvin/d2ip/internal/sourcereg/providers/ipverse"
	_ "github.com/goodvin/d2ip/internal/sourcereg/providers/mmdb"
	_ "github.com/goodvin/d2ip/internal/sourcereg/providers/plaintext"
	_ "github.com/goodvin/d2ip/internal/sourcereg/providers/v2flygeosite"
	_ "github.com/goodvin/d2ip/internal/sourcereg/providers/v2flygeoip"
	"github.com/goodvin/d2ip/internal/source"
	"github.com/rs/zerolog/log"
)

var (
	// Version is set via ldflags during build
	Version = "dev"
	// BuildTime is set via ldflags during build
	BuildTime = "unknown"
)

func main() {
	// Define subcommands
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	subcommand := os.Args[1]

	switch subcommand {
	case "version", "--version", "-v":
		printVersion()
	case "serve":
		serveCmd()
	case "dump":
		dumpCmd()
	case "resolve":
		resolveCmd()
	case "export":
		exportCmd()
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", subcommand)
		printUsage()
		os.Exit(1)
	}
}

func printVersion() {
	fmt.Printf("d2ip version %s (built %s)\n", Version, BuildTime)
}

func printUsage() {
	fmt.Fprintf(os.Stderr, "Usage: %s <command> [options]\n\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "Commands:\n")
	fmt.Fprintf(os.Stderr, "  serve      Start the d2ip server\n")
	fmt.Fprintf(os.Stderr, "  dump       Fetch and parse dlc.dat, output selected categories\n")
	fmt.Fprintf(os.Stderr, "  resolve    Resolve domains and populate cache database\n")
	fmt.Fprintf(os.Stderr, "  export     Export aggregated IPs to ipv4.txt and ipv6.txt\n")
	fmt.Fprintf(os.Stderr, "  version    Print version information\n")
	fmt.Fprintf(os.Stderr, "  help       Show this help message\n")
}

func serveCmd() {
	// Parse flags for serve command
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	logLevel := fs.String("log-level", "", "Log level (debug, info, warn, error)")
	logJSON := fs.Bool("log-json", true, "Output logs in JSON format")
	configFile := fs.String("config", "", "Optional YAML config file path")
	if err := fs.Parse(os.Args[2:]); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing flags: %v\n", err)
		os.Exit(1)
	}

	// Load configuration
	cfg, err := config.Load(config.LoadOptions{
		ConfigFile: *configFile,
		// KVOverrides will be populated from DB in Iteration 4
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Initialize logging (config/ENV first, CLI flag overrides if set)
	level := cfg.Logging.Level
	if *logLevel != "" {
		level = *logLevel
	}
	jsonFmt := *logJSON
	if err := logging.Setup(level, jsonFmt); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to setup logging: %v\n", err)
		os.Exit(1)
	}

	log.Info().
		Str("version", Version).
		Str("build_time", BuildTime).
		Str("listen", cfg.Listen).
		Msg("d2ip starting")

	// Initialize metrics
	if err := metrics.Setup(); err != nil {
		log.Fatal().Err(err).Msg("Failed to setup metrics")
	}
	log.Info().Msg("Metrics initialized")

	ctx := context.Background()

	// Initialize cache
	dbPath := "./d2ip.db"
	if cfg.Cache.DBPath != "" {
		dbPath = cfg.Cache.DBPath
	}
	cacheDB, err := cache.Open(ctx, dbPath)
	if err != nil {
		log.Fatal().Err(err).Str("db", dbPath).Msg("Failed to open cache database")
	}
	defer cacheDB.Close()
	log.Info().Str("db", dbPath).Msg("Cache database ready")

	// Apply KV overrides from cache database (highest priority)
	kvOverrides, err := cacheDB.GetAll(ctx)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to load KV overrides from cache")
	} else if len(kvOverrides) > 0 {
		if err := config.ApplyOverrides(cfg, kvOverrides); err != nil {
			log.Warn().Err(err).Msg("Failed to apply KV overrides from cache")
		} else {
			log.Info().Int("overrides", len(kvOverrides)).Msg("Applied KV overrides from cache")
		}
	}

	// Create source registry
	registry, err := sourcereg.NewDBRegistry(cacheDB.DB())
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create source registry")
	}
	defer registry.Close()

	// Seed registry from config sources
	for _, srcCfg := range cfg.Sources {
		sc := sourcereg.SourceConfig{
			ID:       srcCfg.ID,
			Provider: sourcereg.SourceType(srcCfg.Provider),
			Prefix:   srcCfg.Prefix,
			Enabled:  srcCfg.Enabled,
			Config:   srcCfg.Config,
		}
		if err := registry.AddSource(ctx, sc); err != nil {
			log.Warn().Err(err).Str("id", srcCfg.ID).Msg("failed to seed source")
		}
	}

	// Load all sources
	if err := registry.LoadAll(ctx); err != nil {
		log.Warn().Err(err).Msg("initial source load failed")
	}

	// Initialize resolver
	resolverCfg := resolver.Config{
		Upstream:      cfg.Resolver.Upstream,
		Network:       cfg.Resolver.Network,
		Concurrency:   cfg.Resolver.Concurrency,
		QPS:           cfg.Resolver.QPS,
		Timeout:       cfg.Resolver.Timeout,
		Retries:       cfg.Resolver.Retries,
		BackoffBase:   cfg.Resolver.BackoffBase,
		BackoffMax:    cfg.Resolver.BackoffMax,
		FollowCNAME:   cfg.Resolver.FollowCNAME,
		MaxCNAMEChain: 8,
	}
	resolverAgent, err := resolver.New(resolverCfg)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create resolver")
	}
	defer resolverAgent.Close()

	// Initialize aggregator
	aggAgent := aggregator.New()

	// Initialize exporter
	exportDir := cfg.Export.Dir
	if exportDir == "" {
		exportDir = "./out"
	}
	exportAgent, err := exporter.New(exportDir)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create exporter")
	}

	// Create event bus
	eventBus := events.NewBus()

	// Create config watcher with initial config
	cfgWatcher := config.NewWatcher(*cfg, 1, eventBus)

	// Config snapshot function
	configSnapshot := func() config.Config {
		return cfgWatcher.Current().Config
	}

	// Create legacy router (deprecated, noop — policies use CompositeRouter)
	routerAgent, err := routing.New(cfg.Routing)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create router")
	}
	log.Info().
		Bool("enabled", cfg.Routing.Enabled).
		Msg("Router initialized")

	// Create policy exporter and router
	policyExp := exporter.NewPolicyExporter(cfg.Export.Dir)
	policyRtr := routing.NewCompositeRouter(cfg.Routing)

	// Create and run backend validator (Layer 1 + 2)
	validator := routing.NewValidator()
	if err := validator.Validate(ctx, cfg.Routing.Policies); err != nil {
		log.Warn().Err(err).Msg("routing: backend validation failed, some policies may not be routable")
	}
	policyRtr.SetValidator(validator)

	// Create orchestrator with all agents
	orch := orchestrator.New(
		registry,
		resolverAgent,
		cacheDB,
		aggAgent,
		exportAgent,
		routerAgent,
		configSnapshot,
		eventBus,
		policyExp,
		policyRtr,
	)
	log.Info().Msg("Orchestrator initialized with all agents")

	// Start scheduler if resolve cycle is configured
	var sched *scheduler.Scheduler
	if cfg.Scheduler.ResolveCycle > 0 {
		// Convert duration to cron expression (@every syntax)
		cronExpr := fmt.Sprintf("@every %s", cfg.Scheduler.ResolveCycle)
		sched, err = scheduler.New(orch, cronExpr)
		if err != nil {
			log.Fatal().Err(err).Str("cron", cronExpr).Msg("Failed to create scheduler")
		}
		if err := sched.Start(ctx); err != nil {
			log.Fatal().Err(err).Msg("Failed to start scheduler")
		}
		defer sched.Stop()
		log.Info().Dur("interval", cfg.Scheduler.ResolveCycle).Msg("Scheduler started")
	}

	// Create API server
	apiServer := api.New(orch, routerAgent, cfgWatcher, cacheDB, nil, nil, cacheDB, eventBus, registry)
	apiServer.SetVersion(Version, BuildTime)
	apiServer.SetPolicyRouter(policyRtr)
	httpServer := &http.Server{
		Addr:        cfg.Listen,
		Handler:     apiServer.Handler(),
		ReadTimeout: 10 * time.Second,
		// WriteTimeout is disabled to support long-lived SSE streams.
		// The /events endpoint sends keepalive pings every 30s.
		IdleTimeout: 60 * time.Second,
	}

	// Start HTTP server in background
	go func() {
		log.Info().Str("addr", cfg.Listen).Msg("HTTP server listening")
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("HTTP server failed")
		}
	}()

	// Setup signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Block until signal received
	sig := <-sigChan
	log.Info().Str("signal", sig.String()).Msg("Received shutdown signal")

	// Graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Error().Err(err).Msg("HTTP server shutdown error")
	}

	eventBus.Close()
	log.Info().Msg("Event bus closed")

	log.Info().Msg("Server stopped")
}

func dumpCmd() {
	// Parse flags for dump command
	fs := flag.NewFlagSet("dump", flag.ExitOnError)

	// Repeatable --category flag
	type categoryList []string
	var categories categoryList
	fs.Func("category", "Category to select (e.g., geosite:ru). Repeatable.", func(s string) error {
		categories = append(categories, s)
		return nil
	})

	sourceURL := fs.String("source-url", "https://github.com/v2fly/domain-list-community/releases/latest/download/dlc.dat", "URL to fetch dlc.dat")
	cachePath := fs.String("cache-path", "/tmp/d2ip-dlc.dat", "Local cache path for dlc.dat")
	maxAge := fs.Duration("max-age", 24*time.Hour, "Maximum age before refresh")
	logLevel := fs.String("log-level", "info", "Log level")
	listCategories := fs.Bool("list-categories", false, "List all available categories and exit")

	if err := fs.Parse(os.Args[2:]); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing flags: %v\n", err)
		os.Exit(1)
	}

	// Setup logging
	if err := logging.Setup(*logLevel, false); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to setup logging: %v\n", err)
		os.Exit(1)
	}

	ctx := context.Background()

	// Create source store
	store, err := source.NewHTTPStore(*sourceURL, *cachePath, 30*time.Second)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create source store")
	}

	// Fetch dlc.dat
	dlcPath, version, err := store.Get(ctx, *maxAge)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to fetch dlc.dat")
	}

	log.Info().
		Str("path", dlcPath).
		Str("sha256", version.SHA256[:16]+"...").
		Int64("size", version.Size).
		Time("fetched_at", version.FetchedAt).
		Msg("dlc.dat ready")

	// Parse dlc.dat
	provider := domainlist.NewProvider()
	if err := provider.Load(dlcPath); err != nil {
		log.Fatal().Err(err).Msg("Failed to load dlc.dat")
	}

	// Handle --list-categories
	if *listCategories {
		cats := provider.Categories()
		fmt.Printf("# Available categories (%d total)\n", len(cats))
		for _, cat := range cats {
			fmt.Printf("  - %s\n", cat)
		}
		return
	}

	// Validate at least one category
	if len(categories) == 0 {
		fmt.Fprintf(os.Stderr, "Error: at least one --category required\n\n")
		fs.Usage()
		os.Exit(1)
	}

	// Build selectors
	var selectors []domainlist.CategorySelector
	for _, cat := range categories {
		selectors = append(selectors, domainlist.CategorySelector{Code: cat})
	}

	// Select rules
	rules, err := provider.Select(selectors)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to select categories")
	}

	// Output rules to stdout
	fmt.Printf("# d2ip dump - %d rules from %d categories\n", len(rules), len(categories))
	for _, rule := range rules {
		resolvable := ""
		if !rule.Type.IsResolvable() {
			resolvable = " [UNRESOLVABLE]"
		}
		fmt.Printf("%s\t%s\t%s%s\n", rule.Cat, rule.Type, rule.Value, resolvable)
	}
}

func resolveCmd() {
	// Parse flags for resolve command
	fs := flag.NewFlagSet("resolve", flag.ExitOnError)

	// Repeatable --category flag
	type categoryList []string
	var categories categoryList
	fs.Func("category", "Category to resolve (e.g., geosite:cn). Repeatable.", func(s string) error {
		categories = append(categories, s)
		return nil
	})

	sourceURL := fs.String("source-url", "https://github.com/v2fly/domain-list-community/releases/latest/download/dlc.dat", "URL to fetch dlc.dat")
	cachePath := fs.String("cache-path", "/tmp/d2ip-dlc.dat", "Local cache path for dlc.dat")
	dbPath := fs.String("db", "./d2ip.db", "SQLite database path")
	maxAge := fs.Duration("max-age", 24*time.Hour, "Maximum dlc.dat age before refresh")
	logLevel := fs.String("log-level", "info", "Log level")

	// Resolver configuration
	upstream := fs.String("upstream", "8.8.8.8:53", "Upstream DNS server")
	concurrency := fs.Int("concurrency", 64, "Number of concurrent resolver workers")
	qps := fs.Int("qps", 1000, "Queries per second limit")

	if err := fs.Parse(os.Args[2:]); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing flags: %v\n", err)
		os.Exit(1)
	}

	// Setup logging
	if err := logging.Setup(*logLevel, false); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to setup logging: %v\n", err)
		os.Exit(1)
	}

	ctx := context.Background()

	// Create source store
	store, err := source.NewHTTPStore(*sourceURL, *cachePath, 30*time.Second)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create source store")
	}

	// Fetch dlc.dat
	dlcPath, version, err := store.Get(ctx, *maxAge)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to fetch dlc.dat")
	}

	log.Info().
		Str("path", dlcPath).
		Str("sha256", version.SHA256[:16]+"...").
		Int64("size", version.Size).
		Msg("dlc.dat ready")

	// Parse dlc.dat
	provider := domainlist.NewProvider()
	if err := provider.Load(dlcPath); err != nil {
		log.Fatal().Err(err).Msg("Failed to load dlc.dat")
	}

	// Build selectors
	var selectors []domainlist.CategorySelector
	for _, cat := range categories {
		selectors = append(selectors, domainlist.CategorySelector{Code: cat})
	}

	// Select rules
	rules, err := provider.Select(selectors)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to select categories")
	}

	// Filter only resolvable rules (Full + RootDomain)
	var domains []string
	for _, rule := range rules {
		if rule.Type.IsResolvable() {
			domains = append(domains, rule.Value)
		}
	}

	log.Info().
		Int("total_rules", len(rules)).
		Int("resolvable", len(domains)).
		Msg("Rules selected")

	if len(domains) == 0 {
		log.Warn().Msg("No resolvable domains found")
		return
	}

	// Create resolver
	resolverCfg := resolver.DefaultConfig()
	resolverCfg.Upstream = *upstream
	resolverCfg.Concurrency = *concurrency
	resolverCfg.QPS = *qps

	r, err := resolver.New(resolverCfg)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create resolver")
	}
	defer r.Close()

	// Open cache database
	db, err := cache.Open(ctx, *dbPath)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to open cache database")
	}
	defer db.Close()

	// Resolve domains
	log.Info().Int("domains", len(domains)).Msg("Starting resolution...")
	startTime := time.Now()

	resultCh := r.ResolveBatch(ctx, domains)

	// Collect results and upsert to cache in batches
	var results []cache.ResolveResult
	validCount, failedCount, nxdomainCount := 0, 0, 0

	for result := range resultCh {
		// Convert resolver.Status to cache.Status
		var cacheStatus cache.Status
		switch result.Status {
		case resolver.StatusValid:
			cacheStatus = cache.StatusValid
			validCount++
		case resolver.StatusFailed:
			cacheStatus = cache.StatusFailed
			failedCount++
		case resolver.StatusNXDomain:
			cacheStatus = cache.StatusNXDomain
			nxdomainCount++
		}

		cacheResult := cache.ResolveResult{
			Domain:     result.Domain,
			IPv4:       result.IPv4,
			IPv6:       result.IPv6,
			Status:     cacheStatus,
			ResolvedAt: result.ResolvedAt,
			Err:        result.Err,
		}

		results = append(results, cacheResult)

		// Batch upsert every 1000 results
		if len(results) >= 1000 {
			if err := db.UpsertBatch(ctx, results); err != nil {
				log.Error().Err(err).Msg("Failed to upsert batch")
			}
			log.Info().Int("upserted", len(results)).Msg("Batch upserted")
			results = results[:0]
		}
	}

	// Upsert remaining results
	if len(results) > 0 {
		if err := db.UpsertBatch(ctx, results); err != nil {
			log.Error().Err(err).Msg("Failed to upsert final batch")
		}
	}

	duration := time.Since(startTime)

	// Print summary
	fmt.Printf("\n=== Resolution Summary ===\n")
	fmt.Printf("Total domains:  %d\n", len(domains))
	fmt.Printf("Valid:          %d\n", validCount)
	fmt.Printf("Failed:         %d\n", failedCount)
	fmt.Printf("NXDOMAIN:       %d\n", nxdomainCount)
	fmt.Printf("Duration:       %s\n", duration)
	fmt.Printf("Rate:           %.1f domains/sec\n", float64(len(domains))/duration.Seconds())
	fmt.Printf("Database:       %s\n", *dbPath)

	log.Info().Msg("Resolution complete")
}

func exportCmd() {
	// Parse flags for export command
	fs := flag.NewFlagSet("export", flag.ExitOnError)

	dbPath := fs.String("db", "./d2ip.db", "SQLite database path")
	outputDir := fs.String("output", "./out", "Output directory for ipv4.txt and ipv6.txt")
	logLevel := fs.String("log-level", "info", "Log level")
	aggLevel := fs.String("agg", "off", "Aggregation level: off, conservative, balanced, aggressive")

	if err := fs.Parse(os.Args[2:]); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing flags: %v\n", err)
		os.Exit(1)
	}

	// Setup logging
	if err := logging.Setup(*logLevel, false); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to setup logging: %v\n", err)
		os.Exit(1)
	}

	ctx := context.Background()

	// Open cache database
	db, err := cache.Open(ctx, *dbPath)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to open cache database")
	}
	defer db.Close()

	log.Info().Str("db", *dbPath).Msg("Reading cache snapshot...")

	// Get IP snapshot from cache
	ipv4Addrs, ipv6Addrs, err := db.Snapshot(ctx)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to get cache snapshot")
	}

	log.Info().
		Int("ipv4_addrs", len(ipv4Addrs)).
		Int("ipv6_addrs", len(ipv6Addrs)).
		Msg("Snapshot loaded")

	// Parse aggregation level
	var level aggregator.Aggressiveness
	switch *aggLevel {
	case "off":
		level = aggregator.AggOff
	case "conservative":
		level = aggregator.AggConservative
	case "balanced":
		level = aggregator.AggBalanced
	case "aggressive":
		level = aggregator.AggAggressive
	default:
		log.Fatal().Str("agg", *aggLevel).Msg("Unknown aggregation level")
	}

	// Aggregate
	agg := aggregator.New()
	log.Info().Str("level", *aggLevel).Msg("Aggregating...")

	v4Prefixes := agg.AggregateV4(ipv4Addrs, level, 16)
	v6Prefixes := agg.AggregateV6(ipv6Addrs, level, 32)

	log.Info().
		Int("ipv4_prefixes", len(v4Prefixes)).
		Int("ipv6_prefixes", len(v6Prefixes)).
		Msg("Aggregation complete")

	// Export
	exp, err := exporter.New(*outputDir)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create exporter")
	}

	log.Info().Str("output", *outputDir).Msg("Exporting...")

	report, err := exp.Write(ctx, v4Prefixes, v6Prefixes)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to export")
	}

	// Print summary
	fmt.Printf("\n=== Export Summary ===\n")
	fmt.Printf("IPv4 file:      %s\n", report.IPv4Path)
	fmt.Printf("IPv4 prefixes:  %d\n", report.IPv4Count)
	fmt.Printf("IPv4 SHA256:    %s\n", report.IPv4Digest)
	fmt.Printf("\n")
	fmt.Printf("IPv6 file:      %s\n", report.IPv6Path)
	fmt.Printf("IPv6 prefixes:  %d\n", report.IPv6Count)
	fmt.Printf("IPv6 SHA256:    %s\n", report.IPv6Digest)
	fmt.Printf("\n")
	if report.Unchanged {
		fmt.Printf("Status:         Unchanged (no-op)\n")
	} else {
		fmt.Printf("Status:         Updated\n")
	}

	log.Info().Bool("unchanged", report.Unchanged).Msg("Export complete")
}
