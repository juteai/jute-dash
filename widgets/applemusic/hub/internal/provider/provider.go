package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

const apiBase = "https://api.music.apple.com/v1"

type Settings struct {
	DeveloperToken string
	UserToken      string
}

type Client struct {
	settings Settings
}

type Playback struct {
	TrackTitle  string `json:"track_title"`
	ArtistName  string `json:"artist_name"`
	AlbumArtURL string `json:"album_art_url,omitempty"`
	URI         string `json:"uri,omitempty"`
	IsPlaying   bool   `json:"is_playing"`
	Volume      int    `json:"volume"`
	ProgressMS  int    `json:"progress_ms"`
	DurationMS  int    `json:"duration_ms"`
	Shuffle     bool   `json:"shuffle"`
	RepeatState string `json:"repeat_state"`
	TopAlbums   []PlayableItem
}

type PlayableItem struct {
	ID          string `json:"id"`
	Type        string `json:"type"`
	Name        string `json:"name"`
	Subtitle    string `json:"subtitle,omitempty"`
	URI         string `json:"uri"`
	AlbumArtURL string `json:"album_art_url,omitempty"`
}

type SearchResult = PlayableItem

func NewClient(settings Settings) Client {
	return Client{settings: settings}
}

func (c Client) FetchPlayback(context.Context) (Playback, error) {
	if c.mock() {
		return Playback{
			TrackTitle:  "Mock Track",
			ArtistName:  "Mock Artist",
			AlbumArtURL: "https://example.test/apple-album.jpg",
			URI:         "apple-music:songs:mock-song",
			IsPlaying:   true,
			Volume:      75,
			ProgressMS:  48_000,
			DurationMS:  192_000,
			Shuffle:     false,
			RepeatState: "off",
			TopAlbums: []PlayableItem{
				{
					ID:          "mock-album-1",
					Type:        "album",
					Name:        "Mock Album",
					Subtitle:    "Mock Artist",
					URI:         "apple-music:albums:mock-album-1",
					AlbumArtURL: "https://example.test/apple-album.jpg",
				},
			},
		}, nil
	}
	return Playback{
		TrackTitle:  "Not Playing",
		ArtistName:  "Unknown",
		IsPlaying:   false,
		Volume:      50,
		RepeatState: "off",
	}, nil
}

func (c Client) FetchSuggestedAlbums(ctx context.Context, limit int) ([]PlayableItem, error) {
	if c.mock() {
		return mockSearchResults("mock", "album", limit), nil
	}
	if limit <= 0 {
		limit = 8
	}
	results, err := c.fetchRecentlyPlayed(ctx, limit)
	if err == nil && len(results) > 0 {
		return results, nil
	}
	library, libraryErr := c.fetchLibraryAlbums(ctx, limit)
	if libraryErr == nil && len(library) > 0 {
		return library, nil
	}
	if err != nil {
		return nil, err
	}
	return library, libraryErr
}

func (c Client) Search(ctx context.Context, query string, itemType string, limit int) ([]SearchResult, error) {
	if c.mock() {
		return mockSearchResults(query, itemType, limit), nil
	}
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, nil
	}
	if limit <= 0 {
		limit = 5
	}
	storefront, err := c.Storefront(ctx)
	if err != nil {
		return nil, err
	}
	appleType := appleSearchType(itemType)
	searchURL := fmt.Sprintf(
		"%s/catalog/%s/search?term=%s&types=%s&limit=%d",
		apiBase,
		url.PathEscape(storefront),
		url.QueryEscape(query),
		url.QueryEscape(appleType),
		min(max(limit, 1), 10),
	)
	resp, err := c.doGet(ctx, searchURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("apple music API returned status %d", resp.StatusCode)
	}
	var searchResp searchResponse
	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return nil, err
	}
	return playableItemsFromResources(searchResp.Results.resources(appleType), itemType, limit), nil
}

func (c Client) ApplyAction(_ context.Context, actionID string, _ map[string]any) error {
	switch actionID {
	case "play", "pause", "next", "previous", "restart_track", "seek",
		"play_album", "play_track", "play_playlist", "set_volume", "set_shuffle",
		"set_repeat", "transfer_playback":
		return nil
	default:
		return fmt.Errorf("unknown action: %s", actionID)
	}
}

func (c Client) Storefront(ctx context.Context) (string, error) {
	resp, err := c.doGet(ctx, apiBase+"/me/storefront")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("apple music API returned status %d", resp.StatusCode)
	}
	var payload struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", err
	}
	if len(payload.Data) == 0 || strings.TrimSpace(payload.Data[0].ID) == "" {
		return "", errors.New("apple music storefront is unavailable")
	}
	return strings.TrimSpace(payload.Data[0].ID), nil
}

func (c Client) fetchRecentlyPlayed(ctx context.Context, limit int) ([]PlayableItem, error) {
	resp, err := c.doGet(
		ctx,
		fmt.Sprintf("%s/me/recent/played/tracks?limit=%d", apiBase, min(max(limit*3, limit), 25)),
	)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("apple music API returned status %d", resp.StatusCode)
	}
	var payload resourceListResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}
	return albumItemsFromSongs(payload.Data, limit), nil
}

func (c Client) fetchLibraryAlbums(ctx context.Context, limit int) ([]PlayableItem, error) {
	resp, err := c.doGet(
		ctx,
		fmt.Sprintf("%s/me/library/albums?limit=%d", apiBase, min(max(limit, 1), 25)),
	)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("apple music API returned status %d", resp.StatusCode)
	}
	var payload resourceListResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}
	return playableItemsFromResources(payload.Data, "album", limit), nil
}

