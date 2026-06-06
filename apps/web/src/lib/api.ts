import {
  Client,
  JsonRpcTransportFactory,
  type Transport
} from '@a2a-js/sdk/client';
import {
  Role,
  TaskState,
  taskStateToJSON,
  type AgentCard,
  type Message as A2AMessage,
  type Part as A2APart,
  type Task as A2ATask
} from '@a2a-js/sdk';
import type {
  Agent,
  AppStatus,
  BackgroundImage,
  Conversation,
  ConversationDetail,
  ConversationStreamEvent,
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
  VoiceStatus,
  WidgetCatalogItem,
  WidgetLayout
} from '$lib/types';

interface LegacyMessage {
  id?: string;
  messageId?: string;
  role: string;
  text?: string;
  parts?: Array<{ kind: string; text?: string }>;
}

interface LegacyTask {
  messages?: LegacyMessage[];
  text?: string;
  updatedAt?: string;
}

const API_BASE = import.meta.env.VITE_JUTE_API_URL ?? 'http://127.0.0.1:8787';
const proxyAgentCard = {
  capabilities: { streaming: true }
} as AgentCard;

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

async function getTransport(
  fetcher: typeof fetch,
  agentId: string
): Promise<Transport> {
  return new JsonRpcTransportFactory({ fetchImpl: fetcher }).create(
    `${API_BASE}/api/v1/proxy/agents/${encodeURIComponent(agentId)}`,
    proxyAgentCard
  );
}

async function getClient(fetcher: typeof fetch, agentId: string) {
  const transport = await getTransport(fetcher, agentId);
  return new Client(transport, proxyAgentCard);
}

function textFromParts(parts: A2APart[] | undefined): string {
  return (parts ?? [])
    .map((part) => (part.content?.$case === 'text' ? part.content.value : ''))
    .join('');
}

function statusFromTask(task: A2ATask): string {
  return task.status
    ? taskStateToJSON(task.status.state)
        .replace(/^TASK_STATE_/, '')
        .toLowerCase()
    : 'completed';
}

function isTerminalTaskState(state: TaskState): boolean {
  return [
    TaskState.TASK_STATE_COMPLETED,
    TaskState.TASK_STATE_FAILED,
    TaskState.TASK_STATE_CANCELED,
    TaskState.TASK_STATE_REJECTED
  ].includes(state);
}

