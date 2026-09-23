import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { createRequire } from 'node:module';
import { spawnSync } from 'node:child_process';

const repository = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const require = createRequire(import.meta.url);
export function developmentLaunch(kind, environment = process.env) {
  const env = { ...environment };
  if (kind === 'backend') {
    env.MYSTOCKTRACER_ADDR = (Object.hasOwn(env, 'MYSTOCKTRACER_ADDR') ? env.MYSTOCKTRACER_ADDR : env.A_STOCK_ADDR) || '127.0.0.1:20081';
    return { command: 'go', args: ['run', './cmd/server'], cwd: path.join(repository, 'backend'), env };
  }
  if (kind === 'desktop') {
    env.ELECTRON_RENDERER_URL ||= 'http://127.0.0.1:20073';
    return { command: require('electron'), args: ['.'], cwd: path.join(repository, 'desktop'), env };
  }
  throw new Error('Choose backend or desktop development mode');
}
export function runDevelopment(kind, runner = spawnSync) {
  const { command, args, cwd, env } = developmentLaunch(kind);
  const result = runner(command, args, { cwd, env, stdio: 'inherit', windowsHide: true });
  if (result.error) throw result.error;
  return result.status ?? 1;
}
if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  try { process.exitCode = runDevelopment(process.argv[2]); }
  catch (error) { console.error(error.message); process.exitCode = 1; }
}
