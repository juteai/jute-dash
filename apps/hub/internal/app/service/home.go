package service

import (
	"time"
)

type State struct {
	GeneratedAt time.Time     `json:"generatedAt"`
	Home        HomeConfig    `json:"home"`
	Rooms       []RoomSummary `json:"rooms"`
	Tiles       []TileState   `json:"tiles"`
}

type RoomSummary struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Summary string `json:"summary"`
	Status  string `json:"status"`
}

type TileState struct {
	ID     string `json:"id"`
	Kind   string `json:"kind"`
	Label  string `json:"label"`
	Value  string `json:"value"`
	Detail string `json:"detail"`
}

// FromConfig projects the current home configuration, rooms, and tiles into a unified home State.
func FromConfig(home HomeConfig, rooms []RoomConfig, tiles []TileConfig, now time.Time) State {
	summaries := make([]RoomSummary, 0, len(rooms))
	for _, room := range rooms {
		summaries = append(summaries, RoomSummary(room))
	}

	tileStates := make([]TileState, 0, len(tiles))
	for _, tile := range tiles {
		tileStates = append(tileStates, TileState(tile))
	}

	return State{
		GeneratedAt: now.UTC(),
		Home:        home,
		Rooms:       summaries,
		Tiles:       tileStates,
	}
}
