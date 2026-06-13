/* eslint-disable @typescript-eslint/no-explicit-any */
import ChatHistoryWidget from "./chathistory/web/ChatHistoryWidget.svelte";
import DateTimeWidget from "./datetime/web/DateTimeWidget.svelte";
import WeatherWidget from "./weather/web/WeatherWidget.svelte";
import RSSWidget from "./rss/web/RSSWidget.svelte";
import MarketsWidget from "./markets/web/MarketsWidget.svelte";
import SpotifyWidget from "./spotify/web/SpotifyWidget.svelte";
import AppleMusicWidget from "./applemusic/web/AppleMusicWidget.svelte";
import PhilipsHueWidget from "./philipshue/web/PhilipsHueWidget.svelte";
import Zigbee2MQTTWidget from "./zigbee2mqtt/web/Zigbee2MQTTWidget.svelte";
import { chatStore } from "$lib/chatStore";
import { navigationStore } from "$lib/navigationStore";
import { dispatchWidgetAction } from "$lib/hubClient";
import type {
  DashboardData,
  ChatMessage,
  Agent,
  AgentAvailability,
  WidgetInstance,
} from "$lib/types";

function widgetPayload(widget: WidgetInstance): any {
  const payload = widget.data as
    | { data?: unknown; status?: string }
    | undefined;
  if (payload && "data" in payload) {
    return payload.data;
  }
  return widget.data;
}

async function refreshAfterWidgetAction() {
  const { hubStream } = await import("$lib/hubStream");
  await hubStream.refreshAfterMutation(fetch);
  if (typeof window === "undefined") return;
  for (const delay of [750, 1750, 3500]) {
    window.setTimeout(() => {
      void hubStream.refreshAfterMutation(fetch);
    }, delay);
  }
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
  "date-time": {
    component: DateTimeWidget,
    props: ({ widget, stale }) => ({
      settings: {
        timezone: "UTC",
        locale: "en",
        style: "digital",
        ...(widget.settings || {}),
      },
      stale,
    }),
  },
  weather: {
    component: WeatherWidget,
    props: ({ widget, stale }) => ({
      weather: widgetPayload(widget) ?? {
        locationName: "Not configured",
        temperature: null,
        temperatureUnit: "celsius",
        apparentTemperature: null,
        condition: "Weather unavailable",
        icon: "cloud",
        weatherCode: null,
        humidity: null,
        windSpeed: null,
        windSpeedUnit: "kmh",
        sunrise: "",
        sunset: "",
        isDay: null,
        updatedAt: "",
        source: "widget",
        status: "unavailable",
      },
      stale,
    }),
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
          fetch,
        );
      },
    }),
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
          fetch,
        );
      },
    }),
  },
  spotify: {
    component: SpotifyWidget,
    props: ({ widget, stale }) => ({
      connectionId: widget.connectionRefs?.account ?? "",
      data: widgetPayload(widget) ?? {
        track_title: "Not Playing",
        artist_name: "Unknown",
        is_playing: false,
        volume: 50,
      },
      stale,
      dispatch: async (action: string, args: Record<string, any> = {}) => {
        await dispatchWidgetAction(fetch, widget.id, action, args);
        await refreshAfterWidgetAction();
      },
    }),
  },
  "apple-music": {
    component: AppleMusicWidget,
    props: ({ widget, stale }) => ({
      instanceId: widget.id,
      data: widgetPayload(widget) ?? {
        track_title: "Not Playing",
        artist_name: "Unknown",
        is_playing: false,
      },
      stale,
      dispatch: async (action: string, args: Record<string, any> = {}) => {
        await dispatchWidgetAction(fetch, widget.id, action, args);
        await refreshAfterWidgetAction();
      },
    }),
  },
  "philips-hue": {
    component: PhilipsHueWidget,
    props: ({ widget, stale }) => ({
      instanceId: widget.id,
      data: widgetPayload(widget) ?? {
        devices: [],
      },
      stale,
      dispatch: async (action: string, args: Record<string, any> = {}) => {
        await dispatchWidgetAction(fetch, widget.id, action, {
          deviceId: args.device_id || args.deviceId || "",
          value: args.state !== undefined ? args.state : args.value,
        });
        await refreshAfterWidgetAction();
      },
    }),
  },
  zigbee2mqtt: {
    component: Zigbee2MQTTWidget,
    props: ({ widget, stale }) => ({
      data: widgetPayload(widget) ?? {
        devices: [],
      },
      stale,
      dispatch: async (action: string, args: Record<string, any> = {}) => {
        await dispatchWidgetAction(fetch, widget.id, action, {
          deviceId: args.device_id || args.deviceId || "",
          value: args.state !== undefined ? args.state : args.value,
        });
        await refreshAfterWidgetAction();
      },
    }),
  },
  "chat-history": {
    component: ChatHistoryWidget,
    props: ({
      data,
      messages,
      selectedAgent,
      selectedAvailability,
      onOpenChat,
    }) => ({
      agents: data.agents,
      messages,
      selectedAgent,
      selectedAvailability,
      onOpenChat,
    }),
  },
};
