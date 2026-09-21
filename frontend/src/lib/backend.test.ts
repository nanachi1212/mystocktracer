import { afterEach, describe, expect, it, vi } from 'vitest';
import { BackendRequestError, requestJSON, resolveBackendConfig } from './backend';

afterEach(() => {
	vi.restoreAllMocks();
});

describe('backend configuration', () => {
  it('prefers Electron bridge configuration', async () => {
    const config = await resolveBackendConfig({
      bridge: {
        getBackendConfig: async () => ({
          backendUrl: 'http://127.0.0.1:20001',
          token: 'desktop-token',
        }),
      },
      env: { VITE_A_STOCK_BACKEND_URL: 'http://127.0.0.1:20081' },
    });

    expect(config).toEqual({
      backendUrl: 'http://127.0.0.1:20001',
      token: 'desktop-token',
    });
  });

  it('falls back to Vite development environment', async () => {
    const config = await resolveBackendConfig({
      env: { VITE_A_STOCK_BACKEND_URL: 'http://127.0.0.1:20081' },
    });

    expect(config).toEqual({
      backendUrl: 'http://127.0.0.1:20081',
      token: '',
    });
  });

	it('prefers canonical MYSTOCKTRACER environment names', async () => {
		const config = await resolveBackendConfig({
			env: {
				VITE_MYSTOCKTRACER_BACKEND_URL: 'http://127.0.0.1:20091',
				VITE_MYSTOCKTRACER_TOKEN: 'new-token',
				VITE_A_STOCK_BACKEND_URL: 'http://127.0.0.1:20081',
				VITE_A_STOCK_TOKEN: 'legacy-token',
			},
		});
		expect(config).toEqual({ backendUrl: 'http://127.0.0.1:20091', token: 'new-token' });
	});

	it('adds a correlation ID to HTTP requests', async () => {
		const fetchMock = vi.spyOn(globalThis, 'fetch').mockResolvedValue(new Response('{"ok":true}', {
			status: 200,
			headers: { 'Content-Type': 'application/json', 'X-Request-ID': 'backend-id' },
		}));

		await requestJSON({ backendUrl: 'http://127.0.0.1:20001', token: 'desktop-token' }, '/api/v1/tw/data-status');

		const requestInit = fetchMock.mock.calls[0][1];
		const headers = requestInit?.headers as Headers;
		expect(headers.get('Authorization')).toBe('Bearer desktop-token');
		expect(headers.get('X-Request-ID')).toMatch(/\S+/);
	});

	it('passes AbortSignal through to fetch', async () => {
		const fetchMock = vi.spyOn(globalThis, 'fetch').mockResolvedValue(new Response(null, { status: 204 }));
		const controller = new AbortController();
		await requestJSON({ backendUrl: 'http://127.0.0.1:20001', token: '' }, '/api/health', { signal: controller.signal });
		expect(fetchMock.mock.calls[0][1]?.signal).toBe(controller.signal);
	});

	it('returns undefined for an empty successful response', async () => {
		vi.spyOn(globalThis, 'fetch').mockResolvedValue(new Response('', { status: 200 }));
		await expect(requestJSON({ backendUrl: 'http://127.0.0.1:20001', token: '' }, '/api/health')).resolves.toBeUndefined();
	});

	it('throws a typed error with status and request ID for non-2xx responses', async () => {
		vi.spyOn(globalThis, 'fetch').mockResolvedValue(new Response('{"error":"拒絕請求","code":"denied"}', {
			status: 403,
			headers: { 'X-Request-ID': 'server-request' },
		}));
		const promise = requestJSON({ backendUrl: 'http://127.0.0.1:20001', token: '' }, '/api/v1/settings');
		await expect(promise).rejects.toMatchObject({
			name: 'BackendRequestError', message: '拒絕請求', status: 403, requestID: 'server-request', code: 'denied',
		});
	});

	it('rejects successful responses containing invalid JSON', async () => {
		vi.spyOn(globalThis, 'fetch').mockResolvedValue(new Response('<html>bad gateway</html>', { status: 200 }));
		await expect(requestJSON({ backendUrl: 'http://127.0.0.1:20001', token: '' }, '/api/health'))
			.rejects.toBeInstanceOf(BackendRequestError);
	});

	it('rejects non-HTTP backend schemes', async () => {
		await expect(resolveBackendConfig({ env: { VITE_MYSTOCKTRACER_BACKEND_URL: 'file:///tmp/socket' } }))
			.rejects.toMatchObject({ code: 'invalid_backend_url' });
	});

	it('rejects a remote backend before a token can be sent', async () => {
		await expect(resolveBackendConfig({ env: { VITE_MYSTOCKTRACER_BACKEND_URL: 'https://attacker.example/api' } }))
			.rejects.toMatchObject({ code: 'invalid_backend_url' });
	});

	it('rejects an absolute request URL that leaves the configured backend origin', async () => {
		const fetchMock = vi.spyOn(globalThis, 'fetch');
		await expect(requestJSON({ backendUrl: 'http://127.0.0.1:20001', token: 'secret' }, 'https://attacker.example/collect'))
			.rejects.toMatchObject({ code: 'cross_origin_backend_request' });
		expect(fetchMock).not.toHaveBeenCalled();
	});

	it('validates direct request configuration before attaching a token', async () => {
		const fetchMock = vi.spyOn(globalThis, 'fetch');
		await expect(requestJSON({ backendUrl: 'https://attacker.example', token: 'secret' }, '/collect'))
			.rejects.toMatchObject({ code: 'invalid_backend_url' });
		expect(fetchMock).not.toHaveBeenCalled();
	});
});
