import { expect, type Page, type Request, type Route } from '@playwright/test';

type JSONValue = Record<string, unknown> | unknown[];

type MockHubOptions = {
  agents?: 'available' | 'none';
  status?: 'ok' | 'degraded';
  layout?: 'default' | 'core-widgets';
  widgetState?:
    | 'ok'
    | 'empty'
    | 'loading'
    | 'unavailable'
    | 'error'
    | 'permission_required'
    | 'issue';
  chatFailure?: boolean;
};

type WriteRecord = {
  method: string;
  path: string;
  body: unknown;
};

type MockSSEWindow = Window &
  typeof globalThis & {
    __juteMockSSE: {
      emit(type: string, data: unknown): void;
      error(): void;
    };
  };

const now = '2026-06-17T09:00:00.000Z';

export async function createMockHub(page: Page, options: MockHubOptions = {}) {
  const state = mockState(options);
  const writes: WriteRecord[] = [];

  await page.addInitScript(() => {
    const sources: EventTarget[] = [];
    class MockEventSource extends EventTarget {
      static CONNECTING = 0;
      static OPEN = 1;
      static CLOSED = 2;
      url: string;
      readyState = MockEventSource.CONNECTING;
      onopen: ((event: Event) => void) | null = null;
      onerror: ((event: Event) => void) | null = null;
      constructor(url: string) {
        super();
        this.url = url;
        sources.push(this);
        setTimeout(() => {
          this.readyState = MockEventSource.OPEN;
          const event = new Event('open');
          this.dispatchEvent(event);
          this.onopen?.(event);
        }, 0);
      }
      close() {
        this.readyState = MockEventSource.CLOSED;
      }
    }
    const mockWindow = window as MockSSEWindow;
    mockWindow.EventSource = MockEventSource as unknown as typeof EventSource;
    mockWindow.__juteMockSSE = {
      emit(type: string, data: unknown) {
        for (const source of sources) {
          source.dispatchEvent(
            new MessageEvent(type, { data: JSON.stringify(data) })
          );
        }
      },
      error() {
        for (const source of sources) {
          const event = new Event('error');
          source.dispatchEvent(event);
          (source as EventSource).onerror?.(event);
        }
      }
    };
  });

  await page.route('**/api/v1/**', async (route) => {
    await handleAPI(route, state, writes, options);
  });
  await page.route('**/healthz', async (route) => json(route, { ok: true }));

  return {
    writes,
    state,
    emit: (type: string, data: unknown) =>
      page.evaluate(
        ({ eventType, payload }) =>
          (window as MockSSEWindow).__juteMockSSE.emit(eventType, payload),
        { eventType: type, payload: data }
      ),
    eventStreamError: () =>
      page.evaluate(() => (window as MockSSEWindow).__juteMockSSE.error()),
    expectWrite: async (method: string, path: string) =>
      expect
        .poll(() => writes.some((w) => w.method === method && w.path === path))
        .toBe(true),
    storageShouldNotContain: async (needle: string) => {
      const found = await page.evaluate(async (value) => {
        const haystack = [
          ...Object.values(localStorage),
          ...Object.values(sessionStorage),
          document.cookie
        ].join('\n');
        return haystack.includes(value);
      }, needle);
      expect(found).toBe(false);
    }
  };
}

