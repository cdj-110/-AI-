import http from 'node:http'

const now=Date.now()
const point={deviceKey:'noise-meter-1',metric:'NOISE_DB',name:'噪声值',dataType:'float64',protocol:'modbus-rtu',function:3,register:0,quantity:2,byteOrder:'big',wordOrder:'normal',scale:1,offset:0,unit:'dB',enabled:true}
const config={gatewayKey:'WK-DESIGN-QA',resources:[{resourceKey:'serial-1',name:'RS485-1',type:'serial',enabled:true,serial:{port:'/dev/ttyS1',baudRate:9600,dataBits:8,stopBits:1,parity:'none'}}],channels:[{channelKey:'channel-noise',resourceKey:'serial-1',name:'噪声采集',type:'serial',role:'collect',protocol:'modbus-rtu',collectIntervalSeconds:1,enabled:true}],devices:[{deviceKey:'noise-meter-1',channelKey:'channel-noise',name:'噪声仪',protocol:'modbus-rtu',address:'/dev/ttyS1',enabled:true,points:[point]}],forwardDevices:[],edgeComputing:{groups:[]},mqttChannels:[]}
const task={taskKey:'history-noise',name:'噪声趋势记录',enabled:true,selectionMode:'selected',points:[{deviceKey:point.deviceKey,metric:point.metric}],saveMode:'interval',saveIntervalSeconds:1,changeDeadband:0}
const values=Array.from({length:100},(_,index)=>{
  const wave=Math.sin(index/7)*.65+Math.cos(index/13)*.28
  const spike=index===25?6.7:index===26?4.4:index===46?3.2:index===61?5.1:0
  return 42.8+wave+spike
})
const records=values.map((value,index)=>({id:index+1,time:new Date(now-index*30000).toISOString(),deviceKey:point.deviceKey,metric:point.metric,value,valueText:value.toFixed(1),actualDataType:'float64',quality:'GOOD',sourceTimestamp:new Date(now-index*30000-80).toISOString(),receiveTimestamp:new Date(now-index*30000).toISOString(),collectionMode:'poll'}))
let statusTick=0

function json(response,status,payload){response.writeHead(status,{'Content-Type':'application/json; charset=utf-8','Cache-Control':'no-store'});response.end(JSON.stringify(payload))}

http.createServer((request,response)=>{
  const url=new URL(request.url||'/',`http://${request.headers.host||'127.0.0.1'}`)
  if(url.pathname==='/api/config/bootstrap')return json(response,200,{content:JSON.stringify(config),pointCounts:{[point.deviceKey]:1},totalPoints:1,largeMode:false})
  if(url.pathname==='/api/config')return json(response,200,{content:JSON.stringify(config)})
  if(url.pathname==='/api/history-storage')return json(response,200,{configured:true,active:true,storagePath:'/userdata/weikong/data',storageType:'local',maxSizeMB:128,databasePath:'/userdata/weikong/data/telemetry-history.db',bytes:286720,samples:2209,writable:true,freeBytes:318767104,minFreeBytes:33554432,lastWriteAt:new Date(now).toISOString(),recoveryCount:0,retentionDays:30,tasks:[task],taskStats:[{taskKey:task.taskKey,samples:2209,estimatedBytes:286720,lastWriteAt:new Date(now).toISOString()}],devices:[]})
  if(url.pathname==='/api/history-storage/points')return json(response,200,{offset:0,limit:500,total:1,points:[point]})
  if(url.pathname==='/api/history-storage/query')return json(response,200,{offset:0,limit:100,total:2209,resolution:'',items:records})
  if(url.pathname==='/api/history-storage/exports')return json(response,200,{jobs:[]})
  if(url.pathname==='/api/history-storage/maintenance')return json(response,200,{database:{},integrity:{mode:'quick',result:'ok',ok:true,checkedAt:new Date(now).toISOString(),durationMs:8},backups:[],backupPath:'/userdata/weikong/data/backups',backupEnabled:false,backupIntervalHours:24,backupKeep:3,encryptBackups:false,aggregates:{enabled:false,hourlyRows:0,dailyRows:0}})
  if(url.pathname==='/api/point-status'){
    const capturedAt=new Date(),value=42.8+Math.sin(statusTick/2)*.9+(statusTick%9===7?2.4:0);statusTick++
    return json(response,200,[{...point,address:'/dev/ttyS1',value,valueText:value.toFixed(3),actualDataType:'float64',quality:'GOOD',sourceTimestamp:new Date(capturedAt.getTime()-85).toISOString(),receiveTimestamp:capturedAt.toISOString(),updatedAt:capturedAt.toISOString(),collectionMode:'sync'}])
  }
  if(url.pathname==='/api/status'){
    const capturedAt=new Date(),value=42.8+Math.sin(statusTick/2)*.9+(statusTick%9===7?2.4:0);statusTick++
    return json(response,200,{gatewayKey:'WK-DESIGN-QA',uptimeSeconds:900,collectSeconds:1,mqttEnabled:false,mqttConnected:false,mqttChannels:{},lastCollectAt:capturedAt.toISOString(),pointCount:1,healthyCount:1,pendingCount:0,staleCount:0,errorCount:0,points:[{...point,address:'/dev/ttyS1',value,valueText:value.toFixed(3),actualDataType:'float64',quality:'GOOD',sourceTimestamp:new Date(capturedAt.getTime()-85).toISOString(),receiveTimestamp:capturedAt.toISOString(),updatedAt:capturedAt.toISOString(),collectionMode:'sync'}],errors:[]})
  }
  if(url.pathname==='/api/alarms')return json(response,200,{active:[],history:[],activeCount:0,criticalCount:0,unackedCount:0})
  if(url.pathname==='/api/network/wifi')return json(response,200,{available:false})
  if(url.pathname==='/api/network/cellular')return json(response,200,{enabled:false,available:false})
  return json(response,200,{})
}).listen(8088,'127.0.0.1',()=>console.log('history mock API ready on http://127.0.0.1:8088'))
