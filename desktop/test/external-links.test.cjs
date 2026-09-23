const test = require('node:test');
const assert = require('node:assert/strict');
const { externalWindowHandler } = require('../external-links.cjs');

test('citation handler opens HTTP(S) in the system browser and always denies child windows', async () => {
  const opened = [];
  const handler = externalWindowHandler(url => opened.push(url), assert.fail);
  for (const url of ['https://mops.twse.com.tw/mops/web/index', 'http://example.test/citation']) {
    assert.deepEqual(handler({ url }), { action: 'deny' });
  }
  for (const url of ['file:///private', 'javascript:alert(1)', 'data:text/html,test', 'https://user:pass@example.test/', 'invalid']) {
    assert.deepEqual(handler({ url }), { action: 'deny' });
  }
  await new Promise(resolve => setImmediate(resolve));
  assert.deepEqual(opened, ['https://mops.twse.com.tw/mops/web/index', 'http://example.test/citation']);
});

test('system browser failures are reported without an unhandled rejection', async () => {
  const error = new Error('synthetic launch failure');
  const received = [];
  externalWindowHandler(async () => { throw error; }, value => received.push(value))({ url: 'https://example.test/' });
  await new Promise(resolve => setImmediate(resolve));
  assert.deepEqual(received, [error]);
});
