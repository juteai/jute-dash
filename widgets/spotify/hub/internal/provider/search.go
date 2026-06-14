package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

func (c Client) Search(ctx context.Context, query string, itemType string, limit int) ([]SearchResult, error) {
	if c.settings.ClientID == "mock-spotify" || c.settings.ClientID == "test" {
		return mockSearchResults(query, itemType, limit), nil
	}
	if limit <= 0 {
		limit = 5
	}
	searchURL := fmt.Sprintf(
		"https://api.spotify.com/v1/search?type=%s&limit=%d&q=%s",
		url.QueryEscape(itemType),
		min(max(limit, 1), 10),
		url.QueryEscape(query),
	)
	resp, err := c.doRequest(ctx, http.MethodGet, searchURL, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("spotify API returned status %d", resp.StatusCode)
	}

	var searchResp struct {
		Tracks struct {
			Items []searchItem `json:"items"`
		} `json:"tracks"`
		Playlists struct {
			Items []searchItem `json:"items"`
		} `json:"playlists"`
		Albums struct {
			Items []searchItem `json:"items"`
		} `json:"albums"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return nil, err
	}

	var items []searchItem
	switch itemType {
	case "playlist":
		items = searchResp.Playlists.Items
	case "album":
		items = searchResp.Albums.Items
	default:
		items = searchResp.Tracks.Items
	}

	results := make([]SearchResult, 0, len(items))
	for i, item := range items {
		result := searchResultFromItem(item, itemType, i)
		if result.URI != "" {
			results = append(results, result)
		}
	}
	return results, nil
}

func searchResultFromItem(item searchItem, itemType string, index int) SearchResult {
	name := strings.TrimSpace(item.Name)
	var subtitle string
	var albumArtURL string
	switch itemType {
	case "playlist":
		subtitle = strings.TrimSpace(item.Description)
		albumArtURL = bestSearchImage(item.Images)
	case "album":
		subtitle = artistList(item.Artists)
		albumArtURL = bestSearchImage(item.Images)
	default:
		subtitle = artistList(item.Artists)
		albumArtURL = bestAlbumImage(item.Album.Images)
	}
	return SearchResult{
		ID:          fmt.Sprintf("%s:%d:%s", itemType, index, strings.TrimSpace(item.URI)),
		Type:        itemType,
		Name:        name,
		Subtitle:    subtitle,
		URI:         strings.TrimSpace(item.URI),
		AlbumArtURL: albumArtURL,
	}
}

func bestSearchImage(images []struct {
	URL string `json:"url"`
}) string {
	for _, image := range images {
		if url := strings.TrimSpace(image.URL); url != "" {
			return url
		}
	}
	return ""
}

func withDeviceID(urlStr string, arguments map[string]any) string {
	deviceID, ok := stringArgument(arguments, "device_id")
	if !ok {
		return urlStr
	}
	separator := "?"
	if strings.Contains(urlStr, "?") {
		separator = "&"
	}
	return urlStr + separator + "device_id=" + url.QueryEscape(deviceID)
}

func artistList(artists []struct {
	Name string `json:"name"`
}) string {
	if len(artists) == 0 {
		return ""
	}
	names := make([]string, 0, len(artists))
	for _, artist := range artists {
		if name := strings.TrimSpace(artist.Name); name != "" {
			names = append(names, name)
		}
	}
	return strings.Join(names, ", ")
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
			ID:          fmt.Sprintf("%s:mock-1", itemType),
			Type:        itemType,
			Name:        "Mock Track",
			Subtitle:    "Mock Artist",
			URI:         fmt.Sprintf("spotify:%s:mock-1", itemType),
			AlbumArtURL: "https://example.test/mock-album.jpg",
		},
		{
			ID:          fmt.Sprintf("%s:mock-2", itemType),
			Type:        itemType,
			Name:        "Late Night Mock",
			Subtitle:    "Mock Artist",
			URI:         fmt.Sprintf("spotify:%s:mock-2", itemType),
			AlbumArtURL: "https://example.test/mock-album-2.jpg",
		},
	}
	if limit < len(results) {
		return results[:limit]
	}
	return results
}
