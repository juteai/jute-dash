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

interface LegacyPart {
  text?: string;
}

interface LegacyTask {
  messages?: LegacyMessage[];
  text?: string;
  updatedAt?: string;
}

type TurnRequestOptions = {
  signal?: AbortSignal;
};

type PartWithData = A2APart & {
  data?: {
    id?: string;
    name?: string;
    response?: {
      output?: unknown;
    };
  };
};

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

const hiddenReasoningBlocks = [
  /<think>[\s\S]*?<\/think>/gi,
  /<thinking>[\s\S]*?<\/thinking>/gi,
  /<reasoning>[\s\S]*?<\/reasoning>/gi,
  /<scratchpad>[\s\S]*?<\/scratchpad>/gi,
  /```(?:thinking|reasoning|scratchpad)\s+[\s\S]*?```/gi,
  /<tool_call>[\s\S]*?<\/tool_call>/gi,
  /<tool_response>[\s\S]*?<\/tool_response>/gi
];

function looksLikeReasoningParagraph(paragraph: string): boolean {
  const lower = paragraph.trim().toLowerCase();
  if (
    lower.startsWith('okay, the user') ||
    lower.startsWith('the user ') ||
    lower.startsWith('we need ') ||
    lower.startsWith('i need ') ||
    lower.startsWith('i should ') ||
    lower.startsWith('let me ')
  ) {
    return true;
  }
  const phrases = [
    'the user',
    'i should',
    "i'll",
    'i will',
    'no need to',
    'need to call',
    'call any function',
    'call tools',
    'use the tool',
    'tool choice',
    'final answer'
  ];
  let signals = 0;
  for (const phrase of phrases) {
    if (lower.includes(phrase)) {
      signals++;
    }
  }
  return signals >= 2;
}

/**
 * Detects ADK-style tool invocations that models emit as plain text instead
 * of structured function calls. Covers patterns like:
 *   "jute_skill_read_context - {\"skillId\": ...}"
 *   "function_name - {\"key\": ...}"
 *   Bare JSON tool-response payloads with known keys.
 */
