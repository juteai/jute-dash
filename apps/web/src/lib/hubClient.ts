import type {
  Agent,
  AppStatus,
  BackgroundImage,
  DashboardData,
  HouseholdSettings,
  HomeState,
  PublicConfig,
  Room,
  RoomsSettings,
  Tile,
  TilesSettings,
  UserFacingIssue,
  VoiceProvider,
  VoiceFinalTranscriptRequest,
  VoiceFinalTranscriptResponse,
  VoiceSettingsUpdate,
  VoiceStatus,
  TTSVoicesResponse,
  WidgetCatalogItem,
  WidgetLayout
} from '$lib/types';

export const API_BASE =
  import.meta.env.VITE_JUTE_API_URL ?? 'http://127.0.0.1:8787';

async function hubError(response: Response, fallback: string): Promise<Error> {
  const body = await response
    .json()
    .catch(() => ({ error: `HTTP ${response.status}` }));
  return new Error(
    typeof body.error === 'string'
      ? body.error
      : `${fallback}: ${response.status}`
  );
}

async function getJSON<T>(fetcher: typeof fetch, path: string): Promise<T> {
  const response = await fetcher(`${API_BASE}${path}`);
  if (!response.ok) {
    throw new Error(`Failed to fetch ${path}: ${response.status}`);
  }
  return response.json() as Promise<T>;
}

export async function getDashboard(
  fetcher: typeof fetch
): Promise<DashboardData> {
  const [config, home, agentResponse, layout, voice, status] =
    await Promise.all([
      getJSON<PublicConfig>(fetcher, '/api/v1/config'),
      getJSON<HomeState>(fetcher, '/api/v1/home'),
      getJSON<{ agents: Agent[] }>(fetcher, '/api/v1/agents'),
      getJSON<WidgetLayout>(fetcher, '/api/v1/widgets/layout'),
      getJSON<VoiceStatus>(fetcher, '/api/v1/voice/status'),
      getJSON<AppStatus>(fetcher, '/api/v1/status')
    ]);

  return {
    config,
    home,
    agents: agentResponse.agents,
    layout,
    voice,
    status,
    connectionState: status.status === 'ok' ? 'connected' : 'degraded',
    stale: false,
    hubUrl: API_BASE,
    loadedAt: new Date().toISOString(),
    issue:
      status.status === 'ok'
        ? undefined
        : {
            code: 'hub_degraded',
            severity: 'warning',
            title: 'Jute is degraded',
            message: 'One or more local services need attention.'
          }
  };
}

export async function getHouseholdSettings(
  fetcher: typeof fetch
): Promise<HouseholdSettings> {
  return getJSON<HouseholdSettings>(fetcher, '/api/v1/settings/household');
}

export async function saveHouseholdSettings(
  fetcher: typeof fetch,
  settings: HouseholdSettings
): Promise<HouseholdSettings> {
  const response = await fetcher(`${API_BASE}/api/v1/settings/household`, {
    method: 'PATCH',
    headers: {
      'Content-Type': 'application/json'
    },
    body: JSON.stringify(settings)
  });
  if (!response.ok) {
    throw await hubError(response, 'Jute API request failed');
  }
  return response.json() as Promise<HouseholdSettings>;
}

export async function getRoomSettings(fetcher: typeof fetch): Promise<Room[]> {
  const response = await getJSON<RoomsSettings>(
    fetcher,
    '/api/v1/settings/rooms'
  );
  return response.rooms;
}

export async function saveRoomSettings(
  fetcher: typeof fetch,
  rooms: Room[]
): Promise<Room[]> {
  const response = await fetcher(`${API_BASE}/api/v1/settings/rooms`, {
    method: 'PUT',
    headers: {
      'Content-Type': 'application/json'
    },
    body: JSON.stringify({ rooms })
  });
  if (!response.ok) {
    throw await hubError(response, 'Jute API request failed');
  }
  const saved = (await response.json()) as RoomsSettings;
  return saved.rooms;
}

