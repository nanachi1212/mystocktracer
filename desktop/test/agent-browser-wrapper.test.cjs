const test = require('node:test');
const assert = require('node:assert/strict');
const { spawnSync } = require('node:child_process');
const path = require('node:path');
const wrapper=path.resolve(__dirname,'../scripts/browser-bin/agent-browser');
test('actual Python wrapper forwards canonical binary arguments without importing login state',()=>{
  const result=spawnSync('python',['-I',wrapper,'-e','process.stdout.write(JSON.stringify(process.argv.slice(1)))','--','--session','fixture','open','https://example.invalid/'],{
    encoding:'utf8',env:{...process.env,MYSTOCKTRACER_AGENT_BROWSER_REAL:process.execPath,A_STOCK_AGENT_BROWSER_REAL:'invalid',AGENT_BROWSER_STATE:'never-import-this-state'}});
  assert.equal(result.status,0,result.stderr);
  assert.deepEqual(JSON.parse(result.stdout),['--session','fixture','open','https://example.invalid/']);
});
test('missing product browser binary fails without execution',()=>{
  const result=spawnSync('python',['-I',wrapper],{encoding:'utf8',env:{...process.env,MYSTOCKTRACER_AGENT_BROWSER_REAL:'invalid',A_STOCK_AGENT_BROWSER_REAL:''}});
  assert.equal(result.status,127);assert.match(result.stderr,/mystocktracer/);
});