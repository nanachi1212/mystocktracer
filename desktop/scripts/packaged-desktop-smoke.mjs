import assert from 'node:assert/strict';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import net from 'node:net';
import { spawn, spawnSync } from 'node:child_process';

// Exercise only synthetic state; never select the current user's Electron profile.
const bundle = path.resolve(process.argv[2] || 'desktop/dist/builder-dir/win-unpacked');
const executable = path.join(bundle, 'mystocktracer.exe');
const temporary = fs.mkdtempSync(path.join(os.tmpdir(), 'mystocktracer-desktop-smoke-'));
const profile = path.join(temporary, 'mystocktracer');
const legacy = path.join(temporary, 'easy-stock');
const resources = path.join(bundle, 'resources');
const python = path.join(resources, 'resources/hermes-runtime/python/python.exe');
let child;
let socket;
let diagnostics = '';
try {
  const version = spawnSync('powershell.exe', ['-NoProfile', '-Command', '(Get-Item -LiteralPath $env:MYSTOCKTRACER_SMOKE_EXE).VersionInfo | Select-Object ProductName,FileDescription | ConvertTo-Json -Compress'], { encoding: 'utf8', windowsHide: true, env: { ...process.env, MYSTOCKTRACER_SMOKE_EXE: executable } });
  assert.equal(version.status, 0, 'read packaged Windows product metadata');
  assert.equal(JSON.parse(version.stdout).ProductName, 'mystocktracer');
  assert.equal(JSON.parse(version.stdout).FileDescription, 'mystocktracer');
  fs.mkdirSync(legacy);
  const settings = Buffer.from('{}\n');
  fs.writeFileSync(path.join(legacy, 'settings.json'), settings);
  fs.writeFileSync(path.join(legacy, 'user-note.txt'), 'synthetic migration sentinel');
  const db = path.join(legacy, 'fixture.db');
  const created = spawnSync(python, ['-I', '-c', 'import sqlite3,sys; c=sqlite3.connect(sys.argv[1]); c.execute("create table sentinel (value text)"); c.execute("insert into sentinel values (?)", ("preserved",)); c.commit(); c.close()', db], { windowsHide: true });
  assert.equal(created.status, 0, 'bundled SQLite fixture creation');
  const originalDB = fs.readFileSync(db);
  const migration = path.join(temporary, 'migrate.cjs');
  fs.writeFileSync(migration, `const assert=require('node:assert/strict');
const {selectUserData}=require(${JSON.stringify(path.join(resources, 'app.asar/user-data-migration.cjs'))});
const result=selectUserData({appDataPath:${JSON.stringify(temporary)},env:{},python:${JSON.stringify(python)},helper:${JSON.stringify(path.join(resources, 'state-copy.py'))}});
assert.equal(result.path,${JSON.stringify(profile)});
console.log('Packaged migration activated synthetic canonical profile');\n`);
  const copied = spawnSync(executable, [migration], { env: { ...process.env, ELECTRON_RUN_AS_NODE: '1' }, encoding: 'utf8', windowsHide: true });
  assert.equal(copied.status, 0, copied.stderr);
  for (const directory of [legacy, profile]) {
    assert.deepEqual(fs.readFileSync(path.join(directory, 'fixture.db')), originalDB);
    assert.deepEqual(fs.readFileSync(path.join(directory, 'settings.json')), settings);
    assert.equal(fs.readFileSync(path.join(directory, 'user-note.txt'), 'utf8'), 'synthetic migration sentinel');
  }
  const port = await new Promise((resolve, reject) => {
    const server = net.createServer(); server.on('error', reject);
    server.listen(0, '127.0.0.1', () => { const port = server.address().port; server.close(() => resolve(port)); });
  });
  const env = Object.fromEntries(Object.entries(process.env).filter(([key]) => !/^(MYSTOCKTRACER_|A_STOCK_|HERMES_)/i.test(key)));
  Object.assign(env, { MYSTOCKTRACER_USER_DATA_DIR: profile, MYSTOCKTRACER_LOG_DIR: path.join(profile, 'logs') });
  delete env.ELECTRON_RUN_AS_NODE;
  child = spawn(executable, [`--remote-debugging-port=${port}`, '--remote-debugging-address=127.0.0.1'], { cwd: bundle, env, stdio: ['ignore', 'pipe', 'pipe'], windowsHide: true });
  child.stdout.on('data', bytes => { diagnostics = (diagnostics + bytes).slice(-4000); });
  child.stderr.on('data', bytes => { diagnostics = (diagnostics + bytes).slice(-4000); });
  let target;
  await until(async () => {
    assert.equal(child.exitCode, null, 'packaged desktop exited before readiness');
    const pages = await fetch(`http://127.0.0.1:${port}/json/list`).then(response => response.json());
    target = pages.find(page => page.type === 'page' && page.url.startsWith('file:'));
    return Boolean(target);
  });
  socket = new WebSocket(target.webSocketDebuggerUrl);
  await new Promise((resolve, reject) => { socket.addEventListener('open', resolve, { once: true }); socket.addEventListener('error', reject, { once: true }); });
  let id = 0;
  const evaluate = expression => new Promise((resolve, reject) => {
    const request = ++id;
    const timer = setTimeout(() => { socket.removeEventListener('message', listener); reject(new Error('Renderer evaluation timed out')); }, 5000);
    const listener = event => {
      const message = JSON.parse(event.data);
      if (message.id !== request) return;
      clearTimeout(timer); socket.removeEventListener('message', listener);
      if (message.error || message.result.exceptionDetails) reject(new Error('Renderer evaluation failed'));
      else resolve(message.result.result.value);
    };
    socket.addEventListener('message', listener);
    socket.send(JSON.stringify({ id: request, method: 'Runtime.evaluate', params: { expression, returnByValue: true } }));
  });
  await until(async () => evaluate(`Boolean(window.mystocktracer && document.querySelector('#root')?.textContent?.includes('台股'))`));
  const state = await evaluate(`({title:document.title, bridge:typeof window.mystocktracer, oldBridge:typeof window.easyStock, images:[...document.images].map(image=>({loaded:image.complete && image.naturalWidth>0,src:image.getAttribute('src')}))})`);
  assert.match(state.title, /mystocktracer/i);
  assert.equal(state.bridge, 'object');
  assert.equal(state.oldBridge, 'undefined');
  assert.ok(state.images.length > 0 && state.images.every(image => image.loaded), 'brand assets must render');
  await until(async () => fs.readFileSync(path.join(profile, 'logs/desktop.log'), 'utf8').includes('frontend ready'));
  assert.deepEqual(fs.readFileSync(db), originalDB, 'legacy database remains untouched');
  console.log('PASS: exact packaged executable, canonical bridge, rendered brand assets, backend/frontend readiness and packaged migration with source preservation');
} catch (error) {
  throw new Error(`${error.message}\n${diagnostics}`, { cause: error });
} finally {
  socket?.close();
  if (child && child.exitCode === null) {
    spawnSync('taskkill', ['/PID', String(child.pid), '/T', '/F'], { windowsHide: true, stdio: 'ignore' });
    if (child.exitCode === null) await new Promise(resolve => { child.once('exit', resolve); setTimeout(resolve, 3000); });
  }
  fs.rmSync(temporary, { recursive: true, force: true, maxRetries: 5, retryDelay: 200 });
}

async function until(check) {
  const deadline = Date.now() + 25000;
  let last;
  while (Date.now() < deadline) {
    try { if (await check()) return; } catch (error) { last = error; }
    await new Promise(resolve => setTimeout(resolve, 150));
  }
  throw new Error('Packaged desktop readiness timed out', { cause: last });
}
