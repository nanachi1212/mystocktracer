import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { desktop,arch,version,release,run } from './release-tools.mjs';
import { verifyReleasePackage } from './verify-release-package.mjs';
if(process.platform!=='darwin') throw new Error('DMG creation requires macOS');
const bundle=path.join(desktop,'dist','builder-release',arch==='x64'?'mac':'mac-'+arch,'mystocktracer.app');
verifyReleasePackage(bundle,'macos');
fs.mkdirSync(release,{recursive:true});
const temporary=fs.mkdtempSync(path.join(os.tmpdir(),'mystocktracer-dmg-'));
const mount=path.join(temporary,'volume'), image=path.join(temporary,'writable.dmg');
let attached=false;
function detach() {
  try { run('hdiutil',['detach',mount]); } catch { run('hdiutil',['detach',mount,'-force']); }
  attached=false;
}
try {
  fs.mkdirSync(mount);
  run('hdiutil',['create','-size','2400m','-fs','Journaled HFS+','-volname','mystocktracer','-type','UDIF',image]);
  run('hdiutil',['attach',image,'-nobrowse','-mountpoint',mount]); attached=true;
  const target=path.join(mount,'mystocktracer.app');
  run('ditto',[bundle,target]);
  verifyReleasePackage(target,'macos');
  fs.symlinkSync('/Applications',path.join(mount,'Applications'));
  run('sync',[]);detach();
  const result=path.join(release,`mystocktracer-v${version}-macos-${arch}.dmg`);
  run('hdiutil',['convert',image,'-format','UDZO','-o',result,'-ov']);
  run('hdiutil',['verify',result]);
  run('hdiutil',['attach',result,'-readonly','-nobrowse','-mountpoint',mount]);attached=true;
  verifyReleasePackage(path.join(mount,'mystocktracer.app'),'macos');detach();
} finally {
  if(attached) detach();
  fs.rmSync(temporary,{recursive:true,force:true});
}