package logging

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// Setup initializes the global logger with the specified level and format.
// Level can be: "debug", "info", "warn", "error", "fatal", "panic"
// If json is false, output will be in console format (human-readable).
func Setup(level string, json bool) error {
	// Parse log level
	logLevel, err := zerolog.ParseLevel(level)
	if err != nil {
		logLevel = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(logLevel)

	// Configure output format
	var output io.Writer = os.Stdout
	if !json {
		output = zerolog.ConsoleWriter{
			Out:        os.Stdout,
			TimeFormat: time.RFC3339,
		}
	}

	// Create logger with timestamp
	logger := zerolog.New(output).With().Timestamp().Logger()
	log.Logger = logger

	return nil
}

// RequestIDMiddleware is a chi middleware that adds request_id to the logger context.
// It uses chi's middleware.RequestID to generate or retrieve the request ID.
func RequestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID := middleware.GetReqID(r.Context())
		if reqID == "" {
			reqID = fmt.Sprintf("%d", middleware.NextRequestID())
		}

		// Add request_id to context logger
		logger := log.With().Str("request_id", reqID).Logger()
		ctx := logger.WithContext(r.Context())
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// FromContext retrieves the logger from context, or returns the global logger if not found.
func FromContext(ctx context.Context) *zerolog.Logger {
	logger := log.Ctx(ctx)
	if logger.GetLevel() == zerolog.Disabled {
		return &log.Logger
	}
	return logger
}

// WithRequestID adds a request_id field to the logger in the context.
func WithRequestID(ctx context.Context, requestID string) context.Context {
	logger := FromContext(ctx).With().Str("request_id", requestID).Logger()
	return logger.WithContext(ctx)
}
