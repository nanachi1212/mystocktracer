const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { scheduleUpdateRestart, resumeUpdateAfterRestart, awaitNativeInstall } = require('../update-restart.cjs');
const { UpdateManager } = require('../update-manager.cjs');

function request(t, contents = { fromVersion: '0.9.2', toVersion: '0.9.3' }) {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'mystocktracer-restart-'));
  t.after(() => fs.rmSync(root, { recursive: true, force: true }));
  const requestPath = path.join(root, 'pending-install.json');
  fs.writeFileSync(requestPath, JSON.stringify(contents));
  return requestPath;
}
test('controller requests full process restart before any backup or native install', async () => {
  const updater = new (require('node:events').EventEmitter)();
  const calls = [];
  updater.quitAndInstall = assert.fail;
  const manager = new UpdateManager({ updater, enabled: true, platform: 'win32', currentVersion: '0.9.2', stopRuntime: assert.fail, createBackup: assert.fail, restartForInstall: versions => calls.push(versions) });
  updater.emit('update-downloaded', { version: '0.9.3' });
  await manager.installUpdate();
  assert.deepEqual(calls, [{ fromVersion: '0.9.2', toVersion: '0.9.3' }]);
});
test('restart intent is written before app relaunch and quit', t => {
  const requestPath = request(t); fs.unlinkSync(requestPath);
  const calls = [];
  scheduleUpdateRestart({ requestPath, versions: { fromVersion: '0.9.2', toVersion: '0.9.3' }, app: { relaunch: () => { assert.equal(JSON.parse(fs.readFileSync(requestPath)).toVersion, '0.9.3'); calls.push('relaunch'); }, quit: () => calls.push('quit') } });
  assert.deepEqual(calls, ['relaunch', 'quit']);
});
test('fresh process snapshots before network creates storage, revalidates release, then installs', async t => {
  const requestPath = request(t), calls = [];
  const updater = { checkForUpdates: async () => { calls.push('check'); return { updateInfo: { version: '0.9.3' } }; }, downloadUpdate: async () => calls.push('download'), quitAndInstall: (...args) => calls.push(['install', ...args]) };
  await resumeUpdateAfterRestart({ requestPath, currentVersion: '0.9.2', updater, createBackup: async versions => { calls.push(['backup', versions]); return { path: 'fixture' }; } });
  assert.deepEqual(calls, [['backup', { fromVersion: '0.9.2', toVersion: '0.9.3' }], 'check', 'download', ['install', false, true]]);
  assert.equal(updater.autoInstallOnAppQuit, false);
  assert.equal(fs.existsSync(requestPath), false);
});
test('failed backup cannot reach native installer or create a retry loop', async t => {
  const requestPath = request(t);
  await assert.rejects(resumeUpdateAfterRestart({ requestPath, currentVersion: '0.9.2', updater: { checkForUpdates: async () => ({ updateInfo: { version: '0.9.3' } }), downloadUpdate: async () => {}, quitAndInstall: assert.fail }, createBackup: async () => { throw new Error('busy profile'); } }), /busy profile/);
  assert.equal(fs.existsSync(requestPath), false);
});
test('changed release never downloads or installs', async t => {
  const requestPath = request(t);
  await assert.rejects(resumeUpdateAfterRestart({ requestPath, currentVersion: '0.9.2', updater: { checkForUpdates: async () => ({ updateInfo: { version: '0.9.4' } }), downloadUpdate: assert.fail, quitAndInstall: assert.fail }, createBackup: async () => ({ path: 'synthetic-backup' }) }), /發布版本已變更/);
});
test('malformed intent is consumed without backup or installation', async t => {
  const requestPath = request(t); fs.writeFileSync(requestPath, '{invalid');
  await assert.rejects(resumeUpdateAfterRestart({ requestPath, currentVersion: '0.9.2', updater: {}, createBackup: assert.fail }));
  assert.equal(fs.existsSync(requestPath), false);
});
test('native installer failures reject while quit success removes listeners', async () => {
  const EventEmitter = require('node:events').EventEmitter;
  for (const outcome of ['quit', 'error', 'timeout']) {
    const app = new EventEmitter(), updater = new EventEmitter();
    updater.quitAndInstall = () => { if (outcome === 'quit') setImmediate(() => app.emit('before-quit')); if (outcome === 'error') setImmediate(() => updater.emit('error', new Error('fixture installer error'))); };
    const result = awaitNativeInstall({ updater, app, timeout: 30 });
    if (outcome === 'quit') await result; else await assert.rejects(result);
    assert.equal(app.listenerCount('before-quit'), 0);
    assert.equal(updater.listenerCount('error'), 0);
  }
});
