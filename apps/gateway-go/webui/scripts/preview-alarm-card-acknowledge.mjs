// Disposable local fixture: requests never reach a real gateway.
import {createServer} from 'vite'
import vue from '@vitejs/plugin-vue'
const active=['上限','上上限','压力'].map((name,i)=>({eventId:`ack-${i}`,ruleId:i<2?`temp::${i}`:'pressure',parentRuleId:i<2?'temp':undefined,parentRuleName:i<2?'温度告警':undefined,limitName:i<2?name:undefined,limitKey:i<2?(i?'highHigh':'high'):undefined,ruleName:name,pointName:name,deviceKey:'fixture',metric:i<2?'temp':'pressure',level:i===1?'CRITICAL':'WARNING',state:'ACTIVE',acknowledged:false,popupEnabled:true,value:80,message:'仅用于本地确认操作验证',occurredAt:'2026-09-12T03:00:00Z'}))
let fail=true
const fixture={name:'alarm-ack-fixture',configureServer(server){server.middlewares.use((req,res,next)=>{
 const path=new URL(req.url,'http://localhost').pathname
 if(!path.startsWith('/api/'))return next()
 res.setHeader('Content-Type','application/json')
 if(path==='/api/alarms/acknowledge'){
  let raw='';req.on('data',chunk=>raw+=chunk);req.on('end',()=>{
   const item=active.find(e=>e.eventId===JSON.parse(raw).eventId)
   if(item?.eventId==='ack-1'&&fail){fail=false;res.statusCode=503;res.end(JSON.stringify({message:'模拟服务忙，请重试'}));return}
   if(!item){res.statusCode=404;res.end('{}');return}
   item.acknowledged=true;res.end(JSON.stringify({ok:true,alarm:item}))
  });return
 }
 let data={}
 if(path==='/api/session')data={username:'ack-preview',permissions:['alarms.manage','alarms.view','config.view']}
 else if(path.startsWith('/api/config'))data={content:JSON.stringify({resources:[],channels:[],devices:[]}),largeMode:false}
 else if(path==='/api/alarms')data={active,history:[],activeCount:active.length,unackedCount:active.filter(e=>!e.acknowledged).length,criticalCount:1,popupDurationSeconds:10}
 else if(path==='/api/alarm-rules')data={rules:[]}
 else if(path==='/api/point-status')data=[]
 res.end(JSON.stringify(data))
})}}
const server=await createServer({configFile:false,plugins:[vue(),fixture],server:{host:'127.0.0.1',port:5189,strictPort:true}})
await server.listen();server.printUrls()
