package home

import (
	"time"

	"jute-dash/internal/config"
	"jute-dash/internal/weather"
)

type State struct {
	GeneratedAt time.Time         `json:"generatedAt"`
	Home        config.HomeConfig `json:"home"`
	Rooms       []RoomSummary     `json:"rooms"`
	Tiles       []TileState       `json:"tiles"`
	Weather     weather.State     `json:"weather"`
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

func FromConfig(cfg config.Config, now time.Time, weatherState weather.State) State {
	rooms := make([]RoomSummary, 0, len(cfg.Rooms))
	for _, room := range cfg.Rooms {
		rooms = append(rooms, RoomSummary{
			ID:      room.ID,
			Name:    room.Name,
			Summary: room.Summary,
			Status:  room.Status,
		})
	}

	tiles := make([]TileState, 0, len(cfg.Tiles))
	for _, tile := range cfg.Tiles {
		tiles = append(tiles, TileState{
			ID:     tile.ID,
			Kind:   tile.Kind,
			Label:  tile.Label,
			Value:  tile.Value,
			Detail: tile.Detail,
		})
	}

	return State{
		GeneratedAt: now.UTC(),
		Home:        cfg.Home,
		Rooms:       rooms,
		Tiles:       tiles,
		Weather:     weatherState,
	}
}
