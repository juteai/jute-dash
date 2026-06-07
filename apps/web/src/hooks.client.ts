import { logger } from '$lib/logger';
import type { HandleClientError } from '@sveltejs/kit';

export const handleError: HandleClientError = ({ error, event }) => {
  const message = error instanceof Error ? error.message : String(error);
  const stack = error instanceof Error ? error.stack : '';
  logger.error(
    `Uncaught routing or rendering crash at ${event.url.pathname}: ${message}`,
    { stack }
  );

  return {
    message: 'An unexpected client error occurred.'
  };
};

if (typeof window !== 'undefined') {
  // Listen to uncaught global window errors
  window.addEventListener('error', (event) => {
    logger.error(
      `Uncaught exception: ${event.message} at ${event.filename}:${event.lineno}:${event.colno}`,
      event.error
    );
  });

  window.addEventListener('unhandledrejection', (event) => {
    logger.error(`Unhandled promise rejection: ${event.reason}`);
  });

  // Monkey-patch window.fetch to automatically log all Jute API traffic
  const originalFetch = window.fetch;
  window.fetch = async (input, init) => {
    const start = performance.now();
    const url =
      typeof input === 'string'
        ? input
        : input instanceof URL
          ? input.toString()
          : input.url;
    const method = init?.method ?? 'GET';

    const isJuteApi = url.includes('/api/v1/') || url.includes('/healthz');

    try {
      const response = await originalFetch(input, init);
      const duration = performance.now() - start;
      if (isJuteApi) {
        if (response.ok) {
          logger.api(method, url, response.status, duration);
        } else {
          let errMsg = `Response status ${response.statusText || response.status}`;
          try {
            const clone = response.clone();
            const body = await clone.json();
            if (body && typeof body.error === 'string') {
              errMsg = body.error;
            }
          } catch {
            // ignore JSON parse failures
          }
          logger.apiError(method, url, response.status, duration, errMsg);
        }
      }
      return response;
    } catch (err) {
      const duration = performance.now() - start;
      if (isJuteApi) {
        logger.apiError(
          method,
          url,
          0,
          duration,
          err instanceof Error ? err.message : String(err)
        );
      }
      throw err;
    }
  };
}
