import { defineStore } from 'pinia'
import { ref } from 'vue'

export const useConfigStore = defineStore('gateway-config', () => {
  const value = ref<Record<string, any>>({})
  const revision = ref('')
  const loading = ref(false)
  const largeMode = ref(false)
  const totalPoints = ref(0)
  const pointCounts = ref<Record<string, number>>({})

  function requireOK(response: Response) {
    if (response.status === 401) {
      window.location.assign('/login?next=' + encodeURIComponent(window.location.pathname + window.location.search + window.location.hash))
      throw new Error('登录已失效')
    }
    if (!response.ok) return response.text().then(text => { throw new Error(text.trim() || '配置读取失败') })
  }

  async function load() {
    loading.value = true
    try {
      const bootstrapResponse = await fetch('/api/config/bootstrap')
      await requireOK(bootstrapResponse)
      const bootstrap = await bootstrapResponse.json() as { content: string; pointCounts?: Record<string, number>; totalPoints?: number; largeMode?: boolean }
      revision.value = bootstrapResponse.headers.get('ETag') || ''
      pointCounts.value = bootstrap.pointCounts || {}
      totalPoints.value = Number(bootstrap.totalPoints || 0)
      largeMode.value = Boolean(bootstrap.largeMode)
      if (largeMode.value) {
        const data = JSON.parse(bootstrap.content)
        for (const device of [...(data.devices || []), ...(data.forwardDevices || [])]) device._pointCount = pointCounts.value[device.deviceKey] || 0
        value.value = data
        return value.value
      }
      const response = await fetch('/api/config')
      await requireOK(response)
      revision.value = response.headers.get('ETag') || ''
      const data = await response.json() as { content: string }
      value.value = JSON.parse(data.content)
      return value.value
    } finally { loading.value = false }
  }

  async function loadDevicePoints(deviceKey: string, offset = 0, limit = 200, search = '', functionCode = 0) {
    const query = new URLSearchParams({ deviceKey, offset: String(offset), limit: String(limit) })
    if (search) query.set('search', search)
    if (functionCode) query.set('function', String(functionCode))
    const response = await fetch('/api/config/points?' + query)
    await requireOK(response)
    return await response.json() as { offset: number; limit: number; total: number; points: any[] }
  }

  async function mutateDevicePoints(payload: Record<string, any>) {
    const previousCount = payload.deviceKey ? Number(pointCounts.value[payload.deviceKey] || 0) : 0
    const response = await fetch('/api/config/point-mutations', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', ...(revision.value ? { 'If-Match': revision.value } : {}) },
      body: JSON.stringify(payload),
    })
    await requireOK(response)
    revision.value = response.headers.get('ETag') || revision.value
    const result = await response.json() as { ok: boolean; count: number; deleted?: number; createdMetrics?: string[] }
    if (payload.deviceKey) {
      pointCounts.value[payload.deviceKey] = result.count
      totalPoints.value = Math.max(0, totalPoints.value - previousCount + result.count)
      const device = [...(value.value.devices || []), ...(value.value.forwardDevices || [])].find((item: any) => item.deviceKey === payload.deviceKey)
      if (device) device._pointCount = result.count
    }
    return result
  }

  async function save(next: Record<string, any> = value.value) {
    const response = await fetch(largeMode.value ? '/api/config/compact' : '/api/config', {
      method: 'PUT',
      // Send the configuration once instead of wrapping an already serialized
      // document in another JSON string. Large point projects otherwise double
      // allocation and can exceed the request limit after a bulk copy.
      headers: { 'Content-Type': 'application/vnd.weikong.config+json', ...(revision.value ? { 'If-Match': revision.value } : {}) },
      body: JSON.stringify(next),
    })
    if (!response.ok) throw new Error((await response.text()).trim() || '配置保存失败')
    revision.value = response.headers.get('ETag') || revision.value
    value.value = next
    return await response.json() as { ok: boolean; restartRequired: boolean }
  }

  return { value, revision, loading, largeMode, totalPoints, pointCounts, load, loadDevicePoints, mutateDevicePoints, save }
})