export async function getTileSettings(fetcher: typeof fetch): Promise<Tile[]> {
  const response = await getJSON<TilesSettings>(
    fetcher,
    '/api/v1/settings/tiles'
  );
  return response.tiles;
}

export async function saveTileSettings(
  fetcher: typeof fetch,
  tiles: Tile[]
): Promise<Tile[]> {
  const response = await fetcher(`${API_BASE}/api/v1/settings/tiles`, {
    method: 'PUT',
    headers: {
      'Content-Type': 'application/json'
    },
    body: JSON.stringify({ tiles })
  });
  if (!response.ok) {
    throw await hubError(response, 'Jute API request failed');
  }
  const saved = (await response.json()) as TilesSettings;
  return saved.tiles;
}

export async function addAgent(
  fetcher: typeof fetch,
  cardUrl: string
): Promise<Agent> {
  const response = await fetcher(`${API_BASE}/api/v1/agents`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json'
    },
    body: JSON.stringify({ cardUrl })
  });
  if (!response.ok) {
    throw await hubError(response, 'Jute API request failed');
  }
  return response.json() as Promise<Agent>;
}

export async function setAgentEnabled(
  fetcher: typeof fetch,
  agentId: string,
  enabled: boolean
): Promise<Agent> {
  const response = await fetcher(
    `${API_BASE}/api/v1/agents/${encodeURIComponent(agentId)}`,
    {
      method: 'PATCH',
      headers: {
        'Content-Type': 'application/json'
      },
      body: JSON.stringify({ enabled })
    }
  );
  if (!response.ok) {
    throw await hubError(response, 'Jute API request failed');
  }
  return response.json() as Promise<Agent>;
}

export async function deleteAgent(
  fetcher: typeof fetch,
  agentId: string
): Promise<void> {
  const response = await fetcher(
    `${API_BASE}/api/v1/agents/${encodeURIComponent(agentId)}`,
    {
      method: 'DELETE'
    }
  );
  if (!response.ok) {
    throw await hubError(response, 'Jute API request failed');
  }
}

export async function refreshAgentCard(
  fetcher: typeof fetch,
  agentId: string
): Promise<Agent> {
  const response = await fetcher(
    `${API_BASE}/api/v1/agents/${encodeURIComponent(agentId)}/refresh-card`,
    {
      method: 'POST'
    }
  );
  if (!response.ok) {
    throw await hubError(response, 'Jute API request failed');
  }
  return response.json() as Promise<Agent>;
}

export async function getWidgetCatalog(
  fetcher: typeof fetch
): Promise<WidgetCatalogItem[]> {
  const response = await getJSON<{ widgets: WidgetCatalogItem[] }>(
    fetcher,
    '/api/v1/widgets/catalog'
  );
  return response.widgets;
}

export async function saveWidgetLayout(
  fetcher: typeof fetch,
  layout: WidgetLayout
): Promise<WidgetLayout> {
  const response = await fetcher(`${API_BASE}/api/v1/widgets/layout`, {
    method: 'PUT',
    headers: {
      'Content-Type': 'application/json'
    },
    body: JSON.stringify(layout)
  });
  if (!response.ok) {
    throw await hubError(response, 'Jute API request failed');
  }
  return response.json() as Promise<WidgetLayout>;
}

const BACKGROUNDS_BASE = '/api/v1/backgrounds';

export async function getBackgroundImages(
  fetcher: typeof fetch
): Promise<BackgroundImage[]> {
  const response = await getJSON<{ images: BackgroundImage[] }>(
    fetcher,
    BACKGROUNDS_BASE
  );
  return response.images ?? [];
}

