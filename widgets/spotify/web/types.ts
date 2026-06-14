export type SpotifyRepeatState = "off" | "context" | "track";

export type SpotifyPlayableType = "album" | "track" | "playlist";

export type SpotifyAlbum = {
  id: string;
  name: string;
  artist_name?: string;
  uri: string;
  album_art_url?: string;
};

export type SpotifySearchResult = {
  id: string;
  type: SpotifyPlayableType;
  name: string;
  subtitle?: string;
  uri: string;
  album_art_url?: string;
};

export type SpotifyPlayableItem = {
  id: string;
  type: SpotifyPlayableType;
  name: string;
  subtitle?: string;
  uri: string;
  album_art_url?: string;
};

export type SpotifyPlayerState = {
  paused?: boolean;
  position?: number;
  duration?: number;
  track_window?: {
    current_track?: {
      uri?: string;
      duration_ms?: number;
    };
  };
};
