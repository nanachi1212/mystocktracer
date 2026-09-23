import fs from 'node:fs';
import path from 'node:path';
import { createHash } from 'node:crypto';
import { parseMetadata } from './release-tools.mjs';
if(process.argv.length<4) throw new Error('Supply a release directory and metadata names');
const directory=path.resolve(process.argv[2]);
for(const name of process.argv.slice(3)) {
  if(path.basename(name)!==name || !/^latest(?:-mac)?\.yml$/.test(name)) throw new Error('Invalid metadata filename');
  const metadata=parseMetadata(fs.readFileSync(path.join(directory,name),'utf8'));
  for(const file of metadata.files) {
    if(path.basename(file.url)!==file.url || !file.url.startsWith('mystocktracer-v'+metadata.version+'-') || /[\\/:]/.test(file.url)) throw new Error('Invalid release asset identity');
    const content=fs.readFileSync(path.join(directory,file.url));
    if(createHash('sha512').update(content).digest('base64')!==file.sha512) throw new Error('SHA-512 mismatch: '+file.url);
    if(file.size && Number(file.size)!==content.length) throw new Error('Asset size mismatch');
    if(file.url.endsWith('.exe')&&!fs.existsSync(path.join(directory,file.url+'.blockmap'))) throw new Error('Missing Windows blockmap');
  }
  console.log('Verified '+name+' and '+metadata.files.length+' SHA-512 assets');
}