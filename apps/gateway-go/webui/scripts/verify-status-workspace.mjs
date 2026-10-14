import assert from 'node:assert/strict'
import {readFileSync} from 'node:fs'
import vm from 'node:vm'
import ts from 'typescript'

const app=readFileSync(new URL('../src/App.vue',import.meta.url),'utf8')
const navigation=app.slice(app.indexOf('const navItems:'),app.indexOf('const workspaceMode ='))
function menu(permissions,active='status'){
  const context={currentSession:{value:{permissions}},sessionReady:{value:true},active:{value:active},computed:fn=>({get value(){return fn()}})}
  vm.createContext(context)
  vm.runInContext(ts.transpile(navigation)+`;result={items:visibleNavItems.value.map(x=>x.key),active:activeMenu.value,title:current.value.label,target:menuTarget('status'),fallback:firstAccessibleNav(),canStatus:canAccessNav('status'),canTopology:canAccessNav('topology'),invalid:canAccessNav('invalid')}`,context)
  return JSON.parse(JSON.stringify(context.result))
}
const both=menu(['*'],'topology')
assert.equal(both.items.filter(x=>x==='status').length,1)
assert.ok(!both.items.includes('topology'))
assert.equal(both.active,'status')
assert.equal(both.title,'运行状态')
assert.equal(both.target,'topology')
const status=menu(['status.view'])
assert.equal(status.canTopology,false)
assert.equal(status.fallback,'status')
const topology=menu(['config.view'])
assert.equal(topology.canStatus,false)
assert.equal(topology.target,'topology')
assert.equal(topology.fallback,'topology')
assert.equal(topology.items[0],'status')
assert.equal(menu(['alarms.view']).fallback,'alarms')
assert.deepEqual(menu([]).items,[])
assert.equal(both.invalid,false)
assert.ok(app.includes('Object.hasOwn(navPermissions,key)'), 'Old topology deep link remains valid')
const workspace=readFileSync(new URL('../src/components/StatusWorkspace.vue',import.meta.url),'utf8')
assert.ok(workspace.includes('<StatusView v-if="canStatus"'), 'Status stays mounted behind the modal')
assert.ok(workspace.includes('v-if="opened&&canTopology"'), 'Closed topology unmounts and stops polling')
assert.ok(workspace.includes('showModal()')&&workspace.includes('@cancel.prevent='), 'Native modal supports focus containment and Escape')
assert.ok(!workspace.includes('role="tablist"'))
const statusView=readFileSync(new URL('../src/components/StatusView.vue',import.meta.url),'utf8')
assert.equal((statusView.match(/设备拓扑/g)||[]).length,1,'Only one topology entry remains')
assert.ok(statusView.includes('v-if="canTopology"'))
console.log('PASS single sidebar entry, shared selection, permissions, fallback, deep links and modal contracts')
