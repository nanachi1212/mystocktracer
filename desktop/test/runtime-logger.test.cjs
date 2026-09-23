const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const fixture = require('./fixture.cjs');
const { createRotatingLogger, redactRuntimeLog } = require('../runtime-logger.cjs');

test('secret forms are redacted on disk and on the console mirror', t => {
  const f = fixture(t), mirrored = [];
  const log = createRotatingLogger({ directory: f.root, fileName: 'events.log', mirror: { error: line => mirrored.push(line) } });
  for (const input of ['Bearer synthetic-one', 'token=synthetic-two', 'api_key:synthetic-three', 'cookie=synthetic-four', '{"credential":"synthetic-five"}']) log.error(input);
  const persisted = f.read('events.log').toString();
  assert.doesNotMatch(persisted + mirrored.join('\n'), /synthetic-(one|two|three|four|five)/);
  assert.equal(mirrored.length, 5);
  assert.ok(mirrored.every(line => line.includes('<redacted>')));
  assert.equal(persisted.split('<redacted>').length - 1, 5);
});

test('rotation retains exactly the newest bounded records and the requested backup count', t => {
  const f = fixture(t);
  const log = createRotatingLogger({ directory: f.root, fileName: 'bounded.log', maxBytes: 128, backups: 2 });
  for (let n = 0; n < 15; n++) log.info('record-' + n + '-' + 'x'.repeat(200));
  assert.deepEqual(fs.readdirSync(f.root).sort(), ['bounded.log', 'bounded.log.1', 'bounded.log.2']);
  for (const file of fs.readdirSync(f.root)) assert.ok(fs.statSync(f.at(file)).size <= 128);
  assert.match(f.read('bounded.log').toString(), /record-14-/);
  assert.match(f.read('bounded.log.1').toString(), /record-13-/);
  assert.match(f.read('bounded.log.2').toString(), /record-12-/);
});

test('query redaction retains the endpoint and public parameters', () => {
  assert.equal(redactRuntimeLog('GET /api/v1/settings?token=synthetic&mode=test'), 'GET /api/v1/settings?token=<redacted>&mode=test');
});

test('zero-backup mode does not accumulate rotated files', t => {
  const f = fixture(t);
  const log = createRotatingLogger({ directory: f.root, fileName: 'single.log', backups: 0, maxBytes: 80 });
  for (let n = 0; n < 6; n++) log.info('entry-' + n + 'x'.repeat(100));
  assert.deepEqual(fs.readdirSync(f.root), ['single.log']);
  assert.match(f.read('single.log').toString(), /entry-5/);
});
