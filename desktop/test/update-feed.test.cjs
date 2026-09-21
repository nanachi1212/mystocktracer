const assert = require('node:assert/strict');
const test = require('node:test');

const { UPDATE_REPOSITORY_OWNER, UPDATE_REPOSITORY_NAME, resolveUpdateFeed, releasePageURL } = require('../update-feed.cjs');

test('resolves updates from the mystocktracer GitHub releases only', () => {
  assert.deepEqual(resolveUpdateFeed(), { provider: 'github', owner: 'nanachi1212', repo: 'mystocktracer' });
  assert.equal(UPDATE_REPOSITORY_OWNER, 'nanachi1212');
  assert.equal(UPDATE_REPOSITORY_NAME, 'mystocktracer');
});

test('the update source cannot be redirected by configuration or environment', () => {
  process.env.A_STOCK_UPDATE_FEED_URL = 'https://attacker.example.com/desktop';
  try {
    assert.deepEqual(resolveUpdateFeed(), { provider: 'github', owner: 'nanachi1212', repo: 'mystocktracer' });
  } finally {
    delete process.env.A_STOCK_UPDATE_FEED_URL;
  }
  const source = require('node:fs').readFileSync(require('node:path').resolve(__dirname, '..', 'update-feed.cjs'), 'utf8');
  assert.doesNotMatch(source, /easy-stock-fs|aliyuncs|jundizhou/);
});

test('release page links stay inside the mystocktracer repository', () => {
  assert.equal(releasePageURL('1.2.3'), 'https://github.com/nanachi1212/mystocktracer/releases/tag/v1.2.3');
  assert.equal(releasePageURL('v1.2.3'), 'https://github.com/nanachi1212/mystocktracer/releases/tag/v1.2.3');
  assert.equal(releasePageURL(''), 'https://github.com/nanachi1212/mystocktracer/releases');
  // A hostile "version" cannot escape the releases path.
  assert.ok(releasePageURL('1.0.0/../../attacker').startsWith('https://github.com/nanachi1212/mystocktracer/releases/tag/'));
  assert.doesNotMatch(releasePageURL('1.0.0/../../attacker'), /attacker\.example/);
});
