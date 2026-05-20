package displayactions

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestNotifySanitizesAndPublishes(t *testing.T) {
	dispatcher := NewDispatcher()
	dispatcher.now = func() time.Time { return time.Date(2026, 5, 20, 10, 0, 0, 0, time.UTC) }
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	events := dispatcher.Subscribe(ctx)

	notification, err := dispatcher.Notify("token=secret-value connected", "warning")
	if err != nil {
		t.Fatalf("Notify returned error: %v", err)
	}
	if notification.Severity != "warning" {
		t.Fatalf("severity = %q", notification.Severity)
	}
	if strings.Contains(notification.Message, "secret-value") || !strings.Contains(notification.Message, "[redacted]") {
		t.Fatalf("notification was not redacted: %q", notification.Message)
	}

	select {
	case event := <-events:
		if event.Type != EventNotification {
			t.Fatalf("event type = %q", event.Type)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for notification event")
	}
}

func TestNotifyRequiresMessage(t *testing.T) {
	dispatcher := NewDispatcher()
	if _, err := dispatcher.Notify("   ", "info"); !errors.Is(err, ErrEmptyMessage) {
		t.Fatalf("expected ErrEmptyMessage, got %v", err)
	}
}

func TestFocusWidgetPublishes(t *testing.T) {
	dispatcher := NewDispatcher()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	events := dispatcher.Subscribe(ctx)

	focus, err := dispatcher.FocusWidget("weather", "show forecast")
	if err != nil {
		t.Fatalf("FocusWidget returned error: %v", err)
	}
	if focus.WidgetInstanceID != "weather" {
		t.Fatalf("widget id = %q", focus.WidgetInstanceID)
	}

	select {
	case event := <-events:
		if event.Type != EventFocusWidget {
			t.Fatalf("event type = %q", event.Type)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for focus event")
	}
}

func TestFocusWidgetRequiresWidgetID(t *testing.T) {
	dispatcher := NewDispatcher()
	if _, err := dispatcher.FocusWidget(" ", ""); !errors.Is(err, ErrEmptyWidget) {
		t.Fatalf("expected ErrEmptyWidget, got %v", err)
	}
}
