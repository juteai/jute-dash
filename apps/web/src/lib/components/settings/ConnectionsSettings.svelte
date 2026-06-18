<script lang="ts">
  import { onDestroy, onMount } from 'svelte';
  import { Copy, ExternalLink, Plug, Plus, Save } from 'lucide-svelte';
  import Button from '$lib/components/ui/Button.svelte';
  import {
    getAdapterConnectionKinds,
    getAdapterConnections,
    getWidgetCatalog,
    saveAdapterConnection,
    saveWidgetLayout,
    spotifyAuthURL,
    spotifyOAuthRedirectURI
  } from '$lib/hubClient';
  import { autoLinkWidgetConnections } from '$lib/connectionAutoLink';
  import { hubStream } from '$lib/hubStream';
  import type {
    AdapterConnection,
    AdapterConnectionKind,
    ConnectionField,
    WidgetCatalogItem,
    WidgetLayout
  } from '$lib/types';

  let connections: AdapterConnection[] = [];
  let connectionKinds: AdapterConnectionKind[] = [];
  let widgetCatalog: WidgetCatalogItem[] = [];
  let selectedId = '';
  let draft = blankConnection();
  let loading = false;
  let saving = false;
  let linkingSpotify = false;
  let issue = '';
  let spotifyRedirectUri = '';
  let spotifyReturnUri = '';
  let copiedRedirect = false;
  let popupPoll: number | undefined;

  $: selected = connections.find((connection) => connection.id === selectedId);
  $: selectedKind = connectionKinds.find((kind) => kind.kind === draft.kind);
  $: spotifyAuthType =
    typeof draft.settings?.auth_type === 'string'
      ? draft.settings.auth_type
      : 'user_app_pkce';
  $: spotifyClientId =
    typeof draft.settings?.client_id === 'string'
      ? draft.settings.client_id
      : '';
  $: spotifyClientSecretRef =
    typeof draft.secretRefs?.client_secret === 'string'
      ? draft.secretRefs.client_secret
      : '';
  $: spotifyStatus = spotifyConnectionStatus(draft);

  onMount(() => {
    spotifyRedirectUri = spotifyOAuthRedirectURI();
    spotifyReturnUri = window.location.origin;
    window.addEventListener('message', handleAuthMessage);
    void load();
    return () => {
      window.removeEventListener('message', handleAuthMessage);
      clearPopupPoll();
    };
  });

  onDestroy(clearPopupPoll);

  function defaultKind(): string {
    return connectionKinds[0]?.kind ?? 'philips-hue';
  }

  function blankConnection(kind = defaultKind()): AdapterConnection {
    return withKindDefaults({
      id: '',
      kind,
      name: '',
      settings: {},
      secretRefs: {},
      enabled: true
    });
  }

  function withKindDefaults(connection: AdapterConnection): AdapterConnection {
    if (connection.kind !== 'spotify') {
      return connection;
    }
    return {
      ...connection,
      id: connection.id || 'spotify-main',
      name: connection.name || 'Spotify'
    };
  }

  function spotifyConnectionStatus(connection: AdapterConnection): {
    label: string;
    detail: string;
    tone: 'success' | 'warning' | 'muted';
  } {
    if (connection.kind !== 'spotify') {
      return {
        label: connection.enabled ? 'Enabled' : 'Disabled',
        detail: connection.enabled
          ? 'This connection can be used by widgets.'
          : 'This connection is saved but disabled.',
        tone: connection.enabled ? 'success' : 'muted'
      };
    }
    if (!connection.enabled) {
      return {
        label: 'Disabled',
        detail: 'Enable this connection before widgets or agents can use it.',
        tone: 'muted'
      };
    }
    const hasAccessToken = Boolean(connection.secretRefs?.access_token);
    const hasRefreshToken = Boolean(connection.secretRefs?.refresh_token);
    if (hasAccessToken && hasRefreshToken) {
      return {
        label: 'Authenticated',
        detail:
          'Spotify is linked. Widgets and permitted agents can control playback through Jute.',
        tone: 'success'
      };
    }
    return {
      label: 'Login required',
      detail:
        'Save this connection, then complete Login with Spotify to link playback.',
      tone: 'warning'
    };
  }

  async function load() {
    loading = true;
    issue = '';
    try {
      [connectionKinds, connections, widgetCatalog] = await Promise.all([
        getAdapterConnectionKinds(fetch),
        getAdapterConnections(fetch),
        getWidgetCatalog(fetch)
      ]);
      if (connections.length > 0) {
        selectConnection(connections[0]);
      } else {
        newConnection(defaultKind());
      }
    } catch {
      issue = 'Connections could not be loaded.';
    } finally {
      loading = false;
    }
  }

  function selectConnection(connection: AdapterConnection) {
    selectedId = connection.id;
    draft = structuredClone(connection);
  }

  function newConnection(kind = defaultKind()) {
    selectedId = '';
    draft = blankConnection(kind);
    issue = '';
  }

  function changeKind(kind: string) {
    draft = withKindDefaults({
      ...draft,
      kind,
      settings: {},
      secretRefs: {}
    });
  }

  function fieldValue(field: ConnectionField): unknown {
    const source = field.secret ? draft.secretRefs : draft.settings;
    const current = source?.[field.id];
    if (current !== undefined) return current;
    if (field.default !== undefined) return field.default;
    if (field.type === 'boolean') return false;
    return '';
  }

  function setFieldValue(field: ConnectionField, value: unknown) {
    const key = field.secret ? 'secretRefs' : 'settings';
    const next = { ...(draft[key] ?? {}) };
    if (value === '' || value === undefined) {
      delete next[field.id];
    } else {
      next[field.id] = value;
    }
    draft = { ...draft, [key]: next };
  }

  function setSpotifyClientId(value: string) {
    const settings = { ...(draft.settings ?? {}) };
    if (value.trim() === '') {
      delete settings.client_id;
    } else {
      settings.client_id = value.trim();
    }
    draft = { ...draft, settings };
  }

  function setSpotifyAuthType(value: string) {
    const settings: Record<string, unknown> = {
      ...(draft.settings ?? {}),
      auth_type: value
    };
    const secretRefs = { ...(draft.secretRefs ?? {}) };
    if (value === 'managed_app') {
      delete settings.client_id;
      delete secretRefs.client_secret;
    }
    if (value === 'user_app_pkce') {
      delete secretRefs.client_secret;
    }
    draft = { ...draft, settings, secretRefs };
  }

  function setSpotifyClientSecretRef(value: string) {
    const secretRefs = { ...(draft.secretRefs ?? {}) };
    if (value.trim() === '') {
      delete secretRefs.client_secret;
    } else {
      secretRefs.client_secret = value.trim();
    }
    draft = { ...draft, secretRefs };
  }

  function inputValue(event: Event, field: ConnectionField): unknown {
    const input = event.currentTarget as HTMLInputElement | HTMLSelectElement;
    if (field.type === 'number') {
      return input.value === '' ? '' : Number(input.value);
    }
    if (field.type === 'boolean') {
      return (input as HTMLInputElement).checked;
    }
    return input.value;
  }

  $: canLinkSpotify = draft.kind === 'spotify' && draft.id.trim().length > 0;

  async function save(): Promise<AdapterConnection | undefined> {
    saving = true;
    issue = '';
    try {
      const saved = await saveAdapterConnection(fetch, {
        ...draft,
        id: draft.id.trim(),
        name: draft.name.trim()
      });
      const others = connections.filter(
        (connection) => connection.id !== saved.id
      );
      connections = [...others, saved].sort((a, b) =>
        a.name.localeCompare(b.name)
      );
      selectConnection(saved);
      await refreshDashboardAndAutoLinkWidgets(connections);
      return saved;
    } catch (err) {
      issue = err instanceof Error ? err.message : 'Connection was not saved.';
      return undefined;
    } finally {
      saving = false;
    }
  }

  async function linkSpotify() {
    linkingSpotify = true;
    issue = '';
    const saved = await save();
    if (!saved) {
      linkingSpotify = false;
      return;
    }
    const authURL = spotifyAuthURL(saved.id, undefined, spotifyReturnUri);
    const popup = window.open(
      authURL,
      'jute-spotify-auth',
      'popup,width=520,height=760,noopener=false'
    );
    if (!popup) {
      window.location.assign(authURL);
      return;
    }
    popup.focus();
    clearPopupPoll();
    popupPoll = window.setInterval(() => {
      if (popup.closed) {
        clearPopupPoll();
        linkingSpotify = false;
        void refreshAfterSpotifyAuth();
      }
    }, 700);
  }

  function clearPopupPoll() {
    if (popupPoll) {
      window.clearInterval(popupPoll);
      popupPoll = undefined;
    }
  }

  async function refreshConnections() {
    try {
      connections = await getAdapterConnections(fetch);
      const current = connections.find(
        (connection) => connection.id === selectedId
      );
      if (current) {
        selectConnection(current);
      }
      await refreshDashboardAndAutoLinkWidgets(connections);
    } catch {
      issue = 'Connections could not be refreshed.';
    }
  }

  async function ensureWidgetCatalog() {
    if (widgetCatalog.length > 0) return widgetCatalog;
    widgetCatalog = await getWidgetCatalog(fetch);
    return widgetCatalog;
  }

  async function refreshDashboardAndAutoLinkWidgets(
    currentConnections: AdapterConnection[]
  ) {
    const fresh = await hubStream.refreshAfterMutation(fetch);
    await autoLinkMissingWidgetConnectionRefs(
      currentConnections,
      fresh?.layout
    );
  }

  async function autoLinkMissingWidgetConnectionRefs(
    currentConnections: AdapterConnection[],
    layout: WidgetLayout | undefined
  ) {
    if (!layout) return;
    const catalog = await ensureWidgetCatalog();
    const result = autoLinkWidgetConnections(
      layout,
      catalog,
      currentConnections
    );
    if (!result.changed) return;

    const savedLayout = await saveWidgetLayout(fetch, result.layout);
    hubStream.updateLayout(savedLayout);
    await hubStream.refreshAfterMutation(fetch);
  }

  async function refreshAfterSpotifyAuth() {
    await refreshConnections();
    if (typeof window === 'undefined') return;
    for (const delay of [750, 1750, 3500]) {
      window.setTimeout(() => {
        void refreshConnections();
      }, delay);
    }
  }

  function handleAuthMessage(event: MessageEvent) {
    if (event.origin !== window.location.origin) return;
    if (event.data?.type !== 'jute.spotify.linked') return;
    clearPopupPoll();
    linkingSpotify = false;
    void refreshAfterSpotifyAuth();
  }

  async function copySpotifyRedirectUri() {
    if (!spotifyRedirectUri) return;
    await navigator.clipboard.writeText(spotifyRedirectUri);
    copiedRedirect = true;
    window.setTimeout(() => {
      copiedRedirect = false;
    }, 1600);
  }
