package applemusic

import (
	"context"
	"testing"
)

func TestAppleMusicWidgetSettings(t *testing.T) {
	w := NewWidget()
	if w.Kind() != "apple-music" {
		t.Errorf("expected kind 'apple-music', got %q", w.Kind())
	}

	raw := map[string]any{
		"developer_token": "my_dev_token",
	}
	data, err := w.FetchData(context.Background(), raw)
	if err != nil {
		t.Fatalf("FetchData failed: %v", err)
	}
	m, ok := data.(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any, got %T", data)
	}
	if m["is_configured"] != true {
		t.Errorf("expected is_configured to be true")
	}
}
