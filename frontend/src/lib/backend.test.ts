import { afterEach, describe, expect, it, vi } from 'vitest';
import { BackendRequestError, requestJSON, resolveBackendConfig } from './backend';

afterEach(() => vi.restoreAllMocks());

const local = { backendUrl: 'http://127.0.0.1:21001', token: 'synthetic-token' };

describe('backend connection boundary', () => {
  it('chooses desktop, canonical environment, then old environment by precedence', async () => {
    const env = { VITE_MYSTOCKTRACER_BACKEND_URL: 'http://127.0.0.1:21002', VITE_MYSTOCKTRACER_TOKEN: 'canonical',
      VITE_A_STOCK_BACKEND_URL: 'http://127.0.0.1:21003', VITE_A_STOCK_TOKEN: 'historical' };
    const bridge = { getBackendConfig: async () => local };
    expect(await resolveBackendConfig({ bridge, env })).toEqual(local);
    expect(await resolveBackendConfig({ env })).toEqual({ backendUrl: env.VITE_MYSTOCKTRACER_BACKEND_URL, token: 'canonical' });
    expect(await resolveBackendConfig({ env: { VITE_A_STOCK_BACKEND_URL: env.VITE_A_STOCK_BACKEND_URL } }))
      .toEqual({ backendUrl: env.VITE_A_STOCK_BACKEND_URL, token: '' });
  });

  it('rejects unsafe configuration and cross-origin paths before contacting the network', async () => {
    const fetcher = vi.spyOn(globalThis, 'fetch');
    for (const unsafe of ['file:///local/path', 'https://remote.example/api']) {
      await expect(resolveBackendConfig({ env: { VITE_MYSTOCKTRACER_BACKEND_URL: unsafe } }))
        .rejects.toMatchObject({ code: 'invalid_backend_url' });
    }
    await expect(requestJSON(local, 'https://remote.example/collect')).rejects.toMatchObject({ code: 'cross_origin_backend_request' });
    await expect(requestJSON({ backendUrl: 'https://remote.example', token: local.token }, '/collect'))
      .rejects.toMatchObject({ code: 'invalid_backend_url' });
    expect(fetcher).not.toHaveBeenCalled();
  });

  it('sends credentials only to loopback with a request ID and caller abort signal', async () => {
    const fetcher = vi.spyOn(globalThis, 'fetch').mockResolvedValue(new Response('{"ok":true}', { status: 200 }));
    const controller = new AbortController();
    await expect(requestJSON<{ ok: boolean }>(local, '/api/health', { signal: controller.signal }))
      .resolves.toEqual({ ok: true });
    const init = fetcher.mock.calls[0][1];
    const headers = init?.headers as Headers;
    expect(headers.get('Authorization')).toBe('Bearer synthetic-token');
    expect(headers.get('X-Request-ID')).toMatch(/\S+/);
    expect(init?.signal).toBe(controller.signal);
  });

  it('treats empty success as no data and rejects invalid success JSON', async () => {
    const fetcher = vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(new Response('', { status: 200 }))
      .mockResolvedValueOnce(new Response('<html>invalid</html>', { status: 200 }));
    await expect(requestJSON(local, '/api/health')).resolves.toBeUndefined();
    await expect(requestJSON(local, '/api/health')).rejects.toBeInstanceOf(BackendRequestError);
    expect(fetcher).toHaveBeenCalledTimes(2);
  });

  it('surfaces a typed HTTP failure with correlation ID', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValue(new Response('{"error":"拒絕請求","code":"denied"}', {
      status: 403, headers: { 'X-Request-ID': 'server-trace' },
    }));
    await expect(requestJSON(local, '/api/v1/settings')).rejects.toMatchObject({
      name: 'BackendRequestError', status: 403, code: 'denied', message: '拒絕請求', requestID: 'server-trace',
    });
  });
});
