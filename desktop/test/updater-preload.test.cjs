const test = require('node:test');
const assert = require('node:assert/strict');
const { EventEmitter } = require('node:events');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');

test('isolated canonical bridge implements fixed product channels and revocable subscriptions', async () => {
  const bus = new EventEmitter(), calls = [];
  let bridge;
  const electron = {
    contextBridge: { exposeInMainWorld(name, api) { assert.equal(name, 'mystocktracer'); bridge = api; } },
    ipcRenderer: Object.assign(bus, { invoke: async (...args) => { calls.push(args); return 'synthetic-response'; } }),
  };
  vm.runInNewContext(fs.readFileSync(path.join(__dirname, '../preload.cjs'), 'utf8'), { require: name => { assert.equal(name, 'electron'); return electron; } });
  const channels = {
    getBackendConfig: 'backend-config', getRuntimeLogStatus: 'runtime-log-status', openRuntimeLogs: 'runtime-open-logs',
    logRuntimeEvent: 'runtime-log', getUpdateStatus: 'app-update-status', checkForUpdates: 'app-update-check',
    downloadUpdate: 'app-update-download', installUpdate: 'app-update-install', openUpdateRelease: 'app-update-open-release',
    openUpdateBackups: 'app-update-open-backups', openSubscriptionAI: 'open-subscription-ai',
  };
  assert.deepEqual(Object.keys(bridge).sort(), [...Object.keys(channels), 'onUpdateStatus'].sort());
  assert.equal(Object.isFrozen(bridge), true);
  for (const [method, channel] of Object.entries(channels)) {
    const payload = method === 'openSubscriptionAI' ? 'https://chatgpt.com/' : { fixture: method };
    assert.equal(await bridge[method](payload), 'synthetic-response');
    assert.deepEqual(calls.pop(), [channel, payload]);
  }
  const received = [];
  const cancel = bridge.onUpdateStatus(value => received.push(value));
  bus.emit('app-update-status-changed', { privateEvent: true }, { state: 'downloaded' });
  cancel();
  bus.emit('app-update-status-changed', {}, { state: 'idle' });
  assert.deepEqual(received, [{ state: 'downloaded' }]);
  assert.equal(bus.listenerCount('app-update-status-changed'), 0);
  assert.throws(() => bridge.onUpdateStatus(null), /must be a function/);
});

test('subscription navigation accepts only the three exact provider home URLs', () => {
  const urls = require('../subscription-ai-url.cjs');
  const official = ['https://chatgpt.com/', 'https://claude.ai/', 'https://gemini.google.com/'];
  assert.deepEqual([...urls.ALLOWED_SUBSCRIPTION_AI_URLS].sort(), [...official].sort());
  for (const candidate of official) assert.equal(urls.validateSubscriptionAIURL(candidate), true);
  for (const candidate of [null, '', 'https://example.test/', 'javascript:alert(1)', 'file:///private', 'https://chatgpt.com/evil', 'https://gemini.google.com.attacker.test/']) {
    assert.throws(() => urls.validateSubscriptionAIURL(candidate), /不允許/);
  }
});
