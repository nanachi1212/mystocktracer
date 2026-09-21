import { execFileSync } from 'node:child_process';
import fs from 'node:fs';

const fork = 'b969d05984dda736de9ee3dc2a881e8c65a373c5';
const phaseB2Base = '1b4310da00607a9431d18ccdddd6347f0b94d204';
const codeExtensions = new Set(['.go', '.ts', '.tsx', '.cjs', '.mjs', '.css', '.html']);
const excluded = /(^|\/)(node_modules|vendor|dist|build|out|coverage)(\/|$)|(^|\/)(package-lock\.json|go\.sum)$/;
const unclear = /^(desktop\/assets\/easy-stock\.|frontend\/public\/easy-stock-mark)/;
const phaseB2Replacements = new Set([
  'backend/cmd/server/main.go',
  'backend/internal/appsettings/store.go',
  'backend/internal/httpapi/ai_chat.go',
  'backend/internal/httpapi/config.go',
  'backend/internal/httpapi/server.go',
  'frontend/src/App.tsx',
  'frontend/src/lib/backend.ts',
  'frontend/src/styles.css',
]);

function git(...args) {
  return execFileSync('git', args, { encoding: 'utf8', maxBuffer: 64 * 1024 * 1024 }).replace(/\r/g, '');
}

function treePaths(tree) {
  return git('ls-tree', '-r', '--name-only', tree).trim().split('\n').filter(Boolean);
}

function treeBlobs(tree) {
  const result = new Map();
  for (const line of git('ls-tree', '-r', tree).trim().split('\n')) {
    const match = line.match(/^\d+ blob ([0-9a-f]+)\t(.+)$/);
    if (match) result.set(match[2], match[1]);
  }
  return result;
}

function currentPaths() {
  return git('ls-files', '--cached', '--others', '--exclude-standard').trim().split('\n').filter((path) => path && fs.existsSync(path));
}

function extension(path) {
  const index = path.lastIndexOf('.');
  return index < 0 ? '' : path.slice(index);
}

function treeLineCounts(tree) {
  const result = new Map();
  const patterns = [...codeExtensions].map((item) => `*${item}`);
  let output = '';
  try {
    output = git('grep', '-c', '-e', '^', tree, '--', ...patterns);
  } catch (error) {
    output = String(error.stdout || '').replace(/\r/g, '');
  }
  for (const line of output.trim().split('\n')) {
    const match = line.match(/^[^:]+:(.+):(\d+)$/);
    if (match) result.set(match[1], Number(match[2]));
  }
  return result;
}

function audit(tree, replacementOverride) {
  const forkBlobs = treeBlobs(fork);
  const treeBlobMap = tree === 'WORKTREE' ? null : treeBlobs(tree);
  const paths = (tree === 'WORKTREE' ? currentPaths() : treePaths(tree)).filter((path) => !excluded.test(path));
  const currentHashes = tree === 'WORKTREE'
    ? execFileSync('git', ['hash-object', '--stdin-paths'], { input: paths.join('\n') + '\n', encoding: 'utf8' }).replace(/\r/g, '').trim().split('\n')
    : [];
  const currentHashByPath = new Map(tree === 'WORKTREE' ? paths.map((path, index) => [path, currentHashes[index]]) : []);
  const historicalLines = tree === 'WORKTREE' ? null : treeLineCounts(tree);
  const result = { total: paths.length, confirmed: 0, likely: 0, original: 0, unclear: 0, inheritedLOC: 0, originalLOC: 0, unclearLOC: 0 };
  const files = [];
  for (const path of paths) {
    let category;
    if (unclear.test(path)) category = 'unclear';
    else if (replacementOverride && phaseB2Replacements.has(path)) category = 'original';
    else if (!forkBlobs.has(path)) category = 'original';
    else {
      const hash = tree === 'WORKTREE' ? currentHashByPath.get(path) : treeBlobMap.get(path);
      category = hash === forkBlobs.get(path) ? 'confirmed' : 'likely';
    }
    result[category] += 1;
    const lines = !codeExtensions.has(extension(path)) ? 0
      : tree === 'WORKTREE' ? fs.readFileSync(path, 'utf8').replace(/\r/g, '').split('\n').length - 1
      : historicalLines.get(path) || 0;
    if (category === 'confirmed' || category === 'likely') result.inheritedLOC += lines;
    else if (category === 'original') result.originalLOC += lines;
    else result.unclearLOC += lines;
    files.push({ path, category, lines });
  }
  result.inherited = result.confirmed + result.likely;
  return { result, files };
}

const before = audit(phaseB2Base, false);
const after = audit('WORKTREE', true);
const remainingRuntime = after.files.filter(({ path, category }) =>
  (category === 'confirmed' || category === 'likely') &&
  /^(backend\/(cmd|internal)|frontend\/src|desktop\/)/.test(path)
);

process.stdout.write(JSON.stringify({ fork, phaseB2Base, before: before.result, after: after.result, remainingRuntime }, null, 2) + '\n');
