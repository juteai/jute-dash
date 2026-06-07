/**
 * Simple client-side logger for Jute Web with a consistent filterable prefix.
 */
export const logger = {
  info(msg: string, ...args: unknown[]) {
    console.log(
      `%c[Jute Web] [Info]%c ${msg}`,
      'color: #3b82f6; font-weight: bold;',
      '',
      ...args
    );
  },
  warn(msg: string, ...args: unknown[]) {
    console.warn(
      `%c[Jute Web] [Warn]%c ${msg}`,
      'color: #f59e0b; font-weight: bold;',
      '',
      ...args
    );
  },
  error(msg: string, ...args: unknown[]) {
    console.error(
      `%c[Jute Web] [Error]%c ${msg}`,
      'color: #ef4444; font-weight: bold;',
      '',
      ...args
    );
  },
  api(method: string, url: string, status: number, durationMs: number) {
    const urlPath = getRelativePath(url);
    console.log(
      `%c[Jute Web] [API]%c ${method} ${urlPath} | Status ${status} | ${durationMs.toFixed(1)}ms`,
      'color: #10b981; font-weight: bold;',
      ''
    );
  },
  apiError(
    method: string,
    url: string,
    status: number,
    durationMs: number,
    err: string
  ) {
    const urlPath = getRelativePath(url);
    const statusText = status > 0 ? `Status ${status}` : 'Network Error';
    console.error(
      `%c[Jute Web] [API] [Failure]%c ${method} ${urlPath} | ${statusText} | ${durationMs.toFixed(1)}ms | ${err}`,
      'color: #ef4444; font-weight: bold;',
      ''
    );
  },
  sse(event: string, details?: string) {
    const suffix = details ? ` | ${details}` : '';
    console.log(
      `%c[Jute Web] [SSE]%c Event: ${event}${suffix}`,
      'color: #8b5cf6; font-weight: bold;',
      ''
    );
  },
  sseError(err: string) {
    console.error(
      `%c[Jute Web] [SSE] [Error]%c ${err}`,
      'color: #ef4444; font-weight: bold;',
      ''
    );
  }
};

function getRelativePath(url: string): string {
  try {
    const parsed = new URL(url);
    return parsed.pathname + parsed.search;
  } catch {
    return url;
  }
}
