package events

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"jute-dash/apps/hub/internal/pkg/displayactions"
)

type EventSource interface {
	Subscribe(ctx context.Context) <-chan displayactions.Event
}

type Broker struct {
	sources []EventSource
}

func NewBroker(sources ...EventSource) *Broker {
	return &Broker{sources: sources}
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

	rc := http.NewResponseController(w)
	_ = rc.SetWriteDeadline(time.Time{})

	mergedCh := make(chan displayactions.Event, 32)
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	var wg sync.WaitGroup
	for _, src := range b.sources {
		if src == nil {
			continue
		}
		ch := src.Subscribe(ctx)
		wg.Add(1)
		go func(ch <-chan displayactions.Event) {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case ev, ok := <-ch:
					if !ok {
						return
					}
					select {
					case <-ctx.Done():
						return
					case mergedCh <- ev:
					}
				}
			}
		}(ch)
	}

	go func() {
		wg.Wait()
		close(mergedCh)
	}()

	sendDisplaySSE(w, flusher, "hub.connected", map[string]any{
		"connectedAt": time.Now().UTC().Format(time.RFC3339Nano),
	})

	heartbeat := time.NewTicker(10 * time.Second)
	defer heartbeat.Stop()

	for {
		select {
		case <-r.Context().Done():
			cancel()
			wg.Wait()
			return
		case event, ok := <-mergedCh:
			if !ok {
				cancel()
				wg.Wait()
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
