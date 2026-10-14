import assert from 'node:assert/strict'
import {readFileSync} from 'node:fs'
const component=readFileSync(new URL('../src/components/AlarmNotificationCard.vue',import.meta.url),'utf8')
const animation=component.match(/@keyframes alarm-critical-pulse\{.*\}/)?.[0]
assert.ok(animation)
assert.equal((animation.match(/border-left-color:var\(--alarm-critical-color,/g)||[]).length,1)
assert.match(animation,/border-left-color:var\(--alarm-critical-pale,/)
assert.match(component,/border-width:2px 2px 2px 3px;animation:alarm-critical-pulse 2s steps\(1,end\) infinite/)
assert.ok(!animation.includes('--alarm-critical-text'),'darkened text color must never become a border')
assert.match(animation,/0%,100%\{border-color:var\(--alarm-critical-pale,/)
assert.match(animation,/50%\{border-color:var\(--alarm-critical-color,/)
assert.ok(!/opacity|background|transform/.test(animation),'only borders should flash')
assert.match(component,/@media\(prefers-reduced-motion:reduce\).*animation:none/)
console.log('PASS: entire border visibly flashes pale/original, no dark border or content animation, reduced-motion preserved')
