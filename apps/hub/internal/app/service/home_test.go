package service

import (
	"testing"
	"time"
)

func TestFromConfigProjectsHomeRoomsAndTiles(t *testing.T) {
	now := time.Date(2026, 6, 20, 12, 0, 0, 0, time.FixedZone("BST", 3600))

	state := FromConfig(
		HomeConfig{Name: "Jute"},
		[]RoomConfig{{ID: "kitchen", Name: "Kitchen", Status: "ok"}},
		[]TileConfig{{ID: "temp", Kind: "sensor", Label: "Temp", Value: "21"}},
		now,
	)

	if state.GeneratedAt.Location() != time.UTC || state.GeneratedAt.Hour() != 11 {
		t.Fatalf("expected UTC generated time, got %s", state.GeneratedAt)
	}
	if state.Home.Name != "Jute" || state.Rooms[0].ID != "kitchen" || state.Tiles[0].Value != "21" {
		t.Fatalf("unexpected state: %+v", state)
	}
}
