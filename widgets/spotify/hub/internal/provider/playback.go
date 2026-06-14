package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

func (c Client) FetchPlayback(ctx context.Context) (Playback, error) {
	if c.settings.ClientID == "mock-spotify" || c.settings.ClientID == "test" {
		return Playback{
			TrackTitle:  "Mock Track",
			ArtistName:  "Mock Artist",
			AlbumArtURL: "https://example.test/mock-album.jpg",
			URI:         "spotify:track:mock",
			IsPlaying:   true,
			Volume:      75,
			ProgressMS:  48_000,
			DurationMS:  192_000,
			Shuffle:     true,
			RepeatState: "context",
			TopAlbums: []Album{
				{
					ID:          "mock-album-1",
					Name:        "Mock Album",
					ArtistName:  "Mock Artist",
					URI:         "spotify:album:mock-album-1",
					AlbumArtURL: "https://example.test/mock-album.jpg",
				},
				{
					ID:          "mock-album-2",
					Name:        "Late Night Mock",
					ArtistName:  "Mock Artist",
					URI:         "spotify:album:mock-album-2",
					AlbumArtURL: "https://example.test/mock-album-2.jpg",
				},
			},
		}, nil
	}

	resp, err := c.doRequest(ctx, http.MethodGet, "https://api.spotify.com/v1/me/player", nil)
	if err != nil {
		return Playback{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNoContent {
		return Playback{
			TrackTitle: "Not Playing",
			ArtistName: "Unknown",
			IsPlaying:  false,
			Volume:     50,
		}, nil
	}

	if resp.StatusCode != http.StatusOK {
		return Playback{}, fmt.Errorf("spotify API returned status %d", resp.StatusCode)
	}

	var playerState struct {
		IsPlaying  bool   `json:"is_playing"`
		ProgressMS int    `json:"progress_ms"`
		Shuffle    bool   `json:"shuffle_state"`
		Repeat     string `json:"repeat_state"`
		Device     struct {
			VolumePercent int `json:"volume_percent"`
		} `json:"device"`
		Item *struct {
			Name       string `json:"name"`
			URI        string `json:"uri"`
			DurationMS int    `json:"duration_ms"`
			Artists    []struct {
				Name string `json:"name"`
			} `json:"artists"`
			Album spotifyAlbum `json:"album"`
		} `json:"item"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&playerState); err != nil {
		return Playback{}, err
	}
	topAlbums, _ := c.FetchSuggestedAlbums(ctx, 8)

	track := "Not Playing"
	artist := "Unknown"
	albumArtURL := ""
	durationMS := 0
	uri := ""
	if playerState.Item != nil {
		track = playerState.Item.Name
		uri = playerState.Item.URI
		durationMS = playerState.Item.DurationMS
		var artistNames []string
		for _, a := range playerState.Item.Artists {
			artistNames = append(artistNames, a.Name)
		}
		if len(artistNames) > 0 {
			artist = strings.Join(artistNames, ", ")
		}
		albumArtURL = bestAlbumImage(playerState.Item.Album.Images)
	}

	return Playback{
		TrackTitle:  track,
		ArtistName:  artist,
		AlbumArtURL: albumArtURL,
		URI:         uri,
		IsPlaying:   playerState.IsPlaying,
		Volume:      playerState.Device.VolumePercent,
		ProgressMS:  playerState.ProgressMS,
		DurationMS:  durationMS,
		Shuffle:     playerState.Shuffle,
		RepeatState: strings.TrimSpace(playerState.Repeat),
		TopAlbums:   topAlbums,
	}, nil
}
