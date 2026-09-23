import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { deflateSync } from 'node:zlib';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../..');
const assets = path.join(root, 'desktop/assets');
const points = [[30, 66], [45, 49], [59, 57], [74, 34]];
const svg = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100"><rect width="100" height="100" rx="22" fill="#102b40"/><circle cx="50" cy="50" r="33" fill="none" stroke="#357087" stroke-width="3"/><polyline points="${points.map((p) => p.join(',')).join(' ')}" fill="none" stroke="#76edc4" stroke-width="6" stroke-linecap="round" stroke-linejoin="round"/>${points.map(([x, y]) => `<circle cx="${x}" cy="${y}" r="4" fill="#f5dc8f"/>`).join('')}</svg>\n`;
fs.writeFileSync(path.join(assets, 'mystocktracer-mark.svg'), svg);
fs.writeFileSync(path.join(root, 'frontend/public/mystocktracer-mark.svg'), svg);

const palette = [[16,43,64,255],[53,112,135,255],[118,237,196,255],[245,220,143,255]];
function color(x, y) {
  const dx = Math.max(22 - x, x - 78, 0), dy = Math.max(22 - y, y - 78, 0);
  if (dx * dx + dy * dy > 22 ** 2) return [0,0,0,0];
  let index = Math.abs(Math.hypot(x - 50, y - 50) - 33) <= 1.5 ? 1 : 0;
  for (let n = 1; n < points.length; n++) {
    const [a,b] = points[n-1], [c,d] = points[n];
    const t = Math.max(0,Math.min(1, ((x-a)*(c-a)+(y-b)*(d-b))/((c-a)**2+(d-b)**2)));
    if (Math.hypot(x-a-t*(c-a),y-b-t*(d-b)) <= 3) index = 2;
  }
  if (points.some(([a,b]) => Math.hypot(x-a,y-b) <= 4)) index = 3;
  return palette[index];
}
function crc(buffer) {
  let value = 0xffffffff;
  for (const byte of buffer) { value ^= byte; for (let n=0;n<8;n++) value=(value>>>1)^((value&1)?0xedb88320:0); }
  return (value ^ 0xffffffff) >>> 0;
}
function chunk(name, data) {
  const type = Buffer.from(name), size = Buffer.alloc(4), sum = Buffer.alloc(4);
  size.writeUInt32BE(data.length); sum.writeUInt32BE(crc(Buffer.concat([type,data])));
  return Buffer.concat([size,type,data,sum]);
}
function png(size) {
  const pixels = Buffer.alloc(size*(size*4+1));
  for(let y=0;y<size;y++) for(let x=0;x<size;x++) {
    const values = [0,0,0,0];
    for(const ox of [0.25,0.75]) for(const oy of [0.25,0.75]) color((x+ox)*100/size,(y+oy)*100/size).forEach((v,i)=>values[i]+=v/4);
    values.forEach((v,i)=>pixels[y*(size*4+1)+1+x*4+i]=Math.round(v));
  }
  const header=Buffer.alloc(13); header.writeUInt32BE(size);header.writeUInt32BE(size,4);header[8]=8;header[9]=6;
  return Buffer.concat([Buffer.from([137,80,78,71,13,10,26,10]),chunk('IHDR',header),chunk('IDAT',deflateSync(pixels)),chunk('IEND',Buffer.alloc(0))]);
}
const image = png(512), small=png(256);
fs.writeFileSync(path.join(assets,'mystocktracer.png'),image);
fs.writeFileSync(path.join(root,'frontend/public/mystocktracer-mark.png'),small);
const ico=Buffer.alloc(22);ico.writeUInt16LE(1,2);ico.writeUInt16LE(1,4);ico.writeUInt16LE(1,10);ico.writeUInt16LE(32,12);ico.writeUInt32LE(small.length,14);ico.writeUInt32LE(22,18);
fs.writeFileSync(path.join(assets,'mystocktracer.ico'),Buffer.concat([ico,small]));
const icns=Buffer.alloc(16);icns.write('icns');icns.writeUInt32BE(image.length+16,4);icns.write('ic09',8);icns.writeUInt32BE(image.length+8,12);
fs.writeFileSync(path.join(assets,'mystocktracer.icns'),Buffer.concat([icns,image]));
console.log('Generated original mystocktracer SVG / PNG / ICO / ICNS');