async function handleAPI(
  route: Route,
  state: ReturnType<typeof mockState>,
  writes: WriteRecord[],
  options: MockHubOptions
) {
  const request = route.request();
  const url = new URL(request.url());
  const path = url.pathname;
  const method = request.method();

  if (method !== 'GET') {
    writes.push({
      method,
      path,
      body: await safeBody(request)
    });
  }

  if (path === '/api/v1/config' && method === 'GET')
    return json(route, state.config);
  if (path === '/api/v1/home' && method === 'GET')
    return json(route, state.home);
  if (path === '/api/v1/agents' && method === 'GET') {
    return json(route, { agents: state.agents });
  }
  if (path === '/api/v1/agents' && method === 'POST') {
    state.agents = [agent()];
    return json(route, state.agents[0], 201);
  }
  if (path.match(/^\/api\/v1\/agents\/[^/]+$/) && method === 'PATCH') {
    const patch = (await safeBody(route.request())) as { enabled?: boolean };
    state.agents = state.agents.map((a) =>
      a.id === 'house' ? { ...a, enabled: Boolean(patch.enabled) } : a
    );
    return json(route, state.agents[0]);
  }
  if (path.match(/^\/api\/v1\/agents\/[^/]+$/) && method === 'DELETE') {
    state.agents = [];
    return json(route, {}, 204);
  }
  if (path.endsWith('/refresh-card') && method === 'POST') {
    state.agents = state.agents.map((a) => ({ ...a, cardStatus: 'available' }));
    return json(route, state.agents[0]);
  }
  if (path === '/api/v1/widgets/layout' && method === 'GET')
    return json(route, state.layout);
  if (path === '/api/v1/widgets/layout' && method === 'PUT') {
    state.layout = (await safeBody(request)) as typeof state.layout;
    return json(route, state.layout);
  }
  if (path === '/api/v1/widgets/layout/reset' && method === 'POST') {
    state.layout = layout(options.widgetState ?? 'ok');
    return json(route, state.layout);
  }
  if (path === '/api/v1/widgets/layout/active-screen' && method === 'PATCH') {
    const body = (await safeBody(request)) as { screenId?: string };
    state.layout = { ...state.layout, activeScreenId: body.screenId };
    return json(route, state.layout);
  }
  if (path === '/api/v1/widgets/catalog' && method === 'GET') {
    return json(route, { widgets: state.catalog });
  }
  if (path === '/api/v1/voice/status' && method === 'GET')
    return json(route, state.voice);
  if (path === '/api/v1/voice/providers' && method === 'GET') {
    return json(route, { providers: state.voiceProviders });
  }
  if (path === '/api/v1/tts/voices' && method === 'GET') {
    return json(route, state.ttsVoices);
  }
  if (path === '/api/v1/voice/settings' && method === 'PATCH') {
    state.voice = { ...state.voice, ...((await safeBody(request)) as object) };
    return json(route, state.voice);
  }
  if (path === '/api/v1/voice/unmute' && method === 'POST') {
    state.voice = { ...state.voice, muted: false, state: 'wake_listening' };
    return json(route, state.voice);
  }
  if (path === '/api/v1/voice/mute' && method === 'POST') {
    state.voice = { ...state.voice, muted: true, state: 'muted' };
    return json(route, state.voice);
  }
  if (path === '/api/v1/voice/cancel' && method === 'POST') {
    return json(route, state.voice);
  }
  if (path === '/api/v1/voice/audio' && method === 'POST') {
    if (!state.voice.enabled || state.voice.muted) {
      return json(route, { error: 'voice is not listening' }, 409);
    }
    return json(route, {
      conversation: { id: 'browser-voice-audio-test' },
      followup: { active: false, turns: 1, maxTurns: 5 }
    });
  }
  if (path === '/api/v1/voice/transcripts/final' && method === 'POST') {
    if (!state.voice.enabled || state.voice.muted) {
      return json(route, { error: 'voice is not listening' }, 409);
    }
    return json(route, {
      conversation: { id: 'browser-voice-test' },
      followup: { active: false, turns: 1, maxTurns: 5 }
    });
  }
  if (path === '/api/v1/status' && method === 'GET')
    return json(route, state.status);
  if (path === '/api/v1/settings/household' && method === 'GET')
    return json(route, state.household);
  if (path === '/api/v1/settings/household' && method === 'PATCH') {
    state.household = (await safeBody(request)) as typeof state.household;
    state.config = { ...state.config, home: state.household.home };
    return json(route, state.household);
  }
  if (path === '/api/v1/settings/rooms' && method === 'GET') {
    return json(route, { rooms: state.home.rooms });
  }
  if (path === '/api/v1/settings/rooms' && method === 'PUT') {
    const body = (await safeBody(request)) as {
      rooms: typeof state.home.rooms;
    };
    state.home = { ...state.home, rooms: body.rooms };
    return json(route, body);
  }
  if (path === '/api/v1/settings/tiles' && method === 'GET') {
    return json(route, { tiles: state.home.tiles });
  }
  if (path === '/api/v1/settings/tiles' && method === 'PUT') {
    const body = (await safeBody(request)) as {
      tiles: typeof state.home.tiles;
    };
    state.home = { ...state.home, tiles: body.tiles };
    return json(route, body);
  }
  if (path === '/api/v1/backgrounds' && method === 'GET')
    return json(route, { images: [] });
  if (path === '/api/v1/settings/connections' && method === 'GET') {
    return json(route, { connections: state.connections });
  }
  if (path === '/api/v1/settings/connections' && method === 'PUT') {
    const connection = (await safeBody(
      request
    )) as (typeof state.connections)[number];
    state.connections = [
      ...state.connections.filter((item) => item.id !== connection.id),
      connection
    ];
    return json(route, connection);
  }
  if (path === '/api/v1/settings/connection-kinds' && method === 'GET') {
    return json(route, { kinds: state.connectionKinds });
  }
  if (path === '/api/v1/integrations/spotify/callback' && method === 'GET') {
    if (url.searchParams.get('code') === 'bad') {
      return json(route, { error: 'Spotify account could not be linked' }, 400);
    }
    return json(route, { status: 'linked', connectionId: 'spotify-main' });
  }
  if (
    path === '/api/v1/integrations/spotify/web-playback-token' &&
    method === 'GET'
  ) {
    return json(route, {
      accessToken: 'mock-web-playback-token',
      expiresAt: Date.now() + 3_600_000,
      scope: 'streaming user-read-playback-state'
    });
  }
  if (
    path === '/api/v1/integrations/apple-music/music-kit-token' &&
    method === 'GET'
  ) {
    return json(route, {
      developerToken: 'mock-music-kit-token',
      userToken: 'mock-apple-music-user-token'
    });
  }
  if (
    path === '/api/v1/integrations/apple-music/user-token' &&
    method === 'POST'
  ) {
    return json(route, { status: 'linked', connectionId: 'apple-music-main' });
  }
  if (
    path.match(/^\/api\/v1\/widgets\/[^/]+\/actions\/[^/]+$/) &&
    method === 'POST'
  ) {
    const action = path.split('/').pop();
    return json(route, {
      status: 'accepted',
      action,
      results: [
        {
          id: 'mock-result',
          name: 'Mock Result',
          uri: 'spotify:track:mock-result'
        }
      ]
    });
  }
  if (path.startsWith('/api/v1/proxy/agents/') && method === 'POST') {
    return handleA2A(route, options.chatFailure);
  }

  return json(
    route,
    { error: `Unexpected mocked hub request: ${method} ${path}` },
    599
  );
}

