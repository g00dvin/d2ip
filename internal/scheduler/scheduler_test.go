package scheduler

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/goodvin/d2ip/internal/orchestrator"
)

// mockOrchestrator counts Run() invocations.
type mockOrchestrator struct {
	runCount atomic.Int32
}

func (m *mockOrchestrator) Run(ctx context.Context, req orchestrator.PipelineRequest) (orchestrator.PipelineReport, error) {
	m.runCount.Add(1)
	return orchestrator.PipelineReport{}, nil
}

func TestScheduler_New(t *testing.T) {
	t.Parallel()

	mock := &mockOrchestrator{}

	cases := []struct {
		name      string
		orch      Orchestrator
		cronExpr  string
		wantError bool
	}{
		{
			name:      "nil orchestrator",
			orch:      nil,
			cronExpr:  "@every 1h",
			wantError: true,
		},
		{
			name:      "empty cron expression",
			orch:      mock,
			cronExpr:  "",
			wantError: true,
		},
		{
			name:      "invalid cron expression",
			orch:      mock,
			cronExpr:  "not-a-cron",
			wantError: true,
		},
		{
			name:      "valid @every expression",
			orch:      mock,
			cronExpr:  "@every 1h",
			wantError: false,
		},
		{
			name:      "valid standard cron expression",
			orch:      mock,
			cronExpr:  "0 0 * * *",
			wantError: false,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := New(tc.orch, tc.cronExpr)
			if tc.wantError && err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !tc.wantError && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestScheduler_StartStop(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping timing-dependent test in short mode")
	}

	mock := &mockOrchestrator{}
	s, err := New(mock, "@every 1s")
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := s.Start(ctx); err != nil {
		t.Fatalf("Start() error: %v", err)
	}

	// Allow at least one tick to fire.
	time.Sleep(1500 * time.Millisecond)

	s.Stop()

	count := mock.runCount.Load()
	if count < 1 {
		t.Fatalf("expected at least 1 run, got %d", count)
	}
}

func TestScheduler_Start_AlreadyRunning(t *testing.T) {
	mock := &mockOrchestrator{}
	s, err := New(mock, "@every 1h")
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := s.Start(ctx); err != nil {
		t.Fatalf("first Start() error: %v", err)
	}
	defer s.Stop()

	if err := s.Start(ctx); err == nil {
		t.Fatal("second Start() expected error, got nil")
	}
}

func TestScheduler_Stop_NotRunning(t *testing.T) {
	mock := &mockOrchestrator{}
	s, err := New(mock, "@every 1h")
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	// Should not panic or block when scheduler is not running.
	s.Stop()
}
