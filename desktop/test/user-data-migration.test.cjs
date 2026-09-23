const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const os = require('node:os');
const { spawnSync } = require('node:child_process');
const { selectUserData, MARKER } = require('../user-data-migration.cjs');
const python = process.env.MYSTOCKTRACER_TEST_PYTHON || 'python';

function fixture(t) {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'mystocktracer-資料 test-'));
  t.after(() => fs.rmSync(root, { recursive: true, force: true }));
  const put = (name, data = 'fixture') => {
    const file = path.join(root, name);
    fs.mkdirSync(path.dirname(file), { recursive: true });
    fs.writeFileSync(file, data);
  };
  const run = (options = {}) => selectUserData({ appDataPath: root, env: {}, python, ...options });
  return { root, put, run, read: (name) => fs.readFileSync(path.join(root, name)) };
}

test('fresh install uses canonical identity', (t) => {
  const f = fixture(t); const result = f.run();
  assert.equal(result.mode, 'fresh'); assert.equal(result.path, path.join(f.root, 'mystocktracer'));
});
for (const identity of ['easy-stock', 'desktop']) {
  test(`migrates ${identity}, preserves unknown state and secret bytes, rerun is idempotent`, (t) => {
    const f = fixture(t);
    const state = ['settings.json', 'hermes-home/.env', 'hermes-home/profiles/custom.yaml', 'hermes-home/config.yaml', 'Local Storage/leveldb/000001.ldb', 'Session Storage/000001.log', 'Partitions/keep/state', 'unknown/Cache/keep', 'logs/old.log'];
    for (const name of state) f.put(`${identity}/${name}`, 'synthetic-secret-never-log');
    f.put(`${identity}/Cache/discard`, 'cache');
    const first = f.run(); assert.equal(first.mode, 'migrated');
    for (const name of state) assert.deepEqual(f.read(`mystocktracer/${name}`), f.read(`${identity}/${name}`));
    assert.equal(fs.existsSync(path.join(first.path, 'Cache')), false);
    const manifest = f.read(`mystocktracer/${MARKER}`).toString();
    assert.doesNotMatch(manifest, /synthetic-secret-never-log/); assert.equal(manifest.includes(f.root), false);
    f.put(`${identity}/late-change`, 'leave old alone');
    assert.equal(f.run().mode, 'existing');
    assert.equal(fs.existsSync(path.join(first.path, 'late-change')), false);
    assert.equal(f.read(`mystocktracer/${MARKER}`).toString(), manifest);
  });
}
test('prefers easy-stock over historical desktop', (t) => {
  const f = fixture(t); f.put('easy-stock/settings.json', 'preferred'); f.put('desktop/settings.json', 'older');
  f.run(); assert.equal(f.read('mystocktracer/settings.json').toString(), 'preferred');
});
test('canonical data wins over both legacy directories without mutation', (t) => {
  const f = fixture(t); for (const id of ['easy-stock', 'desktop', 'mystocktracer']) f.put(`${id}/settings.json`, id);
  assert.equal(f.run().mode, 'existing');
  for (const id of ['easy-stock', 'desktop', 'mystocktracer']) assert.equal(f.read(`${id}/settings.json`).toString(), id);
});
for (const envName of ['MYSTOCKTRACER_USER_DATA_DIR', 'A_STOCK_USER_DATA_DIR']) {
  test(`explicit ${envName} never receives automatic migration`, (t) => {
    const f = fixture(t); f.put('easy-stock/settings.json');
    const target = path.join(f.root, 'explicit');
    const result = f.run({ env: { [envName]: target } });
    assert.equal(result.path, target); assert.deepEqual(fs.readdirSync(target), []);
  });
}
test('canonical environment takes precedence', (t) => {
  const f = fixture(t); const target = path.join(f.root, 'new');
  assert.equal(f.run({ env: { MYSTOCKTRACER_USER_DATA_DIR: target, A_STOCK_USER_DATA_DIR: path.join(f.root, 'old') } }).path, target);
});
test('empty and cache-only legacy state does not trigger migration', (t) => {
  const f = fixture(t); f.put('easy-stock/Cache/cache'); assert.equal(f.run().mode, 'fresh');
});
test('interrupted staging stays untouched and a complete copy is rebuilt', (t) => {
  const f = fixture(t); f.put('easy-stock/settings.json'); f.put('mystocktracer.migration-interrupted/partial', 'preserve');
  assert.equal(f.run().mode, 'migrated'); assert.equal(f.read('mystocktracer.migration-interrupted/partial').toString(), 'preserve');
});
test('copy failure keeps source, staging and missing destination', (t) => {
  const f = fixture(t); f.put('easy-stock/settings.json');
  assert.throws(() => f.run({ copy: () => { throw new Error('copy failed'); } }), /copy failed/);
  assert.equal(f.read('easy-stock/settings.json').toString(), 'fixture');
  assert.equal(fs.existsSync(path.join(f.root, 'mystocktracer')), false);
  assert.equal(fs.readdirSync(f.root).filter((name) => name.startsWith('mystocktracer.migration-')).length, 1);
});
test('invalid verification result cannot activate a target', (t) => {
  const f = fixture(t); f.put('easy-stock/settings.json');
  assert.throws(() => f.run({ copy: () => [] }), /驗證/);
  assert.equal(fs.existsSync(path.join(f.root, 'mystocktracer')), false);
});
test('concurrent non-empty destination wins without overwrite', (t) => {
  const f = fixture(t); f.put('easy-stock/settings.json', 'old');
  assert.throws(() => f.run({ beforeActivate: () => f.put('mystocktracer/settings.json', 'winner') }));
  assert.equal(f.read('mystocktracer/settings.json').toString(), 'winner');
  assert.equal(f.read('easy-stock/settings.json').toString(), 'old');
});
test('cache-only nonempty canonical directory is never removed', (t) => {
  const f = fixture(t); f.put('easy-stock/settings.json'); f.put('mystocktracer/Cache/keep');
  assert.throws(() => f.run(), /非空/); assert.equal(f.read('mystocktracer/Cache/keep').toString(), 'fixture');
});
test('SQLite state survives byte-for-byte and is readable', (t) => {
  const f = fixture(t); f.put('easy-stock/settings.json', '{}');
  const db = path.join(f.root, 'easy-stock', 'taiwan-watchlist.db');
  const create = spawnSync(python, ['-I', '-c', 'import sqlite3,sys; c=sqlite3.connect(sys.argv[1]); c.execute("CREATE TABLE fixture(value TEXT)"); c.execute("INSERT INTO fixture VALUES (?)", ("Taiwan",)); c.commit(); c.close()', db]);
  assert.equal(create.status, 0);
  f.run(); assert.deepEqual(f.read('mystocktracer/taiwan-watchlist.db'), f.read('easy-stock/taiwan-watchlist.db'));
  const verify = spawnSync(python, ['-I', '-c', 'import sqlite3,sys; c=sqlite3.connect(sys.argv[1]); assert c.execute("SELECT value FROM fixture").fetchone()[0]=="Taiwan"; c.close()', path.join(f.root, 'mystocktracer', 'taiwan-watchlist.db')]);
  assert.equal(verify.status, 0);
});
test('corrupt SQLite fails closed and does not replace source', (t) => {
  const f = fixture(t); f.put('easy-stock/taiwan-watchlist.db', Buffer.concat([Buffer.from('SQLite format 3\0'), Buffer.alloc(80)]));
  assert.throws(() => f.run(), /驗證失敗/); assert.equal(fs.existsSync(path.join(f.root, 'mystocktracer')), false);
});
test('junction/symlink escape is rejected without touching external fixture', (t) => {
  const f = fixture(t); f.put('outside/keep', 'do not touch'); fs.mkdirSync(path.join(f.root, 'easy-stock'));
  fs.symlinkSync(path.join(f.root, 'outside'), path.join(f.root, 'easy-stock', 'unsafe'), process.platform === 'win32' ? 'junction' : 'dir');
  assert.throws(() => f.run(), /symlink|junction/); assert.equal(f.read('outside/keep').toString(), 'do not touch');
});
