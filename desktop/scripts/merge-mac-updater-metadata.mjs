import fs from 'node:fs';
import path from 'node:path';
import { parseMetadata,metadataText } from './release-tools.mjs';
if(!process.argv[2]) throw new Error('Supply a release directory');
const directory=path.resolve(process.argv[2]);
const variants=['arm64','x64'].map((arch)=>parseMetadata(fs.readFileSync(path.join(directory,'latest-mac-'+arch+'.yml'),'utf8')));
if(variants[0].version!==variants[1].version || variants.some((item)=>item.files.length!==1)) throw new Error('Incompatible macOS metadata');
fs.writeFileSync(path.join(directory,'latest-mac.yml'),metadataText({...variants[0],files:variants.flatMap((item)=>item.files)}));
console.log('Merged both macOS architectures');