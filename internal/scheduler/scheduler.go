// Package scheduler runs the d2ip pipeline on cron-like intervals.
// It wraps github.com/robfig/cron/v3 and delegates each tick to the orchestrator.
package scheduler

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/goodvin/d2ip/internal/orchestrator"
	"github.com/robfig/cron/v3"
	"github.com/rs/zerolog/log"
)

// Orchestrator is the minimal interface the scheduler needs from orchestrator.
type Orchestrator interface {
	Run(ctx context.Context, req orchestrator.PipelineRequest) (orchestrator.PipelineReport, error)
}

// Scheduler runs pipeline executions on a cron schedule.
type Scheduler struct {
	orch     Orchestrator
	cronExpr string
	cron     *cron.Cron

	mu      sync.Mutex
	running bool
	entryID cron.EntryID
	cancel  context.CancelFunc
}

// New creates a Scheduler with the given orchestrator and cron expression.
// The cron expression is validated but the scheduler is not started until Start() is called.
// Cron format: "MIN HOUR DOM MONTH DOW" or @every syntax (e.g., "@every 1h").
func New(orch Orchestrator, cronExpr string) (*Scheduler, error) {
	if orch == nil {
		return nil, errors.New("scheduler: orchestrator cannot be nil")
	}
	if cronExpr == "" {
		return nil, errors.New("scheduler: cron expression cannot be empty")
	}

	// Validate cron expression by parsing it.
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)
	if _, err := parser.Parse(cronExpr); err != nil {
		return nil, fmt.Errorf("scheduler: invalid cron expression %q: %w", cronExpr, err)
	}

	return &Scheduler{
		orch:     orch,
		cronExpr: cronExpr,
	}, nil
}

// Start begins the background cron loop. Each tick triggers orchestrator.Run().
// Returns an error if the scheduler is already running.
func (s *Scheduler) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return errors.New("scheduler: already running")
	}

	// Create a child context so we can cancel on Stop().
	ctx, cancel := context.WithCancel(ctx)
	s.cancel = cancel

	// Create cron instance with logger.
	s.cron = cron.New(
		cron.WithParser(cron.NewParser(cron.Minute|cron.Hour|cron.Dom|cron.Month|cron.Dow|cron.Descriptor)),
		cron.WithLogger(cronLogger{}),
	)

	// Schedule the pipeline execution.
	entryID, err := s.cron.AddFunc(s.cronExpr, func() {
		s.runPipeline(ctx)
	})
	if err != nil {
		cancel()
		return fmt.Errorf("scheduler: failed to schedule cron job: %w", err)
	}

	s.entryID = entryID
	s.running = true
	s.cron.Start()

	log.Info().Str("schedule", s.cronExpr).Msg("scheduler: started")
	return nil
}

// Stop gracefully shuts down the scheduler.
// It waits for any in-flight pipeline run to complete.
func (s *Scheduler) Stop() {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return
	}

	log.Info().Msg("scheduler: stopping")

	// Cancel context and stop cron.
	if s.cancel != nil {
		s.cancel()
	}
	if s.cron != nil {
		stopCtx := s.cron.Stop()
		s.mu.Unlock()
		// Wait for running jobs to complete.
		<-stopCtx.Done()
	} else {
		s.mu.Unlock()
	}

	s.mu.Lock()
	s.running = false
	s.mu.Unlock()

	log.Info().Msg("scheduler: stopped")
}

// runPipeline executes the orchestrator pipeline for a single cron tick.
func (s *Scheduler) runPipeline(ctx context.Context) {
	log.Info().Str("schedule", s.cronExpr).Msg("scheduler: tick - starting pipeline run")

	req := orchestrator.PipelineRequest{
		DryRun:       false,
		ForceResolve: false,
		SkipRouting:  false,
	}

	report, err := s.orch.Run(ctx, req)
	if err != nil {
		if errors.Is(err, orchestrator.ErrBusy) {
			log.Warn().
				Int64("blocked_by_run_id", report.RunID).
				Msg("scheduler: tick skipped - pipeline already running")
		} else {
			log.Error().
				Err(err).
				Int64("run_id", report.RunID).
				Msg("scheduler: tick failed")
		}
		return
	}

	log.Info().
		Int64("run_id", report.RunID).
		Dur("duration", report.Duration).
		Int("domains", report.Domains).
		Int("resolved", report.Resolved).
		Int("failed", report.Failed).
		Msg("scheduler: tick completed")
}

// cronLogger adapts zerolog to cron.Logger interface.
type cronLogger struct{}

func (cronLogger) Info(msg string, keysAndValues ...interface{}) {
	log.Debug().Fields(keysAndValues).Msg(msg)
}

func (cronLogger) Error(err error, msg string, keysAndValues ...interface{}) {
	log.Error().Err(err).Fields(keysAndValues).Msg(msg)
}
