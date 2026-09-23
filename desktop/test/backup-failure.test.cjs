const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { createUpdateBackup } = require('../data-protection.cjs');

test('failed verification removes only its own partial backup and preserves source and previous backups', t => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'mystocktracer-backup-failure-'));
  t.after(() => fs.rmSync(root, { recursive: true, force: true }));
  const source = path.join(root, 'profile'), backups = path.join(root, 'backups');
  fs.mkdirSync(source);
  fs.writeFileSync(path.join(source, 'settings.json'), '{}');
  const previous = createUpdateBackup({ userDataPath: source, backupRoot: backups });
  const invalid = Buffer.concat([Buffer.from('SQLite format 3\0'), Buffer.alloc(1024)]);
  fs.writeFileSync(path.join(source, 'invalid.db'), invalid);
  for (let attempt = 0; attempt < 2; attempt++) {
    assert.throws(() => createUpdateBackup({ userDataPath: source, backupRoot: backups }), /驗證失敗/);
    assert.deepEqual(fs.readdirSync(backups), [path.basename(previous.path)]);
    assert.deepEqual(fs.readFileSync(path.join(source, 'invalid.db')), invalid);
    assert.equal(fs.readFileSync(path.join(previous.path, 'data', 'settings.json'), 'utf8'), '{}');
  }
});
