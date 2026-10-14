import { onBeforeUnmount, watch } from 'vue'
import { recoverPlatformSession, sessionRecovery } from './sessionRecovery'

type TelemetryPayload = {
  deviceId?: string
  time?: string
  metrics?: Record<string, unknown>
}

// 默认 500px 宽的柱状图显示 6 组数据；每增加一个柱位宽度，多保留 1 组。
const DEFAULT_CHART_WIDTH = 500
const DEFAULT_REALTIME_DATA_WINDOW_SIZE = 6
const MAX_REALTIME_DATA_WINDOW_SIZE = 60
const CHINA_TIME_FORMATTER = new Intl.DateTimeFormat('zh-CN', {
  timeZone: 'Asia/Shanghai',
  month: '2-digit',
  day: '2-digit',
  hour: '2-digit',
  minute: '2-digit',
  second: '2-digit',
  hourCycle: 'h23',
})

const tokenValue = () => {
  const stored = sessionStorage.getItem('wk_Token')
  if (!stored) return ''
  try {
    return JSON.parse(stored)?.token || ''
  } catch {
    return ''
  }
}

const apiBase = () => (sessionStorage.getItem('wk_ApiBaseUrl') || '').replace(/\/$/, '')

const bindingsOf = (component: any) => {
  if (component?.request?.requestHttpType !== '实时数据') return []
  const params = component?.request?.requestParams?.Params
  return Array.isArray(params)
    ? params.filter((item: any) => item?.value1 && item?.value2)
    : []
}

const toValue = (value: unknown) => {
  if (typeof value === 'number') return value
  const parsed = Number(value)
  return Number.isFinite(parsed) ? parsed : value
}

const formatTelemetryTime = (value?: string) => {
  const date = value ? new Date(value) : new Date()
  if (Number.isNaN(date.getTime())) return String(value || '')
  const parts = Object.fromEntries(CHINA_TIME_FORMATTER.formatToParts(date).map(part => [part.type, part.value]))
  return `${parts.month}-${parts.day} ${parts.hour}:${parts.minute}:${parts.second}`
}

const realtimeDataWindowSize = (component: any) => {
  const series = Array.isArray(component?.option?.series) ? component.option.series : []
  const isBarChart = series.some((item: any) => item?.type === 'bar')
  if (!isBarChart) return DEFAULT_REALTIME_DATA_WINDOW_SIZE

  const width = Number(component?.attr?.w)
  if (!Number.isFinite(width) || width <= 0) return DEFAULT_REALTIME_DATA_WINDOW_SIZE
  return Math.min(
    MAX_REALTIME_DATA_WINDOW_SIZE,
    Math.max(1, Math.floor((width * DEFAULT_REALTIME_DATA_WINDOW_SIZE) / DEFAULT_CHART_WIDTH)),
  )
}

const updateComponents = (components: any[], payload: TelemetryPayload) => {
  if (!payload.deviceId || !payload.metrics) return
  const timestamp = formatTelemetryTime(payload.time)

  components.forEach(component => {
    const bindings = bindingsOf(component).filter((binding: any) => binding.value1 === payload.deviceId)
    if (!bindings.length || !component?.option) return

    const dataset = component.option.dataset
    const firstMetric = bindings[0]?.value2
    // 开关必须保留点位原始类型：false 不能变成 0，文本 '1' 不能变成数字 1。
    // 即使上次值为 null，也继续接收后续有效值，不落入图表 dataset 分支。
    if (component.chartConfig?.key === 'SwitchFun') {
      if (firstMetric in payload.metrics!) component.option.dataset = payload.metrics![firstMetric]
      return
    }
    if (typeof dataset === 'number' || typeof dataset === 'string' || typeof dataset === 'boolean' || component.chartConfig?.key === 'CirclePoint') {
      if (firstMetric in payload.metrics!) component.option.dataset = toValue(payload.metrics![firstMetric])
      return
    }

    const allBindings = bindingsOf(component)
    const metricNames = [...new Set<string>(allBindings.map((binding: any) => String(binding.value2)))]
    const row: Record<string, unknown> = { timestamp }
    metricNames.forEach(metric => {
      if (metric in payload.metrics!) row[metric] = toValue(payload.metrics![metric])
    })
    if (Object.keys(row).length === 1) return

    const current = dataset && typeof dataset === 'object' ? dataset : {}
    const source = Array.isArray(current.source) ? current.source : []
    const windowSize = realtimeDataWindowSize(component)
    component.option.dataset = {
      ...current,
      dimensions: ['timestamp', ...metricNames],
      source: [...source, row].slice(-windowSize),
    }
  })
}

