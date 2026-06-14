package provider

import "testing"

func TestWithDeviceIDAddsDeviceQueryParameter(t *testing.T) {
	got := withDeviceID(
		"https://api.spotify.com/v1/me/player/shuffle?state=true",
		map[string]any{"device_id": "jute player/1"},
	)

	want := "https://api.spotify.com/v1/me/player/shuffle?state=true&device_id=jute+player%2F1"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestWithDeviceIDLeavesURLWithoutDeviceID(t *testing.T) {
	got := withDeviceID(
		"https://api.spotify.com/v1/me/player/repeat?state=context",
		map[string]any{},
	)

	want := "https://api.spotify.com/v1/me/player/repeat?state=context"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}
