import { describe, expect, it } from 'vitest';
import {
  looksLikeToolInvocation,
  isReasoningArtifact,
  sanitizeDisplayText,
  getPartText,
  getPartData,
  textFromParts,
  isStructuredArtifact,
  textFromReasoningParts
} from './displaySanitizer';
import type { Part as A2APart } from '@a2a-js/sdk';

describe('displaySanitizer unit tests', () => {
  describe('looksLikeToolInvocation', () => {
    it('detects ADK-style tool invocations', () => {
      expect(looksLikeToolInvocation('my_tool - {"arg": 1}')).toBe(true);
      expect(looksLikeToolInvocation('  mcp_tool - {  }')).toBe(true);
      expect(looksLikeToolInvocation('jute_skill_read - {"id": "1"}')).toBe(
        true
      );
    });

    it('detects hub prefixes', () => {
      expect(looksLikeToolInvocation('jute_skill_something')).toBe(true);
      expect(looksLikeToolInvocation('mcp_something')).toBe(true);
      expect(looksLikeToolInvocation('function_call_something')).toBe(true);
    });

    it('detects bare JSON responses', () => {
      expect(looksLikeToolInvocation('{"skillId": "123"}')).toBe(true);
      expect(looksLikeToolInvocation('{"tool_call_id": "abc"}')).toBe(true);
      expect(looksLikeToolInvocation('{"function_call": "run"}')).toBe(true);
      expect(looksLikeToolInvocation('{"actionId": "xyz"}')).toBe(true);
    });

    it('returns false for normal text', () => {
      expect(looksLikeToolInvocation('')).toBe(false);
      expect(looksLikeToolInvocation('Hello world')).toBe(false);
      expect(looksLikeToolInvocation('Not a tool call - {')).toBe(false);
    });
  });

  describe('sanitizeDisplayText', () => {
    it('returns empty string for empty input', () => {
      expect(sanitizeDisplayText('')).toBe('');
      expect(sanitizeDisplayText('   ')).toBe('');
    });

    it('removes think tags and open thinking tags', () => {
      expect(
        sanitizeDisplayText('Hello <think>secret thought</think> World')
      ).toBe('Hello  World');
      expect(
        sanitizeDisplayText('Hello <thinking>thinking...</thinking> World')
      ).toBe('Hello  World');
      expect(sanitizeDisplayText('Hello <think>thinking forever...')).toBe(
        'Hello'
      );
    });

    it('removes reasoning blocks', () => {
      expect(
        sanitizeDisplayText('Hello <reasoning>evaluating</reasoning> World')
      ).toBe('Hello  World');
      expect(
        sanitizeDisplayText('Hello <scratchpad>notes</scratchpad> World')
      ).toBe('Hello  World');
      expect(
        sanitizeDisplayText('Hello ```thinking\nscratchpad notes\n``` World')
      ).toBe('Hello  World');
    });

    it('removes paragraphs that look like reasoning', () => {
      const text =
        'Okay, the user wants me to calculate 2+2.\n\nI should call the calculate tool.\n\nThe final answer is 4.';
      expect(sanitizeDisplayText(text)).toBe('The final answer is 4.');
    });

    it('removes standalone reasoning when no final answer has arrived', () => {
      const text =
        'Okay, the user is asking for the weather today. I need to check the available Widget Skills first. Let me call jute_skill_list to see which skills are available.';
      expect(sanitizeDisplayText(text)).toBe('');
    });
  });

  describe('getPartText', () => {
    it('extracts text from various part structures', () => {
      const part1 = { text: 'hello' } as unknown as A2APart;
      const part2 = {
        content: { $case: 'text', value: 'world' }
      } as unknown as A2APart;
      const part3 = {
        content: { $case: 'data', value: {} }
      } as unknown as A2APart;

      expect(getPartText(part1)).toBe('hello');
      expect(getPartText(part2)).toBe('world');
      expect(getPartText(part3)).toBe('');
    });
  });

  describe('getPartData', () => {
    it('extracts data from various part structures', () => {
      const part1 = { data: { id: '123' } } as unknown as A2APart;
      const part2 = {
        content: { $case: 'data', value: { name: 'test' } }
      } as unknown as A2APart;
      const part3 = {
        content: { $case: 'text', value: 'hello' }
      } as unknown as A2APart;

      expect(getPartData(part1)).toEqual({ id: '123' });
      expect(getPartData(part2)).toEqual({ name: 'test' });
      expect(getPartData(part3)).toBeUndefined();
    });
  });

  describe('textFromParts', () => {
    it('concatenates text parts and filters thoughts/tool calls', () => {
      const parts: A2APart[] = [
        { text: 'Hello ' } as unknown as A2APart,
        {
          text: 'thinking',
          metadata: { adk_thought: true }
        } as unknown as A2APart,
        { text: 'world' } as unknown as A2APart,
        { text: 'jute_skill_read - {}' } as unknown as A2APart
      ];

      expect(textFromParts(parts)).toBe('Hello world');
    });
  });

  describe('isStructuredArtifact', () => {
    it('returns true if any non-thought part has non-text structure or mediaType', () => {
      const parts1: A2APart[] = [
        { text: 'Hello', mediaType: 'text/markdown' } as unknown as A2APart
      ];
      const parts2: A2APart[] = [
        { text: 'Image', mediaType: 'image/png' } as unknown as A2APart
      ];
      const parts3: A2APart[] = [
        { content: { $case: 'data', value: {} } } as unknown as A2APart
      ];
      const parts4: A2APart[] = [
        {
          text: 'thought',
          metadata: { adk_thought: true },
          mediaType: 'image/png'
        } as unknown as A2APart
      ];

      expect(isStructuredArtifact(parts1)).toBe(false);
      expect(isStructuredArtifact(parts2)).toBe(true);
      expect(isStructuredArtifact(parts3)).toBe(true);
      expect(isStructuredArtifact(parts4)).toBe(false);
    });
  });

  describe('textFromReasoningParts', () => {
    it('concatenates reasoning text parts', () => {
      const parts: A2APart[] = [
        { text: 'Thought 1. ' } as unknown as A2APart,
        {
          content: { $case: 'text', value: 'Thought 2.' }
        } as unknown as A2APart
      ];

      expect(textFromReasoningParts(parts)).toBe('Thought 1. Thought 2.');
    });
  });

  describe('isReasoningArtifact', () => {
    it('detects reasoning artifacts correctly', () => {
      const art1 = {
        parts: [
          {
            text: 'Thought part',
            metadata: { adk_thought: true }
          } as unknown as A2APart,
          { text: 'mcp_tool - {}' } as unknown as A2APart
        ]
      };
      const art2 = {
        parts: [{ text: 'Normal text' } as unknown as A2APart]
      };

      expect(isReasoningArtifact(art1)).toBe(true);
      expect(isReasoningArtifact(art2)).toBe(false);
      expect(isReasoningArtifact(undefined)).toBe(false);
    });
  });
});
