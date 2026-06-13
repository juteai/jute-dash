/* eslint-disable @typescript-eslint/no-explicit-any */
import ChatHistoryWidget from './chathistory/ChatHistoryWidget.svelte';
import DateTimeWidget from './datetime/DateTimeWidget.svelte';
import WeatherWidget from './weather/WeatherWidget.svelte';
import RSSWidget from './rss/RSSWidget.svelte';
import MarketsWidget from './markets/MarketsWidget.svelte';
import SpotifyWidget from './spotify/SpotifyWidget.svelte';
import AppleMusicWidget from './applemusic/AppleMusicWidget.svelte';
import PhilipsHueWidget from './philipshue/PhilipsHueWidget.svelte';
import Zigbee2MQTTWidget from './zigbee2mqtt/Zigbee2MQTTWidget.svelte';
import { chatStore } from '$lib/chatStore';
import { navigationStore } from '$lib/navigationStore';
import { dispatchWidgetAction } from '$lib/hubClient';
import type {
  DashboardData,
  ChatMessage,
  Agent,
  AgentAvailability,
  WidgetInstance
} from '$lib/types';

function widgetPayload(widget: WidgetInstance): any {
  const payload = widget.data as { data?: unknown; status?: string } | undefined;
  if (payload && 'data' in payload) {
    return payload.data;
  }
  return widget.data;
}

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
      weather: widgetPayload(widget) ?? {
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
      data: widgetPayload(widget),
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
      data: widgetPayload(widget),
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
  'spotify': {
    component: SpotifyWidget,
    props: ({ widget, stale }) => ({
      data: widgetPayload(widget) ?? {
        track_title: 'Not Playing',
        artist_name: 'Unknown',
        is_playing: false,
        volume: 50
      },
      stale,
      dispatch: async (action: string, args: Record<string, any> = {}) => {
        await dispatchWidgetAction(fetch, widget.id, action, args);
        const { hubStream } = await import('$lib/hubStream');
        await hubStream.refreshAfterMutation();
      }
    })
  },
  'apple-music': {
    component: AppleMusicWidget,
    props: ({ widget, stale }) => ({
      instanceId: widget.id,
      data: widgetPayload(widget) ?? {
        track_title: 'Not Playing',
        artist_name: 'Unknown',
        is_playing: false
      },
      stale,
      dispatch: async (action: string, args: Record<string, any> = {}) => {
        await dispatchWidgetAction(fetch, widget.id, action, args);
        const { hubStream } = await import('$lib/hubStream');
        await hubStream.refreshAfterMutation();
      }
    })
  },
  'philips-hue': {
    component: PhilipsHueWidget,
    props: ({ widget, stale }) => ({
      instanceId: widget.id,
      data: widgetPayload(widget) ?? {
        devices: []
      },
      stale,
      dispatch: async (action: string, args: Record<string, any> = {}) => {
        await dispatchWidgetAction(fetch, widget.id, action, {
          deviceId: args.device_id || args.deviceId || '',
          value: args.state !== undefined ? args.state : args.value
        });
        const { hubStream } = await import('$lib/hubStream');
        await hubStream.refreshAfterMutation();
      }
    })
  },
  'zigbee2mqtt': {
    component: Zigbee2MQTTWidget,
    props: ({ widget, stale }) => ({
      data: widgetPayload(widget) ?? {
        devices: []
      },
      stale,
      dispatch: async (action: string, args: Record<string, any> = {}) => {
        await dispatchWidgetAction(fetch, widget.id, action, {
          deviceId: args.device_id || args.deviceId || '',
          value: args.state !== undefined ? args.state : args.value
        });
        const { hubStream } = await import('$lib/hubStream');
        await hubStream.refreshAfterMutation();
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
