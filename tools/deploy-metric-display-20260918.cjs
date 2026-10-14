const fs=require('node:fs'),path=require('node:path'),cp=require('node:child_process'),assert=require('node:assert/strict');
const root=path.resolve(__dirname,'..'),stamp=new Date().toISOString().replace(/[-:.TZ]/g,'').slice(0,14);
const out=path.join(root,'artifacts',`metric-display-deploy-${stamp}`),services=['web','admin-web'];
const docker=args=>cp.execFileSync('docker',args,{cwd:root,maxBuffer:96*1024*1024});
function inspect(service){const id=docker(['compose','ps','-q',service]).toString().trim();assert.ok(id&&!id.includes('\n'));return JSON.parse(docker(['inspect',id]))[0]}
function run(args){const result=cp.spawnSync('docker',args,{cwd:root,stdio:'inherit'});assert.equal(result.status,0,'Frontend deployment step failed')}
const preserved=Object.fromEntries(['api','postgres','timescaledb','redis','emqx','ingest'].map(service=>[service,inspect(service).Id]));
fs.mkdirSync(out,{recursive:true});
const previous={},rollback={services:{}};
for(const service of services){
 const container=inspect(service),tag=`wk-metric-display-rollback-${service}:${stamp}`;
 docker(['tag',container.Image,tag]);
 const archive=docker(['exec',container.Id,'tar','-czf','-','/usr/share/nginx/html','/etc/nginx/conf.d/default.conf']);
 assert.equal(archive[0],0x1f);assert.equal(archive[1],0x8b);
 fs.writeFileSync(path.join(out,`${service}.original.tar.gz`),archive,{flag:'wx'});
 previous[service]={image:container.Image,rollbackTag:tag};rollback.services[service]={image:tag};
}
const rollbackFile=path.join(out,'compose.rollback.json');
fs.writeFileSync(rollbackFile,JSON.stringify(rollback,null,2));
fs.writeFileSync(path.join(out,'before.json'),JSON.stringify({previous,preserved},null,2));
let replacing=false;
(async()=>{try{
 console.log('Both frontend backups verified. Updating frontends only; backend, databases and gateway remain unchanged.');
 run(['compose','build',...services]);replacing=true;
 run(['compose','up','-d','--no-deps',...services]);
 const images={};
 for(const [index,service] of services.entries()){
  const current=inspect(service);assert.equal(current.State.Running,true);docker(['exec',current.Id,'nginx','-t']);images[service]=current.Image;
  let response;
  for(let attempt=0;attempt<20;attempt++){try{response=await fetch(`http://127.0.0.1:${index?5176:5174}/`);if(response.ok)break}catch{}await new Promise(resolve=>setTimeout(resolve,500))}
  assert.ok(response?.ok,`${service} not responding`);
  const html=await response.text(),asset=html.match(/src="([^\"]*\/assets\/index-[^\"]+\.js)"/);
  assert.ok(asset,'Built entry asset must exist');assert.ok((await fetch(`http://127.0.0.1:${index?5176:5174}${asset[1]}`)).ok);
 }
 for(const [service,id] of Object.entries(preserved))assert.equal(inspect(service).Id,id,`Unrelated service recreated: ${service}`);
 const result={deployedAt:new Date().toISOString(),services,images,previous,backup:out,bothPortalsRespond:true,apiAndDatabasesUnchanged:true,telemetryIngestUnchanged:true,migrationRequired:false};
 fs.writeFileSync(path.join(out,'deployment.json'),JSON.stringify(result,null,2));console.log(JSON.stringify(result));
}catch(error){if(replacing){run(['compose','-f','docker-compose.yml','-f',rollbackFile,'up','-d','--no-deps','--no-build',...services]);console.error('Both frontends restored to the verified previous images.')}throw error}
})().catch(error=>{console.error(error.message);process.exitCode=1});
