/* eslint-disable @typescript-eslint/no-explicit-any */
import ChatHistoryWidget from './chathistory/ChatHistoryWidget.svelte';
import DateTimeWidget from './datetime/DateTimeWidget.svelte';
import WeatherWidget from './weather/WeatherWidget.svelte';
import RSSWidget from './rss/RSSWidget.svelte';
import MarketsWidget from './markets/MarketsWidget.svelte';
import { chatStore } from '$lib/chatStore';
import { navigationStore } from '$lib/navigationStore';
import type {
  DashboardData,
  ChatMessage,
  Agent,
  AgentAvailability,
  WidgetInstance
} from '$lib/types';

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
      settings: {
        timezone: 'UTC',
        locale: 'en',
        style: 'digital',
        ...(widget.settings || {})
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
    props: ({ widget, stale, data }) => ({
      data: widget.data,
      stale,
      onQueryAgent: async (title: string, link: string) => {
        navigationStore.openChat();
        await chatStore.newConversation(data.agents, fetch);
        void chatStore.submit(
          `Read the article: ${title} (${link})`,
          data.agents,
          undefined,
          fetch
        );
      }
    })
  },
  markets: {
    component: MarketsWidget,
    props: ({ widget, stale, data }) => ({
      data: widget.data,
      stale,
      onQueryAgent: async (symbol: string) => {
        navigationStore.openChat();
        await chatStore.newConversation(data.agents, fetch);
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
