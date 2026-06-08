import { writable } from 'svelte/store';
import type { DisplayMode } from '$lib/types';

export interface NavigationState {
  mode: DisplayMode;
}

const initialState: NavigationState = {
  mode: 'dashboard'
};

function createNavigationStore() {
  const { subscribe, update } = writable<NavigationState>(initialState);

  return {
    subscribe,
    setMode: (mode: DisplayMode) => {
      update((s) => ({ ...s, mode }));
    },
    openChat: () => {
      update((s) => ({ ...s, mode: 'chat' }));
    },
    closeChat: () => {
      update((s) => ({ ...s, mode: 'dashboard' }));
    }
  };
}

export const navigationStore = createNavigationStore();
