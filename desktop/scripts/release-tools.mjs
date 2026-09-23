import fs from 'node:fs';
import path from 'node:path';
import { spawnSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';
export const desktop = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
export const version = JSON.parse(fs.readFileSync(path.join(desktop,'package.json'),'utf8')).version;
export const arch = process.env.MYSTOCKTRACER_DESKTOP_ARCH || process.env.A_STOCK_DESKTOP_ARCH || process.arch;
export const release = path.join(desktop,'dist','release');
export function run(command,args,options={}) {
  const result=spawnSync(command,args,{stdio:'inherit',windowsHide:true,...options});
  if(result.error || result.status!==0) throw new Error('Release command failed: '+command);
  return result;
}
export function parseMetadata(text) {
  const result={files:[]}; let entry;
  for(const line of text.split(/\r?\n/)) {
    let match=line.match(/^(\w+):\s*(.*)$/);
    if(match) { if(match[1]!=='files') result[match[1]]=scalar(match[2]); entry=undefined; continue; }
    match=line.match(/^\s+-\s+url:\s*(.+)$/);
    if(match) { entry={url:scalar(match[1])}; result.files.push(entry); continue; }
    match=line.match(/^\s+(sha512|size):\s*(.+)$/);
    if(match&&entry) entry[match[1]]=scalar(match[2]);
  }
  if(!result.version || !result.files.length || result.files.some((item)=>!item.url||!item.sha512)) throw new Error('Incomplete updater metadata');
  return result;
}
function scalar(value) { return value.replace(/^(['"])(.*)\1$/,'$2').trim(); }
export function metadataText(value) {
  return 'version: '+value.version+'\nfiles:\n'+value.files.map((file)=>'  - url: '+file.url+'\n    sha512: '+file.sha512+(file.size?'\n    size: '+file.size:'')).join('\n')+'\npath: '+value.files[0].url+'\nsha512: '+value.files[0].sha512+'\n'+(value.releaseDate?'releaseDate: '+value.releaseDate+'\n':'');
}