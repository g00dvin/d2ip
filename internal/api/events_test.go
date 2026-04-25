package api

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/goodvin/d2ip/internal/config"
	"github.com/goodvin/d2ip/internal/events"
)

func TestEventStream(t *testing.T) {
	bus := events.NewBus()
	cfg := config.Defaults()
	watcher := config.NewWatcher(cfg, 1, bus)
	server := New(nil, nil, watcher, nil, nil, nil, nil, bus, nil)

	req := httptest.NewRequest("GET", "/events", nil)
	rr := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		server.handleEvents(rr, req)
		close(done)
	}()

	time.Sleep(50 * time.Millisecond)
	bus.Publish("config", events.Event{Type: "config_changed", Data: "hello"})
	time.Sleep(100 * time.Millisecond)

	body := rr.Body.String()
	if !strings.Contains(body, "event: config_changed") {
		t.Errorf("expected 'event: config_changed' in body, got:\n%s", body)
	}
	if !strings.Contains(body, "data: hello") {
		t.Errorf("expected 'data: hello' in body, got:\n%s", body)
	}
}
