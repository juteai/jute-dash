export type AppleMusicRepeatState = "off" | "context" | "track";

export type AppleMusicPlayableType = "album" | "track" | "playlist";

export type AppleMusicPlayableItem = {
  id: string;
  type: AppleMusicPlayableType;
  name: string;
  subtitle?: string;
  uri: string;
  album_art_url?: string;
};

export type AppleMusicSearchResult = AppleMusicPlayableItem;

export type AppleMusicPlayerState = {
  isPlaying?: boolean;
  currentPlaybackTime?: number;
  currentPlaybackDuration?: number;
  nowPlayingItem?: {
    id?: string;
    type?: string;
    title?: string;
    artistName?: string;
    albumName?: string;
    artworkURL?: string;
    artwork?: {
      url?: string;
    };
    playParams?: {
      id?: string;
      kind?: string;
    };
  };
};
