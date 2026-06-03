import type {
  Agent,
  AppStatus,
  Conversation,
  ConversationDetail,
  ConversationStreamEvent,
  DashboardData,
  HouseholdSettings,
  HomeState,
  MessageResponse,
  PublicConfig,
  Room,
  RoomsSettings,
  Tile,
  TilesSettings,
  UserFacingIssue,
  VoiceProvider,
  VoiceStatus,
  WidgetCatalogItem,
  WidgetLayout
} from '$lib/types';

const API_BASE = import.meta.env.VITE_JUTE_API_URL ?? 'http://127.0.0.1:8787';

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

export async function sendMessage(
  fetcher: typeof fetch,
  agentId: string,
  text: string,
  conversationId?: string
): Promise<MessageResponse> {
  const response = await fetcher(`${API_BASE}/api/v1/messages`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json'
    },
    body: JSON.stringify({ agentId, text, conversationId })
  });
  if (!response.ok) {
    const body = await response
      .json()
      .catch(() => ({ error: `HTTP ${response.status}` }));
    throw new Error(
      typeof body.error === 'string'
        ? body.error
        : `Jute API request failed: ${response.status}`
    );
  }
  return response.json() as Promise<MessageResponse>;
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
    const body = await response
      .json()
      .catch(() => ({ error: `HTTP ${response.status}` }));
    throw new Error(
      typeof body.error === 'string'
        ? body.error
        : `Jute API request failed: ${response.status}`
    );
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
    const body = await response
      .json()
      .catch(() => ({ error: `HTTP ${response.status}` }));
    throw new Error(
      typeof body.error === 'string'
        ? body.error
        : `Jute API request failed: ${response.status}`
    );
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
    const body = await response
      .json()
      .catch(() => ({ error: `HTTP ${response.status}` }));
    throw new Error(
      typeof body.error === 'string'
        ? body.error
        : `Jute API request failed: ${response.status}`
    );
  }
  const saved = (await response.json()) as TilesSettings;
  return saved.tiles;
}

export async function getConversations(
  fetcher: typeof fetch,
  agentId: string
): Promise<Conversation[]> {
  if (!agentId) {
    return [];
  }
  const response = await fetcher(
    `${API_BASE}/api/v1/conversations?agentId=${encodeURIComponent(agentId)}`
  );
  if (response.status === 501) {
    return [
      {
        id: `history-unsupported-${agentId}`,
        agentId,
        title: 'History unavailable',
        status: 'unavailable',
        a2aContextId: '',
        latestTaskId: '',
        createdAt: '',
        updatedAt: '',
        historyUnsupported: true
      }
    ];
  }
  if (!response.ok) {
    const body = await response
      .json()
      .catch(() => ({ error: `HTTP ${response.status}` }));
    throw new Error(
      typeof body.error === 'string'
        ? body.error
        : `Jute API request failed: ${response.status}`
    );
  }
  const body = (await response.json()) as { conversations: Conversation[] };
  return body.conversations;
}

export async function createConversation(
  fetcher: typeof fetch,
  agentId: string,
  title?: string
): Promise<ConversationDetail> {
  const response = await fetcher(`${API_BASE}/api/v1/conversations`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json'
    },
    body: JSON.stringify({ agentId, title })
  });
  if (!response.ok) {
    const body = await response
      .json()
      .catch(() => ({ error: `HTTP ${response.status}` }));
    throw new Error(
      typeof body.error === 'string'
        ? body.error
        : `Jute API request failed: ${response.status}`
    );
  }
  return response.json() as Promise<ConversationDetail>;
}

export async function getConversation(
  fetcher: typeof fetch,
  conversationId: string,
  agentId: string
): Promise<ConversationDetail> {
  return getJSON<ConversationDetail>(
    fetcher,
    `/api/v1/conversations/${encodeURIComponent(conversationId)}?agentId=${encodeURIComponent(agentId)}`
  );
}

export async function sendConversationTurn(
  fetcher: typeof fetch,
  conversationId: string,
  agentId: string,
  text: string
): Promise<ConversationDetail> {
  const response = await fetcher(
    `${API_BASE}/api/v1/conversations/${encodeURIComponent(conversationId)}/turns`,
    {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json'
      },
      body: JSON.stringify({ agentId, text })
    }
  );
  if (!response.ok) {
    const body = await response
      .json()
      .catch(() => ({ error: `HTTP ${response.status}` }));
    throw new Error(
      typeof body.error === 'string'
        ? body.error
        : `Jute API request failed: ${response.status}`
    );
  }
  return response.json() as Promise<ConversationDetail>;
}

