import fs from 'node:fs';
import path from 'node:path';
import { spawnSync } from 'node:child_process';

export const HERMES_AGENT_VERSION = '0.18.2';
export const hermesRuntimePython = (root, platform=process.platform) => path.join(root,...(platform==='win32'?['python','python.exe']:['venv','bin','python']));
export const hermesVenvPython = (root, platform=process.platform) => path.join(root,'venv',...(platform==='win32'?['Scripts','python.exe']:['bin','python']));

function execute(program,args,cwd,extra={}) {
  const result=spawnSync(program,args,{cwd,encoding:'utf8',windowsHide:true,env:{...process.env,PYTHONNOUSERSITE:'1',PYTHONDONTWRITEBYTECODE:'1',...extra},maxBuffer:4*1024*1024});
  if(result.error || result.status!==0) throw new Error('Hermes package preparation failed ('+path.basename(program)+')');
  return result.stdout.trim();
}
function inside(root, candidate) {
  const relative=path.relative(path.resolve(root),path.resolve(candidate));
  return !relative || (!relative.startsWith('..'+path.sep)&&relative!=='..'&&!path.isAbsolute(relative));
}
function walk(root,visit) {
  for(const entry of fs.readdirSync(root,{withFileTypes:true})) {
    const file=path.join(root,entry.name);
    visit(file,entry);
    if(entry.isDirectory() && fs.existsSync(file)) walk(file,visit);
  }
}
function localCopy(source,target) {
  if(inside(source,target)||inside(target,source)) throw new Error('Runtime source and destination overlap');
  fs.cpSync(source,target,{recursive:true,verbatimSymlinks:true});
  walk(target,(file,entry)=>{
    if(!entry.isSymbolicLink()) return;
    const raw=fs.readlinkSync(file);
    const old=path.resolve(path.dirname(path.join(source,path.relative(target,file))),raw);
    if(!inside(source,old)) throw new Error('Runtime contains an external link');
    const relocated=path.join(target,path.relative(source,old));
    fs.unlinkSync(file);
    fs.symlinkSync(path.relative(path.dirname(file),relocated),file);
  });
}

export function bundleWindowsRuntime(runtimeRoot,sourceRoot) {
  const root=path.resolve(runtimeRoot), source=fs.realpathSync(sourceRoot);
  if(inside(root,source)||inside(source,root)) throw new Error('Python build roots overlap');
  const site=path.join(root,'venv/Lib/site-packages');
  if(!fs.existsSync(path.join(source,'python.exe')) || !fs.existsSync(site)) throw new Error('Incomplete managed Python environment');
  const target=path.join(root,'python');
  fs.cpSync(source,target,{recursive:true,dereference:true});
  fs.cpSync(site,path.join(target,'Lib/site-packages'),{recursive:true});
  fs.rmSync(path.join(root,'venv'),{recursive:true});
}
function bundlePosixRuntime(root,venvPython) {
  const source=execute(venvPython,['-I','-c','import sys;print(sys.base_prefix)'],root);
  const target=path.join(root,'python');
  localCopy(source,target);
  const libraries=fs.readdirSync(path.join(root,'venv/lib')).filter((name)=>name.startsWith('python'));
  const library=libraries.find((name)=>fs.existsSync(path.join(root,'venv/lib',name,'site-packages')));
  if(!library) throw new Error('Missing venv packages');
  const launcher=['#!/bin/sh','base=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)',
    'export PYTHONPATH="$base/venv/lib/'+library+'/site-packages"',
    'exec "$base/python/bin/python3" "$@"',''].join('\n');
  for(const entry of fs.readdirSync(path.join(root,'venv/bin'))) {
    if(!/^python(?:\d+(?:\.\d+)?)?$/.test(entry)) continue;
    const file=path.join(root,'venv/bin',entry);
    fs.unlinkSync(file);fs.writeFileSync(file,launcher,{mode:0o755});
  }
}

export function prepareHermesRuntime({runtimeRoot,sourcePath=process.env.MYSTOCKTRACER_HERMES_RUNTIME_SOURCE||process.env.HERMES_RUNTIME_SOURCE||process.env.A_STOCK_HERMES_RUNTIME_SOURCE||'',platform=process.platform,arch=process.arch,uv=process.env.UV||'uv'}={}) {
  if(!runtimeRoot || platform!==process.platform || arch!==process.arch) throw new Error('Prepare Hermes on its target platform');
  const root=path.resolve(runtimeRoot);
  if(fs.existsSync(root)) throw new Error('Hermes destination must be a new generated directory');
  fs.mkdirSync(path.dirname(root),{recursive:true});
  if(sourcePath) localCopy(fs.realpathSync(sourcePath),root);
  else {
    fs.mkdirSync(root);
    execute(uv,['venv',path.join(root,'venv'),'--python','3.11','--managed-python','--relocatable','--link-mode','copy'],root);
    const python=hermesVenvPython(root,platform);
    execute(uv,['pip','install','--python',python,'--link-mode','copy','hermes-agent[all]=='+HERMES_AGENT_VERSION],root);
    if(platform==='win32') bundleWindowsRuntime(root,execute(python,['-I','-c','import sys;print(sys.base_prefix)'],root));
    else bundlePosixRuntime(root,python);
  }
  const licenses=[];
  walk(root,(file,entry)=>{
    if(entry.isSymbolicLink()) {
      const resolved=fs.realpathSync(file);
      if(!inside(root,resolved)) throw new Error('External packaged runtime link');
    }
    if(entry.isDirectory()&&entry.name==='__pycache__') fs.rmSync(file,{recursive:true});
    if(entry.isFile()&&entry.name.endsWith('.pyc')) fs.unlinkSync(file);
    if(entry.isFile()&&/^LICENSE(?:\.txt)?$/i.test(entry.name)&&/hermes[_-]agent.*\.dist-info/.test(file)) licenses.push(file);
  });
  if(!licenses.length) throw new Error('Hermes distribution LICENSE missing');
  fs.copyFileSync(licenses.sort()[0],path.join(root,'LICENSE'));
  const python=hermesRuntimePython(root,platform);
  const actual=execute(python,[...(platform==='win32'?['-I']:[]),'-c','import hermes_cli,tui_gateway,importlib.metadata;print(importlib.metadata.version("hermes-agent"))'],root);
  if(actual!==HERMES_AGENT_VERSION) throw new Error('Hermes version differs from reviewed 0.18.2');
  const manifest={schema_version:1,package:'hermes-agent',version:actual,mode:sourcePath?'copied':'installed',target_platform:platform,target_arch:arch,created_at:new Date().toISOString()};
  fs.writeFileSync(path.join(root,'runtime-manifest.json'),JSON.stringify(manifest,null,2)+'\n');
  return manifest;
}