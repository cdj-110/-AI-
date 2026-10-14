// Fill only missing editor defaults when establishing a freshly loaded baseline.
// Never run this from dirty checks or selection changes: those must be read-only.
export function initializeDeviceDraftDefaults(device:Record<string,any>){
  const fill=(target:Record<string,any>,defaults:Record<string,any>)=>{
    for(const [key,value] of Object.entries(defaults)){
      if(target[key]===undefined||target[key]===null||target[key]==='')target[key]=value
    }
  }
  if(device.protocol==='opcda'){
    fill(device,{enabled:true,address:'192.168.1.141',workstation:'EG100',securityMode:'packet_integrity',authentication:'ntlm',connectTimeoutMilliseconds:10000,requestTimeoutMilliseconds:10000,readMode:'sync',readSource:'device',batchSize:100,autoReconnect:true,groupName:'WK_OPCDA',requestedUpdateRateMilliseconds:1000,deadband:0,localeId:1033,reconnectIntervalSeconds:30})
    fill(device,{authenticationLevel:device.securityMode==='legacy_connect'?'connect':device.securityMode==='packet_privacy'?'packet_privacy':'packet_integrity'})
  }
  if(device.protocol==='dlt645'){
    fill(device,{enabled:true})
    if(!device.dlt645)device.dlt645={}
    fill(device.dlt645,{version:'auto',meterAddress:'000000000001',responseTimeoutMilliseconds:1500,retries:1,wakeupBytes:4,writePassword:'',operatorCode:''})
  }
}
export function initializeConfigDraft(config:Record<string,any>){
  for(const field of ['resources','channels','devices','forwardDevices']){
    if(!Array.isArray(config[field]))config[field]=[]
  }
  if(!config.forwardSlave)config.forwardSlave={enabled:false,modbusListen:'0.0.0.0:1502',iec104Listen:'0.0.0.0:2404',iec61850Listen:'0.0.0.0:102'}
  for(const channel of config.channels){
    if(channel.protocol==='iec104'&&!channel.iec104)channel.iec104={acquisitionMode:'auto',generalInterrogationOnStart:true,generalInterrogationIntervalSeconds:0,clockSyncOnStart:true,clockSyncIntervalSeconds:3600,counterInterrogationOnStart:false,counterInterrogationIntervalSeconds:0}
  }
  for(const device of [...config.devices,...config.forwardDevices]){
    initializeDeviceDraftDefaults(device)
    for(const point of device.points||[]){
      if(point.decimals===undefined||point.decimals===null||point.decimals==='')point.decimals=0
    }
  }
}
