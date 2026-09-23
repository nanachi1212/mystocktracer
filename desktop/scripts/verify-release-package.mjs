import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { spawnSync } from 'node:child_process';
import { runtimeModules } from './build-config.mjs';

export function requiredLocalRuntimeModules() { return [...runtimeModules]; }
export function listAsarFiles(archive) {
  const handle=fs.openSync(archive,'r');
  try {
    const lead=Buffer.alloc(16);
    if(fs.readSync(handle,lead,0,16,0)!==16) throw new Error('Invalid ASAR header');
    const bytes=lead.readUInt32LE(12);
    if(bytes<2||bytes>16*1024*1024) throw new Error('Invalid ASAR header size');
    const buffer=Buffer.alloc(bytes);
    if(fs.readSync(handle,buffer,0,bytes,16)!==bytes) throw new Error('Truncated ASAR');
    const result=new Set(), queue=[['',JSON.parse(buffer.toString('utf8'))]];
    while(queue.length) {
      const [prefix,node]=queue.shift();
      for(const [name,item] of Object.entries(node.files||{})) {
        const relative=prefix+name;result.add(relative);
        if(item.files) queue.push([relative+'/',item]);
      }
    }
    return result;
  } finally {fs.closeSync(handle);}
}
function resourceDirectory(root,platform) {return platform==='macos'?path.join(root,'Contents/Resources'):path.join(root,'resources');}
export function forbiddenEntry(name) {
  return ['.runtime','hermes-home','userData','browser-auth','Local Storage','Session Storage','Partitions','Cookies','Cookies-journal','credentials.json','secrets.json','.credentials.json','id_rsa','id_ed25519'].includes(name)
    || name==='.env' || (name.startsWith('.env.')&&name!=='.env.example')
    || /\.(?:db(?:-wal|-shm|-journal)?|sqlite3?|log|pfx|p12|key)$/i.test(name)
    || /^(?:easy-stock(?:-backend)?(?:\.exe|\.app|\.ico|\.icns|\.png|\.svg)?|mystocktracer\.migration-.+|\.partial-.+)$/.test(name);
}
export function verifyAppAsar(root,platform) {
  const archive=path.join(resourceDirectory(root,platform),'app.asar');
  const files=listAsarFiles(archive);
  const absent=runtimeModules.filter((name)=>!files.has(name));
  if(absent.length) throw new Error('Packaged app.asar is missing required local runtime modules:\n'+absent.join('\n'));
  for(const file of files) if(file.split('/').some(forbiddenEntry)||/^(?:test|scripts)\//.test(file)) throw new Error('Forbidden packaged application entry: '+file);
}
export function verifyReleasePackage(root,platform) {
  if(!['windows','macos'].includes(platform)) throw new Error('Specify windows or macos');
  const resources=resourceDirectory(root,platform), content=path.join(resources,'resources');
  const windows=platform==='windows';
  const python=path.join(content,'hermes-runtime',...(windows?['python','python.exe']:['venv','bin','python']));
  const required=[windows?'mystocktracer.exe':'Contents/MacOS/mystocktracer'].map((file)=>path.join(root,file)).concat([
    path.join(content,'backend',windows?'mystocktracer-backend.exe':'mystocktracer-backend'),
    path.join(content,'frontend/dist/index.html'),path.join(content,'hermes-runtime/runtime-manifest.json'),
    path.join(content,'hermes-runtime/LICENSE'),path.join(content,'THIRD_PARTY_NOTICES.md'),path.join(resources,'state-copy.py'),python,
  ]);
  for(const file of required) if(!fs.statSync(file,{throwIfNoEntry:false})?.isFile()) throw new Error('Missing package member: '+path.relative(root,file));
  const queue=[root];
  while(queue.length) for(const entry of fs.readdirSync(queue.shift(),{withFileTypes:true})) {
    const file=path.join(entry.parentPath,entry.name);
    if(forbiddenEntry(entry.name)) throw new Error('Forbidden package entry: '+path.relative(root,file));
    if(entry.isSymbolicLink()) {
      const relative=path.relative(root,fs.realpathSync(file));
      if(path.isAbsolute(fs.readlinkSync(file))||relative.startsWith('..')) throw new Error('Unsafe package symlink');
    } else if(entry.isDirectory()) queue.push(file);
  }
  verifyAppAsar(root,platform);
  if(windows&&fs.existsSync(path.join(content,'hermes-runtime/venv'))) throw new Error('Nonportable Windows venv in release');
  const manifest=JSON.parse(fs.readFileSync(path.join(content,'hermes-runtime/runtime-manifest.json'),'utf8'));
  if(manifest.version!=='0.18.2'||manifest.package!=='hermes-agent') throw new Error('Unreviewed Hermes runtime');
  const result=spawnSync(python,[...(windows?['-I']:[]),'-c','import hermes_cli,tui_gateway,sqlite3'],{cwd:root,encoding:'utf8',windowsHide:true,env:{...process.env,PYTHONNOUSERSITE:'1',PYTHONDONTWRITEBYTECODE:'1',ENABLE_MCP:'0',SKIP_BACKGROUND_TASKS:'1'}});
  if(result.error||result.status!==0) throw new Error('Bundled Python import check failed');
  if(!windows) {
    const signature=spawnSync('codesign',['--verify','--deep','--strict',root],{stdio:'inherit'});
    if(signature.error||signature.status!==0) throw new Error('macOS signature verification failed');
  }
  console.log('Verified mystocktracer '+platform+' release package');
}
if(process.argv[1]&&path.resolve(process.argv[1])===fileURLToPath(import.meta.url)) verifyReleasePackage(path.resolve(process.argv[2]||''),process.argv[3]);