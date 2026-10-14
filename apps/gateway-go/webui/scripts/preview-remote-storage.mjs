// Isolated UI fixture. It never contacts a gateway or a real database.
import {createServer} from 'vite'
import vue from '@vitejs/plugin-vue'
let config={storagePath:'/userdata/weikong/data',storageUUID:'',maxSizeMB:16,targets:[]},revision='fixture-1'
const points=Array.from({length:125},(_,i)=>({deviceKey:'meter-1',metric:'P'+i,name:'测试测点 '+i,unit:'kWh'}))
const statuses=()=>config.targets.map(t=>({id:t.id,enabled:t.enabled,cacheReady:true,pending:t.enabled?23:0,bytes:16384,sent:200,dropped:0,connected:false,lastError:t.enabled?'模拟数据库暂不可达，已进入补传等待':'',nextRetry:new Date(Date.now()+10000).toISOString()}))
const result=()=>({config,revision,statuses:statuses(),devices:[{device:'/dev/sda1',mountPath:'/mnt/udisk',uuid:'fixture-volume',freeBytes:20000000000}],pointGroups:[{deviceKey:'meter-1',name:'测试电表',count:125}]})
const fixture={name:'remote-storage-fixture',configureServer(server){server.middlewares.use((req,res,next)=>{
 const u=new URL(req.url,'http://localhost');if(!u.pathname.startsWith('/api/'))return next();res.setHeader('Content-Type','application/json')
 if(u.pathname==='/api/remote-storage/status'){res.end(JSON.stringify({statuses:statuses()}));return}
 if(u.pathname==='/api/history-storage/points'){const list=points.filter(p=>(p.name+' '+p.metric).includes(u.searchParams.get('search')||'')),offset=Number(u.searchParams.get('offset')||0);res.end(JSON.stringify({total:list.length,points:list.slice(offset,offset+100)}));return}
 if(req.method==='PUT'||req.method==='POST'){let raw='';req.on('data',c=>raw+=c);req.on('end',()=>{const data=JSON.parse(raw);if(u.pathname.endsWith('/test')){res.end(JSON.stringify({ok:false,message:'模拟：连接成功，但专用表尚未创建；本次测试未建表、未写入数据',durationMs:25}));return}if(req.headers['if-match']!==revision){res.statusCode=409;res.end(JSON.stringify({message:'模拟配置版本冲突'}));return}config=data;for(const t of config.targets)if(t.password)t.password='***';revision='fixture-'+Date.now();res.end(JSON.stringify(result()))});return}
 res.end(JSON.stringify(result()))
})}}
const server=await createServer({configFile:false,plugins:[vue(),fixture],server:{host:'127.0.0.1',port:5190,strictPort:true}})
await server.listen();server.printUrls()
