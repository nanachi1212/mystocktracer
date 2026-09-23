const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const fixture = require('./fixture.cjs');

test('runtime path contract distinguishes build environments from shipped interpreters', async t => {
  const f = fixture(t);
  const { hermesRuntimePython, hermesVenvPython } = await import('../scripts/hermes-runtime.mjs');
  const layouts = [
    ['win32', ['python', 'python.exe'], ['venv', 'Scripts', 'python.exe']],
    ['darwin', ['venv', 'bin', 'python'], ['venv', 'bin', 'python']],
  ];
  for (const [platform, shipped, build] of layouts) {
    assert.equal(hermesRuntimePython(f.root, platform), path.join(f.root, ...shipped));
    assert.equal(hermesVenvPython(f.root, platform), path.join(f.root, ...build));
  }
});

test('Windows runtime assembly preserves interpreter, stdlib and packages across a source link', async t => {
  const f = fixture(t);
  const { bundleWindowsRuntime } = await import('../scripts/hermes-runtime.mjs');
  const expected = new Map([
    ['python.exe', Buffer.from([77, 90, 0, 255])],
    ['Lib/os.py', Buffer.from('# synthetic standard library')],
    ['Lib/site-packages/hermes_cli/__init__.py', Buffer.from('# synthetic dependency')],
  ]);
  for (const [relative, bytes] of expected) {
    f.put((relative.includes('site-packages') ? 'assembled/venv/' : 'managed/') + relative, bytes);
  }
  f.put('assembled/venv/pyvenv.cfg', 'synthetic build-only marker');
  fs.symlinkSync(f.at('managed'), f.at('linked'), process.platform === 'win32' ? 'junction' : 'dir');
  bundleWindowsRuntime(f.at('assembled'), f.at('linked'));
  for (const [relative, bytes] of expected) assert.deepEqual(f.read('assembled/python/' + relative), bytes);
  assert.equal(fs.lstatSync(f.at('assembled/python')).isSymbolicLink(), false);
  assert.equal(fs.existsSync(f.at('assembled/venv')), false);
  assert.deepEqual(f.read('managed/python.exe'), expected.get('python.exe'));
});

test('invalid or overlapping runtime sources are rejected', async t => {
  const f = fixture(t);
  const { bundleWindowsRuntime } = await import('../scripts/hermes-runtime.mjs');
  f.put('managed/python.exe');
  assert.throws(() => bundleWindowsRuntime(f.at('managed'), f.at('managed')), /overlap/);
  assert.throws(() => bundleWindowsRuntime(f.at('output'), f.at('managed')), /Incomplete/);
});
