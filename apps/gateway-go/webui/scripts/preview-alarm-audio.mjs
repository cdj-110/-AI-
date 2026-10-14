// Local-only fixture server. No requests are forwarded to EG100.
import {createServer} from 'vite'
import vue from '@vitejs/plugin-vue'
import {mkdir,writeFile} from 'node:fs/promises'
const config={resources:[],channels:[],devices:[{deviceKey:'d1',name:'噪声仪',protocol:'modbus-tcp',points:[{metric:'NOISE_DB',name:'噪声值',dataType:'float64'}]}]}
const rules=[{ruleId:'preview-rule',name:'噪声值 告警',enabled:true,deviceKey:'d1',metric:'NOISE_DB',type:'threshold',hysteresis:0,triggerDelaySeconds:0,recoveryDelaySeconds:0,level:'WARNING',limits:{lowLow:{enabled:false,level:'CRITICAL'},low:{enabled:false,level:'WARNING'},high:{enabled:true,threshold:10,level:'WARNING'},highHigh:{enabled:true,threshold:40,level:'CRITICAL'}}}]
const fixture={name:'alarm-audio-fixture',configureServer(server){server.middlewares.use((req,res,next)=>{
 const path=new URL(req.url,'http://localhost').pathname
 if(path==='/__audio-tests'){res.setHeader('Content-Type','text/html');res.end('<!doctype html><meta charset="UTF-8"><title>告警音频回归测试</title><h1>告警音频回归测试</h1><button id="run">运行本地音频测试</button><pre id="result">尚未运行</pre><script type="module" src="/scripts/alarm-audio-browser-tests.ts"></script>');return}
 if(!path.startsWith('/api/'))return next()
 let data={}
 if(path==='/api/session')data={username:'alarm-audio-preview',permissions:['config.view','alarms.view','alarms.manage']}
 else if(path.startsWith('/api/config'))data={content:JSON.stringify(config),largeMode:false}
 else if(path==='/api/alarms')data={active:[],history:[],activeCount:0,unackedCount:0,criticalCount:0,popupDurationSeconds:0}
 else if(path==='/api/alarm-rules')data={rules}
 else if(path==='/api/alarms/history')data={items:[],total:0,page:1,pageSize:50,totalPages:1}
 else if(path==='/api/point-status')data=[]
 res.setHeader('Content-Type','application/json');res.end(JSON.stringify(data))
})}}
const server=await createServer({configFile:false,plugins:[vue(),fixture],server:{host:'127.0.0.1',port:5187,strictPort:true},logLevel:'info'})
const samples=8000*2,wav=Buffer.alloc(44+samples*2)
wav.write('RIFF');wav.writeUInt32LE(wav.length-8,4);wav.write('WAVEfmt ',8);wav.writeUInt32LE(16,16);wav.writeUInt16LE(1,20);wav.writeUInt16LE(1,22);wav.writeUInt32LE(8000,24);wav.writeUInt32LE(16000,28);wav.writeUInt16LE(2,32);wav.writeUInt16LE(16,34);wav.write('data',36);wav.writeUInt32LE(samples*2,40)
for(let i=0;i<samples;i++)wav.writeInt16LE(Math.round(Math.sin(i*2*Math.PI*800/8000)*2000),44+i*2)
await mkdir('artifacts/alarm-audio',{recursive:true});await writeFile('artifacts/alarm-audio/test-alarm.wav',wav)
await server.listen();server.printUrls()