async function handleA2A(route: Route, fail = false) {
  const body = (await safeBody(route.request())) as {
    id?: unknown;
    method?: string;
  };
  if (body.method === 'ListTasks') {
    return json(route, { jsonrpc: '2.0', id: body.id, result: { tasks: [] } });
  }
  if (body.method === 'GetTask') {
    return json(route, { jsonrpc: '2.0', id: body.id, result: { tasks: [] } });
  }
  if (body.method === 'SendMessage' || body.method === 'message/send') {
    if (fail) {
      return json(route, {
        jsonrpc: '2.0',
        id: body.id,
        error: {
          code: -32000,
          message: 'agent credentials failed: env:AGENT_TOKEN'
        }
      });
    }
    return json(route, {
      jsonrpc: '2.0',
      id: body.id,
      result: {
        message: {
          messageId: 'msg-assistant',
          role: 'ROLE_AGENT',
          parts: [{ text: 'Mock A2A reply from the local hub.' }]
        }
      }
    });
  }
  if (
    body.method === 'SendStreamingMessage' ||
    body.method === 'message/stream'
  ) {
    return json(route, {
      jsonrpc: '2.0',
      id: body.id,
      result: {
        task: {
          id: 'task-1',
          contextId: 'ctx-1',
          status: { state: 'TASK_STATE_COMPLETED' },
          artifacts: [
            { parts: [{ text: 'Mock A2A reply from the local hub.' }] }
          ]
        }
      }
    });
  }
  return json(route, {
    jsonrpc: '2.0',
    id: body.id,
    error: { code: -32601, message: 'method not found' }
  });
}

