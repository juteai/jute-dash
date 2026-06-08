import { describe, expect, it } from 'vitest';
import { cn, numeric, uniqueId } from './utils';

describe('utils unit tests', () => {
  describe('cn', () => {
    it('merges tailwind classes cleanly', () => {
      expect(cn('px-2 py-1', 'bg-red-500')).toBe('px-2 py-1 bg-red-500');
      expect(cn('px-2 py-1', 'px-4')).toBe('py-1 px-4');
    });
  });

  describe('numeric', () => {
    it('parses string numbers correctly', () => {
      expect(numeric('12.3')).toBe(12.3);
      expect(numeric('45')).toBe(45);
      expect(numeric('-2')).toBe(-2);
    });

    it('falls back to 0 for invalid inputs', () => {
      expect(numeric('invalid')).toBe(0);
      expect(numeric('')).toBe(0);
      expect(numeric(null as unknown as string)).toBe(0);
    });

    it('parses values from input events', () => {
      const mockEvent = {
        target: {
          value: '42.5'
        }
      } as unknown as Event;

      expect(numeric(mockEvent)).toBe(42.5);
    });
  });

  describe('uniqueId', () => {
    it('generates random ID without prefix', () => {
      const id1 = uniqueId();
      const id2 = uniqueId();
      expect(id1).not.toBe(id2);
      expect(id1.length).toBeGreaterThan(0);
    });

    it('generates sequential IDs with prefix when not existing', () => {
      const id = uniqueId('test');
      expect(id).toBe('test-1');
    });

    it('avoids existing IDs when generating sequential IDs', () => {
      const id = uniqueId('test', ['test-1', 'test-2']);
      expect(id).toBe('test-3');
    });

    it('falls back to random suffix when sequential IDs are exhausted', () => {
      const existing = Array.from({ length: 999 }, (_, i) => `test-${i + 1}`);
      const id = uniqueId('test', existing);
      expect(id.startsWith('test-')).toBe(true);
      expect(id).not.toContain('test-1000');
    });
  });
});