func (c Client) doGet(ctx context.Context, urlStr string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, urlStr, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.settings.DeveloperToken))
	if c.settings.UserToken != "" {
		req.Header.Set("Music-User-Token", c.settings.UserToken)
	}
	return http.DefaultClient.Do(req)
}

func (c Client) mock() bool {
	return c.settings.DeveloperToken == "mock-applemusic" || c.settings.DeveloperToken == "test"
}

type searchResponse struct {
	Results searchResults `json:"results"`
}

type searchResults struct {
	Songs     resourceListResponse `json:"songs"`
	Albums    resourceListResponse `json:"albums"`
	Playlists resourceListResponse `json:"playlists"`
}

func (r searchResults) resources(appleType string) []resource {
	switch appleType {
	case "albums":
		return r.Albums.Data
	case "playlists":
		return r.Playlists.Data
	default:
		return r.Songs.Data
	}
}

type resourceListResponse struct {
	Data []resource `json:"data"`
}

type resource struct {
	ID            string             `json:"id"`
	Type          string             `json:"type"`
	Href          string             `json:"href"`
	Attributes    resourceAttributes `json:"attributes"`
	Relationships struct {
		Albums resourceListResponse `json:"albums"`
	} `json:"relationships"`
}

type resourceAttributes struct {
	Name        string  `json:"name"`
	ArtistName  string  `json:"artistName"`
	CuratorName string  `json:"curatorName"`
	Description *string `json:"description"`
	Artwork     *struct {
		URL string `json:"url"`
	} `json:"artwork"`
	PlayParams *struct {
		ID   string `json:"id"`
		Kind string `json:"kind"`
	} `json:"playParams"`
}

func playableItemsFromResources(resources []resource, itemType string, limit int) []PlayableItem {
	if limit <= 0 {
		limit = 8
	}
	items := make([]PlayableItem, 0, min(len(resources), limit))
	seen := map[string]struct{}{}
	for _, res := range resources {
		item := playableItemFromResource(res, itemType)
		if item.URI == "" || item.Name == "" {
			continue
		}
		if _, exists := seen[item.URI]; exists {
			continue
		}
		seen[item.URI] = struct{}{}
		items = append(items, item)
		if len(items) >= limit {
			break
		}
	}
	return items
}

func albumItemsFromSongs(songs []resource, limit int) []PlayableItem {
	albums := make([]resource, 0, len(songs))
	for _, song := range songs {
		albums = append(albums, song.Relationships.Albums.Data...)
	}
	return playableItemsFromResources(albums, "album", limit)
}

func playableItemFromResource(res resource, fallbackType string) PlayableItem {
	itemType := normalizeResourceType(res.Type, fallbackType)
	subtitle := strings.TrimSpace(res.Attributes.ArtistName)
	if subtitle == "" {
		subtitle = strings.TrimSpace(res.Attributes.CuratorName)
	}
	if subtitle == "" && res.Attributes.Description != nil {
		subtitle = strings.TrimSpace(*res.Attributes.Description)
	}
	return PlayableItem{
		ID:          strings.TrimSpace(res.ID),
		Type:        itemType,
		Name:        strings.TrimSpace(res.Attributes.Name),
		Subtitle:    subtitle,
		URI:         appleURI(itemType, res),
		AlbumArtURL: artworkURL(res.Attributes.Artwork),
	}
}

func normalizeResourceType(resourceType string, fallback string) string {
	switch resourceType {
	case "albums", "library-albums":
		return "album"
	case "songs", "library-songs":
		return "track"
	case "playlists", "library-playlists":
		return "playlist"
	default:
		if fallback == "album" || fallback == "playlist" {
			return fallback
		}
		return "track"
	}
}

func appleURI(itemType string, res resource) string {
	if res.Attributes.PlayParams != nil {
		kind := strings.TrimSpace(res.Attributes.PlayParams.Kind)
		id := strings.TrimSpace(res.Attributes.PlayParams.ID)
		if kind != "" && id != "" {
			return "apple-music:" + kind + ":" + id
		}
	}
	if res.ID == "" {
		return ""
	}
	switch itemType {
	case "album":
		return "apple-music:albums:" + res.ID
	case "playlist":
		return "apple-music:playlists:" + res.ID
	default:
		return "apple-music:songs:" + res.ID
	}
}

func artworkURL(artwork *struct {
	URL string `json:"url"`
}) string {
	if artwork == nil {
		return ""
	}
	url := strings.TrimSpace(artwork.URL)
	url = strings.ReplaceAll(url, "{w}", "300")
	url = strings.ReplaceAll(url, "{h}", "300")
	return url
}

func appleSearchType(itemType string) string {
	switch itemType {
	case "album":
		return "albums"
	case "playlist":
		return "playlists"
	default:
		return "songs"
	}
}

func mockSearchResults(query string, itemType string, limit int) []SearchResult {
	if strings.TrimSpace(query) == "" {
		return nil
	}
	if limit <= 0 {
		limit = 5
	}
	results := []SearchResult{
		{
			ID:          itemType + ":mock-1",
			Type:        itemType,
			Name:        "Mock Song",
			Subtitle:    "Mock Artist",
			URI:         "apple-music:" + appleSearchType(itemType) + ":mock-1",
			AlbumArtURL: "https://example.test/apple-album.jpg",
		},
		{
			ID:          itemType + ":mock-2",
			Type:        itemType,
			Name:        "Late Night Mock",
			Subtitle:    "Mock Artist",
			URI:         "apple-music:" + appleSearchType(itemType) + ":mock-2",
			AlbumArtURL: "https://example.test/apple-album-2.jpg",
		},
	}
	if limit < len(results) {
		return results[:limit]
	}
	return results
}
