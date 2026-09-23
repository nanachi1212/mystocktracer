const test = require('node:test');
const assert = require('node:assert/strict');
const { EventEmitter } = require('node:events');
const fs = require('node:fs');
const path = require('node:path');
const fixture = require('./fixture.cjs');
const { UpdateManager } = require('../update-manager.cjs');
const { createUpdateBackup } = require('../data-protection.cjs');

function rig(overrides = {}) {
  const calls = [], updater = new EventEmitter();
  Object.assign(updater, {
    checkForUpdates: async () => { calls.push('check'); updater.emit('checking-for-update'); },
    downloadUpdate: async () => { calls.push('download'); },
    quitAndInstall: (...args) => calls.push(['install', ...args]),
  });
  const manager = new UpdateManager({
    updater, enabled: true, currentVersion: '0.9.2', platform: 'win32',
    stopRuntime: async () => { calls.push('stop'); },
    createBackup: async versions => { calls.push(versions); return { path: 'synthetic-backup', manifest: { createdAt: '2026-09-23T00:00:00Z' } }; },
    logger: { error() {} }, ...overrides,
  });
  return { manager, calls, event: (name, value) => updater.emit(name, value) };
}
function downloaded(rig) {
  rig.event('update-available', { version: '0.9.3', releaseName: 'fixture', releaseNotes: 'synthetic notes' });
  rig.event('update-downloaded', { version: '0.9.3' });
}

test('Windows checks, downloads and orders stop, verified backup, then install', async () => {
  const r = rig();
  await r.manager.checkForUpdates();
  assert.equal(r.manager.getStatus().state, 'checking');
  r.event('update-available', { version: '0.9.3', releaseNotes: 'synthetic notes' });
  assert.equal(r.manager.getStatus().state, 'available');
  await r.manager.downloadUpdate();
  r.event('download-progress', { percent: 42, transferred: 42, total: 100, bytesPerSecond: 10 });
  assert.equal(r.manager.getStatus().progress, 42);
  r.event('update-downloaded', { version: '0.9.3' });
  await r.manager.installUpdate();
  assert.deepEqual(r.calls, ['check', 'download', 'stop', { fromVersion: '0.9.2', toVersion: '0.9.3' }, ['install', false, true]]);
  assert.equal(r.manager.getStatus().backupPath, 'synthetic-backup');
});

test('failed snapshots stop installation and hide local path details', async () => {
  const r = rig({ createBackup: async () => { throw new Error('snapshot failed /private/synthetic/profile'); } });
  downloaded(r);
  await assert.rejects(r.manager.installUpdate(), /snapshot failed/);
  assert.deepEqual(r.calls, ['stop']);
  assert.equal(r.manager.getStatus().state, 'error');
  assert.match(r.manager.getStatus().message, /\[本機路徑\]/);
  assert.doesNotMatch(r.manager.getStatus().message, /private|synthetic/);
});

test('development never calls the updater', async () => {
  const r = rig({ enabled: false });
  assert.equal(r.manager.getStatus().state, 'disabled');
  await assert.rejects(r.manager.checkForUpdates(), /不支援/);
  assert.deepEqual(r.calls, []);
});

test('unsigned macOS remains manual even if download events arrive', async () => {
  const r = rig({ platform: 'darwin' });
  assert.equal(r.manager.getStatus().installMode, 'manual');
  r.event('update-available', { version: '0.9.3' });
  assert.match(r.manager.getStatus().message, /發布頁/);
  await assert.rejects(r.manager.downloadUpdate(), /Apple Developer ID/);
  r.event('update-downloaded', { version: '0.9.3' });
  await assert.rejects(r.manager.installUpdate(), /Apple Developer ID/);
  assert.deepEqual(r.calls, []);
});

test('signature failures retain actionable guidance without a private path', () => {
  const r = rig({ platform: 'darwin' });
  r.event('error', new Error('Code signature at /private/synthetic.zip did not pass validation'));
  assert.match(r.manager.getStatus().message, /Apple Developer ID/);
  assert.doesNotMatch(r.manager.getStatus().message, /private|synthetic/);
});

test('installation consumes the real snapshot helper before invoking the native updater', async t => {
  const f = fixture(t), expected = new Map([
    ['settings.json', Buffer.from('{}')],
    ['taiwan-watchlist.db', Buffer.from([2, 0, 255])],
    ['hermes-home/.env', Buffer.from('SYNTHETIC_KEY=fixture\n')],
    ['browser-auth/session.json', Buffer.from('{"synthetic":true}')],
  ]);
  for (const [name, bytes] of expected) f.put('profile/' + name, bytes);
  const r = rig({ createBackup: versions => createUpdateBackup({ ...versions, userDataPath: f.at('profile') }) });
  downloaded(r);
  await r.manager.installUpdate();
  for (const [name, bytes] of expected) {
    assert.deepEqual(fs.readFileSync(path.join(r.manager.getStatus().backupPath, 'data', name)), bytes);
    assert.deepEqual(f.read('profile/' + name), bytes);
  }
  assert.deepEqual(r.calls, ['stop', ['install', false, true]]);
});

test('progress is bounded and checking cannot reset a downloaded update', async () => {
  const r = rig();
  r.event('download-progress', { percent: 900 });
  assert.equal(r.manager.getStatus().progress, 100);
  downloaded(r);
  await r.manager.checkForUpdates();
  assert.equal(r.manager.getStatus().state, 'downloaded');
  assert.deepEqual(r.calls, []);
});
