import { spawn, spawnSync } from 'node:child_process';
import fs from 'node:fs';
import net from 'node:net';
import os from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const desktopRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const repoRoot = path.resolve(desktopRoot, '..');
const packageRoot = path.resolve(process.argv[2] || path.join(desktopRoot, 'dist', 'builder-dir', 'win-unpacked'));
const backend = path.join(packageRoot, 'resources', 'resources', 'backend', 'mystocktracer-backend.exe');
if (!fs.existsSync(backend)) throw new Error(`Packaged backend not found: ${backend}`);

const temporary = fs.mkdtempSync(path.join(os.tmpdir(), 'mystocktracer-packaged-smoke-'));
const fakeRuntime = path.join(temporary, 'fake-agent-runtime.exe');
const build = spawnSync('go', ['build', '-trimpath', '-o', fakeRuntime, path.join(desktopRoot, 'scripts', 'testdata', 'fake-agent-runtime.go')], { cwd: repoRoot, stdio: 'inherit' });
if (build.status !== 0) throw new Error('Unable to build the fake runtime');

const port = await availablePort();
const token = 'packaged-smoke-token';
const home = path.join(temporary, 'runtime-home');
fs.mkdirSync(home, { recursive: true });
fs.writeFileSync(path.join(temporary, 'settings.json'), JSON.stringify({ llm: { provider:'custom', base_url:'http://127.0.0.1:9/v1', model:'fake-model', api_mode:'chat_completions', api_key:'test-only-key', response_timeout_seconds:30 }, llm_profiles:[{ id:'smoke', name:'Smoke', provider:'custom', base_url:'http://127.0.0.1:9/v1', model:'fake-model', api_mode:'chat_completions' }], active_llm_profile_id:'smoke', taiwan_alerts:{ corporate_events_enabled:true } }));

const child = spawn(backend, [], { cwd: packageRoot, windowsHide: true, stdio: ['ignore', 'pipe', 'pipe'], env: { ...process.env, MYSTOCKTRACER_ADDR:`127.0.0.1:${port}`, MYSTOCKTRACER_TOKEN:token, MYSTOCKTRACER_SETTINGS_PATH:path.join(temporary,'settings.json'), MYSTOCKTRACER_TAIWAN_WATCHLIST_DB:path.join(temporary,'watchlist.db'), MYSTOCKTRACER_TAIWAN_PORTFOLIO_DB:path.join(temporary,'portfolio.db'), MYSTOCKTRACER_CASHFLOW_CACHE:path.join(temporary,'cashflow'), MYSTOCKTRACER_LOG_DIR:path.join(temporary,'logs'), MYSTOCKTRACER_HERMES_HOME:home, MYSTOCKTRACER_HERMES_WORKDIR:temporary, MYSTOCKTRACER_HERMES_PYTHON:fakeRuntime, MYSTOCKTRACER_HERMES_RUNTIME_ROOT:path.join(packageRoot,'resources','resources','hermes-runtime') } });
let diagnostics = '';
child.stdout.on('data', (chunk) => { diagnostics += chunk.toString(); });
child.stderr.on('data', (chunk) => { diagnostics += chunk.toString(); });

try {
	await waitForHealth(`http://127.0.0.1:${port}/api/health`);
	const result = await streamSmoke(`ws://127.0.0.1:${port}/api/v1/ai/ws?token=${encodeURIComponent(token)}`);
	if (result.deltas < 2 || result.content !== 'packaged stream ok') throw new Error(`Unexpected stream result: ${JSON.stringify(result)}`);
	console.log(`Packaged fake-runtime streaming smoke passed: ${result.deltas} deltas, ${result.content}`);
} catch (error) {
	throw new Error(`${error instanceof Error ? error.message : error}\nBackend diagnostics:\n${diagnostics.slice(-4000)}`);
} finally {
	if (child.exitCode === null) child.kill();
	if (child.exitCode === null) await new Promise((resolve) => { const timer = setTimeout(resolve, 3000); child.once('exit', () => { clearTimeout(timer); resolve(); }); });
	fs.rmSync(temporary, { recursive: true, force: true });
}

function availablePort() { return new Promise((resolve, reject) => { const server = net.createServer(); server.on('error', reject); server.listen(0, '127.0.0.1', () => { const address = server.address(); const port = typeof address === 'object' && address ? address.port : 0; server.close((error) => error ? reject(error) : resolve(port)); }); }); }
async function waitForHealth(url) { const deadline = Date.now() + 15000; while (Date.now() < deadline) { try { const response = await fetch(url); if (response.ok) return; } catch {} await new Promise((resolve) => setTimeout(resolve, 100)); } throw new Error('Packaged backend health check timed out'); }
function streamSmoke(url) { return new Promise((resolve, reject) => { const socket = new WebSocket(url); let deltas=0; const timeout=setTimeout(()=>{socket.close();reject(new Error('Streaming smoke timed out'))},15000); socket.addEventListener('error',()=>{clearTimeout(timeout);reject(new Error('WebSocket connection failed'))}); socket.addEventListener('message',(message)=>{ const event=JSON.parse(String(message.data)); if(event.type==='runtime.ready') socket.send(JSON.stringify({version:1,type:'session.start',payload:{seed_messages:[]}})); if(event.type==='session.ready') socket.send(JSON.stringify({version:1,type:'prompt.submit',payload:{session_id:event.payload.session_id,text:'stream smoke'}})); if(event.type==='message.delta') deltas++; if(event.type==='runtime.error'){clearTimeout(timeout);socket.close();reject(new Error(event.error?.message||'runtime error'))} if(event.type==='message.complete'){clearTimeout(timeout);socket.close();resolve({deltas,content:event.payload.content})} }); }); }
