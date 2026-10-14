import assert from 'node:assert/strict'
import { applySerialBusProtocol, serialBusProtocol } from '../src/serialBusProtocol.ts'

const channels = [
  { channelKey: 'channel-a', role: 'collect', protocol: 'modbus-rtu' },
  { channelKey: 'channel-b', role: 'collect', protocol: 'dlt645' },
]
assert.equal(serialBusProtocol(channels), 'mixed')
assert.equal(serialBusProtocol([{ channelKey: 'invalid', protocol: 'opcua' }]), 'mixed')

const devices = [
  { channelKey: 'channel-a', protocol: 'modbus-rtu', points: [{ metric: 'mbrtu-P01' }] },
  { channelKey: 'channel-b', protocol: 'dlt645', points: [], dlt645: { meterAddress: '010000000000' } },
]
const result = applySerialBusProtocol(channels, devices, 'dlt645', { name: 'RS485-2', port: '/dev/ttyS2' })

assert.equal(serialBusProtocol(channels), 'dlt645')
assert.equal(result.changedDevices, 1)
assert.equal(result.removedPoints, 1)
assert.deepEqual(devices[0].points, [])
assert.equal(devices[0].protocol, 'dlt645')
assert.equal(devices[0].address, '/dev/ttyS2')
assert.equal(devices[0].dlt645?.meterAddress, '000000000001')
assert.equal(devices[1].dlt645?.meterAddress, '010000000000')

console.log('serial bus protocol checks passed')
