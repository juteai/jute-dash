import { describe, expect, it } from 'vitest';
import {
  actionForPlayableItem,
  playableItemsFromAlbums,
  playableItemsFromSearchResults
} from '../../../../widgets/spotify/web/discovery';

describe('spotify widget discovery', () => {
  it('normalizes top albums into playable items', () => {
    expect(
      playableItemsFromAlbums([
        {
          id: 'album-1',
          name: 'Glue',
          artist_name: 'BICEP',
          uri: 'spotify:album:album-1',
          album_art_url: 'https://example.test/glue.jpg'
        },
        {
          id: 'empty',
          name: 'No URI',
          uri: ''
        }
      ])
    ).toEqual([
      {
        id: 'album-1',
        type: 'album',
        name: 'Glue',
        subtitle: 'BICEP',
        uri: 'spotify:album:album-1',
        album_art_url: 'https://example.test/glue.jpg'
      }
    ]);
  });

  it('normalizes search results into playable items', () => {
    expect(
      playableItemsFromSearchResults([
        {
          id: 'track-1',
          type: 'track',
          name: 'Glue',
          subtitle: 'BICEP',
          uri: 'spotify:track:glue'
        },
        {
          id: 'bad-1',
          type: 'artist',
          name: 'Invalid',
          uri: 'spotify:artist:bad'
        }
      ])
    ).toEqual([
      {
        id: 'track-1',
        type: 'track',
        name: 'Glue',
        subtitle: 'BICEP',
        uri: 'spotify:track:glue',
        album_art_url: undefined
      }
    ]);
  });

  it('maps playable item types to action IDs', () => {
    expect(
      actionForPlayableItem({
        id: 'playlist-1',
        type: 'playlist',
        name: 'House',
        uri: 'spotify:playlist:house'
      })
    ).toBe('play_playlist');
  });
});
