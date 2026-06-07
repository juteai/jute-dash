package events

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"jute-dash/apps/hub/internal/pkg/displayactions"
)

type eventSourceFunc func(context.Context) <-chan displayactions.Event

func (f eventSourceFunc) Subscribe(ctx context.Context) <-chan displayactions.Event {
	return f(ctx)
}

type responseWriterWithoutFlusher struct {
	recorder *httptest.ResponseRecorder
}

func (w responseWriterWithoutFlusher) Header() http.Header {
	return w.recorder.Header()
}

func (w responseWriterWithoutFlusher) Write(bytes []byte) (int, error) {
	return w.recorder.Write(bytes)
}

func (w responseWriterWithoutFlusher) WriteHeader(statusCode int) {
	w.recorder.WriteHeader(statusCode)
}

func TestBrokerRejectsUnsupportedMethods(t *testing.T) {
	broker := NewBroker(eventSourceFunc(func(context.Context) <-chan displayactions.Event {
		return nil
	}))
	req := httptest.NewRequest(http.MethodPost, "/api/v1/events", nil)
	rec := httptest.NewRecorder()

	broker.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status 405, got %d", rec.Code)
	}
	if rec.Header().Get("Allow") != http.MethodGet {
		t.Fatalf("expected Allow GET, got %q", rec.Header().Get("Allow"))
	}
	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["error"] != "method not allowed" {
		t.Fatalf("unexpected error response: %+v", body)
	}
}

func TestBrokerRejectsWritersWithoutStreamingSupport(t *testing.T) {
	broker := NewBroker(eventSourceFunc(func(context.Context) <-chan displayactions.Event {
		return nil
	}))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/events", nil)
	rec := httptest.NewRecorder()

	broker.ServeHTTP(responseWriterWithoutFlusher{recorder: rec}, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", rec.Code)
	}
	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["error"] != "event stream is unavailable" {
		t.Fatalf("unexpected error response: %+v", body)
	}
}

func TestBrokerStreamsConnectedAndSourceEvents(t *testing.T) {
	source := eventSourceFunc(func(context.Context) <-chan displayactions.Event {
		events := make(chan displayactions.Event, 1)
		events <- displayactions.Event{
			Type: displayactions.EventNotification,
			Data: map[string]any{
				"id":       "notification-1",
				"message":  "Dinner is ready",
				"severity": "info",
			},
		}
		close(events)
		return events
	})
	broker := NewBroker(source)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/events", nil)
	rec := httptest.NewRecorder()

	broker.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "text/event-stream" {
		t.Fatalf("expected event-stream content type, got %q", got)
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("expected no-store cache policy, got %q", got)
	}
	body := rec.Body.String()
	for _, expected := range []string{
		"event: hub.connected\n",
		`"connectedAt":`,
		"event: display.notification\n",
		`"id":"notification-1"`,
		`"message":"Dinner is ready"`,
	} {
		if !strings.Contains(body, expected) {
			t.Fatalf("expected stream to contain %q, got:\n%s", expected, body)
		}
	}
}
