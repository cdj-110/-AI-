import assert from 'node:assert/strict'
import {readFileSync} from 'node:fs'
import {newRemoteTarget,changeRemoteDriver,validateRemoteTarget,pointKey} from '../src/remoteStorage.ts'
const t=newRemoteTarget()
assert.equal(t.enabled,false);assert.equal(t.createTable,false);assert.equal(t.tlsMode,'verify-full')
t.host='db.example';t.database='plant';t.username='writer';t.points=[{deviceKey:'d',metric:'p'}]
assert.equal(validateRemoteTarget({...t,points:[]},false),'');assert.match(validateRemoteTarget({...t,points:[]}),/请选择/)
for(const [driver,port,schema] of [['mysql',3306,''],['sqlserver',1433,'dbo'],['postgresql',5432,'public']]){t.driver=driver;changeRemoteDriver(t);assert.equal(t.port,port);assert.equal(t.schema,schema);assert.equal(validateRemoteTarget(t),'')}
t.table='bad;sql';assert.match(validateRemoteTarget(t),/表名/);t.table='samples'
t.points.push(t.points[0]);assert.match(validateRemoteTarget(t),/重复/)
assert.notEqual(pointKey({deviceKey:'a/b',metric:'c'}),pointKey({deviceKey:'a',metric:'b/c'}))
const view=readFileSync(new URL('../src/components/RemoteStorageView.vue',import.meta.url),'utf8')
assert.match(view,/usePageDraftGuard/);assert.match(view,/'If-Match':revision.value/);assert.match(view,/测试连接（只读）/)
const poll=view.slice(view.indexOf('async function poll'),view.indexOf('function selectVolume'))
assert.doesNotMatch(poll,/config.value\s*=/,'poll must not overwrite unsaved form')
const storage=readFileSync(new URL('../src/components/StorageView.vue',import.meta.url),'utf8')
assert.match(storage,/requestLeave\(\)/);assert.match(storage,/<RemoteStorageView v-if="activeHistoryTab==='remote'"/)
console.log('remote storage: defaults, driver mapping, validation, point identity, draft protection and polling passed')
