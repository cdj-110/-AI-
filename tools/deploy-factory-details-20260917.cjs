const fs=require('node:fs'),path=require('node:path'),cp=require('node:child_process'),assert=require('node:assert/strict');
const root=path.resolve(__dirname,'..');
const stamp=new Date().toISOString().replace(/[-:.TZ]/g,'').slice(0,14);
const out=path.join(root,'artifacts',`factory-details-deploy-${stamp}`);
const docker=args=>cp.execFileSync('docker',args,{cwd:root,maxBuffer:192*1024*1024});
const inspect=service=>{
  const ids=docker(['compose','ps','-q',service]).toString().trim().split(/\s+/).filter(Boolean);
  assert.equal(ids.length,1,`Expected one running container: ${service}`);
  return JSON.parse(docker(['inspect',ids[0]]))[0];
};
const config=JSON.parse(docker(['compose','config','--format','json']));
const live=Object.fromEntries(inspect('api').Config.Env.map(v=>{const p=v.indexOf('=');return[v.slice(0,p),v.slice(p+1)];}));
for(const [key,value] of Object.entries(config.services.api.environment)) assert.equal(String(value??''),live[key]??'',`API environment would change: ${key}`);
fs.mkdirSync(out,{recursive:true});
const preserved={};
for(const service of ['postgres','redis','ingest','timescaledb','emqx','web']) preserved[service]=inspect(service).Id;
const rollbacks={};
for(const service of ['api','admin-web']){
  const item=inspect(service),tag=`wk-factory-details-rollback-${service}:${stamp}`;
  const tagged=cp.spawnSync('docker',['tag',item.Image,tag],{cwd:root,encoding:'utf8'}).status===0;
  const paths=service==='api'?['/app/dist','/app/prisma','/app/node_modules','/app/package.json','/app/package-lock.json']:['/usr/share/nginx/html','/etc/nginx/conf.d/default.conf'];
  const archive=docker(['exec',item.Id,'tar','-czf','-',...paths]);
  assert.equal(archive[0],0x1f);assert.equal(archive[1],0x8b);
  fs.writeFileSync(path.join(out,`${service}.original.tar.gz`),archive,{flag:'wx'});
  rollbacks[service]={oldImage:item.Image,tag:tagged?tag:null};
}
console.log('Backups verified; API credentials unchanged. Updating API and management frontend only; no migration.');
for(const args of [['compose','build','api','admin-web'],['compose','up','-d','--no-deps','api','admin-web']]){
  const result=cp.spawnSync('docker',args,{cwd:root,stdio:'inherit'});
  if(result.status!==0)throw new Error('Deployment step failed; rollback backups retained.');
}
for(const [service,id] of Object.entries(preserved)) assert.equal(inspect(service).Id,id,`Unrelated service recreated: ${service}`);
docker(['exec',inspect('admin-web').Id,'nginx','-t']);
const result={deployedAt:new Date().toISOString(),services:['api','admin-web'],credentialsUnchanged:true,unrelatedServicesUnchanged:true,migrationRequired:false,rollbacks,backup:out};
fs.writeFileSync(path.join(out,'deployment.json'),JSON.stringify(result,null,2));
console.log(JSON.stringify(result));
