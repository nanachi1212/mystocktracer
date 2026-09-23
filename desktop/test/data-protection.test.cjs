const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const fixture = require('./fixture.cjs');
const backup = require('../data-protection.cjs');

test('snapshot preserves synthetic settings, databases, secrets, unknown files and nested browser state', t => {
  const f = fixture(t);
  const bytes = {
    'settings.json': '{"model":"synthetic"}',
    'taiwan-watchlist.db': Buffer.from([0, 17, 255]),
    'taiwan-portfolio.db': Buffer.from([1, 2, 3]),
    'hermes-home/.env': 'SYNTHETIC_KEY=fixture-only\n',
    'hermes-home/config.yaml': 'model: fixture\n',
    'hermes-home/memories/session.json': '{"synthetic":true}',
    'hermes-workspace/imported/note.md': 'synthetic note',
    'unknown-extension/config.json': '{"keep":true}',
    'unknown-extension/state.bin': Buffer.from([255, 4]),
    'browser-auth/retained-session.json': '{"synthetic":true}',
    'Partitions/persist_fixture/Cookies': Buffer.from([8, 9]),
    'Partitions/persist_fixture/Code Cache/keep.bin': 'nested cache is user state',
  };
  for (const [name, value] of Object.entries(bytes)) f.put('profile/' + name, value);
  f.put('profile/Cache/discard.bin');
  const result = backup.createUpdateBackup({ userDataPath: f.at('profile'), fromVersion: '0.9.2', toVersion: '0.9.3' });
  for (const [name, value] of Object.entries(bytes)) {
    assert.deepEqual(fs.readFileSync(require('node:path').join(result.path, 'data', name)), Buffer.from(value));
    assert.deepEqual(f.read('profile/' + name), Buffer.from(value));
  }
  assert.equal(fs.existsSync(require('node:path').join(result.path, 'data/Cache')), false);
  assert.equal(result.manifest.files.filter(entry => entry.type === 'file').length, Object.keys(bytes).length);
  assert.equal(result.manifest.schemaVersion, 2);
  assert.equal(backup.listUpdateBackups(result.backupRoot).length, 1);
});

test('snapshots retain all previous versions outside the profile and list newest first', t => {
  const f = fixture(t);
  f.put('profile/settings.json', '{}');
  assert.equal(backup.resolveBackupRoot(f.at('profile')), f.at('mystocktracer-update-backups'));
  const targets = ['0.9.3', '0.9.4', '0.9.5', '0.9.6'];
  for (const [index, version] of targets.entries()) backup.createUpdateBackup({ userDataPath: f.at('profile'), fromVersion: '0.9.2', toVersion: version, now: new Date(Date.UTC(2026, 8, 20 + index)) });
  const found = backup.listUpdateBackups(f.at('mystocktracer-update-backups'));
  assert.deepEqual(found.map(item => item.manifest.toVersion), [...targets].reverse());
  assert.ok(found.every(item => fs.existsSync(require('node:path').join(item.path, 'data/settings.json'))));
});

test('backup cannot target the profile itself or any descendant', t => {
  const f = fixture(t);
  for (const relative of ['profile', 'profile/nested/copies']) assert.throws(() => backup.resolveBackupRoot(f.at('profile'), f.at(relative)), /不能位於/);
});
