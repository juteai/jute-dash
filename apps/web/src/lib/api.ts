import type {
  Agent,
  DashboardData,
  HomeState,
  MessageResponse,
  PublicConfig,
  UserFacingIssue,
  VoiceProvider,
  VoiceStatus,
  WidgetCatalogItem,
  WidgetLayout
} from '$lib/types';

const API_BASE = import.meta.env.VITE_JUTE_API_URL ?? 'http://127.0.0.1:8787';

export async function getDashboard(fetcher: typeof fetch): Promise<DashboardData> {
  const [config, home, agentResponse, layout, voice] = await Promise.all([
    getJSON<PublicConfig>(fetcher, '/api/v1/config'),
    getJSON<HomeState>(fetcher, '/api/v1/home'),
    getJSON<{ agents: Agent[] }>(fetcher, '/api/v1/agents'),
    getJSON<WidgetLayout>(fetcher, '/api/v1/widgets/layout'),
    getJSON<VoiceStatus>(fetcher, '/api/v1/voice/status')
  ]);

  return {
    config,
    home,
    agents: agentResponse.agents,
    layout,
    voice,
    connectionState: 'connected',
    stale: false,
    hubUrl: API_BASE,
    loadedAt: new Date().toISOString()
  };
}

export async function sendMessage(
  fetcher: typeof fetch,
  agentId: string,
  text: string
): Promise<MessageResponse> {
  const response = await fetcher(`${API_BASE}/api/v1/messages`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json'
    },
    body: JSON.stringify({ agentId, text })
  });
  if (!response.ok) {
    const body = await response.json().catch(() => ({ error: `HTTP ${response.status}` }));
    throw new Error(typeof body.error === 'string' ? body.error : `Jute API request failed: ${response.status}`);
  }
  return response.json() as Promise<MessageResponse>;
}

export async function getWidgetCatalog(fetcher: typeof fetch): Promise<WidgetCatalogItem[]> {
  const response = await getJSON<{ widgets: WidgetCatalogItem[] }>(fetcher, '/api/v1/widgets/catalog');
  return response.widgets;
}

export async function saveWidgetLayout(fetcher: typeof fetch, layout: WidgetLayout): Promise<WidgetLayout> {
  const response = await fetcher(`${API_BASE}/api/v1/widgets/layout`, {
    method: 'PUT',
    headers: {
      'Content-Type': 'application/json'
    },
    body: JSON.stringify(layout)
  });
  if (!response.ok) {
    const body = await response.json().catch(() => ({ error: `HTTP ${response.status}` }));
    throw new Error(typeof body.error === 'string' ? body.error : `Jute API request failed: ${response.status}`);
  }
  return response.json() as Promise<WidgetLayout>;
}

export async function resetWidgetLayout(fetcher: typeof fetch, profileId: string): Promise<WidgetLayout> {
  const suffix = profileId ? `?profileId=${encodeURIComponent(profileId)}` : '';
  const response = await fetcher(`${API_BASE}/api/v1/widgets/layout/reset${suffix}`, {
    method: 'POST'
  });
  if (!response.ok) {
    const body = await response.json().catch(() => ({ error: `HTTP ${response.status}` }));
    throw new Error(typeof body.error === 'string' ? body.error : `Jute API request failed: ${response.status}`);
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

export async function getVoiceProviders(fetcher: typeof fetch): Promise<VoiceProvider[]> {
  const response = await getJSON<{ providers: VoiceProvider[] }>(fetcher, '/api/v1/voice/providers');
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
      accentColor: 'teal',
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
      tiles: [],
      weather: {
        locationName: 'London',
        temperature: null,
        temperatureUnit: '°C',
        apparentTemperature: null,
        condition: 'Weather unavailable',
        icon: 'cloud',
        weatherCode: null,
        humidity: null,
        windSpeed: null,
        windSpeedUnit: 'km/h',
        sunrise: '',
        sunset: '',
        isDay: null,
        updatedAt: '',
        source: 'open-meteo',
        status: 'unavailable'
      }
    },
    agents: [],
    layout: fallbackLayout(),
    voice: fallbackVoiceStatus(),
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

export function hubURL() {
  return API_BASE;
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

async function postVoiceControl(fetcher: typeof fetch, path: string): Promise<VoiceStatus> {
  const response = await fetcher(`${API_BASE}${path}`, {
    method: 'POST'
  });
  if (!response.ok) {
    const body = await response.json().catch(() => ({ error: `HTTP ${response.status}` }));
    throw new Error(typeof body.error === 'string' ? body.error : `Jute API request failed: ${response.status}`);
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
