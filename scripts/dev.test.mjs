import test from 'node:test';
import assert from 'node:assert/strict';
import path from 'node:path';
import { developmentLaunch, runDevelopment } from './dev.mjs';

test('development startup carries settings as environment values, without shell syntax', () => {
  const env = { A_STOCK_ADDR: '127.0.0.1:24001', MYSTOCKTRACER_ADDR: '127.0.0.1:24002' };
  const backend = developmentLaunch('backend', env);
  assert.equal(backend.command, 'go');
  assert.deepEqual(backend.args, ['run', './cmd/server']);
  assert.equal(backend.env.MYSTOCKTRACER_ADDR, '127.0.0.1:24002');
  assert.equal(path.basename(backend.cwd), 'backend');
  assert.equal(developmentLaunch('backend', { A_STOCK_ADDR: env.A_STOCK_ADDR }).env.MYSTOCKTRACER_ADDR, env.A_STOCK_ADDR);
  assert.equal(developmentLaunch('backend', { ...env, MYSTOCKTRACER_ADDR: '' }).env.MYSTOCKTRACER_ADDR, '127.0.0.1:20081');
  const desktop = developmentLaunch('desktop', {});
  assert.equal(path.basename(desktop.cwd), 'desktop');
  assert.equal(desktop.env.ELECTRON_RENDERER_URL, 'http://127.0.0.1:20073');
  assert.deepEqual(desktop.args, ['.']);
  assert.throws(() => developmentLaunch('unrecognized'));
});

test('development runner propagates failure and process startup errors', () => {
  assert.equal(runDevelopment('backend', (_command, _args, options) => { assert.equal(options.shell, undefined); return { status: 7 }; }), 7);
  assert.equal(runDevelopment('backend', () => ({ status: 0 })), 0);
  assert.equal(runDevelopment('backend', () => ({ status: null })), 1);
  assert.throws(() => runDevelopment('backend', () => ({ error: new Error('synthetic missing executable') })), /missing executable/);
});
