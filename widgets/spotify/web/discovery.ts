import type {
  SpotifyAlbum,
  SpotifyPlayableItem,
  SpotifyPlayableType,
  SpotifySearchResult,
} from "./types";

export function albumToPlayableItem(album: SpotifyAlbum): SpotifyPlayableItem {
  return {
    id: album.id,
    type: "album",
    name: album.name,
    subtitle: album.artist_name,
    uri: album.uri,
    album_art_url: album.album_art_url,
  };
}

export function searchResultToPlayableItem(
  result: SpotifySearchResult,
): SpotifyPlayableItem {
  return {
    id: result.id,
    type: result.type,
    name: result.name,
    subtitle: result.subtitle,
    uri: result.uri,
    album_art_url: result.album_art_url,
  };
}

export function playableItemsFromAlbums(
  albums: unknown,
): SpotifyPlayableItem[] {
  if (!Array.isArray(albums)) return [];
  return albums
    .filter((album): album is SpotifyAlbum => {
      return Boolean(album && typeof album === "object" && "uri" in album);
    })
    .map(albumToPlayableItem)
    .filter((item) => Boolean(item.uri));
}

export function playableItemsFromSearchResults(
  results: unknown,
): SpotifyPlayableItem[] {
  if (!Array.isArray(results)) return [];
  return results
    .filter((result): result is SpotifySearchResult => {
      if (!result || typeof result !== "object") return false;
      const candidate = result as Partial<SpotifySearchResult>;
      return (
        Boolean(candidate.uri) &&
        isPlayableType(candidate.type) &&
        typeof candidate.name === "string"
      );
    })
    .map(searchResultToPlayableItem);
}

export function actionForPlayableItem(item: SpotifyPlayableItem) {
  if (item.type === "playlist") return "play_playlist";
  if (item.type === "album") return "play_album";
  return "play_track";
}

function isPlayableType(value: unknown): value is SpotifyPlayableType {
  return value === "album" || value === "track" || value === "playlist";
}