async function safeBody(request: Request) {
  try {
    return request.postDataJSON();
  } catch {
    return request.postData() ?? '';
  }
}

function mockState(options: MockHubOptions) {
  const agents = options.agents === 'none' ? [] : [agent()];
  const status = appStatus(options.status ?? 'ok', agents.length);
  const widgetLayout =
    options.layout === 'core-widgets'
      ? coreWidgetsLayout(options.widgetState ?? 'ok')
      : layout(options.widgetState ?? 'ok');
  return {
    config: config(agents),
    home: home(),
    agents,
    layout: widgetLayout,
    catalog: catalog(),
    voice: voice(),
    voiceProviders: voiceProviders(),
    ttsVoices: ttsVoices(),
    status,
    household: {
      home: { name: 'Jute Test Home' },
      display: config(agents).display,
      setup: { complete: true, missing: [] }
    },
    connections: [
      {
        id: 'spotify-main',
        kind: 'spotify',
        name: 'Spotify',
        settings: { auth_type: 'user_app_pkce', client_id: 'client-id' },
        secretRefs: { refresh_token: 'secret:spotify-refresh' },
        enabled: true
      },
      {
        id: 'apple-music-main',
        kind: 'apple-music',
        name: 'Apple Music',
        settings: { auth_type: 'music_kit' },
        secretRefs: { user_token: 'secret:apple-music-user' },
        enabled: true
      }
    ],
    connectionKinds: [
      {
        kind: 'spotify',
        displayName: 'Spotify',
        description: 'Spotify playback',
        fields: []
      },
      {
        kind: 'philips-hue',
        displayName: 'Philips Hue',
        description: 'Hue bridge',
        fields: [
          {
            id: 'bridgeHost',
            type: 'string',
            label: 'Bridge host',
            required: false,
            secret: false
          },
          {
            id: 'username',
            type: 'string',
            label: 'Username secret reference',
            required: false,
            secret: true
          }
        ]
      },
      {
        kind: 'apple-music',
        displayName: 'Apple Music',
        description: 'Apple Music playback',
        fields: []
      }
    ]
  };
}

function config(agents: ReturnType<typeof agent>[]) {
  return {
    home: { name: 'Jute Test Home' },
    display: {
      theme: 'jute-mono',
      colorMode: 'light',
      themeId: 'jute-mono',
      density: 'comfortable',
      motion: 'none',
      background: {
        kind: 'theme',
        value: '',
        fit: 'cover',
        position: 'center',
        overlay: 'none'
      },
      widgetChrome: { default: 'solid' },
      accentColor: 'neutral',
      idleMode: 'none'
    },
    agents,
    rooms: home().rooms,
    tiles: home().tiles
  };
}

function home() {
  return {
    generatedAt: now,
    home: { name: 'Jute Test Home' },
    rooms: [{ id: 'kitchen', name: 'Kitchen', summary: 'Ready', status: 'ok' }],
    tiles: [
      {
        id: 'front-door',
        kind: 'status',
        label: 'Front Door',
        value: 'Locked',
        detail: 'Closed'
      }
    ]
  };
}

