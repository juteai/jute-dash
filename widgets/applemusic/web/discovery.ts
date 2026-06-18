import type {
  AppleMusicPlayableItem,
  AppleMusicPlayableType,
  AppleMusicSearchResult,
} from "./types";

export function playableItemsFromAlbums(
  albums: unknown,
): AppleMusicPlayableItem[] {
  if (!Array.isArray(albums)) return [];
  return albums
    .filter((album): album is AppleMusicPlayableItem => {
      return Boolean(album && typeof album === "object" && "uri" in album);
    })
    .map((album) => ({
      id: album.id,
      type: "album" as const,
      name: album.name,
      subtitle: album.subtitle,
      uri: album.uri,
      album_art_url: album.album_art_url,
    }))
    .filter((item) => Boolean(item.uri));
}

export function playableItemsFromSearchResults(
  results: unknown,
): AppleMusicPlayableItem[] {
  if (!Array.isArray(results)) return [];
  return results
    .filter((result): result is AppleMusicSearchResult => {
      if (!result || typeof result !== "object") return false;
      const candidate = result as Partial<AppleMusicSearchResult>;
      return (
        Boolean(candidate.uri) &&
        isPlayableType(candidate.type) &&
        typeof candidate.name === "string"
      );
    })
    .map((result) => ({
      id: result.id,
      type: result.type,
      name: result.name,
      subtitle: result.subtitle,
      uri: result.uri,
      album_art_url: result.album_art_url,
    }));
}

export function actionForPlayableItem(item: AppleMusicPlayableItem) {
  if (item.type === "playlist") return "play_playlist";
  if (item.type === "album") return "play_album";
  return "play_track";
}

export function musicKitQueueDescriptor(item: AppleMusicPlayableItem) {
  const parsed = parseAppleMusicURI(item.uri);
  if (!parsed) return undefined;
  if (parsed.kind === "albums") return { album: parsed.id };
  if (parsed.kind === "playlists") return { playlist: parsed.id };
  return { song: parsed.id };
}

export function parseAppleMusicURI(uri: string) {
  const parts = uri.split(":");
  if (parts.length < 3 || parts[0] !== "apple-music") return null;
  return { kind: parts[1], id: parts.slice(2).join(":") };
}

function isPlayableType(value: unknown): value is AppleMusicPlayableType {
  return value === "album" || value === "track" || value === "playlist";
}