export const usePlatformTelemetry = (components: () => any[]) => {
  const flattenComponents = (items: any[]): any[] => items.flatMap(item =>
    Array.isArray(item?.groupList) ? flattenComponents(item.groupList) : [item]
  )
  const boundComponents = () => flattenComponents(components())
  const sockets = new Map<string, WebSocket>()
  const reconnectTimers = new Map<string, ReturnType<typeof setTimeout>>()
  let stopped = false
  let generation = 0

  const closeAll = () => {
    generation++
    sockets.forEach(socket => socket.close())
    sockets.clear()
    reconnectTimers.forEach(timer => clearTimeout(timer))
    reconnectTimers.clear()
  }

  const loadLatest = async (deviceId: string) => {
    if (stopped || sessionRecovery.paused) return
    const startedGeneration = generation
    const base = apiBase()
    const token = tokenValue()
    if (!base || !token) return
    try {
      const response = await fetch(`${base}/devices/${encodeURIComponent(deviceId)}/telemetry`, {
        headers: { Authorization: `Bearer ${token}` },
      })
      if (response.status === 401) { void recoverPlatformSession(); return }
      if (!response.ok || stopped || sessionRecovery.paused || startedGeneration !== generation) return
      const body = await response.json()
      if (stopped || sessionRecovery.paused || startedGeneration !== generation) return
      const latest = body?.data?.items?.[0]
      if (latest) updateComponents(boundComponents(), latest)
    } catch {
      // WebSocket will continue retrying; an unavailable initial snapshot is non-fatal.
    }
  }

  const connect = (deviceId: string) => {
    const base = apiBase()
    const token = tokenValue()
    if (stopped || sessionRecovery.paused || !base || !token) return
    const url = new URL(base)
    url.protocol = url.protocol === 'https:' ? 'wss:' : 'ws:'
    url.pathname = '/api/realtime'
    url.search = ''
    const socket = new WebSocket(url.toString())
    sockets.set(deviceId, socket)
    socket.addEventListener('open', () => {
      socket.send(JSON.stringify({ type: 'auth', token }))
    })
    socket.addEventListener('message', event => {
      try {
        const message = JSON.parse(String(event.data))
        if (message.type === 'auth.ok') {
          socket.send(JSON.stringify({ type: 'telemetry.subscribe', deviceId }))
        } else if (message.type === 'telemetry.update') {
          if (!sessionRecovery.paused) updateComponents(boundComponents(), message.payload || {})
        } else if (message.type === 'auth.error') {
          void recoverPlatformSession()
        }
      } catch {
        // Ignore malformed frames; the authenticated stream remains usable.
      }
    })
    socket.addEventListener('close', event => {
      if (sockets.get(deviceId) !== socket) return
      sockets.delete(deviceId)
      if (!stopped && !sessionRecovery.paused) {
        if (event.code === 4001) void recoverPlatformSession()
        else reconnectTimers.set(deviceId, setTimeout(() => connect(deviceId), 3000))
      }
    })
  }

  const bindingSignature = () => JSON.stringify(
    boundComponents().flatMap(component => bindingsOf(component).map((binding: any) => [binding.value1, binding.value2]))
  )

  const restart = () => {
    closeAll()
    if (sessionRecovery.paused) return
    const deviceIds = [...new Set(boundComponents().flatMap(component => bindingsOf(component).map((binding: any) => binding.value1)))] as string[]
    deviceIds.forEach(deviceId => {
      void loadLatest(deviceId)
      connect(deviceId)
    })
  }
  watch([bindingSignature, () => sessionRecovery.paused, () => sessionRecovery.revision], restart, { immediate: true })

  onBeforeUnmount(() => {
    stopped = true
    closeAll()
  })
}
