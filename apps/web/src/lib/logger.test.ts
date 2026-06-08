import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest';
import { logger } from './logger';

describe('logger', () => {
  beforeEach(() => {
    vi.spyOn(console, 'log').mockImplementation(() => {});
    vi.spyOn(console, 'warn').mockImplementation(() => {});
    vi.spyOn(console, 'error').mockImplementation(() => {});
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('logs info messages', () => {
    logger.info('hello info', { data: 123 });
    expect(console.log).toHaveBeenCalledWith(
      '%c[Jute Web] [Info]%c hello info',
      'color: #3b82f6; font-weight: bold;',
      '',
      { data: 123 }
    );
  });

  it('logs warning messages', () => {
    logger.warn('hello warn', 'arg1');
    expect(console.warn).toHaveBeenCalledWith(
      '%c[Jute Web] [Warn]%c hello warn',
      'color: #f59e0b; font-weight: bold;',
      '',
      'arg1'
    );
  });

  it('logs error messages', () => {
    logger.error('hello error', new Error('test'));
    expect(console.error).toHaveBeenCalledWith(
      '%c[Jute Web] [Error]%c hello error',
      'color: #ef4444; font-weight: bold;',
      '',
      expect.any(Error)
    );
  });

  it('logs api calls with relative url path', () => {
    logger.api(
      'GET',
      'http://localhost:8080/api/v1/status?param=value',
      200,
      15.234
    );
    expect(console.log).toHaveBeenCalledWith(
      '%c[Jute Web] [API]%c GET /api/v1/status?param=value | Status 200 | 15.2ms',
      'color: #10b981; font-weight: bold;',
      ''
    );
  });

  it('logs api errors', () => {
    logger.apiError(
      'POST',
      'http://localhost:8080/api/v1/settings',
      500,
      45.67,
      'Internal Server Error'
    );
    expect(console.error).toHaveBeenCalledWith(
      '%c[Jute Web] [API] [Failure]%c POST /api/v1/settings | Status 500 | 45.7ms | Internal Server Error',
      'color: #ef4444; font-weight: bold;',
      ''
    );
  });

  it('logs sse events', () => {
    logger.sse('update', 'data updated');
    expect(console.log).toHaveBeenCalledWith(
      '%c[Jute Web] [SSE]%c Event: update | data updated',
      'color: #8b5cf6; font-weight: bold;',
      ''
    );
  });

  it('logs sse errors', () => {
    logger.sseError('connection lost');
    expect(console.error).toHaveBeenCalledWith(
      '%c[Jute Web] [SSE] [Error]%c connection lost',
      'color: #ef4444; font-weight: bold;',
      ''
    );
  });
});
