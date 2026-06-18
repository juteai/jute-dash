package provider

type Settings struct {
	ClientID     string
	ClientSecret string
	AccessToken  string
	RefreshToken string
	ExpiresAt    int64
}

type Playback struct {
	TrackTitle string `json:"track_title"`
	ArtistName string `json:"artist_name"`

	AlbumArtURL string `json:"album_art_url,omitempty"`
	URI         string `json:"uri,omitempty"`
	IsPlaying   bool   `json:"is_playing"`
	Volume      int    `json:"volume"`
	ProgressMS  int    `json:"progress_ms"`
	DurationMS  int    `json:"duration_ms"`
	Shuffle     bool   `json:"shuffle"`
	RepeatState string `json:"repeat_state"`
	TopAlbums   []Album
}

type Album struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	ArtistName  string `json:"artist_name"`
	URI         string `json:"uri"`
	AlbumArtURL string `json:"album_art_url,omitempty"`
}

type spotifyAlbum struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	URI     string `json:"uri"`
	Artists []struct {
		Name string `json:"name"`
	} `json:"artists"`
	Images []struct {
		URL    string `json:"url"`
		Height int    `json:"height"`
		Width  int    `json:"width"`
	} `json:"images"`
}

type searchItem struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	URI         string `json:"uri"`
	Images      []struct {
		URL string `json:"url"`
	} `json:"images"`
	Album   spotifyAlbum `json:"album"`
	Artists []struct {
		Name string `json:"name"`
	} `json:"artists"`
}

type SearchResult struct {
	ID          string `json:"id"`
	Type        string `json:"type"`
	Name        string `json:"name"`
	Subtitle    string `json:"subtitle,omitempty"`
	URI         string `json:"uri"`
	AlbumArtURL string `json:"album_art_url,omitempty"`
}
