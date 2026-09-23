const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const fixture = require('./fixture.cjs');
const runtime = require('../hermes-runtime-root.cjs');

for (const platform of ['win32', 'darwin']) {
  test('runtime selection honors explicit, packaged, checkout and missing candidates: ' + platform, t => {
    const f = fixture(t);
    const candidates = ['explicit', 'bundle/hermes-runtime', 'project/desktop/resources/hermes-runtime'];
    for (const relative of candidates) {
      const interpreter = runtime.runtimePython(f.at(relative), platform);
      fs.mkdirSync(path.dirname(interpreter), { recursive: true });
      fs.writeFileSync(interpreter, 'synthetic interpreter');
    }
    const options = { platform, configuredRoot: f.at(candidates[0]), bundledRoot: f.at('bundle'), projectRoot: f.at('project') };
    for (const relative of candidates) {
      assert.equal(runtime.resolveHermesRuntimeRoot(options), f.at(relative));
      fs.rmSync(f.at(relative), { recursive: true });
    }
    assert.equal(runtime.resolveHermesRuntimeRoot(options), '');
    assert.equal(runtime.validRuntimeRoot(f.at('absent'), platform), false);
  });
}

test('worktree commondir resolves the owning checkout without a runtime copy', t => {
  const f = fixture(t);
  const common = f.at('checkout/.git');
  f.put('checkout/.git/worktrees/topic/commondir', '../..\n');
  f.put('topic/.git', 'gitdir: ' + path.join(common, 'worktrees/topic') + '\n');
  f.put('checkout/desktop/resources/hermes-runtime/python/python.exe');
  assert.equal(runtime.commonGitDir(f.at('topic')), common);
  assert.equal(runtime.resolveHermesRuntimeRoot({ projectRoot: f.at('topic'), platform: 'win32' }), f.at('checkout/desktop/resources/hermes-runtime'));
});
