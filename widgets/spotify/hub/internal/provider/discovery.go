package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

func (c Client) FetchTopAlbums(ctx context.Context, limit int) ([]Album, error) {
	if limit <= 0 {
		limit = 8
	}
	resp, err := c.doRequest(
		ctx,
		http.MethodGet,
		fmt.Sprintf(
			"https://api.spotify.com/v1/me/top/tracks?time_range=medium_term&limit=%d",
			min(max(limit*3, limit), 50),
		),
		nil,
	)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("spotify API returned status %d", resp.StatusCode)
	}

	var topTracks struct {
		Items []struct {
			Album spotifyAlbum `json:"album"`
		} `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&topTracks); err != nil {
		return nil, err
	}

	builder := newAlbumBuilder(limit)
	for _, track := range topTracks.Items {
		if builder.add(track.Album) {
			break
		}
	}
	return builder.albums, nil
}

func (c Client) FetchSuggestedAlbums(ctx context.Context, limit int) ([]Album, error) {
	albums, err := c.FetchTopAlbums(ctx, limit)
	if err == nil && len(albums) > 0 {
		return albums, nil
	}
	albums, savedErr := c.FetchSavedAlbums(ctx, limit)
	if savedErr == nil && len(albums) > 0 {
		return albums, nil
	}
	albums, trackErr := c.FetchSavedTrackAlbums(ctx, limit)
	if trackErr == nil && len(albums) > 0 {
		return albums, nil
	}
	if err != nil {
		return nil, err
	}
	if savedErr != nil {
		return nil, savedErr
	}
	return albums, trackErr
}

func (c Client) FetchSavedAlbums(ctx context.Context, limit int) ([]Album, error) {
	if limit <= 0 {
		limit = 8
	}
	resp, err := c.doRequest(
		ctx,
		http.MethodGet,
		fmt.Sprintf("https://api.spotify.com/v1/me/albums?limit=%d", min(max(limit, 1), 50)),
		nil,
	)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("spotify API returned status %d", resp.StatusCode)
	}

	var savedAlbums struct {
		Items []struct {
			Album spotifyAlbum `json:"album"`
		} `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&savedAlbums); err != nil {
		return nil, err
	}

	builder := newAlbumBuilder(limit)
	for _, item := range savedAlbums.Items {
		if builder.add(item.Album) {
			break
		}
	}
	return builder.albums, nil
}

func (c Client) FetchSavedTrackAlbums(ctx context.Context, limit int) ([]Album, error) {
	if limit <= 0 {
		limit = 8
	}
	resp, err := c.doRequest(
		ctx,
		http.MethodGet,
		fmt.Sprintf(
			"https://api.spotify.com/v1/me/tracks?limit=%d",
			min(max(limit*3, limit), 50),
		),
		nil,
	)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("spotify API returned status %d", resp.StatusCode)
	}

	var savedTracks struct {
		Items []struct {
			Track struct {
				Album spotifyAlbum `json:"album"`
			} `json:"track"`
		} `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&savedTracks); err != nil {
		return nil, err
	}

	builder := newAlbumBuilder(limit)
	for _, item := range savedTracks.Items {
		if builder.add(item.Track.Album) {
			break
		}
	}
	return builder.albums, nil
}

func bestAlbumImage(images []struct {
	URL    string `json:"url"`
	Height int    `json:"height"`
	Width  int    `json:"width"`
}) string {
	if len(images) == 0 {
		return ""
	}
	for _, image := range images {
		if strings.TrimSpace(image.URL) != "" && image.Width <= 320 && image.Height <= 320 {
			return image.URL
		}
	}
	return strings.TrimSpace(images[len(images)-1].URL)
}

type albumBuilder struct {
	albums []Album
	seen   map[string]struct{}
	limit  int
}

func newAlbumBuilder(limit int) albumBuilder {
	if limit <= 0 {
		limit = 8
	}
	return albumBuilder{
		albums: make([]Album, 0, limit),
		seen:   map[string]struct{}{},
		limit:  limit,
	}
}

func (b *albumBuilder) add(album spotifyAlbum) bool {
	albumID := strings.TrimSpace(album.ID)
	if albumID == "" {
		return false
	}
	if _, exists := b.seen[albumID]; exists {
		return false
	}
	b.seen[albumID] = struct{}{}
	artist := ""
	if len(album.Artists) > 0 {
		artist = strings.TrimSpace(album.Artists[0].Name)
	}
	b.albums = append(b.albums, Album{
		ID:          albumID,
		Name:        strings.TrimSpace(album.Name),
		ArtistName:  artist,
		URI:         strings.TrimSpace(album.URI),
		AlbumArtURL: bestAlbumImage(album.Images),
	})
	return len(b.albums) >= b.limit
}