</script>

<div class="connections-settings">
  <div class="connections-list" aria-label="Adapter connections">
    <div class="section-heading">
      <div>
        <strong>Connections</strong>
        <span>{connections.length} shared Adapter Connections</span>
      </div>
      <Button size="sm" variant="outline" on:click={() => newConnection()}>
        <Plus size={15} /><span>New</span>
      </Button>
    </div>

    {#if loading}
      <p class="muted">Loading connections...</p>
    {:else if connections.length === 0}
      <p class="muted">No shared connections yet.</p>
    {:else}
      {#each connections as connection (connection.id)}
        <button
          type="button"
          class="connection-row"
          class:connection-row--active={selected?.id === connection.id}
          on:click={() => selectConnection(connection)}
        >
          <Plug size={16} />
          <span>
            <strong>{connection.name || connection.id}</strong>
            <small>{connection.kind}</small>
          </span>
          {#if connection.kind === 'spotify'}
            {@const status = spotifyConnectionStatus(connection)}
            <span
              class:status-pill--success={status.tone === 'success'}
              class:status-pill--warning={status.tone === 'warning'}
              class="status-pill"
            >
              {status.label}
            </span>
          {/if}
        </button>
      {/each}
    {/if}
  </div>

  <form class="connection-editor" on:submit|preventDefault={save}>
    <div class="section-heading">
      <div>
        <strong>{selectedId ? 'Edit connection' : 'New connection'}</strong>
        <span>Widget instances link to these by ID.</span>
      </div>
    </div>

    {#if issue}
      <p class="settings-issue">{issue}</p>
    {/if}

    <label class="field">
      <span class="field-label">ID</span>
      <input class="text-input" type="text" bind:value={draft.id} required />
    </label>

    <label class="field">
      <span class="field-label">Kind</span>
      <select
        class="text-input"
        value={draft.kind}
        on:change={(event) =>
          changeKind((event.currentTarget as HTMLSelectElement).value)}
      >
        {#each connectionKinds as kind (kind.kind)}
          <option value={kind.kind}>{kind.displayName || kind.kind}</option>
        {/each}
      </select>
    </label>

    <label class="field">
      <span class="field-label">Name</span>
      <input class="text-input" type="text" bind:value={draft.name} required />
    </label>

    <label class="field field-inline">
      <input type="checkbox" bind:checked={draft.enabled} />
      <span class="field-label">Enabled</span>
    </label>

    {#if draft.kind === 'spotify'}
      <div class="spotify-setup">
        <div class="setup-card">
          <div class="setup-card-heading">
            <span class="field-label">Spotify login</span>
            <span
              class:status-pill--success={spotifyStatus.tone === 'success'}
              class:status-pill--warning={spotifyStatus.tone === 'warning'}
              class="status-pill"
            >
              {spotifyStatus.label}
            </span>
          </div>
          <p>{spotifyStatus.detail}</p>
          {#if spotifyStatus.tone !== 'success'}
            <p>
              Use Spotify OAuth. You do not need to paste access tokens, refresh
              tokens, or a client secret.
            </p>
          {:else}
            <p>
              Use Login with Spotify again only if playback stops working or you
              want to switch Spotify accounts.
            </p>
          {/if}
        </div>

        <label class="field">
          <span class="field-label">Auth type</span>
          <select
            class="text-input"
            value={spotifyAuthType}
            on:change={(event) =>
              setSpotifyAuthType(
                (event.currentTarget as HTMLSelectElement).value
              )}
          >
            <option value="managed_app">Jute managed app</option>
            <option value="user_app_pkce">My Spotify app</option>
            <option value="confidential_app">Advanced confidential app</option>
          </select>
          <span class="field-help">
            Choose Jute managed app for no developer setup when this hub is
            configured with one. Choose My Spotify app for local development: it
            only needs a Client ID.
          </span>
        </label>

        {#if spotifyAuthType !== 'managed_app'}
          <label class="field">
            <span class="field-label">Client ID</span>
            <input
              class="text-input"
              type="text"
              value={spotifyClientId}
              autocomplete="off"
              placeholder="Spotify Developer Client ID"
              on:input={(event) =>
                setSpotifyClientId(
                  (event.currentTarget as HTMLInputElement).value
                )}
            />
            <span class="field-help">
              Required for this local/self-hosted setup unless the hub is
              started with a managed Spotify app ID. A client secret is not
              required.
            </span>
          </label>
        {/if}

        {#if spotifyAuthType === 'confidential_app'}
          <label class="field">
            <span class="field-label">Client secret reference</span>
            <input
              class="text-input"
              type="text"
              value={spotifyClientSecretRef}
              autocomplete="off"
              placeholder="env:SPOTIFY_CLIENT_SECRET"
              on:input={(event) =>
                setSpotifyClientSecretRef(
                  (event.currentTarget as HTMLInputElement).value
                )}
            />
            <span class="field-help">
              Optional advanced mode. Store only a secret reference here, never
              the raw client secret.
            </span>
          </label>
        {/if}

        <div class="setup-hint">
          <div>
            <span class="field-label">Spotify Redirect URI</span>
            <code>{spotifyRedirectUri || 'https://localhost:5173'}</code>
            <span class="field-help">
              Add this exact URI in the Spotify Developer Dashboard. Spotify
              requires the explicit loopback IP here; localhost is rejected.
            </span>
          </div>
          <Button
            type="button"
            size="sm"
            variant="outline"
            on:click={copySpotifyRedirectUri}
          >
            <Copy size={14} />
            <span>{copiedRedirect ? 'Copied' : 'Copy'}</span>
          </Button>
        </div>
      </div>
    {:else if selectedKind}
      <div class="field-group">
        <span class="field-label">{selectedKind.displayName}</span>
        {#if selectedKind.description}
          <span class="field-help">{selectedKind.description}</span>
        {/if}
        {#each selectedKind.fields as field (field.id)}
          {#if field.type === 'boolean'}
            <label class="field field-inline">
              <input
                type="checkbox"
                checked={Boolean(fieldValue(field))}
                on:change={(event) =>
                  setFieldValue(field, inputValue(event, field))}
              />
              <span class="field-label">{field.label}</span>
            </label>
          {:else if field.type === 'enum'}
            <label class="field">
              <span class="field-label"
                >{field.label}{field.required ? ' *' : ''}</span
              >
              <select
                class="text-input"
                value={fieldValue(field) as string}
                required={field.required}
                on:change={(event) =>
                  setFieldValue(field, inputValue(event, field))}
              >
                <option value=""></option>
                {#each field.options ?? [] as option (option)}
                  <option value={option}>{option}</option>
                {/each}
              </select>
              {#if field.help}<span class="field-help">{field.help}</span>{/if}
            </label>
          {:else}
            <label class="field">
              <span class="field-label"
                >{field.label}{field.required ? ' *' : ''}</span
              >
              <input
                class="text-input"
                type={field.type === 'number' ? 'number' : 'text'}
                value={fieldValue(field) as string | number}
                required={field.required}
                placeholder={field.secret ? 'env:SECRET_NAME' : ''}
                on:input={(event) =>
                  setFieldValue(field, inputValue(event, field))}
              />
              {#if field.help}<span class="field-help">{field.help}</span>{/if}
            </label>
          {/if}
        {/each}
      </div>
    {:else}
      <p class="settings-issue">
        This Adapter Connection kind is not registered by a built-in Widget.
      </p>
    {/if}

    <div class="actions">
      {#if draft.kind === 'spotify'}
        <Button
          type="button"
          variant="outline"
          disabled={!canLinkSpotify || saving || linkingSpotify}
          on:click={linkSpotify}
        >
          <ExternalLink size={15} />
          <span
            >{linkingSpotify
              ? 'Opening Spotify'
              : spotifyStatus.tone === 'success'
                ? 'Refresh Spotify login'
                : 'Login with Spotify'}</span
          >
        </Button>
      {/if}
      <Button type="submit" disabled={saving}>
        <Save size={15} /><span>{saving ? 'Saving' : 'Save'}</span>
      </Button>
    </div>
  </form>
</div>

<style>
  .connections-settings {
    display: grid;
    grid-template-columns: minmax(180px, 260px) minmax(0, 1fr);
    gap: 14px;
    height: 100%;
    min-height: 0;
  }

  .connections-list,
  .connection-editor {
    display: grid;
    align-content: start;
    gap: 10px;
    min-height: 0;
  }

  .connections-list {
    overflow-y: auto;
    padding-right: 2px;
  }

  .connection-editor {
    overflow-y: auto;
    padding-right: 4px;
  }

  .section-heading {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 10px;
  }

  .section-heading strong {
    display: block;
    color: var(--foreground);
  }

  .section-heading span,
  .muted,
  .field-help {
    color: var(--muted);
    font-size: 0.8rem;
    font-weight: 650;
  }

  .connection-row {
    display: grid;
    grid-template-columns: auto minmax(0, 1fr) auto;
    align-items: center;
    gap: 8px;
    min-height: 46px;
    border: 1px solid var(--border);
    border-radius: 8px;
    background: var(--surface);
    color: var(--foreground);
    padding: 8px;
    text-align: left;
    cursor: pointer;
  }

  .connection-row--active {
    border-color: var(--foreground);
    background: var(--surface-muted);
  }

  .connection-row strong,
  .connection-row small {
    display: block;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .connection-row small {
    color: var(--muted);
    font-size: 0.74rem;
    font-weight: 700;
  }

  .field {
    display: grid;
    gap: 6px;
  }

  .field-inline {
    grid-template-columns: auto minmax(0, 1fr);
    align-items: center;
  }

  .field-group {
    display: grid;
    gap: 10px;
    padding-top: 4px;
  }

  .spotify-setup {
    display: grid;
    gap: 10px;
  }

  .setup-card {
    display: grid;
    gap: 5px;
    border: 1px solid var(--border);
    border-radius: 8px;
    background: var(--surface-muted);
    padding: 10px;
  }

  .setup-card-heading {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 10px;
  }

  .setup-card p {
    margin: 0;
    color: var(--muted);
    font-size: 0.82rem;
    font-weight: 650;
    line-height: 1.35;
  }

  .status-pill {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    min-height: 24px;
    padding: 2px 8px;
    border: 1px solid var(--border);
    border-radius: 999px;
    color: var(--muted);
    font-size: 0.68rem;
    font-weight: 800;
    text-transform: uppercase;
    white-space: nowrap;
  }

  .status-pill--success {
    border-color: color-mix(in srgb, var(--success) 45%, var(--border));
    background: color-mix(in srgb, var(--success) 10%, transparent);
    color: var(--success);
  }

  .status-pill--warning {
    border-color: color-mix(in srgb, var(--warning) 55%, var(--border));
    background: color-mix(in srgb, var(--warning) 12%, transparent);
    color: var(--warning);
  }

  .setup-hint {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 10px;
    border: 1px solid var(--border);
    border-radius: 8px;
    background: var(--surface-muted);
    padding: 10px;
  }

  .setup-hint code {
    display: block;
    overflow-wrap: anywhere;
    color: var(--foreground);
    font-size: 0.82rem;
    font-weight: 750;
  }

  .field-label {
    color: var(--foreground);
    font-size: 0.82rem;
    font-weight: 750;
  }

  .text-input {
    min-height: 36px;
    width: 100%;
    border: 1px solid var(--border);
    border-radius: 8px;
    background: var(--surface);
    color: var(--foreground);
    padding: 8px 10px;
    font: inherit;
  }

  .settings-issue {
    margin: 0;
    color: var(--danger, #ef4444);
    font-size: 0.82rem;
    font-weight: 700;
  }

  .actions {
    position: sticky;
    bottom: 0;
    display: flex;
    gap: 8px;
    justify-content: flex-end;
    border-top: 1px solid var(--border);
    background: var(--surface);
    padding-top: 10px;
  }

  @media (max-width: 720px) {
    .connections-settings {
      grid-template-columns: 1fr;
    }
  }
</style>
