const test = require('node:test');
const assert = require('node:assert/strict');
const net = require('node:net');
const path = require('node:path');
const fixture = require('./fixture.cjs');
const backend = require('../backend-process.cjs');

test('backend environment preserves caller state while enforcing canonical child identity', () => {
  const source = { PATH: 'fixture-path' };
  const result = backend.buildBackendEnv({ addr: '127.0.0.1:23456', token: 'synthetic', baseEnv: source, extraEnv: { MYSTOCKTRACER_ADDR: 'invalid', TEST_VALUE: 'kept' } });
  assert.deepEqual(source, { PATH: 'fixture-path' });
  assert.deepEqual(result, { PATH: 'fixture-path', TEST_VALUE: 'kept', MYSTOCKTRACER_ADDR: '127.0.0.1:23456', MYSTOCKTRACER_TOKEN: 'synthetic', MYSTOCKTRACER_DESKTOP_CHILD: '1' });
  for (const addr of ['0.0.0.0:23456', 'https://localhost', 'invalid']) assert.throws(() => backend.buildBackendEnv({ addr, token: 'synthetic' }));
  assert.throws(() => backend.buildBackendEnv({ addr: '127.0.0.1:23456', token: '' }));
});

test('packaged execution never silently falls back to a Go development server', t => {
  const f = fixture(t), cwd = f.at('backend'), binary = f.at('mystocktracer-backend.exe');
  for (const isPackaged of [true, false]) assert.deepEqual(backend.resolveBackendCommand({ backendBin: binary, backendDir: cwd, isPackaged }), { command: binary, args: [], cwd });
  assert.deepEqual(backend.resolveBackendCommand({ backendDir: cwd, isPackaged: false }), { command: 'go', args: ['run', './cmd/server'], cwd });
  assert.throws(() => backend.resolveBackendCommand({ backendDir: cwd, isPackaged: true }), /not found/);
});

test('desktop port selection binds loopback and refuses an occupied bounded range', async t => {
  const port = await backend.findFreePort();
  assert.ok(port >= 20000 && port <= 29999);
  const listener = net.createServer();
  t.after(() => new Promise(resolve => listener.close(resolve)));
  await new Promise((resolve, reject) => { listener.once('error', reject); listener.listen(port, '127.0.0.1', resolve); });
  await assert.rejects(backend.findFreePort('127.0.0.1', port, port), /No desktop/);
  await assert.rejects(backend.findFreePort('0.0.0.0'), /loopback/);
});

test('health readiness fails for an exited child or an external endpoint', async () => {
  await assert.rejects(backend.waitForHealth('http://127.0.0.1:23456', 100, { exitCode: 1 }), /exited/);
  await assert.rejects(backend.waitForHealth('https://example.test'), /Invalid/);
});
