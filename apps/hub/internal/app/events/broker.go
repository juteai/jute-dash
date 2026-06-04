package events

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"jute-dash/apps/hub/internal/pkg/displayactions"
)

type EventSource interface {
	Subscribe(ctx context.Context) <-chan displayactions.Event
}

type Broker struct {
	source EventSource
}

func NewBroker(source EventSource) *Broker {
	return &Broker{source: source}
}

func (b *Broker) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "method not allowed"})
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "event stream is unavailable"})
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	sendDisplaySSE(w, flusher, "hub.connected", map[string]any{
		"connectedAt": time.Now().UTC().Format(time.RFC3339Nano),
	})

	events := b.source.Subscribe(r.Context())
	heartbeat := time.NewTicker(25 * time.Second)
	defer heartbeat.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case event, ok := <-events:
			if !ok {
				return
			}
			sendDisplayEventSSE(w, flusher, event)
		case <-heartbeat.C:
			_, _ = w.Write([]byte(": heartbeat\n\n"))
			flusher.Flush()
		}
	}
}

func sendDisplayEventSSE(w http.ResponseWriter, flusher http.Flusher, event displayactions.Event) {
	sendDisplaySSE(w, flusher, event.Type, event.Data)
}

func sendDisplaySSE(w http.ResponseWriter, flusher http.Flusher, event string, data any) {
	bytes, err := json.Marshal(data)
	if err != nil {
		return
	}
	_, _ = fmt.Fprintf(w, "event: %s\n", event)
	_, _ = fmt.Fprintf(w, "data: %s\n\n", bytes)
	flusher.Flush()
}
