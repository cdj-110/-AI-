export type UserInfo = {
  id: string
  username: string
  nickname?: string
  role: 'SUPER_ADMIN' | 'TENANT_ADMIN' | 'TENANT_USER'
  tenantId: string | null
}

export type Device = {
  id: string
  tenantId: string
  deviceKey: string
  name: string
  protocol: string
  deviceType: 'GATEWAY' | 'GATEWAY_CHILD' | 'DIRECT'
  status: string
  location?: string
  lastSeenAt?: string
  gatewayId?: string
  gateway?: { id:string; name:string; deviceKey:string; status?:string }
  tenant?: { id:string; name:string }
}

export type DeviceMetric = {
  id: string
  identifier: string
  name: string
  dataType: string
  unit?: string
  decimals?: number
  enabled?: boolean
  ignored?: boolean
  sortOrder?: number
}

export type DeviceCommandPoint = {
  identifier: string
  name: string
  dataType: 'NUMBER' | 'BOOLEAN' | 'STRING' | string
  unit?: string
}

export type DeviceCommandCapabilities = {
  points: DeviceCommandPoint[]
  available: boolean
  reason?: string
  gatewayId?: string
}

export type DeviceCommandTask = {
  id: string
  requestId: string
  metric: string
  value: unknown
  status: 'SENT' | 'SUCCEEDED' | 'FAILED' | 'TIMEOUT' | string
  message?: string
  createdAt: string
  completedAt?: string
}

export type TelemetryItem = {
  time: string
  deviceKey?: string
  metrics: Record<string, unknown>
}

export type Alarm = {
  id: string
  deviceId: string
  deviceName?: string
  type: string
  level: 'INFO' | 'WARNING' | 'CRITICAL' | string
  message: string
  status: 'OPEN' | 'RESOLVED' | string
  value?: number
  threshold?: number
  createdAt: string
  resolvedAt?: string
  notificationRead?: boolean
  device?: { id:string; name:string; deviceKey:string }
}

export type GatewayOption = { id:string; name:string; deviceKey:string }
export type ModelTemplate = { id:string; name:string; deviceType:string; _count?:{metrics:number} }

export type GatewayBindCredential = {
  sn: string
  bindCode?: string
  nfcToken?: string
}

export type GatewayNfcCredential = FactoryGateway & {
  nfcToken: string
  payload: string
}

export type GatewayBindVerification = {
  sn: string
  batchNo?: string
  producedAt?: string
  firstSeenAt?: string
  lastSeenAt?: string
  online: boolean
  suggestedName: string
}

export type GatewayBindResult = {
  factoryGateway: {
    id: string
    sn: string
    status: string
  }
  device: Device
}

export type Tenant = {
  id: string
  name: string
  description?: string
  _count?: { users: number; devices: number }
}

export type FactoryGateway = {
  id: string
  sn: string
  hardwareId: string
  batchNo?: string
  status: string
  producedAt?: string
  firstSeenAt?: string
  lastSeenAt?: string
  boundTenant?: { id:string; name:string }
  boundDevice?: { id:string; name:string; deviceKey:string; status?:string }
}
