import { describe, expect, it, vi, beforeEach } from 'vitest';
import { get } from 'svelte/store';
import { settingsStore } from './settingsStore';
import { hubStream } from './hubStream';
import type { HouseholdSettings } from './types';

function jsonResponse(body: unknown, init: ResponseInit = {}) {
  return new Response(JSON.stringify(body), {
    status: 200,
    headers: { 'content-type': 'application/json' },
    ...init
  });
}

const mockHouseholdSettings: HouseholdSettings = {
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
  weather: {
    enabled: false,
    provider: 'open-meteo',
    locationName: 'London',
    latitude: 51.5072,
    longitude: -0.1276,
    temperatureUnit: 'celsius',
    windSpeedUnit: 'kmh'
  },
  setup: {
    complete: true,
    missing: []
  }
};

describe('settingsStore', () => {
  beforeEach(() => {
    // Reset store to initial state
    settingsStore.clearIssue();
  });

  it('loads settings successfully and populates state', async () => {
    const fetcher = vi.fn<typeof fetch>().mockImplementation(async (url) => {
      if (String(url).includes('/settings/household')) {
        return jsonResponse(mockHouseholdSettings);
      }
      if (String(url).includes('/settings/rooms')) {
        return jsonResponse({ rooms: [] });
      }
      if (String(url).includes('/settings/tiles')) {
        return jsonResponse({ tiles: [] });
      }
      if (String(url).includes('/backgrounds')) {
        return jsonResponse({ images: [] });
      }
      return jsonResponse({ error: 'Not mocked' }, { status: 400 });
    });

    await settingsStore.load(fetcher);
    const state = get(settingsStore);

    expect(state.loading).toBe(false);
    expect(state.householdSettings).toEqual(mockHouseholdSettings);
    expect(state.roomSettings).toEqual([]);
    expect(state.tileSettings).toEqual([]);
    expect(state.issue).toBe('');
  });

  it('handles load error gracefully', async () => {
    const fetcher = vi
      .fn<typeof fetch>()
      .mockRejectedValue(new Error('Network error'));

    await expect(settingsStore.load(fetcher)).rejects.toThrow('Network error');
    const state = get(settingsStore);

    expect(state.loading).toBe(false);
    expect(state.issue).toBe(
      'Settings are unavailable. Check that the hub is running.'
    );
  });

  it('saves household settings and triggers hubStream refresh', async () => {
    const fetcher = vi.fn<typeof fetch>().mockImplementation(async (url) => {
      if (String(url).includes('/settings/household')) {
        return jsonResponse(mockHouseholdSettings);
      }
      if (
        String(url).includes('/config') ||
        String(url).includes('/home') ||
        String(url).includes('/agents') ||
        String(url).includes('/widgets/layout') ||
        String(url).includes('/voice/status') ||
        String(url).includes('/status')
      ) {
        return jsonResponse({});
      }
      return jsonResponse({ error: 'Not mocked' }, { status: 400 });
    });

    const refreshSpy = vi.spyOn(hubStream, 'refreshAfterMutation');

    await settingsStore.saveHousehold(mockHouseholdSettings, fetcher);
    const state = get(settingsStore);

    expect(state.saving).toBe(false);
    expect(state.householdSettings).toEqual(mockHouseholdSettings);
    expect(state.issue).toBe('');
    expect(refreshSpy).toHaveBeenCalled();
  });
});
