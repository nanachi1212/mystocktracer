import fs from 'node:fs';
import path from 'node:path';
import { createHash } from 'node:crypto';
import { execFileSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';

export const FORK = 'b969d05984dda736de9ee3dc2a881e8c65a373c5';
const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const legacyExtensions = new Set(['.go', '.ts', '.tsx', '.cjs', '.mjs', '.css', '.html']);
const executableExtensions = new Set([...legacyExtensions, '.py', '.sh', '.ps1', '.cmd', '.bat', '.json', '.yml', '.yaml']);
const legacyName = /\b(?:easy[-_]?stock(?:_[A-Z_]+)?|a-stock-ai|A_STOCK_[A-Z_]+|VITE_A_STOCK_[A-Z_]+|com\.jundizhou\.easystock|jundizhou)\b/i;
const legacyReviewed = new Set([
  'backend/cmd/server/main.go', 'backend/internal/appsettings/store.go',
  'backend/internal/httpapi/ai_chat.go', 'backend/internal/httpapi/config.go', 'backend/internal/httpapi/server.go',
  'frontend/src/App.tsx', 'frontend/src/lib/backend.ts', 'frontend/src/styles.css',
  'backend/internal/httpapi/llm_connection.go', 'backend/internal/httpapi/llm_models.go',
  'frontend/src/components/AIChatWorkspace.tsx', 'frontend/src/components/MarkdownContent.tsx',
  'frontend/src/components/SettingsDrawer.tsx',
]);
export const fingerprint = bytes => createHash('sha256').update(bytes.includes(0) ? bytes : bytes.toString('utf8').replace(/\r\n/g, '\n')).digest('hex');
function git(cwd, ...args) {
  return execFileSync('git', ['-C', cwd, ...args], { maxBuffer: 64 * 1024 * 1024, windowsHide: true });
}
function snapshot(cwd, ref) {
  if (ref === 'WORKTREE') {
    const names = git(cwd, 'ls-files', '-z', '--cached', '--others', '--exclude-standard').toString('utf8').split('\0').filter(Boolean);
    return new Map([...new Set(names)].sort().filter(name => fs.existsSync(path.join(cwd, name))).map(name => [name, fs.readFileSync(path.join(cwd, name))]));
  }
  const entries = git(cwd, 'ls-tree', '-r', '-z', ref).toString('utf8').split('\0').filter(Boolean);
  const blobs = entries.map(entry => { const match = entry.match(/^\d+ blob ([a-f0-9]+)\t([\s\S]+)$/); if (!match) throw new Error('Unsupported tree entry: ' + entry); return { hash: match[1], name: match[2] }; });
  const output = execFileSync('git', ['-C', cwd, 'cat-file', '--batch'], { input: blobs.map(blob => blob.hash).join('\n') + '\n', maxBuffer: 128 * 1024 * 1024, windowsHide: true });
  let offset = 0;
  return new Map(blobs.map(blob => {
    const end = output.indexOf(10, offset), header = output.subarray(offset, end).toString();
    const size = Number(header.split(' ')[2]);
    if (!Number.isSafeInteger(size)) throw new Error('Invalid git blob response');
    const bytes = output.subarray(end + 1, end + 1 + size);
    offset = end + 2 + size;
    return [blob.name, bytes];
  }));
}
function renames(cwd, base, ref) {
  const args = ['diff', '--name-status', '-z', '-M30%', '-C30%', '--find-copies-harder', base];
  if (ref !== 'WORKTREE') args.push(ref);
  const fields = git(cwd, ...args).toString('utf8').split('\0'), results = new Map();
  for (let i = 0; i < fields.length && fields[i];) {
    const status = fields[i++];
    const source = fields[i++];
    if (/^[RC]/.test(status)) results.set(fields[i++], source);
  }
  return results;
}
function area(name, bytes) {
  if (/^(LICENSE|NOTICE\.md|THIRD_PARTY_NOTICES\.md)$/.test(name)) return 'legal';
  // Only the preserved upstream license qualifies; renamed code or replacement
  // content under LICENSES must still participate in technical blocker checks.
  if (name === 'LICENSES/easy-stock-PolyForm-Noncommercial-1.0.0.txt'
      && fingerprint(bytes) === 'c33d0f2551b1f6dd06d0643109c84ef8e8f4465dae0d3d2208d7ad60ff00963f') return 'legal';
  if (name.startsWith('.github/release-notes/')) return 'historical-document';
  if (/\.(png|svg|ico|icns|jpg|woff2?|ttf)$/.test(name)) return 'asset';
  if (/(?:^|\/)(?:test|testdata)\//.test(name) || /(?:\.test\.|_test\.go$)/.test(name)) return 'test';
  if (/\.(md|txt)$/.test(name)) return 'document';
  if (/^(\.github\/workflows\/|scripts\/|desktop\/scripts\/)/.test(name)) return 'packaging-tool';
  if (/^(package-lock.json|backend\/go.sum)$/.test(name)) return 'dependency-resolution';
  if (/(\.json|\.yml|\.yaml|go.mod|\.d\.ts)$/.test(name) || name.startsWith('.')) return 'config';
  return 'runtime';
}
const lineCount = bytes => {
  if (bytes.includes(0)) return 0;
  const text = bytes.toString('utf8').replace(/\r\n/g, '\n');
  return text ? text.split('\n').length - Number(text.endsWith('\n')) : 0;
};
export function inspect({ cwd = root, base = FORK, ref = 'WORKTREE', decisions = [] } = {}) {
  const original = snapshot(cwd, base), current = snapshot(cwd, ref), moved = renames(cwd, base, ref);
  const hashes = new Map();
  for (const [name, bytes] of original) {
    const digest = fingerprint(bytes);
    if (!hashes.has(digest)) hashes.set(digest, name);
  }
  const code = name => /\.(go|ts|tsx|cjs|mjs|py|css)$/.test(name);
  const expressions = bytes => bytes.toString('utf8').split(/\r?\n/).map(line => line.trim().replace(/\s+/g, ' ')).filter(line => line.length >= 30 && !/^(?:import\b|from\b|\/\/|#|\/?\*|const .*require\(|package\b)/.test(line));
  const owners = new Map();
  for (const [name, bytes] of original) if (code(name)) for (const line of new Set(expressions(bytes))) {
    if (!owners.has(line)) owners.set(line, new Set());
    owners.get(line).add(name);
  }
  const overlap = (name, bytes) => {
    if (!code(name)) return null;
    const scores = new Map(), lines = [...new Set(expressions(bytes))];
    for (const line of lines) for (const owner of owners.get(line) || []) {
      const score = scores.get(owner) || { lines: 0, characters: 0 };
      score.lines++; score.characters += line.length; scores.set(owner, score);
    }
    const total = lines.reduce((sum, line) => sum + line.length, 0);
    const best = [...scores].sort((a, b) => b[1].characters - a[1].characters)[0];
    return best && best[1].lines >= 4 && best[1].characters >= 200 && best[1].characters / Math.max(1, total) >= 0.2 ? best[0] : null;
  };
  const reviewed = new Map(decisions.map(item => [item.path, item]));
  const files = [], references = [];
  for (const [name, bytes] of current) {
    const kind = area(name, bytes), text = bytes.includes(0) ? '' : bytes.toString('utf8');
    const digest = fingerprint(bytes);
    const upstreamPath = original.has(name) ? name : hashes.get(digest) || moved.get(name) || overlap(name, bytes) || null;
    const decision = reviewed.get(name);
    let category = upstreamPath ? (fingerprint(bytes) === fingerprint(original.get(upstreamPath)) ? 'confirmed-inherited' : 'likely-inherited') : 'original';
    let reason = upstreamPath ? 'fork blob/path, Git rename/copy or substantive-line overlap candidate; no complete replacement attestation' : 'post-fork file; exact-blob, Git rename/copy and substantive-line screens find no upstream origin';
    if (decision && fingerprint(bytes) === decision.sha256) {
      category = 'original';
      reason = decision.evidence;
    } else if (kind === 'asset' && !upstreamPath) {
      category = 'unclear'; reason = 'new artwork requires generator/source evidence';
    } else if (/easy-stock\.(png|svg|ico|icns)$/.test(name)) {
      category = 'unclear'; reason = 'legacy artwork has no authored source evidence';
    }
    if (kind === 'dependency-resolution' || (name === 'frontend/src/vite-env.d.ts' && text.trim() === '/// <reference types="vite/client" />')) {
      category = 'third-party';
      reason = 'dependency resolution or standard Vite type directive, not inherited product implementation';
    }
    const inherited = category.endsWith('inherited');
    const compatibility = legacyName.test(text);
    let disposition = inherited ? 'replace-or-obtain-rights' : 'maintain';
    if (kind === 'legal') disposition = 'retain-legal-attribution';
    else if (kind === 'historical-document') disposition = 'retain-historical-attribution';
    else if (kind === 'dependency-resolution') disposition = 'third-party-dependency';
    else if (inherited && /(?:backend\/internal\/(?:foundation\/(?:market_data|market_index|source)|agent\/types|appsettings\/model)|backend\/internal\/httpapi\/(?:settings_contract|settings_view|agent_settings_contract|agent_settings_model))\.go$/.test(name)) disposition = 'retained-contract-compatibility';
    else if (!inherited && compatibility) disposition = 'original-with-retained-compatibility-or-attribution';
    const loc = lineCount(bytes);
    const codeLOC = executableExtensions.has(path.extname(name)) || /^#!/.test(text) ? loc : 0;
    const legacyLOC = legacyExtensions.has(path.extname(name)) ? loc : 0;
    const replacementRequired = category === 'unclear' || (inherited && !['legal', 'historical-document', 'dependency-resolution'].includes(kind));
    const blocker = category === 'unclear' || inherited;
    files.push({ path: name, area: kind, category, upstreamPath, disposition, blockingRelicensing: blocker, replacementRequired, lines: loc, codeLOC, legacyLOC, sha256: digest, reason });
    for (const [index, line] of text.split(/\r?\n/).entries()) {
      if (legacyName.test(line)) {
        const referenceKind = kind === 'legal' || kind === 'historical-document' || /copyright|衍生|原作者|attribution|授權|license|upstream|Git 歷史/i.test(line) || name.startsWith('docs/oss-')
          ? 'legal-attribution'
          : category === 'third-party' ? 'third-party-dependency'
          : inherited ? 'inherited-implementation'
          : kind === 'test' ? 'original-regression-test'
          : 'backward-compatibility-migration';
        references.push({ path: name, line: index + 1, referenceKind, disposition, inherited, area: kind });
      }
    }
  }
  const inherited = files.filter(file => file.category.endsWith('inherited'));
  const summary = {
    total: files.length,
    inherited: inherited.length,
    confirmed: files.filter(file => file.category === 'confirmed-inherited').length,
    likely: files.filter(file => file.category === 'likely-inherited').length,
    original: files.filter(file => file.category === 'original').length,
    unclear: files.filter(file => file.category === 'unclear').length,
    thirdParty: files.filter(file => file.category === 'third-party').length,
    inheritedLOC: inherited.reduce((sum, file) => sum + file.codeLOC, 0),
    inheritedLegacyLOC: inherited.reduce((sum, file) => sum + file.legacyLOC, 0),
    blockingFiles: files.filter(file => file.blockingRelicensing).length,
    technicalBlockingFiles: files.filter(file => file.replacementRequired).length,
  };
  // Reproduce the previous path-based method for an honest historical comparison.
  const legacy = { total: 0, inherited: 0, inheritedLOC: 0, unclear: 0 };
  for (const [name, bytes] of current) {
    if (/(^|\/)(node_modules|vendor|dist|build|out|coverage)(\/|$)|(^|\/)(package-lock\.json|go\.sum)$/.test(name)) continue;
    legacy.total++;
    if (/^(desktop\/assets\/easy-stock\.|frontend\/public\/easy-stock-mark)/.test(name)) { legacy.unclear++; continue; }
    if (original.has(name) && !legacyReviewed.has(name)) {
      legacy.inherited++;
      if (legacyExtensions.has(path.extname(name))) legacy.inheritedLOC += bytes.toString('utf8').replace(/\r/g, '').split('\n').length - 1;
    }
  }
  return { ref, summary, legacy, files, references };
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  const manifest = JSON.parse(fs.readFileSync(path.join(root, 'docs/oss-provenance-decisions.json'), 'utf8'));
  const baselineManifest = JSON.parse(git(root, 'show', `${manifest.baseline}:docs/oss-provenance-decisions.json`).toString('utf8'));
  const before = inspect({ ref: manifest.baseline, decisions: baselineManifest.files });
  const after = inspect({ decisions: manifest.files });
  const result = { schema: 2, auditDate: new Date().toISOString().slice(0, 10), fork: FORK, baseline: manifest.baseline, before: { summary: before.summary, legacy: before.legacy }, after: { summary: after.summary, legacy: after.legacy }, files: after.files, references: after.references };
  const staleDecisions = manifest.files.filter(item => !after.files.some(file => file.path === item.path && file.sha256 === item.sha256)).map(item => item.path);
  result.staleDecisions = staleDecisions;
  if (process.argv.includes("--summary")) { delete result.files; delete result.references; }
  process.stdout.write(JSON.stringify(result, null, 2) + "\n");
  if (after.summary.unclear || staleDecisions.length) process.exitCode = 1;
}
