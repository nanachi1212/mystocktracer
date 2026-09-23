import { describe, expect, it } from 'vitest';
import { runtimeErrorDetails, runtimeFeatureForPath, sanitizeRuntimeMessage } from './runtime-log';

describe('renderer diagnostics contract', () => {
  it('classifies only the request path and keeps unrelated paths generic', () => {
    for (const [path, feature] of [
      ['/api/v1/tw/dashboard', 'taiwan'], ['/api/v1/settings', 'settings'],
      ['/api/v1/ai/ws', 'ai-chat'], ['/api/health', 'health'], ['/elsewhere', 'renderer'],
    ]) expect(runtimeFeatureForPath(path)).toBe(feature);
  });

  it('bounds stack traces and produces a safe fallback for non-errors', () => {
    const failure = new TypeError('synthetic network failure');
    failure.stack = Array.from({ length: 20 }, (_, id) => `line ${id}`).join('\n');
    const details = runtimeErrorDetails(failure);
    expect(details.name).toBe('TypeError');
    expect(details.message).toBe('synthetic network failure');
    expect(details.stack?.split('\n')).toHaveLength(8);
    expect(runtimeErrorDetails({ private: true })).toMatchObject({ name: 'Error', message: 'Unknown runtime error' });
  });

  it('removes tokens and URL query strings before sending a diagnostic', () => {
    const input = 'GET https://model.example/path?token=synthetic-secret Bearer synthetic-bearer api_key="synthetic-key"';
    const output = sanitizeRuntimeMessage(input);
    for (const secret of ['synthetic-secret', 'synthetic-bearer', 'synthetic-key']) expect(output).not.toContain(secret);
    expect(output).toContain('https://model.example/path');
    expect(sanitizeRuntimeMessage('x'.repeat(9000)).length).toBeLessThanOrEqual(8192);
  });
});
