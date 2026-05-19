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
  authConfigured: boolean;
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
  connectionState: AppConnectionState;
  stale: boolean;
  hubUrl: string;
  loadedAt: string;
  issue?: UserFacingIssue;
};

export type DisplayMode = 'dashboard' | 'edit' | 'chat';

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
};

export type ChatState = 'idle' | 'listening' | 'thinking' | 'streaming' | 'error';

export type ChatMessageRole = 'user' | 'assistant' | 'system';

export type ChatMessage = {
  id: string;
  role: ChatMessageRole;
  content: string;
  createdAt: string;
  status?: 'sending' | 'sent' | 'failed';
  retryText?: string;
  agentId?: string;
};

export type MessageResponse = {
  conversationId: string;
  agentId: string;
  status: string;
  message: string;
};