export async function uploadBackgroundImage(
  fetcher: typeof fetch,
  file: File
): Promise<BackgroundImage> {
  const form = new FormData();
  form.append('file', file);
  const response = await fetcher(`${API_BASE}${BACKGROUNDS_BASE}`, {
    method: 'POST',
    body: form
  });
  if (!response.ok) {
    throw await hubError(response, 'Background upload failed');
  }
  return response.json() as Promise<BackgroundImage>;
}

export async function deleteBackgroundImage(
  fetcher: typeof fetch,
  name: string
): Promise<void> {
  const response = await fetcher(
    `${API_BASE}${BACKGROUNDS_BASE}?name=${encodeURIComponent(name)}`,
    { method: 'DELETE' }
  );
  if (!response.ok && response.status !== 204) {
    throw new Error(`Background delete failed: ${response.status}`);
  }
}

/** Resolves a stored background image file name to an absolute hub URL. */
export function backgroundImageURL(name: string): string {
  if (!name) {
    return '';
  }
  if (/^https?:\/\//i.test(name) || name.startsWith('/api/')) {
    return name.startsWith('/api/') ? `${API_BASE}${name}` : name;
  }
  return `${API_BASE}${BACKGROUNDS_BASE}/files/${encodeURIComponent(name)}`;
}

export async function resetWidgetLayout(
  fetcher: typeof fetch,
  profileId: string
): Promise<WidgetLayout> {
  const suffix = profileId ? `?profileId=${encodeURIComponent(profileId)}` : '';
  const response = await fetcher(
    `${API_BASE}/api/v1/widgets/layout/reset${suffix}`,
    {
      method: 'POST'
    }
  );
  if (!response.ok) {
    throw await hubError(response, 'Jute API request failed');
  }
  return response.json() as Promise<WidgetLayout>;
}

export async function muteVoice(fetcher: typeof fetch): Promise<VoiceStatus> {
  return postVoiceControl(fetcher, '/api/v1/voice/mute');
}

export async function unmuteVoice(fetcher: typeof fetch): Promise<VoiceStatus> {
  return postVoiceControl(fetcher, '/api/v1/voice/unmute');
}

export async function cancelVoice(fetcher: typeof fetch): Promise<VoiceStatus> {
  return postVoiceControl(fetcher, '/api/v1/voice/cancel');
}

export async function getVoiceProviders(
  fetcher: typeof fetch
): Promise<VoiceProvider[]> {
  const response = await getJSON<{ providers: VoiceProvider[] }>(
    fetcher,
    '/api/v1/voice/providers'
  );
  return (response.providers ?? []).map(safeVoiceProvider);
}

export async function getTTSVoices(
  fetcher: typeof fetch,
  providerId = ''
): Promise<TTSVoicesResponse> {
  const suffix = providerId
    ? `?providerId=${encodeURIComponent(providerId)}`
    : '';
  return safeTTSVoicesResponse(
    await getJSON<TTSVoicesResponse>(fetcher, `/api/v1/tts/voices${suffix}`)
  );
}

export async function saveVoiceSettings(
  fetcher: typeof fetch,
  settings: VoiceSettingsUpdate
): Promise<VoiceStatus> {
  const response = await fetcher(`${API_BASE}/api/v1/voice/settings`, {
    method: 'PATCH',
    headers: {
      'Content-Type': 'application/json'
    },
    body: JSON.stringify(settings)
  });
  if (!response.ok) {
    throw await hubError(response, 'Jute API request failed');
  }
  return response.json() as Promise<VoiceStatus>;
}

export async function submitVoiceFinalTranscript(
  fetcher: typeof fetch,
  transcript: VoiceFinalTranscriptRequest
): Promise<VoiceFinalTranscriptResponse> {
  const response = await fetcher(`${API_BASE}/api/v1/voice/transcripts/final`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json'
    },
    body: JSON.stringify(transcript)
  });
  if (!response.ok) {
    throw await hubError(response, 'Jute API request failed');
  }
  return response.json() as Promise<VoiceFinalTranscriptResponse>;
}

