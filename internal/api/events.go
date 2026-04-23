package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/goodvin/d2ip/internal/events"
	"github.com/rs/zerolog/log"
)

// handleEvents serves a Server-Sent Events stream.
func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	if s.eventBus == nil {
		fmt.Fprint(w, "event: error\ndata: {\"message\":\"event bus not initialized\"}\n\n")
		flusher.Flush()
		return
	}

	ch := s.eventBus.Subscribe("pipeline", "config", "routing")
	defer s.eventBus.Unsubscribe(ch)

	fmt.Fprint(w, "event: connected\ndata: {}\n\n")
	flusher.Flush()

	keepAlive := time.NewTicker(30 * time.Second)
	defer keepAlive.Stop()

	for {
		select {
		case ev := <-ch:
			data, err := json.Marshal(ev.Data)
			if err != nil {
				log.Warn().Err(err).Str("type", ev.Type).Msg("api: failed to marshal SSE event")
				continue
			}
			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", ev.Type, data)
			flusher.Flush()

		case <-r.Context().Done():
			return

		case <-keepAlive.C:
			fmt.Fprint(w, ":ping\n\n")
			flusher.Flush()
		}
	}
}
