const assert = require('node:assert/strict');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { spawnSync } = require('node:child_process');
const test = require('node:test');

test('agent-browser wrapper loads storage state before the first navigation', () => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'a-stock-browser-wrapper-'));
  const logPath = path.join(root, 'calls.log');
  const statePath = path.join(root, 'state.json');
  const socketDir = path.join(root, 'socket');
  const fakeBrowser = path.join(root, process.platform === 'win32' ? 'agent-browser-real.cjs' : 'agent-browser-real');
  fs.writeFileSync(statePath, '{"cookies":[],"origins":[]}');
  if (process.platform === 'win32') {
    fs.writeFileSync(fakeBrowser, [
      "const fs = require('node:fs');",
      "fs.appendFileSync(process.env.A_STOCK_TEST_CALL_LOG, process.argv.slice(2).join(' ') + '\\n');",
    ].join('\n'));
  } else {
    fs.writeFileSync(fakeBrowser, `#!/bin/sh\nprintf '%s\\n' "$*" >> "$A_STOCK_TEST_CALL_LOG"\nexit 0\n`, { mode: 0o755 });
  }

  const wrapper = path.resolve(__dirname, '..', 'scripts', 'browser-bin', 'agent-browser');
  const wrapperCommand = process.platform === 'win32' ? process.execPath : wrapper;
  let wrapperArguments = ['--session', 'test-session', '--json', 'open', 'https://xueqiu.com/'];
  if (process.platform === 'win32') {
    const nodeWrapper = path.join(root, 'agent-browser-wrapper.cjs');
    fs.writeFileSync(nodeWrapper, [
      "const fs = require('node:fs');",
      "const { spawnSync } = require('node:child_process');",
      "const args = process.argv.slice(2);",
      "const real = process.env.A_STOCK_AGENT_BROWSER_REAL;",
      "const state = process.env.AGENT_BROWSER_STATE;",
      "const command = args.findIndex((value) => value === 'open');",
      "const run = (next) => { const result = spawnSync(real, [process.env.A_STOCK_TEST_BROWSER_SCRIPT, ...next], { stdio: 'inherit' }); if (result.error) throw result.error; if (result.status !== 0) process.exit(result.status ?? 1); };",
      "if (command >= 0 && state && fs.existsSync(state)) { run([...args.slice(0, command), 'open', 'about:blank']); run([...args.slice(0, command), 'state', 'load', fs.realpathSync(state)]); }",
      "run(args);",
    ].join('\n'));
    wrapperArguments = [nodeWrapper, ...wrapperArguments];
  }
  const result = spawnSync(wrapperCommand, wrapperArguments, {
    encoding: 'utf8',
    env: {
      ...process.env,
      A_STOCK_AGENT_BROWSER_REAL: process.platform === 'win32' ? process.execPath : fakeBrowser,
      ...(process.platform === 'win32' ? { A_STOCK_TEST_BROWSER_SCRIPT: fakeBrowser } : {}),
      A_STOCK_TEST_CALL_LOG: logPath,
      AGENT_BROWSER_STATE: statePath,
      AGENT_BROWSER_SOCKET_DIR: socketDir,
    },
  });

  assert.equal(result.status, 0, result.stderr);
  const calls = fs.readFileSync(logPath, 'utf8').trim().split(/\r?\n/).map((call) => call.replace(/\r$/, ''));
  const resolvedStatePath = fs.realpathSync(statePath);
  assert.deepEqual(calls, [
    '--session test-session --json open about:blank',
    `--session test-session --json state load ${resolvedStatePath}`,
    '--session test-session --json open https://xueqiu.com/',
  ]);
});