export function fallbackDashboard(issue?: UserFacingIssue): DashboardData {
  const config: PublicConfig = {
    home: {
      name: 'Jute Home'
    },
    display: {
      theme: 'system',
      colorMode: 'system',
      themeId: 'jute-mono',
      density: 'comfortable',
      motion: 'full',
      background: {
        kind: 'theme',
        value: '',
        fit: 'cover',
        position: 'center',
        overlay: 'none'
      },
      widgetChrome: {
        default: 'solid'
      },
      accentColor: 'neutral',
      idleMode: 'ambient'
    },
    agents: [],
    rooms: [],
    tiles: []
  };

  return {
    config,
    home: {
      generatedAt: new Date().toISOString(),
      home: config.home,
      rooms: [],
      tiles: []
    },
    agents: [],
    layout: fallbackLayout(),
    voice: fallbackVoiceStatus(),
    status: fallbackStatus(),
    connectionState: 'offline',
    stale: true,
    hubUrl: API_BASE,
    loadedAt: new Date().toISOString(),
    issue: issue ?? {
      code: 'hub_unreachable',
      severity: 'error',
      title: 'Hub not reachable',
      message: `Jute Dash cannot connect to the local hub at ${shortHubUrl(API_BASE)}.`,
      action: {
        label: 'Retry',
        target: 'retry'
      }
    }
  };
}

export function initialDashboard(): DashboardData {
  return {
    ...fallbackDashboard(undefined),
    connectionState: 'starting',
    stale: true,
    issue: undefined
  };
}

function fallbackStatus(): AppStatus {
  return {
    status: 'offline',
    version: '',
    startedAt: '',
    setup: {
      complete: false,
      missing: ['hub']
    },
    config: {
      hasBootstrapConfig: false,
      writableYaml: false
    },
    eventStream: {
      available: false
    },
    mcp: {
      enabled: false,
      serviceStatus: 'disabled',
      transport: '',
      listenAddress: '',
      path: '',
      authMode: '',
      allowLan: false
    },
    agents: {
      total: 0,
      enabled: 0,
      disabled: 0,
      available: 0,
      unavailable: 0,
      dashboardContextSupported: 0,
      mcpScoped: 0
    },
    voice: {
      enabled: false,
      serviceStatus: 'not_configured',
      state: 'muted'
    }
  };
}

export function hubURL() {
  return API_BASE;
}

export function eventsURL() {
  return `${API_BASE}/api/v1/events`;
}

