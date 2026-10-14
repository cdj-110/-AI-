export const SERIAL_BUS_PROTOCOLS = ['modbus-rtu', 'dlt645'] as const

export type SerialBusProtocol = (typeof SERIAL_BUS_PROTOCOLS)[number]

type ChannelLike = {
  channelKey?: string
  role?: string
  protocol?: string
}

type DeviceLike = {
  channelKey?: string
  protocol?: string
  interfaceType?: string
  interfaceName?: string
  address?: string
  slaveId?: number
  dlt645?: Record<string, unknown>
  points?: Array<Record<string, unknown>>
}

export function isSerialBusProtocol(value: unknown): value is SerialBusProtocol {
  return SERIAL_BUS_PROTOCOLS.includes(String(value).toLowerCase() as SerialBusProtocol)
}

export function serialBusProtocol(channels: ChannelLike[]): SerialBusProtocol | 'mixed' {
  const configured = channels
    .filter(channel => channel.role !== 'forward')
    .map(channel => String(channel.protocol || 'modbus-rtu').toLowerCase())
  if (configured.some(protocol => !isSerialBusProtocol(protocol))) return 'mixed'
  const protocols = new Set(configured)
  if (protocols.size > 1) return 'mixed'
  const [protocol] = protocols
  return isSerialBusProtocol(protocol) ? protocol : 'modbus-rtu'
}

export function applySerialBusProtocol(
  channels: ChannelLike[],
  devices: DeviceLike[],
  protocol: SerialBusProtocol,
  serial: { name?: string; port?: string } = {},
) {
  const channelKeys = new Set<string>()
  for (const channel of channels) {
    if (channel.role === 'forward') continue
    channel.protocol = protocol
    if (channel.channelKey) channelKeys.add(channel.channelKey)
  }

  let changedDevices = 0
  let removedPoints = 0
  for (const device of devices) {
    if (!device.channelKey || !channelKeys.has(device.channelKey)) continue
    const changed = String(device.protocol || '').toLowerCase() !== protocol
    if (changed) {
      changedDevices++
      removedPoints += device.points?.length || 0
      device.points = []
    }
    device.protocol = protocol
    device.interfaceType = 'serial'
    device.interfaceName = serial.name || device.interfaceName || ''
    device.address = serial.port || device.address || ''
    if (protocol === 'dlt645') {
      device.dlt645 = {
        version: 'auto',
        meterAddress: '000000000001',
        responseTimeoutMilliseconds: 1500,
        retries: 1,
        wakeupBytes: 4,
        writePassword: '',
        operatorCode: '',
        ...(device.dlt645 || {}),
      }
    } else {
      delete device.dlt645
      if (!device.slaveId) device.slaveId = 1
    }
  }
  return { changedDevices, removedPoints }
}
