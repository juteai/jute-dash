import type { Agent, UserFacingIssue } from '$lib/types';

export interface QueueCallbacks {
  submitMessage: (
    text: string,
    agents: Agent[],
    retryMessageId: string,
    fetcher: typeof fetch,
    onMarkConnected?: () => void,
    onMarkIssue?: (issue: UserFacingIssue) => void
  ) => Promise<void>;
  startDismissTimer: () => void;
  stopDismissTimer: () => void;
  isSettled: () => boolean;
  onQueueChange?: (queue: { id: string; text: string }[]) => void;
  onMessageSending?: (id: string) => void;
}

export function createMessageQueue(callbacks: QueueCallbacks): {
  enqueue: (id: string, text: string) => void;
  drain: (
    agents: Agent[],
    fetcher: typeof fetch,
    onMarkConnected?: () => void,
    onMarkIssue?: (issue: UserFacingIssue) => void
  ) => Promise<void>;
  cancel: () => string[]; // returns IDs of canceled messages
  peek: () => { id: string; text: string } | undefined;
  isEmpty: () => boolean;
} {
  let queue: { id: string; text: string }[] = [];
  let draining = false;

  const emit = () => {
    if (callbacks.onQueueChange) {
      callbacks.onQueueChange([...queue]);
    }
  };

  const drain = async (
    agents: Agent[],
    fetcher: typeof fetch,
    onMarkConnected?: () => void,
    onMarkIssue?: (issue: UserFacingIssue) => void
  ) => {
    if (draining) return;
    draining = true;

    try {
      while (queue.length > 0) {
        callbacks.stopDismissTimer();
        const next = queue[0];
        queue = queue.slice(1);
        emit();

        if (callbacks.onMessageSending) {
          callbacks.onMessageSending(next.id);
        }

        try {
          await callbacks.submitMessage(
            next.text,
            agents,
            next.id,
            fetcher,
            onMarkConnected,
            onMarkIssue
          );
        } catch {
          // Turn logic handles individual message turn errors; continue draining
        }
      }
    } finally {
      draining = false;
      if (callbacks.isSettled()) {
        callbacks.startDismissTimer();
      }
    }
  };

  return {
    enqueue: (id: string, text: string) => {
      queue.push({ id, text });
      emit();
    },
    drain,
    cancel: () => {
      const canceledIds = queue.map((item) => item.id);
      queue = [];
      emit();
      return canceledIds;
    },
    peek: () => queue[0],
    isEmpty: () => queue.length === 0
  };
}
