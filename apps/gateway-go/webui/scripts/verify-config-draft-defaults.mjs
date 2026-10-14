import assert from 'node:assert/strict'
import {initializeConfigDraft} from '../src/configDraftDefaults.ts'
const supplied={gatewayKey:'G1',channels:[{protocol:'iec104',iec104:{acquisitionMode:'periodic-gi',clockSyncOnStart:false}}],devices:[{protocol:'opcda',address:'custom-host:135',points:[{metric:'custom',name:'custom name',decimals:3,protocol:'explicit',address:'explicit-point-address'}]}],forwardSlave:{enabled:true,modbusListen:'127.0.0.1:2502'}}
const original=structuredClone(supplied)
initializeConfigDraft(supplied)
assert.deepEqual(supplied.channels,original.channels)
for(const [key,value] of Object.entries(original.devices[0]))assert.deepEqual(supplied.devices[0][key],value)
assert.deepEqual(supplied.forwardSlave,original.forwardSlave)
assert.deepEqual(supplied.forwardDevices,[])
const snapshot=JSON.stringify(supplied)
initializeConfigDraft(supplied)
assert.equal(JSON.stringify(supplied),snapshot,'defaults must be idempotent')
const missing={channels:[{protocol:'iec104'}],devices:[{points:[{decimals:null},{decimals:''},{decimals:0},{}]}]}
initializeConfigDraft(missing)
assert.equal(missing.channels[0].iec104.acquisitionMode,'auto')
assert.ok(missing.devices[0].points.every(point=>point.decimals===0))
assert.equal(missing.forwardSlave.enabled,false)
const protocols={devices:[{protocol:'opcda',enabled:false,autoReconnect:false,readMode:'subscription',securityMode:'packet_privacy',authentication:'custom',deadband:0,localeId:0},{protocol:'dlt645',enabled:false,dlt645:{version:'1997',retries:0,wakeupBytes:0,meterAddress:'000000000099'}}]}
const preserved=structuredClone(protocols.devices)
initializeConfigDraft(protocols)
for(let index=0;index<preserved.length;index++){
  for(const [key,value] of Object.entries(preserved[index])){
    if(key==='dlt645')for(const [field,setting] of Object.entries(value))assert.deepEqual(protocols.devices[index][key][field],setting)
    else assert.deepEqual(protocols.devices[index][key],value)
  }
}
assert.equal(protocols.devices[0].authenticationLevel,'packet_privacy')
assert.equal(protocols.devices[1].dlt645.responseTimeoutMilliseconds,1500)
const protocolBaseline=JSON.stringify(protocols)
initializeConfigDraft(protocols)
assert.equal(JSON.stringify(protocols),protocolBaseline,'opening protocol forms must not change the loaded baseline')
console.log('Config draft defaults: missing fields initialized, supplied values preserved, repeated initialization stable.')
