<script lang="ts">
  import { page } from '$app/stores';
  import AlarmFocusOverlay from '$lib/components/alarms/AlarmFocusOverlay.svelte';
  import DashboardGrid from '$lib/components/display/DashboardGrid.svelte';
  import { displayThemeStyle } from '$lib/themes';
  import type {
    DashboardData,
    DisplayConfig,
    WidgetInstance
  } from '$lib/types';

  const baseDisplay: DisplayConfig = {
    theme: 'light',
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
    widgetChrome: {
      default: 'solid',
      smokedOpacity: 0.72,
      frostedOpacity: 0.34
    },
    accentColor: '',
    idleMode: ''
  };

  $: mode = ($page.url.searchParams.get('mode') || 'light') as 'light' | 'dark';
  $: chrome = $page.url.searchParams.get('chrome') || 'solid';
  $: state = $page.url.searchParams.get('state') || 'ok';
  $: display = {
    ...baseDisplay,
    theme: mode,
    colorMode: mode,
    widgetChrome: { ...baseDisplay.widgetChrome, default: chrome },
    background:
      chrome === 'smoked'
        ? {
            ...baseDisplay.background,
            kind: 'dynamic',
            value: 'stardust',
            overlay: 'smoked'
          }
        : baseDisplay.background
  };
  $: data = buildDashboard(display, state);
  $: displayStyle = displayThemeStyle(display, mode);

  function widgetData(kind: string, visualState: string) {
    if (visualState === 'empty') {
      return undefined;
    }
    if (visualState === 'unavailable') {
      return {
        status: 'unavailable',
        issue: {
          code: `${kind}.unavailable`,
          severity: 'warning',
          title: 'Unavailable',
          message: 'Mock dependency is unavailable.'
        }
      };
    }
    return okPayload(kind, visualState);
  }

  function okPayload(kind: string, visualState: string) {
    switch (kind) {
      case 'weather':
        return {
          status: 'ok',
          data: {
            locationName: 'Kitchen',
            temperature: 19,
            temperatureUnit: 'celsius',
            apparentTemperature: 18,
            condition: 'Partly cloudy',
            icon: 'cloud-sun',
            weatherCode: 2,
            humidity: 62,
            windSpeed: 9,
            windSpeedUnit: 'kmh',
            sunrise: '2026-06-13T05:01:00Z',
            sunset: '2026-06-13T21:18:00Z',
            isDay: true,
            updatedAt: '2026-06-13T12:00:00Z',
            source: 'Open-Meteo',
            status: 'available'
          }
        };
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
                  pubDate: new Date().toISOString()
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
            top_albums: [
              {
                id: 'visual-album-1',
                name: 'Morning Loop',
                artist_name: 'Jute Dash',
                uri: 'spotify:album:visual-1',
                album_art_url:
                  'https://i.scdn.co/image/ab67616d00001e02ff9ca10b55ce82ae553c8228'
              },
              {
                id: 'visual-album-2',
                name: 'Kitchen Mix',
                artist_name: 'Jute Dash',
                uri: 'spotify:album:visual-2',
                album_art_url:
                  'https://i.scdn.co/image/ab67616d00001e02ff9ca10b55ce82ae553c8228'
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
                id: 'timer-visual',
                kind: 'timer',
                label: 'Tea',
                status: 'active',
                dueAt:
                  visualState === 'ringing'
                    ? new Date(Date.now() - 1000).toISOString()
                    : new Date(Date.now() + 305000).toISOString(),
                durationSeconds: 300,
                sound: 'chime'
              },
              {
                id: 'alarm-visual',
                kind: 'alarm',
                label: 'School run',
                status: 'active',
                dueAt: new Date(Date.now() + 3600000).toISOString(),
                time: '07:30',
                weekdays: [1, 2, 3, 4, 5],
                recurring: true,
                sound: 'bell'
              }
            ],
            ringing:
              visualState === 'ringing'
                ? [
                    {
                      id: 'timer-visual',
                      kind: 'timer',
                      label: 'Tea',
                      status: 'active',
                      dueAt: new Date(Date.now() - 1000).toISOString(),
                      sound: 'chime',
                      ringing: true
                    }
                  ]
                : [],
            notificationSound: 'chime',
            defaultSnoozeMins: 9,
            timezone: 'Europe/London'
          }
        };
      case 'calendar': {
        const eventStart =
          visualState === 'calendar-ringing'
            ? new Date(Date.now() + 3 * 60000).toISOString()
            : new Date(Date.now() + 45 * 60000).toISOString();
        const eventEnd = new Date(
          new Date(eventStart).getTime() + 45 * 60000
        ).toISOString();
        const alert = {
          id: 'calendar:school-assembly',
          kind: 'calendar-event',
          label: 'School assembly',
          status: 'active',
          dueAt:
            visualState === 'calendar-ringing'
              ? new Date(Date.now() - 1000).toISOString()
              : new Date(Date.now() + 35 * 60000).toISOString(),
          eventStart,
          eventEnd,
          calendar: 'Family',
          sound: 'chime',
          ringing: visualState === 'calendar-ringing',
          defaultSnoozeMins: 9
        };
        return {
          status: 'ok',
          data: {
            events: [
              {
                id: 'school-assembly',
                uid: 'school-assembly',
                title: 'School assembly',
                calendar: 'Family',
                start: eventStart,
                end: eventEnd,
                allDay: false,
                location: 'Hall',
                source: 'ics'
              },
              {
                id: 'bin-day',
                uid: 'bin-day',
                title: 'Bins out',
                calendar: 'Home',
                start: new Date(Date.now() + 24 * 3600000).toISOString(),
                end: new Date(
                  Date.now() + 24 * 3600000 + 15 * 60000
                ).toISOString(),
                allDay: false,
                source: 'ics'
              }
            ],
            nextEvent: {
              id: 'school-assembly',
              title: 'School assembly',
              calendar: 'Family',
              start: eventStart,
              end: eventEnd,
              location: 'Hall'
            },
            alerts: [alert],
            ringing: visualState === 'calendar-ringing' ? [alert] : [],
            ringingCount: visualState === 'calendar-ringing' ? 1 : 0,
            alertLeadMinutes: 10,
            defaultSnoozeMins: 9,
            notificationSound: 'chime',
            generatedAt: new Date().toISOString(),
            source: 'ics'
          }
        };
      }
      default:
        return { status: 'ok', data: {} };
    }
  }

  function buildDashboard(
    display: DisplayConfig,
    visualState: string
  ): DashboardData {
    const stale = visualState === 'stale';
    const widgets: WidgetInstance[] = [
      widget('date-time', 'date-time', 'Date & Time', 0, 0, 3, 2, visualState),
      widget('weather', 'weather', 'Weather', 3, 0, 3, 2, visualState),
      widget('rss', 'rss', 'Headlines', 6, 0, 3, 2, visualState),
      widget('markets', 'markets', 'Markets', 9, 0, 3, 2, visualState),
      widget('spotify', 'spotify', 'Spotify', 0, 2, 3, 2, visualState),
      widget(
        'apple-music',
        'apple-music',
        'Apple Music',
        3,
        2,
        3,
        2,
        visualState
      ),
      widget('philips-hue', 'philips-hue', 'Hue', 6, 2, 3, 2, visualState),
      widget('zigbee2mqtt', 'zigbee2mqtt', 'Zigbee', 9, 2, 3, 2, visualState),
      widget(
        'chat-history',
        'chat-history',
        'Saved Chats',
        0,
        4,
        3,
        2,
        visualState
      ),
      widget(
        'timers-alarms',
        'timers-alarms',
        'Timers',
        3,
        4,
        3,
        2,
        visualState
      ),
      widget('calendar', 'calendar', 'Calendar', 6, 4, 3, 2, visualState)
    ];

    return {
      config: {
        home: { name: 'Visual Smoke Home' },
        display,
        agents: [],
        rooms: [],
        tiles: []
      },
      home: {
        generatedAt: '2026-06-13T12:00:00Z',
        home: { name: 'Visual Smoke Home' },
        rooms: [],
        tiles: []
      },
      agents: [
        {
          id: 'agent-1',
          name: 'House Agent',
          description: '',
          cardUrl: 'http://localhost/agent-card.json',
          endpointUrl: 'http://localhost/agent',
          protocolBinding: 'JSONRPC',
          enabled: true,
          capabilities: [],
          authConfigured: false,
          cardStatus: 'available'
        }
      ],
      layout: {
        profileId: 'visual',
        widgets
      },
      voice: {
        enabled: false,
        muted: true,
        state: 'muted',
        serviceStatus: 'not_configured',
        deviceProfileId: '',
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
        updatedAt: '2026-06-13T12:00:00Z'
      },
      connectionState: stale ? 'reconnecting' : 'connected',
      stale,
      hubUrl: 'http://localhost:8787',
      loadedAt: '2026-06-13T12:00:00Z'
    };
  }

  function widget(
    id: string,
    kind: string,
    title: string,
    x: number,
    y: number,
    w: number,
    h: number,
    visualState: string
  ): WidgetInstance {
    return {
      id,
      kind,
      title,
      x,
      y,
      w,
      h,
      minW: 2,
      minH: 1,
      size: 'medium',
      mode: 'ui',
      visible: true,
      settings: {},
      connectionRefs: {},
      data: widgetData(kind, visualState)
    };
  }
</script>

<svelte:head>
  <title>Jute visual smoke</title>
</svelte:head>

<main
  class="display-root visual-smoke-root"
  data-theme={mode}
  data-background-overlay={display.background.overlay}
  style={displayStyle}
>
  <DashboardGrid
    {data}
    stale={data.stale}
    selectedAgent={data.agents[0]}
    selectedAvailability="available"
  />
  {#if state === 'ringing' || state === 'calendar-ringing'}
    <AlarmFocusOverlay {data} />
  {/if}
</main>

<style>
  .visual-smoke-root {
    padding: 16px;
  }
</style>