function agent() {
  return {
    id: 'house',
    name: 'House Agent',
    description: 'Local test agent',
    cardUrl: 'http://127.0.0.1:9797/.well-known/agent-card.json',
    endpointUrl: 'http://127.0.0.1:9797/invoke',
    selectedEndpointUrl: 'http://127.0.0.1:9797/invoke',
    protocolBinding: 'JSONRPC',
    selectedProtocolBinding: 'JSONRPC',
    selectedProtocolVersion: '1.0',
    enabled: true,
    capabilities: ['chat'],
    mcpScopes: ['dashboard:read'],
    authConfigured: false,
    authAvailable: true,
    cardStatus: 'available',
    streaming: false,
    dashboardContextSupported: true,
    skills: [{ id: 'chat', name: 'Local chat', description: 'Test replies' }]
  };
}

function voice() {
  return {
    enabled: true,
    muted: false,
    state: 'wake_listening',
    serviceStatus: 'ready',
    deviceProfileId: 'test-display',
    wakeWordModelId: 'local',
    wakeWordPhrase: 'Hey Jute',
    wakeSensitivity: 0.5,
    sttProviderId: 'builtin',
    ttsProviderId: 'builtin',
    sttModelId: 'tiny',
    ttsModelId: 'local',
    ttsVoiceId: 'test',
    ttsEnabled: true,
    ttsLocale: 'en',
    ttsSpeed: 1,
    ttsVolume: 1,
    preferredAgentId: 'house',
    cloudOptIn: false,
    commandProvidersEnabled: false,
    followupWindowSeconds: 8,
    microphoneProfile: 'default',
    updatedAt: now
  };
}

function voiceProviders() {
  return [
    {
      id: 'local-wake',
      name: 'Local Wake',
      version: '1.0.0',
      kind: 'wake-word',
      transportType: 'command',
      capabilities: {
        streaming: false,
        partialTranscripts: false,
        offline: true
      },
      wakeWord: {
        defaultModelId: 'hey-jute',
        phrase: 'Hey Jute',
        sensitivity: 0.55,
        models: [{ id: 'hey-jute', phrase: 'Hey Jute', sensitivity: 0.55 }]
      },
      healthStatus: 'available',
      updatedAt: now
    },
    {
      id: 'local-stt',
      name: 'Local STT',
      version: '1.0.0',
      kind: 'stt',
      transportType: 'command',
      capabilities: {
        streaming: false,
        partialTranscripts: false,
        offline: true,
        languages: ['en-GB'],
        inputFormats: ['audio/wav']
      },
      healthStatus: 'available',
      updatedAt: now
    },
    {
      id: 'local-tts',
      name: 'Local TTS',
      version: '1.0.0',
      kind: 'tts',
      transportType: 'command',
      capabilities: {
        streaming: false,
        partialTranscripts: false,
        offline: true
      },
      healthStatus: 'available',
      updatedAt: now
    }
  ];
}

function ttsVoices() {
  return {
    providerId: 'local-tts',
    providerName: 'Local TTS',
    healthStatus: 'available',
    setupStatus: 'available',
    selectedVoiceId: 'amy',
    selectedModelId: 'local',
    locale: 'en-GB',
    speed: 1,
    volume: 1,
    cloudProvider: false,
    voices: [{ id: 'amy', label: 'Amy', locale: 'en-GB', modelId: 'local' }]
  };
}

function appStatus(status: 'ok' | 'degraded', agentCount: number) {
  return {
    status,
    version: 'test',
    startedAt: now,
    setup: {
      complete: status === 'ok',
      missing: status === 'ok' ? [] : ['event stream']
    },
    config: { hasBootstrapConfig: true, writableYaml: true },
    eventStream: { available: status === 'ok' },
    mcp: {
      enabled: false,
      serviceStatus: 'disabled',
      transport: 'streamable-http',
      listenAddress: '127.0.0.1',
      path: '/mcp',
      authMode: 'none',
      allowLan: false
    },
    agents: {
      total: agentCount,
      enabled: agentCount,
      disabled: 0,
      available: agentCount,
      unavailable: 0,
      dashboardContextSupported: agentCount,
      mcpScoped: agentCount
    },
    voice: { enabled: true, serviceStatus: 'ready', state: 'idle' }
  };
}