function looksLikeToolInvocation(text: string): boolean {
  const trimmed = text.trim();
  if (!trimmed) return false;

  // ADK-style: "function_name - {json_args}"
  if (/^[a-zA-Z_]\w*\s*-\s*\{/.test(trimmed)) {
    return true;
  }
  // Known hub tool-function prefixes emitted as plain text
  if (
    trimmed.startsWith('jute_skill_') ||
    trimmed.startsWith('mcp_') ||
    trimmed.startsWith('function_call')
  ) {
    return true;
  }
  // Bare JSON that looks like a tool response payload
  if (
    /^\{[\s\S]*"(?:skillId|tool_call_id|function_call|actionId)"/.test(trimmed)
  ) {
    return true;
  }
  return false;
}

export function isReasoningArtifact(artifact: {
  artifactId?: string;
  name?: string;
  description?: string;
  parts?: A2APart[];
}): boolean {
  if (!artifact) return false;
  const idLower = (artifact.artifactId || '').toLowerCase();
  const nameLower = (artifact.name || '').toLowerCase();
  const descLower = (artifact.description || '').toLowerCase();

  const keywords = [
    'reasoning',
    'thinking',
    'scratchpad',
    'thought',
    'internal-thought',
    'internal_thought',
    'chain-of-thought',
    'chain_of_thought',
    'cot',
    'planning',
    'plan',
    'tool-selection',
    'tool_selection'
  ];

  if (
    keywords.some(
      (k) =>
        idLower.includes(k) || nameLower.includes(k) || descLower.includes(k)
    )
  ) {
    return true;
  }

  const artRecord = artifact as Record<string, unknown>;
  const metadata = artRecord.metadata as Record<string, unknown> | undefined;
  const typeLower = (
    String(artRecord.type || '') ||
    String(artRecord.kind || '') ||
    String(metadata?.adk_type || '') ||
    String(metadata?.type || '') ||
    String(metadata?.kind || '') ||
    ''
  ).toLowerCase();
  if (keywords.some((k) => typeLower.includes(k))) {
    return true;
  }

  if (artifact.parts && Array.isArray(artifact.parts)) {
    // If any part of the artifact is a function call, function response, or tool invocation,
    // the entire artifact is classified as a reasoning/internal steps artifact.
    const hasToolOrFunction = artifact.parts.some((part) => {
      if (
        part.metadata?.adk_type === 'function_call' ||
        part.metadata?.adk_type === 'function_response'
      ) {
        return true;
      }
      const data = getPartData(part) as PartWithData['data'];
      if (data && (data.name || data.id || data.response)) {
        return true;
      }
      const text = getPartText(part);
      if (text) {
        const trimmed = text.trim();
        if (
          trimmed.startsWith('<tool_call>') ||
          trimmed.endsWith('</tool_call>') ||
          trimmed.startsWith('<tool_response>') ||
          trimmed.endsWith('</tool_response>') ||
          trimmed.includes('<tool_call>') ||
          trimmed.includes('<tool_response>') ||
          looksLikeToolInvocation(trimmed)
        ) {
          return true;
        }
      }
      return false;
    });

    if (hasToolOrFunction) {
      return true;
    }

    const hasOnlyThoughtsAndTools = artifact.parts.every((part) => {
      let matched = false;
      if (part.metadata?.adk_thought === true) matched = true;
      else if (
        part.metadata?.adk_type === 'function_call' ||
        part.metadata?.adk_type === 'function_response'
      )
        matched = true;
      else {
        const mediaType = (part.mediaType || '').toLowerCase();
        if (keywords.some((k) => mediaType.includes(k))) matched = true;
        else {
          const text = getPartText(part);
          if (!text) {
            matched = true;
          } else {
            const trimmed = text.trim();
            if (!trimmed) matched = true;
            else if (
              trimmed.startsWith('<tool_call>') ||
              trimmed.endsWith('</tool_call>') ||
              trimmed.startsWith('<tool_response>') ||
              trimmed.endsWith('</tool_response>') ||
              trimmed.includes('<tool_call>') ||
              trimmed.includes('<tool_response>') ||
              looksLikeToolInvocation(trimmed) ||
              looksLikeReasoningParagraph(trimmed)
            )
              matched = true;
          }
        }
      }
      return matched;
    });

    if (artifact.parts.length > 0 && hasOnlyThoughtsAndTools) {
      return true;
    }
  }

  return false;
}

export function sanitizeDisplayText(text: string): string {
  let cleaned = text.trim();
  if (!cleaned) return '';

  // Handle active streaming / open tags defensively
  const openTags = [
    { start: '<think>', end: '</think>' },
    { start: '<thinking>', end: '</thinking>' },
    { start: '<reasoning>', end: '</reasoning>' },
    { start: '<scratchpad>', end: '</scratchpad>' },
    { start: '<tool_call>', end: '</tool_call>' },
    { start: '<tool_response>', end: '</tool_response>' }
  ];

  for (const tag of openTags) {
    const startIdx = cleaned.indexOf(tag.start);
    if (startIdx > -1) {
      const endIdx = cleaned.indexOf(tag.end);
      if (endIdx > -1) {
        cleaned =
          cleaned.slice(0, startIdx) + cleaned.slice(endIdx + tag.end.length);
      } else {
        cleaned = cleaned.slice(0, startIdx);
      }
    }
  }

  for (const pattern of hiddenReasoningBlocks) {
    cleaned = cleaned.replace(pattern, '');
  }
  cleaned = cleaned.trim();
  if (!cleaned) return '';

  const paragraphs = cleaned
    .replace(/\r\n/g, '\n')
    .split('\n\n')
    .map((p) => p.trim())
    .filter(Boolean);

  while (paragraphs.length > 1 && looksLikeReasoningParagraph(paragraphs[0])) {
    paragraphs.shift();
  }

  return paragraphs.join('\n\n').trim();
}

function getPartText(part: A2APart): string {
  if ('text' in part && (part as LegacyPart).text) {
    return (part as LegacyPart).text as string;
  }
  return part.content?.$case === 'text' ? part.content.value : '';
}

function getPartData(part: A2APart): Record<string, unknown> | undefined {
  const p = part as unknown as {
    data?: Record<string, unknown>;
    content?: {
      $case: string;
      value?: Record<string, unknown>;
    };
  };
  if (p.data) {
    return p.data;
  }
  if (p.content?.$case === 'data') {
    return p.content.value;
  }
  return undefined;
}

function textFromParts(parts: A2APart[] | undefined): string {
  const raw = (parts ?? [])
    .map((part) => {
      if (part.metadata?.adk_thought === true) {
        return '';
      }
      const text = (
        'text' in part && (part as LegacyPart).text
          ? (part as LegacyPart).text
          : part.content?.$case === 'text'
            ? part.content.value
            : ''
      ) as string;
      // Filter out parts that are raw tool invocations
      if (text && looksLikeToolInvocation(text)) {
        return '';
      }
      return text;
    })
    .join('');
  return sanitizeDisplayText(raw);
}

export function isStructuredArtifact(parts: A2APart[] | undefined): boolean {
  return (parts ?? [])
    .filter((part) => {
      if (part.metadata?.adk_thought === true) return false;
      if (
        part.metadata?.adk_type === 'function_call' ||
        part.metadata?.adk_type === 'function_response'
      ) {
        return false;
      }
      return true;
    })
    .some((part) => {
      const isLegacyText = 'text' in part && (part as LegacyPart).text;
      const isCaseText = part.content?.$case === 'text';
      if (!isLegacyText && !isCaseText) {
        return true; // Not a text part -> structured
      }
      if (part.mediaType) {
        const mt = part.mediaType.toLowerCase();
        if (mt !== 'text/plain' && mt !== 'text/markdown' && mt !== '') {
          return true;
        }
      }
      return false;
    });
}

export function textFromReasoningParts(parts: A2APart[] | undefined): string {
  return (parts ?? [])
    .map((part) => {
      if ('text' in part && (part as LegacyPart).text) {
        return (part as LegacyPart).text;
      }
      return part.content?.$case === 'text' ? part.content.value : '';
    })
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

function terminalTaskFailureMessage(status: string): string | undefined {
  switch (status) {
    case 'failed':
      return 'Agent task failed';
    case 'rejected':
      return 'Agent rejected the request';
    case 'canceled':
      return 'Agent canceled the request';
    default:
      return undefined;
  }
}

function isHistoryUnsupportedError(error: unknown): boolean {
  if (!(error instanceof Error)) {
    return false;
  }
  const message = error.message.toLowerCase();
  return (
    message.includes('status: 501') ||
    message.includes('code: -32601') ||
    message.includes('method not found')
  );
}

function isAbortError(error: unknown): boolean {
  return (
    (error instanceof DOMException && error.name === 'AbortError') ||
    (error instanceof Error && error.name === 'AbortError')
  );
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

function localConversationDetail(
  conversationId: string,
  agentId: string,
  userText: string,
  assistantText: string,
  taskId: string,
  status: string
): ConversationDetail {
  const now = new Date().toISOString();
  return {
    conversation: {
      id: conversationId,
      agentId,
      title: userText,
      status,
      a2aContextId: conversationId,
      latestTaskId: taskId,
      createdAt: now,
      updatedAt: now,
      historyUnsupported: true
    },
    messages: [
      {
        id: crypto.randomUUID(),
        conversationId,
        agentId,
        role: 'user',
        content: userText,
        status: 'sent',
        a2aMessageId: '',
        a2aTaskId: taskId,
        createdAt: now,
        updatedAt: now
      },
      {
        id: crypto.randomUUID(),
        conversationId,
        agentId,
        role: 'assistant',
        content: assistantText,
        status: 'sent',
        a2aMessageId: '',
        a2aTaskId: taskId,
        createdAt: now,
        updatedAt: now
      }
    ]
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
    if (!isHistoryUnsupportedError(err)) {
      throw err;
    }
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
  agentId: string,
  options: TurnRequestOptions = {}
): Promise<ConversationDetail> {
  const client = await getClient(fetcher, agentId);
  const result = await client.listTasks(
    {
      tenant: '',
      contextId: conversationId,
      status: TaskState.TASK_STATE_UNSPECIFIED,
      pageSize: 50,
      pageToken: '',
      statusTimestampAfter: undefined
    },
    { signal: options.signal }
  );
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
        const fullTask = await client.getTask(
          {
            tenant: '',
            id: task.id,
            historyLength: 50
          },
          { signal: options.signal }
        );
        if (fullTask) {
          record = fullTask;
        }
      } catch (e) {
        if (isAbortError(e)) {
          throw e;
        }
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

    const recordInterimSteps: Array<{
      id: string;
      text: string;
      status: string;
    }> = [];
    if (record.status?.message?.parts) {
      const reasoningParts = record.status.message.parts.filter(
        (p) => p.metadata?.adk_thought === true
      );
      if (reasoningParts.length > 0) {
        const text = textFromReasoningParts(reasoningParts);
        if (text) {
          recordInterimSteps.push({
            id: `${record.id}:status-thought`,
            text,
            status: 'completed'
          });
        }
      }
    }
    for (const [index, artifact] of (record.artifacts ?? []).entries()) {
      const isReasoningArt = isReasoningArtifact(artifact);
      const reasoningParts = isReasoningArt
        ? (artifact.parts ?? []).filter((p) => {
            if (p.metadata?.adk_thought === true) return true;
            const pt = getPartText(p);
            // Exclude tool-call and tool-response text from reasoning display
            return (
              !pt.includes('<tool_call>') &&
              !pt.includes('<tool_response>') &&
              !looksLikeToolInvocation(pt)
            );
          })
        : (artifact.parts ?? []).filter(
            (p) => p.metadata?.adk_thought === true
          );
      if (reasoningParts.length > 0) {
        const text = textFromReasoningParts(reasoningParts);
        if (text) {
          recordInterimSteps.push({
            id: `${record.id}:thought:${artifact.artifactId || index}`,
            text,
            status: 'completed'
          });
        }
      }
      for (const [pIdx, part] of (artifact.parts ?? []).entries()) {
        if (part.metadata?.adk_thought === true) continue;

        const data = getPartData(part) as PartWithData['data'];
        const isFunctionCall =
          part.metadata?.adk_type === 'function_call' ||
          (data && !data.response && (data.name || data.id));

        if (isFunctionCall) {
          const toolName = data?.name || 'agent tool';
          recordInterimSteps.push({
            id: `${record.id}:tool:${artifact.artifactId || index}:${pIdx}`,
            text: `Called tool: ${toolName}`,
            status: 'completed'
          });
          continue;
        }

        const text = getPartText(part);
        if (
          text &&
          (text.includes('<tool_call>') || looksLikeToolInvocation(text))
        ) {
          const nameMatch =
            text.match(/"name"\s*:\s*"([^"]+)"/) ||
            text.match(/^([a-zA-Z_]\w*)\s*-\s*\{/);
          const toolName = nameMatch ? nameMatch[1] : 'agent tool';
          recordInterimSteps.push({
            id: `${record.id}:tool:${artifact.artifactId || index}:${pIdx}`,
            text: `Called tool: ${toolName}`,
            status: 'completed'
          });
        }
      }
    }

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

        const messageThoughts: Array<{
          id: string;
          text: string;
          status: string;
        }> = [];
        if (isA2AMessage && msg.parts) {
          for (const [idx, part] of msg.parts.entries()) {
            if (part.metadata?.adk_thought === true) {
              const text = textFromReasoningParts([part]);
              if (text) {
                messageThoughts.push({
                  id: `${msg.messageId || 'msg'}:thought:${idx}`,
                  text,
                  status: 'completed'
                });
              }
            } else {
              // Check for tool calls in history message parts (structured and text/XML/ADK-style)
              const data = getPartData(part) as PartWithData['data'];
              const isFunctionCall =
                part.metadata?.adk_type === 'function_call' ||
                (data && !data.response && (data.name || data.id));

              if (isFunctionCall) {
                const toolName = data?.name || 'agent tool';
                messageThoughts.push({
                  id: `${msg.messageId || 'msg'}:tool:${idx}`,
                  text: `Called tool: ${toolName}`,
                  status: 'completed'
                });
              } else {
                const text = (
                  'text' in part && part.text
                    ? part.text
                    : part.content?.$case === 'text'
                      ? part.content.value
                      : ''
                ) as string;
                if (
                  text &&
                  (text.includes('<tool_call>') ||
                    looksLikeToolInvocation(text))
                ) {
                  const nameMatch =
                    text.match(/"name"\s*:\s*"([^"]+)"/) ||
                    text.match(/^([a-zA-Z_]\w*)\s*-\s*\{/);
                  const toolName = nameMatch ? nameMatch[1] : 'agent tool';
                  messageThoughts.push({
                    id: `${msg.messageId || 'msg'}:tool:${idx}`,
                    text: `Called tool: ${toolName}`,
                    status: 'completed'
                  });
                }
              }
            }
          }
        }

        const isAssistant =
          msg.role === Role.ROLE_AGENT ||
          msg.role === 'agent' ||
          msg.role === 'assistant';
        const combinedSteps = isAssistant
          ? [...recordInterimSteps, ...messageThoughts]
          : [];

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
          updatedAt: recordUpdatedAt,
          interimSteps: combinedSteps.length > 0 ? combinedSteps : undefined
        });
      }
    }
    for (const [index, artifact] of (record.artifacts ?? []).entries()) {
      if (isReasoningArtifact(artifact)) {
        continue;
      }
      const content = textFromParts(artifact.parts);
      const isStructured = isStructuredArtifact(artifact.parts);
      if (!content && !isStructured) {
        continue;
      }
      const artifactID = artifact.artifactId || String(index);
      const title = artifact.name || artifact.artifactId || 'Artifact';

      // Avoid duplicating plain-text artifacts if history already has an assistant reply
      if (!isStructured) {
        const hasAssistantReply = detail.messages.some(
          (m) => m.a2aTaskId === record.id && m.role === 'assistant'
        );
        if (hasAssistantReply) {
          continue;
        }
      }

      detail.messages.push({
        id: `${record.id}:artifact:${artifactID}`,
        conversationId,
        agentId,
        role: 'assistant',
        content,
        status: 'sent',
        a2aMessageId: artifact.artifactId,
        a2aTaskId: record.id || '',
        createdAt: recordUpdatedAt,
        updatedAt: recordUpdatedAt,
        artifact: isStructured
          ? {
              id: artifactID,
              title,
              content
            }
          : undefined,
        interimSteps:
          recordInterimSteps.length > 0 ? recordInterimSteps : undefined
      });
    }

    if (recordInterimSteps.length > 0) {
      const hasAssistant = detail.messages.some(
        (m) => m.role === 'assistant' && m.a2aTaskId === record.id
      );
      if (!hasAssistant) {
        detail.messages.push({
          id: `${record.id}:fallback-assistant`,
          conversationId,
          agentId,
          role: 'assistant',
          content: '',
          status: 'sent',
          a2aMessageId: '',
          a2aTaskId: record.id || '',
          createdAt: recordUpdatedAt,
          updatedAt: recordUpdatedAt,
          interimSteps: recordInterimSteps
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
  text: string,
  options: TurnRequestOptions = {}
): Promise<ConversationDetail> {
  const client = await getClient(fetcher, agentId);
  const result = await client.sendMessage(
    {
      tenant: '',
      message: newUserMessage(conversationId, text),
      configuration: undefined,
      metadata: undefined
    },
    { signal: options.signal }
  );
  if ('messageId' in result) {
    const assistantText = textFromParts(result.parts);
    if (!assistantText.trim()) {
      throw new Error('Agent response contained no displayable text');
    }
    return localConversationDetail(
      result.contextId || conversationId,
      agentId,
      text,
      assistantText,
      result.taskId,
      'completed'
    );
  }
  const failureMessage = terminalTaskFailureMessage(statusFromTask(result));
  if (failureMessage) {
    throw new Error(failureMessage);
  }

  return getConversation(fetcher, conversationId, agentId, options);
}

export async function sendConversationTurnStream(
  fetcher: typeof fetch,
  conversationId: string,
  agentId: string,
  text: string,
  onEvent: (event: ConversationStreamEvent) => void,
  options: TurnRequestOptions = {}
): Promise<void> {
  let assistantText = '';
  let latestTaskId = '';
  let latestStatus = 'completed';
  const emittedLengths = new Map<string, number>();
  const accumulatedRawTexts = new Map<string, string>();

  onEvent({
    type: 'turn_started',
    conversationId,
    agentId
  });

  try {
    const client = await getClient(fetcher, agentId);

    for await (const event of client.sendMessageStream(
      {
        tenant: '',
        message: newUserMessage(conversationId, text),
        configuration: undefined,
        metadata: undefined
      },
      { signal: options.signal }
    )) {
      const payload = event.payload;
      if (payload?.$case === 'message') {
        const content = textFromParts(payload.value.parts);
        if (content) {
          assistantText += content;
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
        const status = payload.value.status
          ? taskStateToJSON(payload.value.status.state)
              .replace(/^TASK_STATE_/, '')
              .toLowerCase()
          : 'working';
        latestTaskId = payload.value.taskId;
        latestStatus = status;
        onEvent({
          type: 'status_changed',
          conversationId,
          agentId,
          taskId: payload.value.taskId,
          status,
          text: statusText,
          terminal: payload.value.status
            ? isTerminalTaskState(payload.value.status.state)
            : false
        });
        const failureMessage = terminalTaskFailureMessage(status);
        if (failureMessage) {
          onEvent({
            type: 'turn_failed',
            conversationId,
            agentId,
            message: failureMessage
          });
          return;
        }
      } else if (payload?.$case === 'artifactUpdate') {
        const artifact = payload.value.artifact;
        if (artifact) {
          latestTaskId = payload.value.taskId;
          const isReasoningArt = isReasoningArtifact(artifact);
          const artifactID = artifact.artifactId || 'streamed-artifact';

          const parts = artifact.parts ?? [];

          // 1. Process all reasoning parts together to prevent separate part indexes from overwriting each other
          const reasoningParts = parts.filter((part) => {
            if (part.metadata?.adk_thought === true) return true;
            if (isReasoningArt) {
              const pt = getPartText(part);
              const isToolText =
                looksLikeToolInvocation(pt) ||
                pt.includes('<tool_call>') ||
                pt.includes('<tool_response>');
              return !isToolText;
            }
            return false;
          });

          if (reasoningParts.length > 0) {
            const combinedText = textFromReasoningParts(reasoningParts);
            if (combinedText) {
              const key = `${artifactID}:reasoning`;
              const prevRaw = accumulatedRawTexts.get(key) || '';
              const accumulatedRaw = payload.value.append
                ? prevRaw + combinedText
                : combinedText;
              accumulatedRawTexts.set(key, accumulatedRaw);

              if (accumulatedRaw) {
                const prevLen = emittedLengths.get(key) || 0;
                if (accumulatedRaw.length > prevLen) {
                  const delta = accumulatedRaw.slice(prevLen);
                  onEvent({
                    type: 'artifact_update',
                    conversationId,
                    agentId,
                    taskId: latestTaskId,
                    artifactId: isReasoningArt
                      ? artifactID
                      : `${artifactID}-thought`,
                    name: isReasoningArt
                      ? artifact.name
                      : `${artifact.name || artifactID} (Thinking)`,
                    text: delta,
                    append: prevLen > 0 || payload.value.append,
                    isStructured: false,
                    isReasoning: true
                  });
                  emittedLengths.set(key, accumulatedRaw.length);
                }
              }
            }
          }

          // 2. Process non-reasoning parts in the loop
          for (const [pIdx, part] of parts.entries()) {
            const data = getPartData(part) as PartWithData['data'];
            const isFunctionCall =
              part.metadata?.adk_type === 'function_call' ||
              (data && !data.response && (data.name || data.id));
            const isFunctionResponse =
              part.metadata?.adk_type === 'function_response' ||
              (data && data.response);

            if (isFunctionCall) {
              const toolName = data?.name || 'agent tool';
              const toolCallId = data?.id || `${latestTaskId}:tool:${pIdx}`;
              const key = `${artifactID}:${pIdx}:func_call`;
              if (!emittedLengths.has(key)) {
                onEvent({
                  type: 'status_changed',
                  conversationId,
                  agentId,
                  taskId: toolCallId,
                  status: 'working',
                  text: `Calling tool: ${toolName}`
                });
                emittedLengths.set(key, 1);
              }
              continue;
            }

            if (isFunctionResponse) {
              const toolName = data?.name || 'agent tool';
              const toolCallId = data?.id || `${latestTaskId}:tool:${pIdx}`;
              const key = `${artifactID}:${pIdx}:func_response`;
              if (!emittedLengths.has(key)) {
                onEvent({
                  type: 'status_changed',
                  conversationId,
                  agentId,
                  taskId: toolCallId,
                  status: 'completed',
                  text: `Called tool: ${toolName}`
                });
                emittedLengths.set(key, 1);
              }
              continue;
            }

            const partText = getPartText(part);
            const isToolText =
              looksLikeToolInvocation(partText) ||
              partText.includes('<tool_call>') ||
              partText.includes('<tool_response>');
            const isPartReasoning =
              part.metadata?.adk_thought === true ||
              (isReasoningArt && !isToolText);
            if (isPartReasoning) {
              continue;
            }

            if (isToolText) {
              // Streamed tool call or tool response — show as status update only
              const key = `${artifactID}:${pIdx}:tool_call`;
              if (!emittedLengths.has(key)) {
                const nameMatch =
                  partText.match(/"name"\s*:\s*"([^"]+)"/) ||
                  partText.match(/^([a-zA-Z_]\w*)\s*-\s*\{/);
                const toolName = nameMatch ? nameMatch[1] : 'agent tool';
                onEvent({
                  type: 'status_changed',
                  conversationId,
                  agentId,
                  taskId: `${latestTaskId}:tool_call:${pIdx}`,
                  status: 'working',
                  text: `Calling tool: ${toolName}`
                });
                emittedLengths.set(key, partText.length);
              }
            } else {
              // Regular content — emit as artifact update
              const key = `${artifactID}:${pIdx}:content`;
              const prevRaw = accumulatedRawTexts.get(key) || '';
              const accumulatedRaw = payload.value.append
                ? prevRaw + partText
                : partText;
              accumulatedRawTexts.set(key, accumulatedRaw);

              if (accumulatedRaw) {
                const cleanText = sanitizeDisplayText(accumulatedRaw);
                const prevCleanLen = emittedLengths.get(key) || 0;
                if (cleanText.length > prevCleanLen) {
                  const delta = cleanText.slice(prevCleanLen);
                  const isStructured = isStructuredArtifact([part]);
                  onEvent({
                    type: 'artifact_update',
                    conversationId,
                    agentId,
                    taskId: latestTaskId,
                    artifactId: artifactID,
                    name: artifact.name,
                    text: delta,
                    append: prevCleanLen > 0 || payload.value.append,
                    isStructured,
                    isReasoning: false
                  });
                  emittedLengths.set(key, cleanText.length);
                }
              }
            }
          }
        }
      } else if (payload?.$case === 'task') {
        const statusText = textFromParts(payload.value.status?.message?.parts);
        const status = statusFromTask(payload.value);
        latestTaskId = payload.value.id;
        latestStatus = status;
        onEvent({
          type: 'status_changed',
          conversationId,
          agentId,
          taskId: payload.value.id,
          status,
          text: statusText,
          terminal: payload.value.status
            ? isTerminalTaskState(payload.value.status.state)
            : false
        });
        const failureMessage = terminalTaskFailureMessage(status);
        if (failureMessage) {
          onEvent({
            type: 'turn_failed',
            conversationId,
            agentId,
            message: failureMessage
          });
          return;
        }
      }
    }

    let detail: ConversationDetail;
    try {
      detail = await getConversation(fetcher, conversationId, agentId, options);
    } catch (err) {
      if (!isHistoryUnsupportedError(err)) {
        throw err;
      }
      if (!assistantText.trim()) {
        throw new Error('Agent response contained no displayable text', {
          cause: err
        });
      }
      detail = localConversationDetail(
        conversationId,
        agentId,
        text,
        assistantText,
        latestTaskId,
        latestStatus
      );
    }
    onEvent({
      type: 'turn_completed',
      ...detail
    });
  } catch (err: unknown) {
    if (isAbortError(err)) {
      onEvent({
        type: 'turn_canceled',
        conversationId,
        agentId
      });
      return;
    }
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
