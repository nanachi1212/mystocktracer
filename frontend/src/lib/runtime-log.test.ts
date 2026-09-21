import { describe, expect, it } from 'vitest';
import { runtimeErrorDetails, runtimeFeatureForPath } from './runtime-log';

describe('runtime logging helpers', () => {
	it('maps API routes to stable feature names without query data', () => {
		expect(runtimeFeatureForPath('/api/v1/tw/dashboard')).toBe('taiwan');
		expect(runtimeFeatureForPath('/api/v1/settings')).toBe('settings');
		expect(runtimeFeatureForPath('/api/v1/ai/chat')).toBe('ai-chat');
	});

	it('keeps error diagnostics bounded to the useful fields', () => {
		const details = runtimeErrorDetails(new TypeError('network failed'));
		expect(details).toMatchObject({ name: 'TypeError', message: 'network failed' });
	});
});
