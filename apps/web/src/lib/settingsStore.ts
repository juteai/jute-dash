import { writable, get } from 'svelte/store';
import {
  getHouseholdSettings,
  saveHouseholdSettings,
  getRoomSettings,
  saveRoomSettings,
  getTileSettings,
  saveTileSettings,
  addAgent as apiAddAgent,
  deleteAgent as apiDeleteAgent,
  setAgentEnabled as apiSetAgentEnabled,
  refreshAgentCard as apiRefreshAgentCard,
  getBackgroundImages as apiGetBackgroundImages,
  uploadBackgroundImage as apiUploadBackgroundImage,
  deleteBackgroundImage as apiDeleteBackgroundImage
} from '$lib/hubClient';
import { hubStream } from '$lib/hubStream';
import { chatStore } from '$lib/chatStore';
import type {
  Agent,
  HouseholdSettings,
  Room,
  Tile,
  BackgroundImage
} from '$lib/types';

export interface SettingsState {
  loading: boolean;
  saving: boolean;
  savingRooms: boolean;
  savingTiles: boolean;
  savingAgent: boolean;
  uploadingBackground: boolean;
  issue: string;
  householdSettings: HouseholdSettings | undefined;
  roomSettings: Room[];
  tileSettings: Tile[];
  backgroundLibrary: BackgroundImage[];
}

const initialState: SettingsState = {
  loading: false,
  saving: false,
  savingRooms: false,
  savingTiles: false,
  savingAgent: false,
  uploadingBackground: false,
  issue: '',
  householdSettings: undefined,
  roomSettings: [],
  tileSettings: [],
  backgroundLibrary: []
};

