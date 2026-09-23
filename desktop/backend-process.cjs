const net = require('node:net');
const { spawn, execFile } = require('node:child_process');
const { setTimeout: delay } = require('node:timers/promises');

function buildBackendEnv({ addr, token, baseEnv = process.env, extraEnv = {} }) {
  if (!/^127\.0\.0\.1:\d+$/.test(addr) || !token) throw new Error('Backend requires loopback address and authentication');
  return { ...baseEnv, ...extraEnv, MYSTOCKTRACER_ADDR: addr, MYSTOCKTRACER_TOKEN: token, MYSTOCKTRACER_DESKTOP_CHILD: '1' };
}
function resolveBackendCommand({ backendBin, backendDir, isPackaged }) {
  if (backendBin) return { command: backendBin, args: [], cwd: backendDir };
  if (isPackaged) throw new Error('packaged backend binary not found');
  return { command: 'go', args: ['run', './cmd/server'], cwd: backendDir };
}
async function findFreePort(host = '127.0.0.1', startPort = 20000, endPort = 29999) {
  if (host !== '127.0.0.1') throw new Error('Only loopback is allowed');
  for (let port = startPort; port <= endPort; port++) {
    const free = await new Promise((resolve, reject) => {
      const probe = net.createServer();
      probe.once('error', (error) => ['EADDRINUSE', 'EACCES'].includes(error.code) ? resolve(false) : reject(error));
      probe.listen(port, host, () => probe.close((error) => error ? reject(error) : resolve(true)));
    });
    if (free) return port;
  }
  throw new Error('No desktop loopback port available');
}
function startBackend(options) {
  return spawn(options.command, options.args, { cwd: options.cwd, env: buildBackendEnv(options), windowsHide: true, stdio: ['pipe', 'pipe', 'pipe'] });
}
async function waitForHealth(backendUrl, timeoutMS = 15000, child) {
  const endpoint = new URL('/api/health', backendUrl);
  if (endpoint.hostname !== '127.0.0.1' || endpoint.protocol !== 'http:') throw new Error('Invalid backend health endpoint');
  const deadline = Date.now() + timeoutMS;
  while (Date.now() < deadline) {
    if (child && child.exitCode !== null) throw new Error('Backend exited before health became ready');
    try {
      const reply = await fetch(endpoint, { signal: AbortSignal.timeout(Math.max(1, Math.min(750, deadline - Date.now()))), redirect: 'error' });
      if (reply.ok) return;
    } catch (error) { if (!['TypeError', 'TimeoutError', 'AbortError'].includes(error.name)) throw error; }
    await delay(Math.min(100, Math.max(0, deadline - Date.now())));
  }
  throw new Error('Backend health check timed out');
}
async function stopBackend(child, timeout = 10000) {
  if (!child || child.exitCode !== null || child.signalCode !== null) return;
  const closed = new Promise((resolve) => child.once('exit', resolve));
  child.stdin?.end();
  await Promise.race([closed, delay(timeout, undefined, { ref: false })]);
  if (child.exitCode !== null || child.signalCode !== null) return;
  if (process.platform === 'win32') {
    await new Promise((resolve, reject) => execFile('taskkill.exe', ['/PID', String(child.pid), '/T', '/F'], { windowsHide: true }, (error) => error && child.exitCode === null ? reject(new Error('Backend process tree did not stop')) : resolve()));
  } else child.kill('SIGKILL');
  await Promise.race([closed, delay(2000, undefined, { ref: false })]);
  if (child.exitCode === null && child.signalCode === null) throw new Error('Backend cleanup timed out');
}
module.exports = { buildBackendEnv, resolveBackendCommand, findFreePort, startBackend, waitForHealth, stopBackend };