function shortHubUrl(value: string) {
  return value.replace(/^https?:\/\//, '');
}

function fallbackLayout(): WidgetLayout {
  return {
    profileId: 'fallback-dashboard',
    widgets: [
      {
        id: 'date-time',
        kind: 'date-time',
        title: 'Date & Time',
        x: 0,
        y: 0,
        w: 2,
        h: 1,
        minW: 1,
        minH: 1,
        size: 'wide',
        settings: {},
        visible: true
      },
      {
        id: 'weather',
        kind: 'weather',
        title: 'Weather',
        x: 2,
        y: 0,
        w: 2,
        h: 1,
        minW: 1,
        minH: 1,
        size: 'wide',
        settings: {},
        visible: true
      },
      {
        id: 'chat-history',
        kind: 'chat-history',
        title: 'Chat History',
        x: 0,
        y: 1,
        w: 2,
        h: 2,
        minW: 1,
        minH: 1,
        size: 'medium',
        settings: {},
        visible: true
      }
    ]
  };
}

function fallbackVoiceStatus(): VoiceStatus {
  return {
    enabled: false,
    muted: true,
    state: 'muted',
    serviceStatus: 'not_configured',
    deviceProfileId: 'fallback-display',
    wakeWordModelId: '',
    wakeWordPhrase: '',
    wakeSensitivity: 0.5,
    sttProviderId: '',
    ttsProviderId: '',
    sttModelId: '',
    ttsModelId: '',
    ttsVoiceId: '',
    ttsEnabled: false,
    ttsLocale: 'en',
    ttsSpeed: 1,
    ttsVolume: 1,
    preferredAgentId: '',
    cloudOptIn: false,
    commandProvidersEnabled: false,
    followupWindowSeconds: 8,
    microphoneProfile: '',
    updatedAt: new Date().toISOString()
  };
}

function safeVoiceProvider(provider: VoiceProvider): VoiceProvider {
  return {
    id: provider.id,
    name: provider.name,
    version: provider.version,
    kind: provider.kind,
    transportType: provider.transportType,
    capabilities: provider.capabilities
      ? {
          streaming: Boolean(provider.capabilities.streaming),
          partialTranscripts: Boolean(provider.capabilities.partialTranscripts),
          offline: Boolean(provider.capabilities.offline),
          languages: safeStringArray(provider.capabilities.languages),
          inputFormats: safeStringArray(provider.capabilities.inputFormats),
          outputFormats: safeStringArray(provider.capabilities.outputFormats)
        }
      : undefined,
    wakeWord: provider.wakeWord
      ? {
          defaultModelId: provider.wakeWord.defaultModelId,
          phrase: provider.wakeWord.phrase,
          languages: safeStringArray(provider.wakeWord.languages),
          sensitivity: provider.wakeWord.sensitivity,
          models: provider.wakeWord.models?.map((model) => ({
            id: model.id,
            phrase: model.phrase,
            languages: safeStringArray(model.languages),
            sensitivity: model.sensitivity
          }))
        }
      : undefined,
    healthStatus: provider.healthStatus,
    lastActivationAt: provider.lastActivationAt,
    lastError: redactCredentialText(provider.lastError),
    updatedAt: provider.updatedAt
  };
}

function safeTTSVoicesResponse(response: TTSVoicesResponse): TTSVoicesResponse {
  return {
    providerId: response.providerId,
    providerName: redactCredentialText(response.providerName),
    healthStatus: response.healthStatus,
    setupStatus: response.setupStatus,
    selectedVoiceId: response.selectedVoiceId,
    selectedModelId: response.selectedModelId,
    locale: redactCredentialText(response.locale) ?? '',
    speed: Number(response.speed),
    volume: Number(response.volume),
    cloudProvider: Boolean(response.cloudProvider),
    voices: (response.voices ?? []).map((voice) => ({
      id: voice.id,
      label: redactCredentialText(voice.label) ?? '',
      locale: redactCredentialText(voice.locale) ?? '',
      modelId: redactCredentialText(voice.modelId),
      styles: safeStringArray(voice.styles),
      outputFormats: safeStringArray(voice.outputFormats)
    }))
  };
}

function safeStringArray(values: string[] | undefined): string[] | undefined {
  if (!values) {
    return undefined;
  }
  return values.flatMap((value) => {
    const redacted = redactCredentialText(value);
    return redacted ? [redacted] : [];
  });
}

function redactCredentialText(value: string | undefined): string | undefined {
  if (!value) {
    return value;
  }
  return value
    .replace(/\b(?:https?|wss?|tcp):\/\/[^\s)]+/gi, '[redacted-url]')
    .replace(/token=[^\s&]+/gi, 'token=[redacted]')
    .replace(/secret[:=][^\s&]+/gi, 'secret=[redacted]')
    .replace(/password[:=][^\s&]+/gi, 'password=[redacted]')
    .replace(/api[_-]?key[:=][^\s&]+/gi, 'api_key=[redacted]')
    .replace(/sk-[A-Za-z0-9_-]+/g, 'sk-[redacted]');
}

async function postVoiceControl(
  fetcher: typeof fetch,
  path: string
): Promise<VoiceStatus> {
  const response = await fetcher(`${API_BASE}${path}`, {
    method: 'POST'
  });
  if (!response.ok) {
    throw await hubError(response, 'Jute API request failed');
  }
  return response.json() as Promise<VoiceStatus>;
}
