import { afterEach, describe, expect, it, vi } from 'vitest';
import { createSpotifyPlayerSession } from '../../../../widgets/spotify/web/playerSession';

describe('spotify player session', () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('wires Spotify SDK player lifecycle events', async () => {
    const listeners: Record<string, (payload: unknown) => void> = {};
    const connect = vi.fn(async () => true);
    const disconnect = vi.fn();
    const activateElement = vi.fn(async () => {});

    class FakePlayer {
      addListener(event: string, callback: (payload: unknown) => void) {
        listeners[event] = callback;
        return true;
      }

      connect = connect;
      disconnect = disconnect;
      activateElement = activateElement;
    }

    vi.stubGlobal('window', {
      Spotify: {
        Player: FakePlayer
      }
    });

    const onReady = vi.fn();
    const onNotReady = vi.fn();
    const onStateChanged = vi.fn();
    const onIssue = vi.fn();

    const session = await createSpotifyPlayerSession({
      volume: 75,
      getOAuthToken: async () => 'token',
      onReady,
      onNotReady,
      onStateChanged,
      onIssue
    });

    await expect(session.connect()).resolves.toBe(true);
    listeners.ready({ device_id: 'jute-player' });
    listeners.not_ready({});
    listeners.player_state_changed({ paused: false, position: 5000 });
    listeners.authentication_error({ message: 'login expired' });
    await session.activateElement();
    session.disconnect();

    expect(connect).toHaveBeenCalledOnce();
    expect(onReady).toHaveBeenCalledWith('jute-player');
    expect(onNotReady).toHaveBeenCalledOnce();
    expect(onStateChanged).toHaveBeenCalledWith({
      paused: false,
      position: 5000
    });
    expect(onIssue).toHaveBeenCalledWith('login expired');
    expect(activateElement).toHaveBeenCalledOnce();
    expect(disconnect).toHaveBeenCalledOnce();
  });
});
