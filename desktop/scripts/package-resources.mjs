import fs from 'node:fs';
import path from 'node:path';

// Preparation recursively replaces this output, so overrides may select only a
// dedicated child of desktop/dist. Source, user data and Git metadata are never outputs.
export function resolvePackageResourcesDir({ desktopRoot, repoRoot, override = process.env.MYSTOCKTRACER_PACKAGE_RESOURCES_DIR || process.env.A_STOCK_PACKAGE_RESOURCES_DIR } = {}) {
  if (!desktopRoot || !repoRoot) throw new Error('desktopRoot and repoRoot are required');
  const repository = path.resolve(repoRoot);
  const desktop = path.resolve(desktopRoot);
  const outputRoot = path.join(desktop, 'dist');
  const candidate = override ? path.resolve(repository, override) : path.join(outputRoot, 'package-resources');
  const below = (parent, child) => {
    const relative = path.relative(parent, child);
    return relative && relative !== '..' && !relative.startsWith('..' + path.sep) && !path.isAbsolute(relative);
  };
  if (!below(repository, desktop) || !below(outputRoot, candidate)) {
    throw new Error('Package resources must be a dedicated generated directory beneath desktop/dist');
  }
  // Check ancestors too: an output parent may be a Windows junction.
  for (let cursor = candidate; ; cursor = path.dirname(cursor)) {
    const stat = fs.lstatSync(cursor, { throwIfNoEntry: false });
    if (stat?.isSymbolicLink()) throw new Error('Package resources cannot traverse a symlink or junction');
    if (path.dirname(cursor) === cursor) break;
  }
  return candidate;
}
