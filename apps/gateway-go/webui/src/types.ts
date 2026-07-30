export interface PointStatus {
  deviceKey: string
  name: string
  metric: string
  protocol: string
  address: string
  unit?: string
  value: unknown
  updatedAt?: string
  error?: string
  stale?: boolean
}

export interface GatewayStatus {
  gatewayKey: string
  uptimeSeconds: number
  collectSeconds: number
  collectMilliseconds?: number
  mqttEnabled: boolean
  mqttConnected: boolean
  mqttChannels: Record<string, { enabled: boolean; connected: boolean; name?: string }>
  lastCollectAt?: string
  lastPublishAt?: string
  hardwareIdentity?: { id: string; source: string; available: boolean; message?: string }
  pointCount: number
  healthyCount: number
  pendingCount: number
  staleCount: number
  errorCount: number
  systemMetrics?: {
    cpu: MetricValue
    memory: MetricValue
    storage: MetricValue
  }
  processMetrics?: {
    memoryBytes: number
    diskBytes: number
    diskPath?: string
  }
  points: PointStatus[]
  errors: Array<{ time: string; level: string; message: string }>
}

export interface MetricValue {
  usedPercent: number
  usedBytes?: number
  totalBytes?: number
}

export interface AuditEvent {
  timestamp: string
  username?: string
  remoteIp?: string
  method: string
  path: string
  status: number
  durationMs?: number
  result: string
  detail?: string
}
