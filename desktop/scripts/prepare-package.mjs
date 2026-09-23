import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { spawnSync } from 'node:child_process';
import { prepareHermesRuntime } from './hermes-runtime.mjs';
import { resolvePackageResourcesDir } from './package-resources.mjs';

const desktop = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const root = path.dirname(desktop);
const resources = resolvePackageResourcesDir({ desktopRoot:desktop, repoRoot:root });
const architecture = process.env.MYSTOCKTRACER_DESKTOP_ARCH || process.env.A_STOCK_DESKTOP_ARCH || process.arch;
if (architecture !== process.arch) throw new Error('Build the native runtime on the target architecture');
function run(command,args,cwd,env={}) {
  const result = spawnSync(command,args,{ cwd,env:{...process.env,...env},stdio:'inherit',windowsHide:true });
  if(result.error || result.status !== 0) throw new Error('Package preparation command failed');
}
if (!process.env.npm_execpath) throw new Error('Use npm run prepare:desktop-package');
run(process.execPath,[process.env.npm_execpath,'--workspace','frontend','run','build'],root);
// Resolve and validate the generated-only path before recursive replacement.
const relative = path.relative(root,resources);
if (!relative || relative.startsWith('..') || path.isAbsolute(relative)) throw new Error('Package resources must remain in the repository');
fs.rmSync(resources,{recursive:true,force:true});
fs.mkdirSync(path.join(resources,'backend'),{recursive:true});
run('go',['build','-trimpath','-o',path.join(resources,'backend',process.platform==='win32'?'mystocktracer-backend.exe':'mystocktracer-backend'),'./cmd/server'],path.join(root,'backend'),{CGO_ENABLED:'0'});
fs.cpSync(path.join(root,'frontend','dist'),path.join(resources,'frontend','dist'),{recursive:true});
const browserRoot=path.join(resources,'agent-browser');
fs.mkdirSync(browserRoot);
const browserName=process.platform==='win32'?'agent-browser-win32-x64.exe':`agent-browser-${process.platform}-${process.arch}`;
fs.copyFileSync(path.join(root,'node_modules','agent-browser','bin',browserName),path.join(browserRoot,process.platform==='win32'?'agent-browser-real.exe':'agent-browser-real'));
fs.copyFileSync(path.join(desktop,'scripts/browser-bin/agent-browser'),path.join(browserRoot,process.platform==='win32'?'agent-browser.py':'agent-browser'));
if(process.platform!=='win32') for(const name of ['agent-browser','agent-browser-real']) fs.chmodSync(path.join(browserRoot,name),0o755);
const runtime=prepareHermesRuntime({runtimeRoot:path.join(resources,'hermes-runtime')});
fs.copyFileSync(path.join(root,'THIRD_PARTY_NOTICES.md'),path.join(resources,'THIRD_PARTY_NOTICES.md'));
console.log(`Prepared mystocktracer resources with Hermes ${runtime.version}`);