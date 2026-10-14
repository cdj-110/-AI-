import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'

const source = readFileSync(new URL('../src/components/EdgeComputeView.vue', import.meta.url), 'utf8')

assert.match(source, /usePageDraftGuard\([^\n]+,'\.edge-group, \.edge-points button:not\(\.edge-add-point\)'\)/)
assert.doesNotMatch(source, /usePageDraftGuard\([^\n]+\.edge-editor-tabs button/)
assert.match(source, /@click="selectPanel\('logic'\)"/)
assert.match(source, /function selectPanel\(value: 'basic' \| 'logic'\)/)

console.log('Edge-compute tabs stay inside the active draft without invoking the unsaved-change guard.')
