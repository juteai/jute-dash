import { clsx, type ClassValue } from 'clsx';
import { twMerge } from 'tailwind-merge';

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}

/** Coerce a string value, input event, or null/undefined to a number, defaulting to 0. */
export function numeric(value: string | Event | null | undefined): number {
  if (!value) return 0;
  const val =
    typeof value === 'string'
      ? value
      : (value.target as HTMLInputElement)?.value;
  const parsed = Number.parseFloat(val || '');
  return Number.isFinite(parsed) ? parsed : 0;
}

/** Generate a short unique ID for new list items, sequential if possible, otherwise random. */
export function uniqueId(prefix = '', existing: string[] = []): string {
  if (prefix) {
    const taken = new Set(existing);
    for (let index = 1; index < 1000; index += 1) {
      const candidate = `${prefix}-${index}`;
      if (!taken.has(candidate)) {
        return candidate;
      }
    }
    return `${prefix}-${Math.random().toString(36).substring(2, 6)}`;
  }
  return Math.random().toString(36).substring(2, 10);
}
