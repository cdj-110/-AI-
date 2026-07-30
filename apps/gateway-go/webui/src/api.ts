export type ApiOptions = RequestInit & { timeoutMs?: number }

export async function api<T>(path: string, options: ApiOptions = {}): Promise<T> {
  const controller = new AbortController()
  const timer = window.setTimeout(() => controller.abort(), options.timeoutMs ?? 20000)
  try {
    const response = await fetch(path, { ...options, signal: controller.signal })
    if (response.status === 401) {
      window.location.assign('/login?next=' + encodeURIComponent(window.location.pathname + window.location.search + window.location.hash))
      throw new Error('登录已失效')
    }
    if (!response.ok) throw new Error((await response.text()).trim() || `请求失败 (${response.status})`)
    return await response.json() as T
  } finally {
    window.clearTimeout(timer)
  }
}

export async function loadGatewayConfig(): Promise<Record<string, unknown>> {
  const data = await api<{ content: string }>('/api/config')
  return JSON.parse(data.content) as Record<string, unknown>
}
