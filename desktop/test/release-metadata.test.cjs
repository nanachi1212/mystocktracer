const test=require('node:test');
const assert=require('node:assert/strict');
const fs=require('node:fs');
const os=require('node:os');
const path=require('node:path');
const {execFileSync}=require('node:child_process');
const crypto=require('node:crypto');
const desktop=path.resolve(__dirname,'..');
const script=(name)=>path.join(desktop,'scripts',name);
function fixture(t) {const dir=fs.mkdtempSync(path.join(os.tmpdir(),'mystocktracer-release-'));t.after(()=>fs.rmSync(dir,{recursive:true,force:true}));return dir;}
test('Windows and macOS config own identity, retain B3 installer GUID and protect userData',async()=>{
  const {desktopBuildConfig,INSTALLER_GUID}=await import('../scripts/build-config.mjs');
  const {UUID}=require('builder-util-runtime');
  assert.equal(INSTALLER_GUID,UUID.v5('com.jundizhou.easystock',UUID.parse('50e065bc-3134-11e6-9bab-38c9862bdaf3')));
  const schema=JSON.parse(fs.readFileSync(require.resolve('app-builder-lib/scheme.json'),'utf8'));
  assert.ok(schema.definitions.NsisOptions.properties.guid);
  for(const platform of ['windows','mac']) {
    const config=desktopBuildConfig({desktopRoot:desktop,resources:path.join(desktop,'dist/test'),version:'0.9.2',electronVersion:'44.2.0',platform,arch:'x64',mode:'release'});
    assert.equal(config.appId,'com.nanachi1212.mystocktracer');assert.equal(config.productName,'mystocktracer');
    assert.equal(config.nsis.guid,INSTALLER_GUID);assert.equal(config.nsis.deleteAppDataOnUninstall,false);
    assert.match(config.nsis.artifactName,/^mystocktracer-/);assert.match(config.mac.icon,/mystocktracer\.icns$/);
    assert.equal(config.mac.identity,'-');assert.equal(config.mac.notarize,false);
    assert.deepEqual(config.publish,[{provider:'github',owner:'nanachi1212',repo:'mystocktracer'}]);
    const {requiredLocalRuntimeModules}=await import('../scripts/verify-release-package.mjs');
    for(const name of requiredLocalRuntimeModules()) assert.ok(config.files.includes(name),name);
    assert.ok(config.extraResources.some((entry)=>entry.to==='state-copy.py'));
  }
});
test('installed B3 updater launches renamed installer with update and force-run flags',()=>{
  const {NsisUpdater}=require('electron-updater/out/NsisUpdater');
  const launched=[];
  const receiver={installerPath:path.join(os.tmpdir(),'mystocktracer-v0.9.3-windows-x64-setup.exe'),spawnLog:(file,args)=>{launched.push({file,args});return Promise.resolve();},dispatchError:()=>assert.fail('unexpected updater error')};
  assert.equal(NsisUpdater.prototype.doInstall.call(receiver,{isSilent:false,isForceRunAfter:true,isAdminRightsRequired:false}),true);
  assert.deepEqual(launched,[{file:receiver.installerPath,args:['--updated','--force-run']}]);
});
test('installed NSIS uses existing registration location and KEEP_APP_DATA during replacement',()=>{
  const builder=path.dirname(require.resolve('app-builder-lib/package.json'));
  const multi=fs.readFileSync(path.join(builder,'templates/nsis/multiUser.nsh'),'utf8');
  const utility=fs.readFileSync(path.join(builder,'templates/nsis/include/installUtil.nsh'),'utf8');
  assert.match(multi,/ReadRegStr \$perUserInstallationFolder HKCU "\$\{INSTALL_REGISTRY_KEY\}" InstallLocation/);
  assert.match(utility,/\/S \/KEEP_APP_DATA/);
  const uninstaller=fs.readFileSync(path.join(builder,'templates/nsis/uninstaller.nsh'),'utf8');
  assert.match(uninstaller,/DELETE_APP_DATA_ON_UNINSTALL/);
});
test('macOS DMG staging retains framework symlinks using ditto and verifies mounted bundle',()=>{
  const source=fs.readFileSync(script('create-dmg.mjs'),'utf8');
  assert.match(source,/run\('ditto',\[bundle,target\]\)/);
  assert.match(source,/Journaled HFS\+/);
  assert.match(source,/verifyReleasePackage\(path.join\(mount,'mystocktracer.app'\),'macos'\)/);
  assert.match(source,/\['detach',mount,'-force'\]/);
  assert.doesNotMatch(source,/-srcfolder/);
});
test('SHA-512 verifier accepts canonical assets and rejects tampering',t=>{
  const root=fixture(t),name='mystocktracer-v0.9.3-windows-x64-setup.exe',bytes=Buffer.from('test installer');
  fs.writeFileSync(path.join(root,name),bytes);fs.writeFileSync(path.join(root,name+'.blockmap'),'block');
  fs.writeFileSync(path.join(root,'latest.yml'),'version: 0.9.3\nfiles:\n  - url: '+name+'\n    sha512: '+crypto.createHash('sha512').update(bytes).digest('base64')+'\n    size: '+bytes.length+'\n');
  execFileSync(process.execPath,[script('verify-updater-artifacts.mjs'),root,'latest.yml']);
  fs.appendFileSync(path.join(root,name),'tampered');
  assert.throws(()=>execFileSync(process.execPath,[script('verify-updater-artifacts.mjs'),root,'latest.yml'],{stdio:'pipe'}));
});
test('merge architecture metadata without npm and stage a complete product release',t=>{
  const root=fixture(t),output=path.join(root,'staged'),version='0.9.3',names=['latest.yml'];
  for(const arch of ['arm64','x64']) {
    const name='mystocktracer-v'+version+'-macos-'+arch+'.zip';
    fs.writeFileSync(path.join(root,'latest-mac-'+arch+'.yml'),'version: '+version+'\nfiles:\n  - url: '+name+'\n    sha512: fixture-hash\n    size: 10\n');
    for(const ext of ['zip','zip.blockmap','dmg']) names.push('mystocktracer-v'+version+'-macos-'+arch+'.'+ext);
  }
  for(const ext of ['exe','exe.blockmap']) names.push('mystocktracer-v'+version+'-windows-x64-setup.'+ext);
  for(const name of names) fs.writeFileSync(path.join(root,name),'fixture');
  execFileSync(process.execPath,[script('merge-mac-updater-metadata.mjs'),root],{cwd:os.tmpdir(),env:{PATH:process.env.PATH}});
  names.push('latest-mac.yml');
  execFileSync(process.execPath,[script('prepare-publish-assets.mjs'),root,output,'v'+version]);
  assert.deepEqual(fs.readdirSync(path.join(output,'github')).sort(),names.sort());
});
test('ASAR verifier rejects absent modules and forbidden local state names',async(t)=>{
  const {verifyAppAsar,forbiddenEntry}=await import('../scripts/verify-release-package.mjs');
  const root=fixture(t);fs.mkdirSync(path.join(root,'resources'));
  const header=Buffer.from(JSON.stringify({files:{'main.cjs':{size:0,offset:'0'}}})),lead=Buffer.alloc(16);
  lead.writeUInt32LE(header.length,12);fs.writeFileSync(path.join(root,'resources/app.asar'),Buffer.concat([lead,header]));
  assert.throws(()=>verifyAppAsar(root,'windows'),/missing required local runtime modules/);
  for(const name of ['easy-stock.exe','easy-stock-backend.exe','easy-stock.ico','.env','secrets.json','test.db','runtime.log','mystocktracer.migration-123']) assert.equal(forbiddenEntry(name),true,name);
});