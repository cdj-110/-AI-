import assert from 'node:assert/strict'
import {readFileSync} from 'node:fs'
import {buildTopology,deviceState,deviceSelection,channelProtocol} from '../src/topology.ts'
import {channelRole,groupChannels} from '../src/channelGroups.ts'
const cfg={resources:[{resourceKey:'s',type:'serial'},{resourceKey:'empty',type:'serial'},{resourceKey:'n',type:'network'}],channels:[{resourceKey:'n',channelKey:'f',role:'forward'},{resourceKey:'n',channelKey:'n',role:'collect'},{resourceKey:'s',channelKey:'s',role:'collect'}],devices:[{deviceKey:'a',channelKey:'s'},{deviceKey:'orphan'}],forwardDevices:[{deviceKey:'f',channelKey:'f'}]}
const healthy={deviceKey:'a',total:2,healthy:2,failed:0,pending:0,stale:0}
const tree=buildTopology(cfg,[healthy])
assert.equal(tree[0].channels[0].devices[0].state,'healthy')
assert.equal(tree[1].channels.length,0,'empty serial retained')
assert.deepEqual(tree[2].channels.map(c=>c.role),['collect','forward'])
assert.equal(tree[2].channels[1].devices[0].state,'forward','source data cannot prove forward link online')
assert.equal(tree[3].channels[0].devices[0].deviceKey,'orphan')
assert.equal(deviceState({}, {}, {}, {...healthy,healthy:1,failed:1}),'partial')
assert.equal(deviceState({}, {}, {}, {...healthy,healthy:0,stale:2}),'failed')
assert.equal(deviceState({}, {}, {enabled:false},healthy),'disabled')
assert.equal(deviceState({}, {}, {},undefined),'pending')
assert.deepEqual(deviceSelection(cfg,'a'),{resourceKey:'s',channelKey:'s',deviceKey:'a'})
for(const forwardProtocol of [undefined,'','none']){
 const channel={channelKey:'s',resourceKey:'s',role:'collect',protocol:'modbus-rtu',forwardProtocol}
 const config={resources:cfg.resources,channels:[channel],devices:[{deviceKey:'a',channelKey:'s'}]}
 const resource=buildTopology(config,[healthy])[0]
 assert.equal(resource.channels[0].role,'collect')
 assert.equal(resource.channels[0].devices[0].state,'healthy')
 assert.equal(channelProtocol(channel),'modbus-rtu')
 assert.equal(groupChannels([channel])[0].channels.length,1)
}
assert.equal(channelRole({forwardProtocol:'none'}),'collect')
assert.equal(channelRole({forwardProtocol:'modbus-tcp-slave'}),'forward')
assert.equal(channelRole({role:'collect',forwardProtocol:'modbus-tcp-slave'}),'collect','explicit role wins')
const mismatched=buildTopology({...cfg,devices:[{deviceKey:'wrong-role',channelKey:'f'}]},[])
assert.equal(mismatched.flatMap(r=>r.channels.flatMap(c=>c.devices)).filter(d=>d.deviceKey==='wrong-role').length,1,'wrong-role binding stays visible as unassigned')
const view=readFileSync(new URL('../src/components/TopologyView.vue',import.meta.url),'utf8')
assert.ok(view.includes('useConfigStore'))
assert.ok(view.includes('await configStore.load()'))
assert.ok(!view.includes("'/api/config/bootstrap'"),'topology must share the sidebar config store')
const app=readFileSync(new URL('../src/App.vue',import.meta.url),'utf8')
assert.ok(app.includes("topology:['config.view']"))
assert.ok(app.includes('@open-device="openTopologyDevice"'))
console.log('PASS topology structure, empty/orphan resources, collect/forward separation, health and navigation contracts')
