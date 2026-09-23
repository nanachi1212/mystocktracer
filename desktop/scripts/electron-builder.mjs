import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { build, Platform, Arch } from 'electron-builder';
import { desktopBuildConfig } from './build-config.mjs';
import { resolvePackageResourcesDir } from './package-resources.mjs';

const desktopRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const manifest = JSON.parse(fs.readFileSync(path.join(desktopRoot,'package.json'),'utf8'));
const [platform, mode = 'release'] = process.argv.slice(2);
const arch = process.env.MYSTOCKTRACER_DESKTOP_ARCH || process.env.A_STOCK_DESKTOP_ARCH || process.arch;
const certificate = process.env.CSC_LINK?.trim();
const certificatePaths = certificate ? (certificate.startsWith('file://') ? [fileURLToPath(certificate)] : [path.resolve(certificate), path.resolve(desktopRoot,certificate), path.resolve(desktopRoot,'..',certificate)]) : [];
const signed = Boolean(certificate && !certificatePaths.some((file) => fs.existsSync(file) && fs.statSync(file).isDirectory()));
if (process.env.CSC_LINK !== undefined && !signed) { delete process.env.CSC_LINK; delete process.env.CSC_KEY_PASSWORD; }
const config = desktopBuildConfig({
  desktopRoot, resources: resolvePackageResourcesDir({ desktopRoot, repoRoot:path.dirname(desktopRoot) }),
  version:manifest.version, electronVersion:process.env.MYSTOCKTRACER_ELECTRON_VERSION || process.env.A_STOCK_ELECTRON_VERSION || manifest.devDependencies.electron,
  platform, arch, mode, signed,
  notarize:Boolean(signed && ((process.env.APPLE_ID && process.env.APPLE_APP_SPECIFIC_PASSWORD && process.env.APPLE_TEAM_ID) || process.env.APPLE_API_KEY || process.env.APPLE_KEYCHAIN)),
});
const electronDist = process.env.MYSTOCKTRACER_ELECTRON_DIST || process.env.A_STOCK_ELECTRON_DIST;
if (electronDist) config.electronDist = electronDist;
const target = platform === 'mac' ? Platform.MAC : Platform.WINDOWS;
await build({ projectDir:desktopRoot, config, targets:target.createTarget(mode === 'dir' ? ['dir'] : platform === 'mac' ? ['zip'] : ['nsis'], Arch[arch]), publish:'never' });