export type HomeConfig = {
  name: string;
  timezone: string;
  locale: string;
};

export type DisplayConfig = {
  theme: string;
  accentColor: string;
  idleMode: string;
};

export type Agent = {
  id: string;
  name: string;
  description: string;
  cardUrl: string;
  endpointUrl: string;
  protocolBinding: string;
  enabled: boolean;
  capabilities: string[];
  mcpScopes?: string[];
  authConfigured: boolean;
  cardStatus?: 'available' | 'unavailable' | 'unknown' | string;
  cardFetchedAt?: string;
  cardError?: string;
  selectedEndpointUrl?: string;
  selectedProtocolBinding?: string;
  selectedProtocolVersion?: string;
  skills?: AgentSkill[];
  streaming?: boolean;
  dashboardContextSupported?: boolean;
};

export type AgentSkill = {
  id?: string;
  name: string;
  description?: string;
  tags?: string[];
  examples?: string[];
  inputModes?: string[];
  outputModes?: string[];
};

export type AppConnectionState = 'starting' | 'connected' | 'reconnecting' | 'offline' | 'degraded';

export type AgentAvailability =
  | 'available'
  | 'disabled'
  | 'missing_credentials'
  | 'unsupported_binding'
  | 'unhealthy'
  | 'offline'
  | 'unknown';

export type UserFacingIssue = {
  code: string;
  severity: 'info' | 'warning' | 'error';
  title: string;
  message: string;
  action?: {
    label: string;
    target: 'retry' | 'settings' | 'setup' | 'details';
  };
};

export type Room = {
  id: string;
  name: string;
  summary: string;
  status: string;
};

export type Tile = {
  id: string;
  kind: string;
  label: string;
  value: string;
  detail: string;
};

export type WeatherState = {
  locationName: string;
  temperature: number | null;
  temperatureUnit: string;
  apparentTemperature: number | null;
  condition: string;
  icon: string;
  weatherCode: number | null;
  humidity: number | null;
  windSpeed: number | null;
  windSpeedUnit: string;
  sunrise: string;
  sunset: string;
  isDay: boolean | null;
  updatedAt: string;
  source: string;
  status: 'available' | 'unavailable' | 'disabled';
};

export type VoiceState = 'muted' | 'idle' | 'wake_listening';

export type VoiceServiceStatus = 'ready' | 'not_configured';

export type VoiceStatus = {
  enabled: boolean;
  muted: boolean;
  state: VoiceState;
  serviceStatus: VoiceServiceStatus;
  deviceProfileId: string;
  wakeWordModelId: string;
  sttProviderId: string;
  ttsProviderId: string;
  sttModelId: string;
  ttsModelId: string;
  ttsVoiceId: string;
  preferredAgentId: string;
  cloudOptIn: boolean;
  commandProvidersEnabled: boolean;
  followupWindowSeconds: number;
  microphoneProfile: string;
  updatedAt: string;
};

export type VoiceProvider = {
  id: string;
  name: string;
  version: string;
  kind: string;
  transportType: string;
  healthStatus: string;
  updatedAt: string;
};

export type PublicConfig = {
  home: HomeConfig;
  display: DisplayConfig;
  agents: Agent[];
  rooms: Room[];
  tiles: Tile[];
};

export type HomeState = {
  generatedAt: string;
  home: HomeConfig;
  rooms: Room[];
  tiles: Tile[];
  weather: WeatherState;
};

export type DashboardData = {
  config: PublicConfig;
  home: HomeState;
  agents: Agent[];
  layout: WidgetLayout;
  voice: VoiceStatus;
  connectionState: AppConnectionState;
  stale: boolean;
  hubUrl: string;
  loadedAt: string;
  issue?: UserFacingIssue;
};

export type DisplayMode = 'dashboard' | 'edit' | 'chat';

export type DisplayNotification = {
  id: string;
  message: string;
  severity: 'info' | 'success' | 'warning' | 'error' | string;
  createdAt: string;
  expiresAt: string;
};

export type DisplayFocusWidget = {
  id: string;
  widgetInstanceId: string;
  reason?: string;
  createdAt: string;
};

export type DisplayEvent =
  | {
      type: 'display.notification';
      data: DisplayNotification;
    }
  | {
      type: 'display.focus_widget';
      data: DisplayFocusWidget;
    }
  | {
      type: 'hub.connected';
      data: { connectedAt: string };
    };

export type WidgetKind = 'date-time' | 'weather' | 'chat-history' | string;

export type WidgetLayout = {
  profileId: string;
  widgets: WidgetInstance[];
};

export type WidgetCatalogItem = {
  kind: string;
  name: string;
  description: string;
  defaultTitle: string;
  defaultW: number;
  defaultH: number;
  minW: number;
  minH: number;
  defaultSize: 'small' | 'medium' | 'wide' | 'large' | string;
  overflow: 'clip' | 'scroll' | 'expand' | string;
  allowMultiple: boolean;
};

export type WidgetInstance = {
  id: string;
  kind: WidgetKind;
  title: string;
  x: number;
  y: number;
  w: number;
  h: number;
  minW: number;
  minH: number;
  size: 'small' | 'medium' | 'wide' | 'large' | string;
  settings: Record<string, unknown>;
  visible: boolean;
  data?: any;
};

export type ChatState = 'idle' | 'listening' | 'thinking' | 'streaming' | 'error';

export type ChatMessageRole = 'user' | 'assistant' | 'system';

export type ChatMessage = {
  id: string;
  conversationId?: string;
  role: ChatMessageRole;
  content: string;
  createdAt: string;
  status?: 'sending' | 'streaming' | 'sent' | 'failed';
  retryText?: string;
  agentId?: string;
};

export type Conversation = {
  id: string;
  agentId: string;
  title: string;
  status: 'idle' | 'streaming' | 'completed' | 'failed' | string;
  a2aContextId: string;
  latestTaskId: string;
  createdAt: string;
  updatedAt: string;
  historyUnsupported?: boolean;
};

export type ConversationMessage = {
  id: string;
  conversationId: string;
  agentId: string;
  role: ChatMessageRole;
  content: string;
  status: 'sending' | 'streaming' | 'sent' | 'failed' | string;
  a2aMessageId: string;
  a2aTaskId: string;
  createdAt: string;
  updatedAt: string;
};

export type ConversationDetail = {
  conversation: Conversation;
  messages: ConversationMessage[];
};

export type ConversationStreamEvent =
  | {
      type: 'turn_started';
      conversationId: string;
      agentId: string;
      taskId?: string;
      status?: string;
    }
  | {
      type: 'assistant_delta';
      conversationId: string;
      agentId: string;
      taskId?: string;
      text: string;
      append: boolean;
    }
  | {
      type: 'status_changed';
      conversationId: string;
      agentId: string;
      taskId?: string;
      status: string;
      terminal?: boolean;
    }
  | ({
      type: 'turn_completed';
    } & ConversationDetail)
  | {
      type: 'turn_failed';
      conversationId?: string;
      agentId?: string;
      message: string;
    };

export type MessageResponse = {
  conversationId: string;
  taskId?: string;
  agentId: string;
  status: string;
  message: string;
};