export async function sendConversationTurnStream(
  fetcher: typeof fetch,
  conversationId: string,
  agentId: string,
  text: string,
  onEvent: (event: ConversationStreamEvent) => void
): Promise<void> {
  const response = await fetcher(
    `${API_BASE}/api/v1/conversations/${encodeURIComponent(conversationId)}/turns/stream`,
    {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json'
      },
      body: JSON.stringify({ agentId, text })
    }
  );
  if (!response.ok) {
    const body = await response
      .json()
      .catch(() => ({ error: `HTTP ${response.status}` }));
    throw new Error(
      typeof body.error === 'string'
        ? body.error
        : `Jute API request failed: ${response.status}`
    );
  }
  if (!response.body) {
    throw new Error('Jute streaming response was empty');
  }

  const reader = response.body.getReader();
  const decoder = new TextDecoder();
  let buffer = '';
  while (true) {
    const { done, value } = await reader.read();
    if (done) {
      break;
    }
    buffer += decoder.decode(value, { stream: true });
    const parts = buffer.split(/\n\n/);
    buffer = parts.pop() ?? '';
    for (const part of parts) {
      const event = parseSSEEvent(part);
      if (event) {
        onEvent(event);
      }
    }
  }
  buffer += decoder.decode();
  const event = parseSSEEvent(buffer);
  if (event) {
    onEvent(event);
  }
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
    const body = await response
      .json()
      .catch(() => ({ error: `HTTP ${response.status}` }));
    throw new Error(
      typeof body.error === 'string'
        ? body.error
        : `Jute API request failed: ${response.status}`
    );
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
    const body = await response
      .json()
      .catch(() => ({ error: `HTTP ${response.status}` }));
    throw new Error(
      typeof body.error === 'string'
        ? body.error
        : `Jute API request failed: ${response.status}`
    );
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
    const body = await response
      .json()
      .catch(() => ({ error: `HTTP ${response.status}` }));
    throw new Error(
      typeof body.error === 'string'
        ? body.error
        : `Jute API request failed: ${response.status}`
    );
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
    const body = await response
      .json()
      .catch(() => ({ error: `HTTP ${response.status}` }));
    throw new Error(
      typeof body.error === 'string'
        ? body.error
        : `Jute API request failed: ${response.status}`
    );
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
    const body = await response
      .json()
      .catch(() => ({ error: `HTTP ${response.status}` }));
    throw new Error(
      typeof body.error === 'string'
        ? body.error
        : `Jute API request failed: ${response.status}`
    );
  }
  return response.json() as Promise<WidgetLayout>;
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
    const body = await response
      .json()
      .catch(() => ({ error: `HTTP ${response.status}` }));
    throw new Error(
      typeof body.error === 'string'
        ? body.error
        : `Jute API request failed: ${response.status}`
    );
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
  return response.providers;
}

export function fallbackDashboard(issue?: UserFacingIssue): DashboardData {
  const config: PublicConfig = {
    home: {
      name: 'Jute Home',
      timezone: 'UTC',
      locale: 'en'
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
    sttProviderId: '',
    ttsProviderId: '',
    sttModelId: '',
    ttsModelId: '',
    ttsVoiceId: '',
    preferredAgentId: '',
    cloudOptIn: false,
    commandProvidersEnabled: false,
    followupWindowSeconds: 8,
    microphoneProfile: '',
    updatedAt: new Date().toISOString()
  };
}

async function postVoiceControl(
  fetcher: typeof fetch,
  path: string
): Promise<VoiceStatus> {
  const response = await fetcher(`${API_BASE}${path}`, {
    method: 'POST'
  });
  if (!response.ok) {
    const body = await response
      .json()
      .catch(() => ({ error: `HTTP ${response.status}` }));
    throw new Error(
      typeof body.error === 'string'
        ? body.error
        : `Jute API request failed: ${response.status}`
    );
  }
  return response.json() as Promise<VoiceStatus>;
}

async function getJSON<T>(fetcher: typeof fetch, path: string): Promise<T> {
  const response = await fetcher(`${API_BASE}${path}`);
  if (!response.ok) {
    throw new Error(`Jute API request failed: ${response.status}`);
  }
  return response.json() as Promise<T>;
}

export function parseSSEEvent(
  raw: string
): ConversationStreamEvent | undefined {
  const lines = raw.split(/\r?\n/);
  let type = '';
  const data: string[] = [];
  for (const line of lines) {
    if (line.startsWith('event:')) {
      type = line.slice('event:'.length).trim();
    } else if (line.startsWith('data:')) {
      data.push(line.slice('data:'.length).trim());
    }
  }
  if (!type || data.length === 0) {
    return undefined;
  }
  const payload = JSON.parse(data.join('\n')) as Record<string, unknown>;
  return { type, ...payload } as ConversationStreamEvent;
}
