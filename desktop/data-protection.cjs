const fs = require('node:fs');
const path = require('node:path');
const { TRANSIENT, plainPath, verifiedCopy } = require('./user-data-migration.cjs');
const CACHE_DIRECTORY_NAMES = TRANSIENT;
const shouldExclude = (relative) => TRANSIENT.has(relative.split(/[\\/]/)[0]);
function resolveBackupRoot(userDataPath, configuredPath = process.env.MYSTOCKTRACER_UPDATE_BACKUP_DIR || process.env.A_STOCK_UPDATE_BACKUP_DIR) {
  const source = plainPath(userDataPath);
  const target = plainPath(configuredPath || path.join(path.dirname(source), 'mystocktracer-update-backups'));
  const relative = path.relative(source, target);
  if (!relative || (!relative.startsWith('..' + path.sep) && relative !== '..' && !path.isAbsolute(relative))) throw new Error('更新備份目錄不能位於應用資料目錄內');
  return target;
}
function createUpdateBackup({ userDataPath, backupRoot, fromVersion, toVersion, now = new Date(), python = 'python', helper }) {
  const source = plainPath(userDataPath);
  const root = resolveBackupRoot(source, backupRoot);
  fs.mkdirSync(root, { recursive: true, mode: 0o700 });
  const stage = fs.mkdtempSync(path.join(root, '.partial-'));
  try {
  const data = path.join(stage, 'data');
  fs.mkdirSync(data, { mode: 0o700 });
  const files = verifiedCopy({ source, staging: data, python, helper: helper || path.join(__dirname, 'state-copy.py') });
  const manifest = { schemaVersion: 2, createdAt: now.toISOString(), fromVersion: String(fromVersion || ''), toVersion: String(toVersion || ''), userDataDirectoryName: path.basename(source), files };
  fs.writeFileSync(path.join(stage, 'manifest.json'), JSON.stringify(manifest, null, 2) + '\n', { flag: 'wx', mode: 0o600 });
  const name = now.toISOString().replace(/[:.]/g, '-') + '-' + path.basename(stage).slice(9);
  const destination = path.join(root, name);
  fs.renameSync(stage, destination);
  return { path: destination, backupRoot: root, manifest };
  } catch (error) {
    // Only discard the unique staging directory owned by this attempt.
    fs.rmSync(stage, { recursive: true, force: true });
    throw error;
  }
}
function listUpdateBackups(root) {
  if (!fs.existsSync(root)) return [];
  const results = [];
  for (const entry of fs.readdirSync(root, { withFileTypes: true })) {
    if (!entry.isDirectory() || entry.name.startsWith('.partial-')) continue;
    const directory = path.join(root, entry.name);
    const manifest = path.join(directory, 'manifest.json');
    if (!fs.existsSync(manifest)) continue;
    try { results.push({ path: directory, manifest: JSON.parse(fs.readFileSync(manifest, 'utf8')) }); }
    catch (error) { if (!(error instanceof SyntaxError)) throw error; }
  }
  return results.sort((a, b) => String(b.manifest.createdAt).localeCompare(String(a.manifest.createdAt)));
}
module.exports = { CACHE_DIRECTORY_NAMES, shouldExclude, resolveBackupRoot, createUpdateBackup, listUpdateBackups };
