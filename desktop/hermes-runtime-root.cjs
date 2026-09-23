const fs = require('node:fs');
const path = require('node:path');
const { execFileSync } = require('node:child_process');
function runtimePython(root, platform = process.platform) {
  return path.join(root, ...(platform === 'win32' ? ['python','python.exe'] : ['venv','bin','python']));
}
function validRuntimeRoot(root, platform) {
  if (!root) return false;
  try { return fs.statSync(runtimePython(root,platform)).isFile(); }
  catch (error) { if (['ENOENT','ENOTDIR'].includes(error.code)) return false; throw error; }
}
function commonGitDir(root) {
  // git resolves worktree indirection rather than duplicating its file parser.
  try { return execFileSync('git',['-C',root,'rev-parse','--path-format=absolute','--git-common-dir'],{encoding:'utf8',stdio:['ignore','pipe','ignore'],windowsHide:true}).trim(); }
  catch { // Tests and incomplete worktrees can have the documented gitdir/commondir layout.
    const dot=path.join(root,'.git');
    if(!fs.existsSync(dot)) return '';
    if(fs.statSync(dot).isDirectory()) return dot;
    const match=fs.readFileSync(dot,'utf8').match(/^gitdir:\s*(.+)/);
    if(!match) return '';
    const gitdir=path.resolve(root,match[1].trim()), common=path.join(gitdir,'commondir');
    return fs.existsSync(common)?path.resolve(gitdir,fs.readFileSync(common,'utf8').trim()):gitdir;
  }
}
function resolveHermesRuntimeRoot({configuredRoot='',bundledRoot='',projectRoot='',platform=process.platform}={}) {
  const roots=[];
  if(configuredRoot) roots.push(path.resolve(configuredRoot));
  if(bundledRoot) roots.push(path.join(bundledRoot,'hermes-runtime'));
  if(projectRoot) {
    roots.push(path.join(projectRoot,'desktop','resources','hermes-runtime'));
    const git=commonGitDir(projectRoot);
    if(git) roots.push(path.join(path.dirname(git),'desktop','resources','hermes-runtime'));
  }
  return roots.find((root)=>validRuntimeRoot(root,platform)) || '';
}
module.exports={runtimePython,validRuntimeRoot,commonGitDir,resolveHermesRuntimeRoot};