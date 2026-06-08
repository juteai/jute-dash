import { describe, expect, it } from 'vitest';
import { navigationStore } from './navigationStore';
import { get } from 'svelte/store';

describe('navigationStore', () => {
  it('should have initial state mode as dashboard', () => {
    const state = get(navigationStore);
    expect(state.mode).toBe('dashboard');
  });

  it('should change mode when calling setMode', () => {
    navigationStore.setMode('edit');
    expect(get(navigationStore).mode).toBe('edit');
  });

  it('should change mode to chat when calling openChat', () => {
    navigationStore.openChat();
    expect(get(navigationStore).mode).toBe('chat');
  });

  it('should change mode back to dashboard when calling closeChat', () => {
    navigationStore.closeChat();
    expect(get(navigationStore).mode).toBe('dashboard');
  });
});
