const fs = require('node:fs');
const path = require('node:path');
const { spawnSync } = require('node:child_process');

const MARKER = '.mystocktracer-migration.json';
const TRANSIENT = new Set(['Cache', 'Code Cache', 'GPUCache', 'Crashpad', 'DawnCache', 'GrShaderCache', 'ShaderCache', 'easy-stock-updater', 'mystocktracer-updater', 'cashflow-cache', 'SingletonLock', 'SingletonSocket', 'SingletonCookie']);

function plainPath(candidate) {
  const absolute = path.resolve(candidate);
  for (let current = absolute; ; current = path.dirname(current)) {
    try {
      if (fs.lstatSync(current).isSymbolicLink()) throw new Error('資料路徑含有 symlink 或 junction；遷移已中止');
    } catch (error) { if (error.code !== 'ENOENT') throw error; }
    if (path.dirname(current) === current) break;
  }
  return absolute;
}

function entries(root) {
  plainPath(root);
  try { return fs.readdirSync(root, { withFileTypes: true }); }
  catch (error) { if (error.code === 'ENOENT') return []; throw new Error('無法檢查資料目錄；請確認存取權限'); }
}

function hasMeaningfulState(root, top = true) {
  return entries(root).some((entry) => {
    if (top && TRANSIENT.has(entry.name)) return false;
    if (entry.isSymbolicLink()) throw new Error('資料包含 symlink 或 junction；遷移已中止');
    return entry.isFile() || (entry.isDirectory() && hasMeaningfulState(path.join(root, entry.name), false));
  });
}

function verifiedCopy({ source, staging, python, helper }) {
  const result = spawnSync(python, ['-I', helper, source, staging], { windowsHide: true, encoding: 'utf8', timeout: 10 * 60 * 1000, maxBuffer: 1024, env: { ...process.env, PYTHONDONTWRITEBYTECODE: '1' } });
  if (result.error || result.status !== 0 || result.stdout.trim() !== 'verified') throw new Error('資料複製／驗證失敗；來源已保留，請關閉舊程式後重試');
  const inventoryPath = path.join(staging, '.snapshot-inventory.json');
  const files = JSON.parse(fs.readFileSync(inventoryPath, 'utf8'));
  fs.unlinkSync(inventoryPath);
  return files;
}

function selectUserData({ appDataPath, env = process.env, python, helper = path.join(__dirname, 'state-copy.py'), copy = verifiedCopy, beforeActivate = () => {} }) {
  const configured = env.MYSTOCKTRACER_USER_DATA_DIR?.trim() || env.A_STOCK_USER_DATA_DIR?.trim() || '';
  if (configured) {
    const target = plainPath(configured);
    fs.mkdirSync(target, { recursive: true, mode: 0o700 });
    return { path: target, mode: 'explicit' };
  }
  const root = plainPath(appDataPath);
  const target = plainPath(path.join(root, 'mystocktracer'));
  if (hasMeaningfulState(target)) return { path: target, mode: 'existing' };
  const source = ['easy-stock', 'desktop'].map((name) => path.join(root, name)).find((candidate) => hasMeaningfulState(candidate));
  if (!source) {
    fs.mkdirSync(target, { recursive: true, mode: 0o700 });
    return { path: target, mode: 'fresh' };
  }
  if (entries(target).length) throw new Error('新資料目錄非空；為避免覆寫，遷移已中止');
  fs.mkdirSync(root, { recursive: true, mode: 0o700 });
  const staging = fs.mkdtempSync(path.join(root, 'mystocktracer.migration-'));
  fs.chmodSync(staging, 0o700);
  const files = copy({ source, staging, python, helper });
  if (!Array.isArray(files) || !files.some((file) => file.type === 'file')) throw new Error('來源驗證未完成；遷移已中止');
  const manifest = { schema: 1, sourceIdentity: path.basename(source), createdAt: new Date().toISOString(), files };
  fs.writeFileSync(path.join(staging, MARKER), JSON.stringify(manifest, null, 2) + '\n', { flag: 'wx', mode: 0o600 });
  beforeActivate();
  plainPath(target);
  // rmdir is deliberately non-recursive: a concurrent destination write fails closed.
  if (fs.existsSync(target)) fs.rmdirSync(target);
  fs.renameSync(staging, target);
  return { path: target, mode: 'migrated', sourceIdentity: path.basename(source) };
}

module.exports = { MARKER, TRANSIENT, hasMeaningfulState, plainPath, selectUserData, verifiedCopy };
