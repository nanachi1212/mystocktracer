import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { execFileSync } from 'node:child_process';
import { inspect, fingerprint } from './provenance-audit.mjs';

function repository(t) {
  const cwd = fs.mkdtempSync(path.join(os.tmpdir(), 'mystocktracer provenance 測試 '));
  t.after(() => fs.rmSync(cwd, { recursive: true, force: true, maxRetries: 4 }));
  const git = (...args) => execFileSync('git', ['-C', cwd, ...args], { encoding: 'utf8', windowsHide: true });
  git('init', '-q');
  git('config', 'user.name', 'Synthetic Fixture');
  git('config', 'user.email', 'fixture@example.invalid');
  git('config', 'core.autocrlf', 'false');
  const write = (name, value) => {
    fs.mkdirSync(path.dirname(path.join(cwd, name)), { recursive: true });
    fs.writeFileSync(path.join(cwd, name), value);
  };
  write('tool.py', 'print("synthetic original")\n');
  write('scripts/run.sh', '#!/bin/sh\nprintf "fixture"\n');
  write('.github/workflows/check.yml', 'name: Fixture\n');
  git('add', '.');
  git('commit', '-qm', 'synthetic baseline');
  return { cwd, git, write, base: git('rev-parse', 'HEAD').trim() };
}

test('renamed upstream bytes cannot become original by changing their path', t => {
  const f = repository(t);
  f.git('mv', 'tool.py', 'renamed.py');
  f.write('copied.py', 'print("synthetic original")\n');
  const result = inspect(f);
  for (const name of ['renamed.py', 'copied.py']) {
    const file = result.files.find(file => file.path === name);
    assert.equal(file.category, 'confirmed-inherited');
    assert.equal(file.upstreamPath, 'tool.py');
  }
});

test('Python, shell and workflow lines participate in expanded inherited LOC', t => {
  const f = repository(t);
  const result = inspect(f);
  assert.equal(result.summary.inheritedLOC, 4);
  assert.equal(result.legacy.inheritedLOC, 0);
  assert.equal(result.summary.total, 3);
});

test('replacement evidence is bound to content and becomes stale on further edits', t => {
  const f = repository(t);
  const replacement = 'print("independent contract")\n';
  f.write('tool.py', replacement);
  const decisions = [{ path: 'tool.py', sha256: fingerprint(Buffer.from(replacement)), evidence: 'synthetic contract proof' }];
  assert.equal(inspect({ ...f, decisions }).files.find(file => file.path === 'tool.py').category, 'original');
  f.write('tool.py', replacement + 'print("unreviewed change")\n');
  assert.equal(inspect({ ...f, decisions }).files.find(file => file.path === 'tool.py').category, 'likely-inherited');
});

test('new artwork requires source evidence and CRLF does not hide upstream identity', t => {
  const f = repository(t);
  f.write('brand.png', Buffer.from([137, 80, 78, 71, 0, 255]));
  f.write('tool.py', 'print("synthetic original")\r\n');
  const result = inspect(f);
  assert.equal(result.files.find(file => file.path === 'brand.png').category, 'unclear');
  assert.equal(result.files.find(file => file.path === 'tool.py').category, 'confirmed-inherited');
});

test('extracting a fragment from a large upstream file still raises a provenance candidate', t => {
  const f = repository(t);
  const lines = Array.from({ length: 40 }, (_, index) => 'const fixture_' + index + ' = "distinct synthetic expression number ' + index + '";');
  f.write('large.ts', lines.join('\n') + '\n');
  f.git('add', '.'); f.git('commit', '-qm', 'large synthetic source');
  f.base = f.git('rev-parse', 'HEAD').trim();
  f.write('fragment.ts', lines.slice(10, 16).join('\n') + '\n');
  const fragment = inspect(f).files.find(file => file.path === 'fragment.ts');
  assert.equal(fragment.category, 'likely-inherited');
  assert.equal(fragment.upstreamPath, 'large.ts');
});

test('a stale artwork decision does not approve changed binary pixels', t => {
  const f = repository(t), bytes = Buffer.from([137, 80, 78, 71, 0, 255]);
  f.write('brand.png', bytes);
  const decisions = [{ path: 'brand.png', sha256: fingerprint(bytes), evidence: 'synthetic generator' }];
  assert.equal(inspect({ ...f, decisions }).files.find(file => file.path === 'brand.png').category, 'original');
  f.write('brand.png', Buffer.from([137, 80, 78, 71, 0, 254]));
  assert.equal(inspect({ ...f, decisions }).files.find(file => file.path === 'brand.png').category, 'unclear');
});
