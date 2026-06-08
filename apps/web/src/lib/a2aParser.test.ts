import { describe, expect, it, vi } from 'vitest';
import { Role, TaskState } from '@a2a-js/sdk';
import {
  statusFromTask,
  statusFromState,
  isTerminalTaskState,
  terminalTaskFailureMessage,
  parseTasksToConversationDetail,
  newUserMessage,
  localConversationDetail
} from './a2aParser';
import type { Task as A2ATask } from '@a2a-js/sdk';

// Mock crypto.randomUUID for predictability
if (typeof crypto === 'undefined') {
  globalThis.crypto = {
    randomUUID: () => '00000000-0000-0000-0000-000000000000'
  } as unknown as Crypto;
} else {
  vi.spyOn(crypto, 'randomUUID').mockReturnValue(
    '00000000-0000-0000-0000-000000000000'
  );
}

describe('a2aParser unit tests', () => {
  describe('statusFromState & statusFromTask', () => {
    it('maps TaskState to string statuses correctly', () => {
      expect(statusFromState(TaskState.TASK_STATE_COMPLETED)).toBe('completed');
      expect(statusFromState(TaskState.TASK_STATE_FAILED)).toBe('failed');
      expect(statusFromState(TaskState.TASK_STATE_WORKING)).toBe('working');
      expect(statusFromState(TaskState.TASK_STATE_SUBMITTED)).toBe('submitted');
      expect(statusFromState(TaskState.TASK_STATE_CANCELED)).toBe('canceled');
      expect(statusFromState(TaskState.TASK_STATE_REJECTED)).toBe('rejected');
    });

    it('maps A2ATask status accurately', () => {
      const task = {
        id: 'task-1',
        contextId: 'ctx-1',
        status: {
          state: TaskState.TASK_STATE_WORKING,
          timestamp: '2026-06-08T15:00:00Z',
          message: { parts: [] }
        },
        input: { parts: [] },
        history: [],
        extensions: []
      } as unknown as A2ATask;

      expect(statusFromTask(task)).toBe('working');
    });

    it('falls back to completed if task has no status', () => {
      const task = {
        id: 'task-1',
        contextId: 'ctx-1',
        input: { parts: [] },
        history: [],
        extensions: []
      } as unknown as A2ATask;

      expect(statusFromTask(task)).toBe('completed');
    });
  });

  describe('isTerminalTaskState', () => {
    it('identifies terminal and non-terminal states', () => {
      expect(isTerminalTaskState(TaskState.TASK_STATE_COMPLETED)).toBe(true);
      expect(isTerminalTaskState(TaskState.TASK_STATE_FAILED)).toBe(true);
      expect(isTerminalTaskState(TaskState.TASK_STATE_CANCELED)).toBe(true);
      expect(isTerminalTaskState(TaskState.TASK_STATE_REJECTED)).toBe(true);

      expect(isTerminalTaskState(TaskState.TASK_STATE_WORKING)).toBe(false);
      expect(isTerminalTaskState(TaskState.TASK_STATE_SUBMITTED)).toBe(false);
    });
  });

  describe('terminalTaskFailureMessage', () => {
    it('returns appropriate failure messages', () => {
      expect(terminalTaskFailureMessage('failed')).toBe('Agent task failed');
      expect(terminalTaskFailureMessage('canceled')).toBe(
        'Agent canceled the request'
      );
      expect(terminalTaskFailureMessage('rejected')).toBe(
        'Agent rejected the request'
      );
      expect(terminalTaskFailureMessage('completed')).toBeUndefined();
    });
  });

  describe('newUserMessage', () => {
    it('generates a formatted A2A user message', () => {
      const msg = newUserMessage('ctx-1', 'hello agent');

      expect(msg.messageId).toBe('00000000-0000-0000-0000-000000000000');
      expect(msg.contextId).toBe('ctx-1');
      expect(msg.role).toBe(Role.ROLE_USER);
      expect(msg.parts).toHaveLength(1);
      expect(msg.parts[0].content).toEqual({
        $case: 'text',
        value: 'hello agent'
      });
    });
  });

  describe('localConversationDetail', () => {
    it('generates local conversation and user/assistant messages', () => {
      const detail = localConversationDetail(
        'ctx-1',
        'agent-1',
        'Ping',
        'Pong',
        'task-1',
        'completed'
      );

      expect(detail.conversation.id).toBe('ctx-1');
      expect(detail.conversation.agentId).toBe('agent-1');
      expect(detail.conversation.title).toBe('Ping');
      expect(detail.conversation.status).toBe('completed');
      expect(detail.conversation.historyUnsupported).toBe(true);

      expect(detail.messages).toHaveLength(2);
      expect(detail.messages[0].role).toBe('user');
      expect(detail.messages[0].content).toBe('Ping');
      expect(detail.messages[1].role).toBe('assistant');
      expect(detail.messages[1].content).toBe('Pong');
    });
  });

  describe('parseTasksToConversationDetail', () => {
    it('returns empty details when no tasks match the conversationId', () => {
      const detail = parseTasksToConversationDetail([], 'ctx-1', 'agent-1');
      expect(detail.conversation).toBeDefined();
      expect(detail.conversation.id).toBe('ctx-1');
      expect(detail.conversation.status).toBe('idle');
      expect(detail.messages).toEqual([]);
    });

    it('parses basic user and agent tasks into a full conversation history', () => {
      const tasks = [
        {
          id: 'task-1',
          contextId: 'ctx-1',
          status: {
            state: TaskState.TASK_STATE_COMPLETED,
            timestamp: '2026-06-08T15:00:00Z',
            message: { parts: [] }
          },
          input: {
            messageId: 'msg-user-1',
            contextId: 'ctx-1',
            role: Role.ROLE_USER,
            parts: [
              {
                content: {
                  $case: 'text',
                  value: 'What is the capital of France?'
                }
              }
            ]
          },
          history: [
            {
              messageId: 'msg-assistant-1',
              contextId: 'ctx-1',
              role: Role.ROLE_AGENT,
              parts: [
                {
                  content: {
                    $case: 'text',
                    value: 'The capital of France is Paris.'
                  }
                }
              ]
            }
          ],
          extensions: []
        }
      ] as unknown as A2ATask[];

      const detail = parseTasksToConversationDetail(tasks, 'ctx-1', 'agent-1');

      expect(detail.conversation).toBeDefined();
      expect(detail.conversation?.id).toBe('ctx-1');
      expect(detail.conversation?.agentId).toBe('agent-1');
      expect(detail.conversation?.title).toBe('What is the capital of France?');
      expect(detail.conversation?.status).toBe('completed');

      expect(detail.messages).toHaveLength(2);
      expect(detail.messages[0].role).toBe('user');
      expect(detail.messages[0].content).toBe('What is the capital of France?');
      expect(detail.messages[0].a2aMessageId).toBe('msg-user-1');

      expect(detail.messages[1].role).toBe('assistant');
      expect(detail.messages[1].content).toBe(
        'The capital of France is Paris.'
      );
      expect(detail.messages[1].a2aMessageId).toBe('msg-assistant-1');
    });

    it('sorts multiple tasks chronologically', () => {
      const tasks = [
        {
          id: 'task-2',
          contextId: 'ctx-1',
          status: {
            state: TaskState.TASK_STATE_COMPLETED,
            timestamp: '2026-06-08T15:10:00Z',
            message: { parts: [] }
          },
          input: {
            messageId: 'msg-user-2',
            contextId: 'ctx-1',
            role: Role.ROLE_USER,
            parts: [{ content: { $case: 'text', value: 'Thank you!' } }]
          },
          history: [
            {
              messageId: 'msg-assistant-2',
              contextId: 'ctx-1',
              role: Role.ROLE_AGENT,
              parts: [{ content: { $case: 'text', value: 'You are welcome!' } }]
            }
          ],
          extensions: []
        },
        {
          id: 'task-1',
          contextId: 'ctx-1',
          status: {
            state: TaskState.TASK_STATE_COMPLETED,
            timestamp: '2026-06-08T15:00:00Z',
            message: { parts: [] }
          },
          input: {
            messageId: 'msg-user-1',
            contextId: 'ctx-1',
            role: Role.ROLE_USER,
            parts: [{ content: { $case: 'text', value: 'Hello' } }]
          },
          history: [
            {
              messageId: 'msg-assistant-1',
              contextId: 'ctx-1',
              role: Role.ROLE_AGENT,
              parts: [{ content: { $case: 'text', value: 'Hi there!' } }]
            }
          ],
          extensions: []
        }
      ] as unknown as A2ATask[];

      const detail = parseTasksToConversationDetail(tasks, 'ctx-1', 'agent-1');

      expect(detail.messages).toHaveLength(4);
      expect(detail.messages[0].content).toBe('Hello');
      expect(detail.messages[1].content).toBe('Hi there!');
      expect(detail.messages[2].content).toBe('Thank you!');
      expect(detail.messages[3].content).toBe('You are welcome!');
    });

    it('extracts reasoning steps and tool calls from parts', () => {
      const tasks = [
        {
          id: 'task-1',
          contextId: 'ctx-1',
          status: {
            state: TaskState.TASK_STATE_COMPLETED,
            timestamp: '2026-06-08T15:00:00Z',
            message: { parts: [] }
          },
          input: {
            messageId: 'msg-user-1',
            contextId: 'ctx-1',
            role: Role.ROLE_USER,
            parts: [{ content: { $case: 'text', value: 'Hello' } }]
          },
          history: [
            {
              messageId: 'msg-assistant-1',
              contextId: 'ctx-1',
              role: Role.ROLE_AGENT,
              parts: [
                {
                  content: {
                    $case: 'text',
                    value: 'Thinking: I should query weather.'
                  },
                  metadata: {
                    adk_thought: true
                  }
                },
                {
                  content: {
                    $case: 'text',
                    value: 'weather_get - {"city": "Paris"}'
                  },
                  metadata: undefined
                },
                {
                  content: {
                    $case: 'text',
                    value: 'It is sunny in Paris.'
                  },
                  metadata: undefined
                }
              ]
            }
          ],
          extensions: []
        }
      ] as unknown as A2ATask[];

      const detail = parseTasksToConversationDetail(tasks, 'ctx-1', 'agent-1');
      const assistantMessage = detail.messages[1];

      expect(assistantMessage.role).toBe('assistant');
      expect(assistantMessage.content).toBe('It is sunny in Paris.');
      expect(assistantMessage.interimSteps).toHaveLength(2);
      expect(assistantMessage.interimSteps?.[0].text).toBe(
        'Thinking: I should query weather.'
      );
      expect(assistantMessage.interimSteps?.[0].status).toBe('completed');
      expect(assistantMessage.interimSteps?.[1].text).toBe(
        'Called tool: weather_get'
      );
      expect(assistantMessage.interimSteps?.[1].args).toEqual({
        city: 'Paris'
      });
    });
  });
});
