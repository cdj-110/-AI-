import assert from 'node:assert/strict'
import { normalizeOPCDAReadMode } from '../src/opcdaConfig.ts'

assert.equal(normalizeOPCDAReadMode('subscription'), 'subscription')
assert.equal(normalizeOPCDAReadMode('sync'), 'sync')
assert.equal(normalizeOPCDAReadMode(''), 'sync')
assert.equal(normalizeOPCDAReadMode(undefined), 'sync')
assert.equal(normalizeOPCDAReadMode('invalid'), 'sync')

console.log('OPC DA read mode normalization: ok')
