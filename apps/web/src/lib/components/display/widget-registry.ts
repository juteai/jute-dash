/* eslint-disable @typescript-eslint/no-explicit-any */
import ChatHistoryWidget from '$widgets/chathistory/ChatHistoryWidget.svelte';
import DateTimeWidget from '$widgets/datetime/DateTimeWidget.svelte';
import WeatherWidget from '$widgets/weather/WeatherWidget.svelte';
import RSSWidget from '$widgets/rss/RSSWidget.svelte';
import MarketsWidget from '$widgets/markets/MarketsWidget.svelte';
import { chatStore } from '../../chatStore';
import { navigationStore } from '../../navigationStore';
import type {
  DashboardData,
  ChatMessage,
  Agent,
  AgentAvailability,
  WidgetInstance
} from '../../types';

export interface WidgetRegistryEntry {
  component: any;
  props: (params: {
    widget: WidgetInstance;
    data: DashboardData;
    stale: boolean;
    messages: ChatMessage[];
    selectedAgent: Agent | undefined;
    selectedAvailability: AgentAvailability;
    onOpenChat: () => void;
  }) => Record<string, any>;
}

export const widgetRegistry: Record<string, WidgetRegistryEntry> = {
  'date-time': {
    component: DateTimeWidget,
    props: ({ widget, stale }) => ({
      settings: widget.settings ?? {
        timezone: 'UTC',
        locale: 'en'
      },
      stale
    })
  },
  weather: {
    component: WeatherWidget,
    props: ({ widget, stale }) => ({
      weather: widget.data ?? {
        locationName: 'Not configured',
        temperature: null,
        temperatureUnit: 'celsius',
        apparentTemperature: null,
        condition: 'Weather unavailable',
        icon: 'cloud',
        weatherCode: null,
        humidity: null,
        windSpeed: null,
        windSpeedUnit: 'kmh',
        sunrise: '',
        sunset: '',
        isDay: null,
        updatedAt: '',
        source: 'widget',
        status: 'unavailable'
      },
      stale
    })
  },
  rss: {
    component: RSSWidget,
    props: ({ widget, stale }) => ({
      data: widget.data,
      stale
    })
  },
  markets: {
    component: MarketsWidget,
    props: ({ widget, stale, data }) => ({
      data: widget.data,
      stale,
      onQueryAgent: (symbol: string) => {
        navigationStore.openChat();
        void chatStore.submit(
          `Show me details and recent news for ${symbol}`,
          data.agents,
          undefined,
          fetch
        );
      }
    })
  },
  'chat-history': {
    component: ChatHistoryWidget,
    props: ({
      data,
      messages,
      selectedAgent,
      selectedAvailability,
      onOpenChat
    }) => ({
      agents: data.agents,
      messages,
      selectedAgent,
      selectedAvailability,
      onOpenChat
    })
  }
};
