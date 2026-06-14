<script lang="ts">
  import { onMount } from 'svelte';
  import DisplayShell from '$lib/components/display/DisplayShell.svelte';
  import {
    completeSpotifyAuth,
    getAdapterConnections,
    getWidgetCatalog,
    saveWidgetLayout,
    spotifyCallbackDisplayURL,
    spotifyCallbackParams
  } from '$lib/hubClient';
  import { autoLinkWidgetConnections } from '$lib/connectionAutoLink';
  import { hubStream } from '$lib/hubStream';
  import type { DashboardData } from '$lib/types';

  export let data: DashboardData;

  async function refreshAfterSpotifyAuth() {
    const fresh = await hubStream.refreshAfterMutation(fetch);
    if (fresh?.layout) {
      const [connections, catalog] = await Promise.all([
        getAdapterConnections(fetch),
        getWidgetCatalog(fetch)
      ]);
      const result = autoLinkWidgetConnections(
        fresh.layout,
        catalog,
        connections
      );
      if (result.changed) {
        const savedLayout = await saveWidgetLayout(fetch, result.layout);
        hubStream.updateLayout(savedLayout);
        await hubStream.refreshAfterMutation(fetch);
      }
    }
    for (const delay of [750, 1750, 3500]) {
      window.setTimeout(() => {
        void hubStream.refreshAfterMutation(fetch);
      }, delay);
    }
  }

  onMount(() => {
    const params = new URLSearchParams(window.location.search);
    const spotifyStatus = params.get('spotify');
    if (spotifyStatus === 'linked' && window.opener) {
      window.opener.postMessage(
        { type: 'jute.spotify.linked' },
        window.location.origin
      );
      window.close();
      return;
    }
    if (spotifyStatus === 'linked') {
      void refreshAfterSpotifyAuth();
      return;
    }

    const callback = spotifyCallbackParams(window.location.search);
    if (!callback) return;

    void completeSpotifyAuth(fetch, callback.code, callback.state)
      .then(async () => {
        window.history.replaceState(
          {},
          '',
          spotifyCallbackDisplayURL(
            window.location.pathname,
            window.location.search,
            window.location.hash,
            'linked'
          )
        );
        void refreshAfterSpotifyAuth();
      })
      .catch(() => {
        window.history.replaceState(
          {},
          '',
          spotifyCallbackDisplayURL(
            window.location.pathname,
            window.location.search,
            window.location.hash,
            'error'
          )
        );
      });
  });
</script>

<DisplayShell {data} />
