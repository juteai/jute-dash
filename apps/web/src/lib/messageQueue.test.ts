import { describe, expect, it, vi, beforeEach, type Mock } from 'vitest';
import { createMessageQueue, type QueueCallbacks } from './messageQueue';
import type { Agent } from '$lib/types';

describe('messageQueue', () => {
  let callbacks: QueueCallbacks;
  let submitMessageMock: Mock;
  let startDismissTimerMock: Mock;
  let stopDismissTimerMock: Mock;
  let isSettledMock: Mock;
  let onQueueChangeMock: Mock;
  let onMessageSendingMock: Mock;

  beforeEach(() => {
    submitMessageMock = vi.fn().mockResolvedValue(undefined);
    startDismissTimerMock = vi.fn();
    stopDismissTimerMock = vi.fn();
    isSettledMock = vi.fn().mockReturnValue(true);
    onQueueChangeMock = vi.fn();
    onMessageSendingMock = vi.fn();

    callbacks = {
      submitMessage:
        submitMessageMock as unknown as QueueCallbacks['submitMessage'],
      startDismissTimer: startDismissTimerMock,
      stopDismissTimer: stopDismissTimerMock,
      isSettled: isSettledMock,
      onQueueChange: onQueueChangeMock,
      onMessageSending: onMessageSendingMock
    };
  });

  it('should initialize empty', () => {
    const q = createMessageQueue(callbacks);
    expect(q.isEmpty()).toBe(true);
    expect(q.peek()).toBeUndefined();
  });

  it('should enqueue items and trigger callbacks', () => {
    const q = createMessageQueue(callbacks);
    q.enqueue('msg-1', 'Hello');
    expect(q.isEmpty()).toBe(false);
    expect(q.peek()).toEqual({ id: 'msg-1', text: 'Hello' });
    expect(onQueueChangeMock).toHaveBeenCalledWith([
      { id: 'msg-1', text: 'Hello' }
    ]);
  });

  it('should cancel queue items', () => {
    const q = createMessageQueue(callbacks);
    q.enqueue('msg-1', 'Hello');
    q.enqueue('msg-2', 'World');
    const canceled = q.cancel();

    expect(canceled).toEqual(['msg-1', 'msg-2']);
    expect(q.isEmpty()).toBe(true);
    expect(onQueueChangeMock).toHaveBeenLastCalledWith([]);
  });

  it('should drain items in sequence', async () => {
    const q = createMessageQueue(callbacks);
    const mockAgents = [] as Agent[];
    const mockFetch = vi.fn() as unknown as typeof fetch;

    q.enqueue('msg-1', 'Hello');
    q.enqueue('msg-2', 'World');

    await q.drain(mockAgents, mockFetch);

    expect(stopDismissTimerMock).toHaveBeenCalledTimes(2);
    expect(onMessageSendingMock).toHaveBeenCalledWith('msg-1');
    expect(onMessageSendingMock).toHaveBeenCalledWith('msg-2');
    expect(submitMessageMock).toHaveBeenCalledWith(
      'Hello',
      mockAgents,
      'msg-1',
      mockFetch,
      undefined,
      undefined
    );
    expect(submitMessageMock).toHaveBeenCalledWith(
      'World',
      mockAgents,
      'msg-2',
      mockFetch,
      undefined,
      undefined
    );
    expect(startDismissTimerMock).toHaveBeenCalled();
    expect(q.isEmpty()).toBe(true);
  });

  it('should continue draining on errors', async () => {
    submitMessageMock.mockRejectedValueOnce(new Error('API failure'));
    const q = createMessageQueue(callbacks);
    const mockAgents = [] as Agent[];
    const mockFetch = vi.fn() as unknown as typeof fetch;

    q.enqueue('msg-1', 'Hello');
    q.enqueue('msg-2', 'World');

    await q.drain(mockAgents, mockFetch);

    expect(submitMessageMock).toHaveBeenCalledTimes(2);
    expect(q.isEmpty()).toBe(true);
  });

  it('should not allow concurrent draining', async () => {
    let resolveFirstSubmit: () => void = () => {};
    submitMessageMock.mockImplementationOnce(() => {
      return new Promise<void>((resolve) => {
        resolveFirstSubmit = resolve;
      });
    });

    const q = createMessageQueue(callbacks);
    const mockAgents = [] as Agent[];
    const mockFetch = vi.fn() as unknown as typeof fetch;

    q.enqueue('msg-1', 'Hello');

    const p1 = q.drain(mockAgents, mockFetch);
    const p2 = q.drain(mockAgents, mockFetch);

    resolveFirstSubmit();
    await Promise.all([p1, p2]);

    expect(submitMessageMock).toHaveBeenCalledTimes(1);
  });
});