function catalog() {
  return [
    {
      kind: 'weather',
      name: 'Weather',
      description: 'Current weather',
      defaultTitle: 'Weather',
      defaultW: 3,
      defaultH: 2,
      minW: 1,
      minH: 1,
      defaultSize: 'wide',
      overflow: 'clip',
      allowMultiple: true,
      settingsSchema: [
        {
          id: 'locationName',
          label: 'Location',
          type: 'string',
          default: 'Kitchen'
        }
      ]
    },
    {
      kind: 'spotify',
      name: 'Spotify',
      description: 'Playback',
      defaultTitle: 'Spotify',
      defaultW: 3,
      defaultH: 2,
      minW: 1,
      minH: 1,
      defaultSize: 'wide',
      overflow: 'clip',
      allowMultiple: true,
      connectionRequirements: [
        {
          slot: 'account',
          kind: 'spotify',
          displayName: 'Spotify account',
          required: true
        }
      ]
    },
    {
      kind: 'philips-hue',
      name: 'Philips Hue',
      description: 'Hue lights',
      defaultTitle: 'Hue',
      defaultW: 3,
      defaultH: 2,
      minW: 1,
      minH: 1,
      defaultSize: 'wide',
      overflow: 'clip',
      allowMultiple: true,
      connectionRequirements: [
        {
          slot: 'bridge',
          kind: 'philips-hue',
          displayName: 'Hue bridge',
          required: true
        }
      ]
    }
  ];
}

function layout(state: NonNullable<MockHubOptions['widgetState']>) {
  const widgets = [
    widget('date-time', 'date-time', 'Date & Time', 0, 0, 3, 2, {
      status: 'available'
    }),
    widget('weather', 'weather', 'Weather', 3, 0, 3, 2, weatherData(state)),
    widget('chat-history', 'chat-history', 'Chat History', 0, 2, 3, 2, {
      status: 'available'
    })
  ];
  return {
    profileId: 'test-profile',
    schemaVersion: 3,
    defaultScreenId: 'main',
    activeScreenId: 'main',
    defaultVariant: 'desktop',
    variants: variants(widgets),
    widgets,
    screens: [
      {
        id: 'main',
        label: 'Main',
        defaultVariant: 'desktop',
        variants: variants(widgets),
        widgets
      }
    ]
  };
}

function coreWidgetsLayout(state: NonNullable<MockHubOptions['widgetState']>) {
  const widgets = [
    widget('date-time', 'date-time', 'Date & Time', 0, 0, 3, 2, {
      status: 'available'
    }),
    widget('weather', 'weather', 'Weather', 3, 0, 3, 2, weatherData(state)),
    widget('rss', 'rss', 'Headlines', 0, 2, 3, 2, okWidgetData('rss')),
    widget(
      'markets',
      'markets',
      'Markets',
      3,
      2,
      3,
      2,
      okWidgetData('markets')
    ),
    withConnection(
      widget(
        'spotify',
        'spotify',
        'Spotify',
        0,
        4,
        3,
        2,
        okWidgetData('spotify')
      ),
      'spotify-main'
    ),
    withConnection(
      widget(
        'apple-music',
        'apple-music',
        'Apple Music',
        3,
        4,
        3,
        2,
        okWidgetData('apple-music')
      ),
      'apple-music-main'
    ),
    widget(
      'philips-hue',
      'philips-hue',
      'Hue',
      0,
      6,
      3,
      2,
      okWidgetData('philips-hue')
    ),
    widget(
      'zigbee2mqtt',
      'zigbee2mqtt',
      'Zigbee',
      3,
      6,
      3,
      2,
      okWidgetData('zigbee2mqtt')
    ),
    widget('chat-history', 'chat-history', 'Saved Chats', 0, 8, 3, 2, {
      status: 'available'
    }),
    widget(
      'timers-alarms',
      'timers-alarms',
      'Timers',
      3,
      8,
      3,
      2,
      okWidgetData('timers-alarms')
    ),
    widget(
      'calendar',
      'calendar',
      'Calendar',
      0,
      10,
      3,
      2,
      okWidgetData('calendar')
    )
  ];
  return {
    profileId: 'test-profile',
    schemaVersion: 3,
    defaultScreenId: 'main',
    activeScreenId: 'main',
    defaultVariant: 'desktop',
    variants: variants(widgets),
    widgets,
    screens: [
      {
        id: 'main',
        label: 'Main',
        defaultVariant: 'desktop',
        variants: variants(widgets),
        widgets
      }
    ]
  };
}

