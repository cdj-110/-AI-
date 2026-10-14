import assert from 'node:assert/strict'
import {dlt645Formats,dlt645ConfiguredType,selectDLT645Option,selectDLT645Type} from '../src/dlt645PointOptions.ts'

const item={key:'energy',name:'正向有功总电能',category:'电能',di2007:'00010000',di1997:'9010',valueFormat:'bcd',dataLength:4,decimals:2,dataType:'float64',unit:'kWh',access:'read'}
const point=()=>({metric:'dlt645-P01',name:'现场名称',scale:2,offset:1,decimals:0,dataType:'float64',dlt645:{itemKey:'',dataIdentifier:'00000000',valueFormat:'bcd',dataLength:4,access:'read'}})
for(const format of dlt645Formats){
  const p=point()
  selectDLT645Option(p,'format:'+format.key,[item])
  assert.equal(p.dlt645.valueFormat,format.key)
  assert.equal(p.dataType,format.type)
  assert.equal(dlt645ConfiguredType(p,[item]),format.type)
  assert.equal(p.dlt645.dataLength,format.length)
}
const p=point()
selectDLT645Option(p,'item:energy',[item],'1997')
assert.equal(p.unit,'kWh');assert.equal(p.decimals,2)
assert.equal(p.metric,'dlt645-P01');assert.equal(p.name,'现场名称')
assert.equal(p.scale,2);assert.equal(p.offset,1)
assert.equal(p.dlt645.dataIdentifier,'')
let saved=structuredClone(p)
selectDLT645Type(p,'string',[item])
assert.deepEqual(p,saved,'standard type cannot override dictionary decoder')
selectDLT645Option(p,'format:bcd',[item],'1997')
assert.equal(p.dlt645.dataIdentifier,'9010')
assert.equal(p.decimals,2,'keep precision when converting to the same manual format')
for(const [type,format] of [['uint64','binary'],['string','ascii'],['object','demand-time'],['float64','bcd']]){
  selectDLT645Type(p,type,[item]);assert.equal(p.dataType,type);assert.equal(p.dlt645.valueFormat,format)
}
saved=structuredClone(p)
selectDLT645Option(p,'item:unknown',[item])
selectDLT645Option(p,'format:unknown',[item])
selectDLT645Type(p,'int16',[item])
selectDLT645Option(p,'item:energy',[{...item,di1997:''}],'1997')
assert.deepEqual(p,saved,'invalid and version-incompatible options leave saved config untouched')
const legacy={...point(),dlt645:{itemKey:'',valueFormat:'raw_hex',dataLength:4}}
assert.equal(dlt645ConfiguredType(legacy,[]),'string')
console.log('DL/T 645: format/type mapping, standard metadata, DI versions, legacy format and invalid selections passed')
