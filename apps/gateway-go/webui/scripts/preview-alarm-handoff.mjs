// Isolated local UI fixture; never connects to a physical gateway.
import {createServer} from 'vite'
import vue from '@vitejs/plugin-vue'
const active=['温度上限','压力上限'].map((name,i)=>({eventId:`test-${i}`,ruleId:`rule-${i}`,ruleName:name,pointName:name,deviceKey:'fixture',metric:`p${i}`,level:i?'WARNING':'CRITICAL',state:'ACTIVE',acknowledged:false,popupEnabled:true,value:80,message:'仅用于本地告警界面验证',occurredAt:'2026-09-12T03:00:00Z'}))
const fixture={name:'alarm-handoff-fixture',configureServer(server){server.middlewares.use((req,res,next)=>{
 const path=new URL(req.url,'http://localhost').pathname
 if(!path.startsWith('/api/'))return next()
 let data={}
 if(path==='/api/session')data={username:'handoff-preview',permissions:['alarms.view','config.view']}
 else if(path.startsWith('/api/config'))data={content:JSON.stringify({resources:[],channels:[],devices:[]}),largeMode:false}
 else if(path==='/api/alarms')data={active,history:[],activeCount:2,unackedCount:2,criticalCount:1,popupDurationSeconds:0}
 else if(path==='/api/alarm-rules')data={rules:[]}
 else if(path==='/api/point-status')data=[]
 res.setHeader('Content-Type','application/json');res.end(JSON.stringify(data))
})}}
const server=await createServer({configFile:false,plugins:[vue(),fixture],server:{host:'127.0.0.1',port:5188,strictPort:true}})
await server.listen();server.printUrls()
