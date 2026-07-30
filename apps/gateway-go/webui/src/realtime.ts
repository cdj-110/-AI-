import type { PointStatus } from './types'

export interface GatewayRealtimeMessage<T = unknown> {
  type: string
  sequence: number
  timestamp: string
  payload: T
}

export interface PointDiff {
  replace?: boolean
  points?: PointStatus[]
}

export function mergePointDiff(current: PointStatus[], diff: PointDiff): PointStatus[] {
  const incoming = Array.isArray(diff.points) ? diff.points : []
  if (diff.replace) return incoming
  if (!incoming.length) return current
  const next = new Map(current.map(point => [`${point.deviceKey}::${point.metric}`, point]))
  for (const point of incoming) next.set(`${point.deviceKey}::${point.metric}`, point)
  return [...next.values()]
}

export function openGatewayRealtime(
  onMessage: (message: GatewayRealtimeMessage<any>) => void,
  onConnectionChange?: (connected: boolean) => void,
) {
  let socket: WebSocket | undefined
  let reconnectTimer = 0
  let reconnectDelay = 1000
  let stopped = false
  let pointFlushTimer = 0
  let pendingPointMessage: GatewayRealtimeMessage<PointDiff> | undefined
  const pendingPoints = new Map<string, PointStatus>()

  const flushPointDiff = () => {
    window.clearTimeout(pointFlushTimer)
    pointFlushTimer = 0
    if (!pendingPointMessage || !pendingPoints.size) return
    onMessage({ ...pendingPointMessage, payload: { points: [...pendingPoints.values()] } })
    pendingPointMessage = undefined
    pendingPoints.clear()
  }

  const dispatch = (message: GatewayRealtimeMessage<any>) => {
    if (message.type !== 'points.diff' || message.payload?.replace) {
      flushPointDiff()
      onMessage(message)
      return
    }
    const points = Array.isArray(message.payload?.points) ? message.payload.points as PointStatus[] : []
    for (const point of points) pendingPoints.set(`${point.deviceKey}::${point.metric}`, point)
    pendingPointMessage = message
    if (!pointFlushTimer) pointFlushTimer = window.setTimeout(flushPointDiff, 160)
  }

  const connect = () => {
    if (stopped) return
    const scheme = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    socket = new WebSocket(`${scheme}//${window.location.host}/api/ws`)
    socket.addEventListener('open', () => {
      reconnectDelay = 1000
      onConnectionChange?.(true)
    })
    socket.addEventListener('message', event => {
      try { dispatch(JSON.parse(String(event.data))) } catch { /* ignore malformed frames */ }
    })
    socket.addEventListener('close', () => {
      onConnectionChange?.(false)
      if (stopped) return
      window.clearTimeout(reconnectTimer)
      reconnectTimer = window.setTimeout(connect, reconnectDelay)
      reconnectDelay = Math.min(reconnectDelay * 2, 10000)
    })
    socket.addEventListener('error', () => socket?.close())
  }

  connect()
  return () => {
    stopped = true
    window.clearTimeout(reconnectTimer)
    window.clearTimeout(pointFlushTimer)
    pendingPoints.clear()
    pendingPointMessage = undefined
    socket?.close()
    socket = undefined
  }
}
