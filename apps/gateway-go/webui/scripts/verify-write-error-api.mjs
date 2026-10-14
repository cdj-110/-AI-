import assert from 'node:assert/strict'
import { ApiError, api } from '../src/api.ts'

globalThis.window = {
  setTimeout,
  clearTimeout,
  location: { assign() {} },
}

const raw = 'OPC DA int8 write value "151" is invalid or out of range: strconv.ParseInt: parsing "151": value out of range'
globalThis.fetch = async () => new Response(JSON.stringify({
  ok: false,
  code: 'opcda_integer_out_of_range',
  message: '写入失败：目标数据类型为 int8，有效范围是 -128～127，输入值 151 超出范围，请修改后重试。',
  detail: raw,
  targetType: 'int8',
  validRange: '-128～127',
}), { status: 502, headers: { 'Content-Type': 'application/json' } })

await assert.rejects(
  api('/api/points/write', { method: 'POST' }),
  error => {
    assert.ok(error instanceof ApiError)
    assert.equal(error.code, 'opcda_integer_out_of_range')
    assert.match(error.message, /-128～127/)
    assert.equal(error.detail, raw)
    assert.equal(error.targetType, 'int8')
    assert.equal(error.validRange, '-128～127')
    return true
  },
)

globalThis.fetch = async () => new Response(JSON.stringify({ ok: true, value: '13' }), {
  status: 200,
  headers: { 'Content-Type': 'application/json' },
})
assert.deepEqual(await api('/api/points/write', { method: 'POST' }), { ok: true, value: '13' })

console.log('write error API mapping: ok')
