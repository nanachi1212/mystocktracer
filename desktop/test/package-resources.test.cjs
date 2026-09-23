const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const fixture = require('./fixture.cjs');

test('package staging resolves the canonical generated output independently of cwd', async t => {
  const f = fixture(t), { resolvePackageResourcesDir } = await import('../scripts/package-resources.mjs');
  const options = { repoRoot: f.root, desktopRoot: f.at('desktop'), override: '' };
  assert.equal(resolvePackageResourcesDir(options), f.at('desktop/dist/package-resources'));
  for (const override of ['desktop/dist/custom', f.at('desktop/dist/custom')]) assert.equal(resolvePackageResourcesDir({ ...options, override }), f.at('desktop/dist/custom'));
});

test('package preparation cannot select Git metadata, local state, source or ancestors for replacement', async t => {
  const f = fixture(t), { resolvePackageResourcesDir } = await import('../scripts/package-resources.mjs');
  for (const override of ['.', '..', '.git', '.runtime', 'desktop', 'desktop/dist', 'desktop/resources', 'backend', 'frontend', 'desktop/dist/../../assets']) {
    assert.throws(() => resolvePackageResourcesDir({ repoRoot: f.root, desktopRoot: f.at('desktop'), override }), /dedicated generated/);
  }
});

test('package staging rejects a junction to a protected fixture directory', async t => {
  const f = fixture(t), { resolvePackageResourcesDir } = await import('../scripts/package-resources.mjs');
  f.put('outside/sentinel', 'preserve');
  fs.mkdirSync(f.at('desktop'), { recursive: true });
  fs.symlinkSync(f.at('outside'), f.at('desktop/dist'), process.platform === 'win32' ? 'junction' : 'dir');
  assert.throws(() => resolvePackageResourcesDir({ repoRoot: f.root, desktopRoot: f.at('desktop'), override: '' }), /symlink or junction/);
  assert.equal(f.read('outside/sentinel').toString(), 'preserve');
});