function createSettingsStore() {
  const { subscribe, update } = writable<SettingsState>(initialState);

  return {
    subscribe,
    load: async (fetcher: typeof fetch = window.fetch) => {
      update((s) => ({ ...s, loading: true, issue: '' }));
      try {
        const [household, rooms, tiles, backgrounds] = await Promise.all([
          getHouseholdSettings(fetcher),
          getRoomSettings(fetcher),
          getTileSettings(fetcher),
          apiGetBackgroundImages(fetcher).catch(() => [] as BackgroundImage[])
        ]);
        update((s) => ({
          ...s,
          loading: false,
          householdSettings: household,
          roomSettings: rooms,
          tileSettings: tiles,
          backgroundLibrary: backgrounds
        }));
      } catch (err) {
        update((s) => ({
          ...s,
          loading: false,
          issue: 'Settings are unavailable. Check that the hub is running.'
        }));
        throw err;
      }
    },
    saveHousehold: async (
      settings: HouseholdSettings,
      fetcher: typeof fetch = window.fetch
    ) => {
      update((s) => ({ ...s, saving: true, issue: '' }));
      try {
        const saved = await saveHouseholdSettings(fetcher, settings);
        update((s) => ({ ...s, householdSettings: saved, saving: false }));
        await hubStream.refreshAfterMutation(fetcher);
      } catch (err) {
        update((s) => ({
          ...s,
          saving: false,
          issue: 'Settings were not saved. Check required fields and try again.'
        }));
        throw err;
      }
    },
    saveRooms: async (rooms: Room[], fetcher: typeof fetch = window.fetch) => {
      update((s) => ({ ...s, savingRooms: true, issue: '' }));
      try {
        const saved = await saveRoomSettings(fetcher, rooms);
        update((s) => ({ ...s, roomSettings: saved, savingRooms: false }));
        await hubStream.refreshAfterMutation(fetcher);
      } catch (err) {
        update((s) => ({
          ...s,
          savingRooms: false,
          issue: 'Rooms were not saved. Check required fields and try again.'
        }));
        throw err;
      }
    },
    saveTiles: async (tiles: Tile[], fetcher: typeof fetch = window.fetch) => {
      update((s) => ({ ...s, savingTiles: true, issue: '' }));
      try {
        const saved = await saveTileSettings(fetcher, tiles);
        update((s) => ({ ...s, tileSettings: saved, savingTiles: false }));
        await hubStream.refreshAfterMutation(fetcher);
      } catch (err) {
        update((s) => ({
          ...s,
          savingTiles: false,
          issue: 'Tiles were not saved. Check required fields and try again.'
        }));
        throw err;
      }
    },
    addAgent: async (cardUrl: string, fetcher: typeof fetch = window.fetch) => {
      update((s) => ({ ...s, savingAgent: true, issue: '' }));
      try {
        const agent = await apiAddAgent(fetcher, cardUrl);
        chatStore.setAgentId(agent.id);
        const fresh = await hubStream.refreshAfterMutation(fetcher);
        if (fresh) {
          await chatStore.loadHistory(fresh.agents, '', agent.id, fetcher);
        }
        update((s) => ({ ...s, savingAgent: false }));
      } catch (err) {
        update((s) => ({
          ...s,
          savingAgent: false,
          issue:
            'Agent was not added. Check the Agent Card URL and that the hub was started with a YAML config.'
        }));
        throw err;
      }
    },
    toggleAgent: async (agent: Agent, fetcher: typeof fetch = window.fetch) => {
      update((s) => ({ ...s, issue: '' }));
      try {
        await apiSetAgentEnabled(fetcher, agent.id, !agent.enabled);
        await hubStream.refreshAfterMutation(fetcher);
      } catch (err) {
        update((s) => ({ ...s, issue: 'Agent state could not be updated.' }));
        throw err;
      }
    },
    removeAgent: async (agent: Agent, fetcher: typeof fetch = window.fetch) => {
      update((s) => ({ ...s, issue: '' }));
      try {
        await apiDeleteAgent(fetcher, agent.id);
        await hubStream.refreshAfterMutation(fetcher);
        if (get(chatStore).selectedAgentId === agent.id) {
          chatStore.clearHistory();
        }
      } catch (err) {
        update((s) => ({ ...s, issue: 'Agent could not be removed.' }));
        throw err;
      }
    },
    refreshAgentCard: async (
      agentId: string,
      fetcher: typeof fetch = window.fetch
    ) => {
      if (!agentId) return;
      const refreshed = await apiRefreshAgentCard(fetcher, agentId);
      const dashboard = get(hubStream).dashboard;
      const updatedAgents = dashboard.agents.map((agent) =>
        agent.id === refreshed.id ? refreshed : agent
      );
      hubStream.updateDashboard({
        ...dashboard,
        agents: updatedAgents
      });
    },
    uploadBackground: async (
      file: File,
      fetcher: typeof fetch = window.fetch
    ) => {
      update((s) => ({ ...s, uploadingBackground: true, issue: '' }));
      try {
        const image = await apiUploadBackgroundImage(fetcher, file);
        update((s) => ({
          ...s,
          uploadingBackground: false,
          backgroundLibrary: [...s.backgroundLibrary, image].sort((a, b) =>
            a.name.localeCompare(b.name)
          )
        }));
      } catch (err) {
        update((s) => ({
          ...s,
          uploadingBackground: false,
          issue: err instanceof Error ? err.message : 'Upload failed.'
        }));
        throw err;
      }
    },
    deleteBackground: async (
      name: string,
      fetcher: typeof fetch = window.fetch
    ) => {
      update((s) => ({ ...s, issue: '' }));
      try {
        await apiDeleteBackgroundImage(fetcher, name);
        update((s) => ({
          ...s,
          backgroundLibrary: s.backgroundLibrary.filter(
            (img) => img.name !== name
          )
        }));
      } catch (err) {
        update((s) => ({ ...s, issue: 'Could not delete image.' }));
        throw err;
      }
    },
    clearIssue: () => {
      update((s) => ({ ...s, issue: '' }));
    },
    setIssue: (issue: string) => {
      update((s) => ({ ...s, issue }));
    }
  };
}

export const settingsStore = createSettingsStore();
