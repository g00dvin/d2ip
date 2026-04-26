package logging

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSetup(t *testing.T) {
	if err := Setup("debug", true); err != nil {
		t.Fatalf("Setup error: %v", err)
	}
}

func TestRequestIDMiddleware(t *testing.T) {
	router := chi.NewRouter()
	router.Use(RequestIDMiddleware)
	router.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		logger := FromContext(r.Context())
		if logger == nil {
			t.Error("logger missing from context")
		}
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

func TestSetup_InvalidLevel(t *testing.T) {
	// Should not error, falls back to info level
	err := Setup("invalid_level", true)
	require.NoError(t, err)
}

func TestFromContext_WithLogger(t *testing.T) {
	logger := zerolog.New(io.Discard)
	ctx := logger.WithContext(context.Background())
	got := FromContext(ctx)
	assert.Equal(t, logger, *got)
}

func TestWithRequestID(t *testing.T) {
	ctx := context.Background()
	ctx = WithRequestID(ctx, "req-123")
	logger := FromContext(ctx)
	assert.NotNil(t, logger)
}
