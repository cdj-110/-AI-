const fs=require('node:fs'),path=require('node:path'),cp=require('node:child_process'),assert=require('node:assert/strict');
const root=path.resolve(__dirname,'..'),stamp=new Date().toISOString().replace(/[-:.TZ]/g,'').slice(0,14);
const out=path.join(root,'artifacts',`cloud-trend-deploy-${stamp}`);
const docker=args=>cp.execFileSync('docker',args,{cwd:root,maxBuffer:96*1024*1024});
function inspect(service){const id=docker(['compose','ps','-q',service]).toString().trim();assert.ok(id&&!id.includes('\n'));return JSON.parse(docker(['inspect',id]))[0]}
function run(args){const result=cp.spawnSync('docker',args,{cwd:root,stdio:'inherit'});assert.equal(result.status,0,'Frontend deployment step failed')}
const preserved=Object.fromEntries(['api','admin-web','postgres','timescaledb','redis','emqx','ingest'].map(service=>[service,inspect(service).Id]));
const previous=inspect('web'),rollbackTag=`wk-cloud-trend-rollback:${stamp}`;
fs.mkdirSync(out,{recursive:true});
docker(['tag',previous.Image,rollbackTag]);
const archive=docker(['exec',previous.Id,'tar','-czf','-','/usr/share/nginx/html','/etc/nginx/conf.d/default.conf']);
assert.equal(archive[0],0x1f);assert.equal(archive[1],0x8b);
fs.writeFileSync(path.join(out,'web.original.tar.gz'),archive,{flag:'wx'});
const rollbackFile=path.join(out,'compose.rollback.json');
fs.writeFileSync(rollbackFile,JSON.stringify({services:{web:{image:rollbackTag}}},null,2));
fs.writeFileSync(path.join(out,'before.json'),JSON.stringify({oldImage:previous.Image,rollbackTag,preserved},null,2));
console.log('Tenant frontend backup verified. Updating web only; API, management portal, databases and telemetry ingest will not restart.');
let replacing=false;
(async()=>{
 try{
  run(['compose','build','web']);replacing=true;run(['compose','up','-d','--no-deps','web']);
  const current=inspect('web');assert.equal(current.State.Running,true);
  docker(['exec',current.Id,'nginx','-t']);
  for(const [service,id] of Object.entries(preserved))assert.equal(inspect(service).Id,id,`Unrelated service recreated: ${service}`);
  let response;
  for(let attempt=0;attempt<20;attempt++){try{response=await fetch('http://127.0.0.1:5174/');if(response.ok)break}catch{}await new Promise(resolve=>setTimeout(resolve,500))}
  assert.ok(response?.ok,'Tenant portal not responding');
  const html=await response.text();assert.ok(html.includes('/assets/index-'));
  const result={deployedAt:new Date().toISOString(),service:'web',oldImage:previous.Image,newImage:current.Image,backup:out,rollbackTag,portalResponds:true,apiAndDatabaseUnchanged:true,managementPortalUnchanged:true,telemetryIngestUnchanged:true,migrationRequired:false};
  fs.writeFileSync(path.join(out,'deployment.json'),JSON.stringify(result,null,2));
  console.log(JSON.stringify(result));
 }catch(error){
  if(replacing){run(['compose','-f','docker-compose.yml','-f',rollbackFile,'up','-d','--no-deps','--no-build','web']);console.error('Frontend rolled back to the verified previous image.');}
  throw error;
 }
})().catch(error=>{console.error(error.message);process.exitCode=1});
