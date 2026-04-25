package logging

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
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
		id := FromContext(r.Context())
		if id == "" {
			t.Error("request ID missing from context")
		}
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	header := rr.Header().Get("X-Request-ID")
	if header == "" {
		t.Error("X-Request-ID header not set in response")
	}
}
