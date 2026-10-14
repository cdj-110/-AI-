const fs = require('node:fs');
const path = require('node:path');
const cp = require('node:child_process');
const assert = require('node:assert/strict');
const root = path.resolve(__dirname,'..');
const stamp = new Date().toISOString().replace(/[-:.TZ]/g,'').slice(0,14);
const out = path.join(root,'artifacts',`admin-table-scroll-deploy-${stamp}`);
const docker = args => cp.execFileSync('docker',args,{cwd:root,maxBuffer:64*1024*1024});
fs.mkdirSync(out,{recursive:true});
const previous = JSON.parse(docker(['inspect','weikong-iot-platform-admin-web-1']))[0];
const archive = docker(['exec',previous.Id,'tar','-czf','-','/usr/share/nginx/html','/etc/nginx/conf.d/default.conf']);
assert.equal(archive[0],0x1f);assert.equal(archive[1],0x8b);
fs.writeFileSync(path.join(out,'admin-web.original.tar.gz'),archive,{flag:'wx'});
const rollbackTag = `wk-admin-table-rollback:${stamp}`;
const tagged = cp.spawnSync('docker',['tag',previous.Image,rollbackTag],{cwd:root,encoding:'utf8'}).status === 0;
console.log('Management frontend backup saved. Building frontend only; API, databases and tenant portal remain unchanged.');
for (const args of [['compose','build','admin-web'],['compose','up','-d','--no-deps','admin-web']]) {
  const result = cp.spawnSync('docker',args,{cwd:root,stdio:'inherit'});
  if(result.status!==0)throw new Error(`Frontend deployment step failed: ${args.join(' ')}`);
}
const current = JSON.parse(docker(['inspect','weikong-iot-platform-admin-web-1']))[0];
assert.equal(current.State.Running,true);
docker(['exec',current.Id,'nginx','-t']);
const result = {deployedAt:new Date().toISOString(),service:'admin-web',oldImage:previous.Image,newImage:current.Image,backup:out,rollbackTag:tagged?rollbackTag:null,apiAndDatabaseUnchanged:true};
fs.writeFileSync(path.join(out,'deployment.json'),JSON.stringify(result,null,2));
console.log(JSON.stringify(result));
