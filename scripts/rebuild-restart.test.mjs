import test from 'node:test';
import assert from 'node:assert/strict';
import {
  resolveBrowserMode,
  getBrowserLaunchPlan,
  findChromeBinary,
  launchBrowser,
} from './rebuild-restart.mjs';

test('resolveBrowserMode: defaults to incognito when env is unset or empty', () => {
  assert.equal(resolveBrowserMode(undefined), 'incognito');
  assert.equal(resolveBrowserMode(''), 'incognito');
  assert.equal(resolveBrowserMode('   '), 'incognito');
});

test('resolveBrowserMode: parses incognito, normal, none (case-insensitive and trimmed)', () => {
  assert.equal(resolveBrowserMode('incognito'), 'incognito');
  assert.equal(resolveBrowserMode('INCOGNITO'), 'incognito');
  assert.equal(resolveBrowserMode(' normal '), 'normal');
  assert.equal(resolveBrowserMode('None'), 'none');
});

test('resolveBrowserMode: invalid value safely falls back to incognito', () => {
  assert.equal(resolveBrowserMode('invalid-mode'), 'incognito');
  assert.equal(resolveBrowserMode('unknown'), 'incognito');
  assert.equal(resolveBrowserMode('firefox'), 'incognito');
});

test('getBrowserLaunchPlan: incognito mode produces --incognito, --new-window and target url', () => {
  const plan = getBrowserLaunchPlan('http://127.0.0.1:20073', 'incognito');
  assert.equal(plan.action, 'open');
  assert.equal(plan.mode, 'incognito');
  assert.deepEqual(plan.args, ['--incognito', '--new-window', 'http://127.0.0.1:20073']);
});

test('getBrowserLaunchPlan: normal mode produces --new-window without --incognito', () => {
  const plan = getBrowserLaunchPlan('http://127.0.0.1:20073', 'normal');
  assert.equal(plan.action, 'open');
  assert.equal(plan.mode, 'normal');
  assert.deepEqual(plan.args, ['--new-window', 'http://127.0.0.1:20073']);
  assert.ok(!plan.args.includes('--incognito'));
});

test('getBrowserLaunchPlan: none mode produces action none', () => {
  const plan = getBrowserLaunchPlan('http://127.0.0.1:20073', 'none');
  assert.equal(plan.action, 'none');
  assert.equal(plan.mode, 'none');
  assert.equal(plan.args, undefined);
});

test('findChromeBinary: finds existing binary when mock candidates exist', () => {
  const mockExistsSync = (filePath) => filePath === 'C:\\Mock\\chrome.exe';
  const binary = findChromeBinary({
    env: {},
    existsSync: mockExistsSync,
    whereLookup: () => null,
  });
  assert.equal(binary, null);

  const found = findChromeBinary({
    env: { ProgramFiles: 'C:\\Mock' },
    existsSync: (p) => p.includes('Mock'),
    whereLookup: () => null,
  });
  assert.ok(found);
});

test('launchBrowser: none mode logs disabled and does not spawn', () => {
  const logs = [];
  let spawned = false;
  launchBrowser({
    url: 'http://127.0.0.1:20073',
    mode: 'none',
    logger: (msg) => logs.push(msg),
    chromeFinder: () => 'C:\\chrome.exe',
    spawner: () => {
      spawned = true;
    },
  });
  assert.equal(spawned, false);
  assert.ok(logs.some((l) => l.includes('browser auto-open disabled')));
});

test('launchBrowser: when chrome is not found, logs warning and does not throw', () => {
  const logs = [];
  let spawned = false;
  assert.doesNotThrow(() => {
    launchBrowser({
      url: 'http://127.0.0.1:20073',
      mode: 'incognito',
      logger: (msg) => logs.push(msg),
      chromeFinder: () => null,
      spawner: () => {
        spawned = true;
      },
    });
  });
  assert.equal(spawned, false);
  assert.ok(logs.some((l) => l.includes('Chrome not found; open manually:')));
  assert.ok(logs.some((l) => l.includes('http://127.0.0.1:20073')));
});

test('launchBrowser: incognito spawns detached process with correct args', () => {
  const logs = [];
  let spawnArgs = null;
  launchBrowser({
    url: 'http://127.0.0.1:20073',
    mode: 'incognito',
    logger: (msg) => logs.push(msg),
    chromeFinder: () => 'C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe',
    spawner: (cmd, args, opts) => {
      spawnArgs = { cmd, args, opts };
      return { unref: () => {} };
    },
  });
  assert.ok(logs.some((l) => l.includes('opening frontend in Chrome incognito mode')));
  assert.ok(logs.some((l) => l.includes('frontend: http://127.0.0.1:20073')));
  assert.equal(spawnArgs.cmd, 'C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe');
  assert.deepEqual(spawnArgs.args, ['--incognito', '--new-window', 'http://127.0.0.1:20073']);
  assert.equal(spawnArgs.opts.detached, true);
  assert.deepEqual(spawnArgs.opts.stdio, 'ignore');
});

test('launchBrowser: normal mode logs and spawns with normal args', () => {
  const logs = [];
  let spawnArgs = null;
  launchBrowser({
    url: 'http://127.0.0.1:20073',
    mode: 'normal',
    logger: (msg) => logs.push(msg),
    chromeFinder: () => 'C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe',
    spawner: (cmd, args, opts) => {
      spawnArgs = { cmd, args, opts };
      return { unref: () => {} };
    },
  });
  assert.ok(logs.some((l) => l.includes('opening frontend in Chrome normal mode')));
  assert.ok(logs.some((l) => l.includes('frontend: http://127.0.0.1:20073')));
  assert.deepEqual(spawnArgs.args, ['--new-window', 'http://127.0.0.1:20073']);
});

test('process safety: launchBrowser does not invoke taskkill, kill, or terminate existing chrome', () => {
  let killed = false;
  launchBrowser({
    url: 'http://127.0.0.1:20073',
    mode: 'incognito',
    logger: () => {},
    chromeFinder: () => 'C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe',
    spawner: (cmd, args, opts) => {
      if (cmd.includes('taskkill') || args.includes('/PID')) {
        killed = true;
      }
      return { unref: () => {} };
    },
  });
  assert.equal(killed, false);
});