function newUserMessage(conversationId: string, text: string): A2AMessage {
  return {
    messageId: crypto.randomUUID(),
    contextId: conversationId,
    taskId: '',
    role: Role.ROLE_USER,
    parts: [
      {
        content: { $case: 'text', value: text },
        metadata: undefined,
        filename: '',
        mediaType: 'text/plain'
      }
    ],
    metadata: undefined,
    extensions: [],
    referenceTaskIds: []
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
  try {
    const client = await getClient(fetcher, agentId);
    const result = await client.listTasks({
      tenant: '',
      contextId: '',
      status: TaskState.TASK_STATE_UNSPECIFIED,
      pageSize: 50,
      pageToken: '',
      statusTimestampAfter: undefined
    });
    const tasks = result.tasks;

    const byContext: Record<string, Conversation> = {};
    for (const task of tasks) {
      const contextId = task.contextId || task.id;
      if (!contextId) continue;

      let title = '';
      const history = (task.history ||
        (task as unknown as LegacyTask).messages) as
        | Array<A2AMessage | LegacyMessage>
        | undefined;
      if (history && history.length > 0) {
        const firstUser = history.find(
          (message) =>
            message.role === Role.ROLE_USER || message.role === 'user'
        );
        if (firstUser && typeof firstUser.role === 'number') {
          title = textFromParts((firstUser as A2AMessage).parts);
        } else if (firstUser) {
          title = firstUser.text ?? '';
        }
      }
      if (!title) {
        title = (task as unknown as LegacyTask).text || 'Conversation';
      }

      const updatedAt =
        task.status?.timestamp ||
        (task as unknown as LegacyTask).updatedAt ||
        new Date().toISOString();
      const conversation: Conversation = byContext[contextId] || {
        id: contextId,
        agentId,
        title,
        status: statusFromTask(task),
        a2aContextId: contextId,
        latestTaskId: task.id || '',
        createdAt: updatedAt,
        updatedAt: updatedAt
      };

      if (updatedAt && updatedAt >= conversation.updatedAt) {
        conversation.updatedAt = updatedAt;
        conversation.latestTaskId = task.id || '';
        conversation.status = statusFromTask(task);
      }
      byContext[contextId] = conversation;
    }

    return Object.values(byContext);
  } catch (err) {
    console.error('Failed to list conversations from proxy:', err);
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
}

export async function createConversation(
  fetcher: typeof fetch,
  agentId: string,
  title?: string
): Promise<ConversationDetail> {
  const contextId = 'ctx-' + Math.random().toString(36).substring(7);
  const now = new Date().toISOString();
  return {
    conversation: {
      id: contextId,
      agentId,
      title: title || 'New Conversation',
      status: 'idle',
      a2aContextId: contextId,
      latestTaskId: '',
      createdAt: now,
      updatedAt: now
    },
    messages: []
  };
}

export async function getConversation(
  fetcher: typeof fetch,
  conversationId: string,
  agentId: string
): Promise<ConversationDetail> {
  const client = await getClient(fetcher, agentId);
  const result = await client.listTasks({
    tenant: '',
    contextId: conversationId,
    status: TaskState.TASK_STATE_UNSPECIFIED,
    pageSize: 50,
    pageToken: '',
    statusTimestampAfter: undefined
  });
  const tasks = result.tasks;

  const detail: ConversationDetail = {
    conversation: {
      id: conversationId,
      agentId,
      title: 'Conversation',
      status: 'idle',
      a2aContextId: conversationId,
      latestTaskId: '',
      createdAt: new Date().toISOString(),
      updatedAt: new Date().toISOString()
    },
    messages: []
  };

  tasks.sort((a, b) => {
    const timeA =
      a.status?.timestamp || (a as unknown as LegacyTask).updatedAt || '';
    const timeB =
      b.status?.timestamp || (b as unknown as LegacyTask).updatedAt || '';
    return timeA.localeCompare(timeB);
  });

  for (const task of tasks) {
    let record = task;
    if (task.id) {
      try {
        const fullTask = await client.getTask({
          tenant: '',
          id: task.id,
          historyLength: 50
        });
        if (fullTask) {
          record = fullTask;
        }
      } catch (e) {
        console.warn('Failed to get full task details for', task.id, e);
      }
    }

    const recordUpdatedAt =
      record.status?.timestamp ||
      (record as unknown as LegacyTask).updatedAt ||
      detail.conversation.updatedAt;
    detail.conversation.latestTaskId = record.id || '';
    detail.conversation.status = statusFromTask(record);
    detail.conversation.updatedAt = recordUpdatedAt;

    const history = (record.history ||
      (record as unknown as LegacyTask).messages) as
      | Array<A2AMessage | LegacyMessage>
      | undefined;
    if (history) {
      for (const msg of history) {
        const isA2AMessage = 'messageId' in msg && typeof msg.role === 'number';
        const legacyMessage = msg as LegacyMessage;
        const content = isA2AMessage
          ? textFromParts(msg.parts as A2APart[])
          : legacyMessage.text || '';
        detail.messages.push({
          id: legacyMessage.id || msg.messageId || Math.random().toString(),
          conversationId,
          agentId,
          role:
            msg.role === Role.ROLE_USER || msg.role === 'user'
              ? 'user'
              : 'assistant',
          content,
          status: 'sent',
          a2aMessageId: legacyMessage.id || msg.messageId || '',
          a2aTaskId: record.id || '',
          createdAt: recordUpdatedAt,
          updatedAt: recordUpdatedAt
        });
      }
    }
  }

  const firstUser = detail.messages.find((m) => m.role === 'user');
  if (firstUser && firstUser.content) {
    detail.conversation.title = firstUser.content;
  }

  return detail;
}

export async function sendConversationTurn(
  fetcher: typeof fetch,
  conversationId: string,
  agentId: string,
  text: string
): Promise<ConversationDetail> {
  const client = await getClient(fetcher, agentId);
  await client.sendMessage({
    tenant: '',
    message: newUserMessage(conversationId, text),
    configuration: undefined,
    metadata: undefined
  });

  return getConversation(fetcher, conversationId, agentId);
}

export async function sendConversationTurnStream(
  fetcher: typeof fetch,
  conversationId: string,
  agentId: string,
  text: string,
  onEvent: (event: ConversationStreamEvent) => void
): Promise<void> {
  onEvent({
    type: 'turn_started',
    conversationId,
    agentId
  });

  try {
    const client = await getClient(fetcher, agentId);

    for await (const event of client.sendMessageStream({
      tenant: '',
      message: newUserMessage(conversationId, text),
      configuration: undefined,
      metadata: undefined
    })) {
      const payload = event.payload;
      if (payload?.$case === 'message') {
        const content = textFromParts(payload.value.parts);
        if (content) {
          onEvent({
            type: 'assistant_delta',
            conversationId,
            agentId,
            text: content,
            append: true
          });
        }
      } else if (payload?.$case === 'statusUpdate') {
        const statusText = textFromParts(payload.value.status?.message?.parts);
        onEvent({
          type: 'status_changed',
          conversationId,
          agentId,
          taskId: payload.value.taskId,
          status: payload.value.status
            ? taskStateToJSON(payload.value.status.state)
                .replace(/^TASK_STATE_/, '')
                .toLowerCase()
            : 'working',
          text: statusText,
          terminal: payload.value.status
            ? isTerminalTaskState(payload.value.status.state)
            : false
        });
      } else if (payload?.$case === 'artifactUpdate') {
        const content = textFromParts(payload.value.artifact?.parts);
        if (content) {
          onEvent({
            type: 'assistant_delta',
            conversationId,
            agentId,
            text: content,
            append: payload.value.append
          });
        }
      } else if (payload?.$case === 'task') {
        const statusText = textFromParts(payload.value.status?.message?.parts);
        onEvent({
          type: 'status_changed',
          conversationId,
          agentId,
          taskId: payload.value.id,
          status: statusFromTask(payload.value),
          text: statusText,
          terminal: payload.value.status
            ? isTerminalTaskState(payload.value.status.state)
            : false
        });
      }
    }

    const detail = await getConversation(fetcher, conversationId, agentId);
    onEvent({
      type: 'turn_completed',
      ...detail
    });
  } catch (err: unknown) {
    const errMsg = err instanceof Error ? err.message : 'Unknown error';
    onEvent({
      type: 'turn_failed',
      conversationId,
      agentId,
      message: errMsg || 'Failed to complete stream turn'
    });
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
    const body = await response
      .json()
      .catch(() => ({ error: `HTTP ${response.status}` }));
    throw new Error(
      typeof body.error === 'string'
        ? body.error
        : `Background upload failed: ${response.status}`
    );
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
