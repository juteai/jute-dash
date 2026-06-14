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

func TestDefaultVolumePercent(t *testing.T) {
	if got := defaultVolumePercent(nil); got != 50 {
		t.Fatalf("expected missing volume to default to 50, got %d", got)
	}

	muted := 0
	if got := defaultVolumePercent(&muted); got != 0 {
		t.Fatalf("expected explicit zero volume to be preserved, got %d", got)
	}

	tooHigh := 120
	if got := defaultVolumePercent(&tooHigh); got != 100 {
		t.Fatalf("expected high volume to clamp to 100, got %d", got)
	}
}