function variants(widgets: ReturnType<typeof widget>[]) {
  const rowCount = Math.max(4, ...widgets.map((item) => item.y + item.h));
  return [
    {
      id: 'phone',
      label: 'Phone',
      minWidth: 0,
      columns: 1,
      rows: Math.max(8, widgets.length * 2),
      gap: 12,
      placements: placements(widgets, 1)
    },
    {
      id: 'tablet-portrait',
      label: 'Tablet',
      minWidth: 641,
      orientation: 'portrait',
      columns: 6,
      rows: rowCount,
      gap: 12,
      placements: placements(widgets, 6)
    },
    {
      id: 'tablet-landscape',
      label: 'Tablet wide',
      minWidth: 768,
      orientation: 'landscape',
      columns: 6,
      rows: rowCount,
      gap: 12,
      placements: placements(widgets, 6)
    },
    {
      id: 'desktop',
      label: 'Desktop',
      minWidth: 1024,
      columns: 6,
      rows: rowCount,
      gap: 12,
      placements: placements(widgets, 6)
    },
    {
      id: 'wall',
      label: 'Wall',
      minWidth: 1600,
      orientation: 'landscape',
      columns: 6,
      rows: rowCount,
      gap: 12,
      placements: placements(widgets, 6)
    }
  ];
}

function placements(widgets: ReturnType<typeof widget>[], columns: number) {
  return Object.fromEntries(
    widgets.map((item, index) => [
      item.id,
      columns === 1
        ? { x: 0, y: index * 2, w: 1, h: 2 }
        : { x: item.x, y: item.y, w: item.w, h: item.h }
    ])
  );
}

function widget(
  id: string,
  kind: string,
  title: string,
  x: number,
  y: number,
  w: number,
  h: number,
  data: unknown
) {
  return {
    id,
    kind,
    title,
    x,
    y,
    w,
    h,
    minW: 1,
    minH: 1,
    size: 'wide',
    settings: {},
    visible: true,
    mode: 'ui',
    data
  };
}

function withConnection<T extends ReturnType<typeof widget>>(
  item: T,
  connectionId: string
) {
  return {
    ...item,
    connectionRefs: { account: connectionId }
  };
}

