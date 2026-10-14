import assert from 'node:assert/strict'
import {readFile} from 'node:fs/promises'
import {timeAxisFractions,contiguousSegments} from '../src/timeAxis.ts'
const times=['2026-09-09T00:00:00Z','2026-09-09T00:00:01Z','2026-09-09T00:01:01Z']
assert.deepEqual(timeAxisFractions(times),[0,1/61,1])
assert.deepEqual(timeAxisFractions([times[0],times[0]]),[.5,.5])
assert.ok(Number.isNaN(timeAxisFractions(['bad'])[0]))
assert.deepEqual(contiguousSegments([1,2,undefined,3,undefined,4,5]),[[1,2],[3],[4,5]])
for(const name of ['StorageView','AlarmView','EdgeComputeView','SmartGatewayView']){
 const source=await readFile(new URL('../src/components/'+name+'.vue',import.meta.url),'utf8')
 assert.match(source,/usePageDraftGuard\(/,name+' must protect unsaved changes')
}
for(const name of ['StorageView','AlarmView']){
 const source=await readFile(new URL('../src/components/'+name+'.vue',import.meta.url),'utf8')
 assert.match(source,/canManage/)
 assert.match(source,/:disabled="!canManage"/)
}
const edge=await readFile(new URL('../src/components/EdgeComputeView.vue',import.meta.url),'utf8')
assert.match(edge,/\/api\/edge-compute\/config/)
assert.doesNotMatch(edge,/await store\.save\(/)
console.log('Audit fixes: elapsed time, quality gaps, readonly controls, draft guards and scoped edge save passed')
const storage=await readFile(new URL('../src/components/StorageView.vue',import.meta.url),'utf8')
assert.doesNotMatch(storage,/await store\.load\(/,'History viewer must not require config.view')
assert.match(storage,/history\.value\.pointGroups/)
