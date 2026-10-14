import assert from 'node:assert/strict'
import {readFileSync} from 'node:fs'
import {normalizeAlarmSoundSettings} from '../src/alarmSound.ts'
import {applyAlarmColors} from '../src/alarmColors.ts'

const variables=new Map()
globalThis.document={documentElement:{style:{setProperty:(key,value)=>variables.set(key,value)}}}
const css=readFileSync(new URL('../src/alarm-level-colors.css',import.meta.url),'utf8')
const badges=css.match(/\.app-shell \.header-alarm-button\.critical b,\.app-shell \.nav-alarm-count\.critical\{([^}]+)\}/)?.[1]
assert.ok(badges,'header and sidebar badges share the palette rule')
assert.match(badges,/color:white/,'badge digits remain white')
assert.match(badges,/background:var\(--alarm-critical-color\)/,'badge uses the configured color, not darkened text color')
for(const color of ['#ff0000','#7c3aed','#e5484d']){
  applyAlarmColors(normalizeAlarmSoundSettings({levels:{CRITICAL:{color}}}))
  assert.equal(variables.get('--alarm-critical-color'),color)
  assert.notEqual(variables.get('--alarm-critical-text'),color,'readable dark text remains separate from badge background')
}
console.log('PASS: header/sidebar badges use original custom color with white digits; dark text palette is unchanged')