function okWidgetData(kind: string) {
  switch (kind) {
    case 'rss':
      return {
        status: 'ok',
        data: [
          {
            feedName: 'Home',
            items: [
              {
                title: 'Household automations are calm today',
                link: 'https://example.com/home',
                pubDate: now
              }
            ]
          }
        ]
      };
    case 'markets':
      return {
        status: 'ok',
        data: [
          {
            symbol: 'AAPL',
            name: 'Apple',
            price: 192.4,
            currency: 'USD',
            change: 1.2,
            changePercent: 0.62
          }
        ]
      };
    case 'spotify':
    case 'apple-music':
      return {
        status: 'ok',
        data: {
          track_title: 'Home Mode',
          artist_name: 'Jute Dash',
          is_playing: true,
          volume: 48,
          progress_ms: 64000,
          duration_ms: 180000,
          track_uri: `${kind}:track:home-mode`,
          top_albums: [
            {
              id: `${kind}-album-1`,
              name: 'Morning Loop',
              artist_name: 'Jute Dash',
              uri: `${kind}:album:morning-loop`,
              album_art_url: ''
            }
          ]
        }
      };
    case 'philips-hue':
      return {
        status: 'ok',
        data: {
          devices: [
            { id: 'kitchen-light', name: 'Kitchen light', state: true },
            { id: 'hall-light', name: 'Hall light', state: false }
          ]
        }
      };
    case 'zigbee2mqtt':
      return {
        status: 'ok',
        data: {
          devices: [
            {
              id: 'desk-lamp',
              name: 'Desk lamp',
              type: 'light',
              state: true
            },
            {
              id: 'entry-temp',
              name: 'Entry temperature',
              type: 'sensor',
              value: '20.1 C'
            }
          ]
        }
      };
    case 'timers-alarms':
      return {
        status: 'ok',
        data: {
          active: [
            {
              id: 'timer-tea',
              kind: 'timer',
              label: 'Tea',
              status: 'active',
              dueAt: '2099-06-17T09:05:00.000Z',
              durationSeconds: 300,
              remainingSeconds: 305,
              sound: 'chime'
            },
            {
              id: 'alarm-school-run',
              kind: 'alarm',
              label: 'School run',
              status: 'active',
              dueAt: '2099-06-17T10:00:00.000Z',
              time: '07:30',
              weekdays: [1, 2, 3, 4, 5],
              remainingSeconds: 3600,
              recurring: true,
              sound: 'bell'
            }
          ],
          ringing: [],
          notificationSound: 'chime',
          defaultSnoozeMins: 9,
          generatedAt: now,
          timezone: 'Europe/London'
        }
      };
    case 'calendar':
      return {
        status: 'ok',
        data: {
          events: [
            {
              id: 'school-assembly',
              uid: 'school-assembly',
              title: 'School assembly',
              calendar: 'Family',
              start: '2099-06-17T09:45:00.000Z',
              end: '2099-06-17T10:30:00.000Z',
              allDay: false,
              location: 'Hall',
              source: 'ics'
            }
          ],
          nextEvent: {
            id: 'school-assembly',
            title: 'School assembly',
            calendar: 'Family',
            start: '2099-06-17T09:45:00.000Z',
            end: '2099-06-17T10:30:00.000Z',
            location: 'Hall'
          },
          alerts: [],
          ringing: [],
          ringingCount: 0,
          alertLeadMinutes: 10,
          defaultSnoozeMins: 9,
          notificationSound: 'chime',
          generatedAt: now,
          source: 'ics'
        }
      };
    default:
      return { status: 'ok', data: {} };
  }
}

function weatherData(state: NonNullable<MockHubOptions['widgetState']>) {
  if (state === 'empty') return undefined;
  if (state === 'issue') {
    return {
      status: 'unavailable',
      issue: {
        code: 'connection.spotify.missing',
        severity: 'warning',
        title: 'Connection needed',
        message: 'Choose a shared Adapter Connection for this widget.',
        action: { label: 'Open settings', target: 'settings' }
      }
    };
  }
  if (state !== 'ok') return { status: state };
  return {
    status: 'available',
    data: {
      locationName: 'Kitchen',
      temperature: 21,
      temperatureUnit: 'celsius',
      apparentTemperature: 20,
      condition: 'Clear',
      icon: 'sun',
      weatherCode: 0,
      humidity: 42,
      windSpeed: 8,
      windSpeedUnit: 'kmh',
      sunrise: '2026-06-17T04:30:00.000Z',
      sunset: '2026-06-17T21:20:00.000Z',
      isDay: true,
      updatedAt: now,
      source: 'Open-Meteo'
    }
  };
}

async function json(
  route: Route,
  body: JSONValue | Record<string, unknown>,
  status = 200
) {
  await route.fulfill({
    status,
    contentType: 'application/json',
    body: status === 204 ? '' : JSON.stringify(body)
  });
}
