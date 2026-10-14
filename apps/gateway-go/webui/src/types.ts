export interface PointStatus {
  deviceKey: string
  name: string
  metric: string
  protocol: string
  address: string
  unit?: string
  decimals?: number
  value: unknown
  valueText?: string
  actualDataType?: string
  updatedAt?: string
  quality?: 'GOOD' | 'UNCERTAIN' | 'BAD' | 'OFFLINE'
  rawQuality?: number
  sourceTimestamp?: string
  receiveTimestamp?: string
  collectionMode?: 'subscription' | 'sync' | 'sync_fallback' | string
  diagnosticCode?: number
  diagnostic?: string
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
  secureChipIdentity?: { usid?: string; available: boolean; message?: string }
  license?: { valid: boolean; restricted: boolean; reason?: string }
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

export interface CollectionIncident {
  id: number
  deviceKey: string
  pointName: string
  metric: string
  protocol: string
  address: string
  reason: string
  explanation: string
  state: 'ACTIVE' | 'RECOVERED' | 'CHANGED' | 'INTERRUPTED'
  occurrences: number
  occurredAt: string
  lastSeenAt: string
  recoveredAt?: string
}

export interface CollectionIncidentPage {
  items: CollectionIncident[]
  page: number
  pageSize: number
  total: number
  totalPages: number
  dropped?: number
  path?: string
  lastError?: string
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
