import assert from 'node:assert/strict'
import {alarmLimitsError,alarmLimitsSummary,createAlarmLimits,normalizeAlarmLimits} from '../src/alarmLimits.ts'
const limits=createAlarmLimits()
assert.match(alarmLimitsError(limits),/请填写上限/)
limits.high.threshold=0
assert.equal(alarmLimitsError(limits),'')
limits.low.enabled=true;limits.low.threshold=10
assert.match(alarmLimitsError(limits),/下限必须小于上限/)
limits.low.threshold=-10;limits.lowLow.enabled=true;limits.lowLow.threshold=-20;limits.highHigh.enabled=true;limits.highHigh.threshold=20
assert.equal(alarmLimitsError(limits),'')
assert.match(alarmLimitsSummary(limits),/下下限 < -20.*下限 < -10.*上限 > 0.*上上限 > 20/)
const partial=normalizeAlarmLimits({high:{threshold:10,level:'WARNING'}})
assert.equal(partial.high.enabled,true);assert.equal(partial.low.enabled,false)
assert.equal(alarmLimitsError(partial),'')
for(const value of [null,'',NaN,Infinity]){partial.high.threshold=value;assert.match(alarmLimitsError(partial),/有效阈值/)}
console.log('Four-limit UI helpers passed')
