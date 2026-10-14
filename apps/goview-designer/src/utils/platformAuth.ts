type PlatformAuthPayload = {
  token?: string
  userId?: string
  user_id?: string
  config_id?: string
  configId?: string
  config_name?: string
  configName?: string
  api_base?: string
  apiBase?: string
  platform_origin?: string
  tenant_id?: string | null
}

type PlatformActivityResponse = {
  accessToken?: string
  idleTimeoutSeconds?: number
  absoluteExpiresAt?: string
}

const activityEvents = ['pointerdown', 'keydown', 'touchstart', 'wheel'] as const
const RENEW_THROTTLE_MS = 60_000
let lastRenewedAt = 0
let renewing: Promise<string> | null = null
let renewalInitialized = false
let renewalPaused = false

const readParams = () => {
  const params = new URLSearchParams(window.location.search)
  const hashQuery = window.location.hash.includes('?') ? window.location.hash.split('?').slice(1).join('?') : ''
  const hashParams = new URLSearchParams(hashQuery)

  hashParams.forEach((value, key) => {
    if (!params.has(key)) params.set(key, value)
  })

  return params
}

export const removePlatformAuthParamsFromAddress = () => {
  const url = new URL(window.location.href)
  const cleanParams = (params: URLSearchParams) => {
    ;[
      'token',
      'user_id',
      'userId',
      'config_id',
      'configId',
      'config_name',
      'configName',
      'api_base',
      'apiBase',
      'auth_bridge',
      'auth_nonce',
      'platform_origin'
    ].forEach(key => params.delete(key))
  }

  cleanParams(url.searchParams)
  if (url.hash.includes('?')) {
    const [hashPath, hashQuery = ''] = url.hash.split('?')
    const hashParams = new URLSearchParams(hashQuery)
    cleanParams(hashParams)
    const query = hashParams.toString()
    url.hash = query ? `${hashPath}?${query}` : hashPath
  }
  window.history.replaceState(window.history.state, document.title, url.toString())
}

export const persistPlatformAuth = (payload: PlatformAuthPayload = {}) => {
  const token = payload.token || ''
  const userId = payload.user_id || payload.userId || ''
  const configId = payload.config_id || payload.configId || ''
  const configName = payload.config_name || payload.configName || ''
  const apiBase = payload.api_base || payload.apiBase || ''

  if (token) sessionStorage.setItem('wk_Token', JSON.stringify({ token }))
  if (userId) sessionStorage.setItem('wk_UserId', userId)
  if (configId) sessionStorage.setItem('wk_Config_id', configId)
  if (configName) sessionStorage.setItem('wk_Config_name', configName)
  if (apiBase) sessionStorage.setItem('wk_ApiBaseUrl', apiBase.replace(/\/$/, ''))
  if (payload.platform_origin) sessionStorage.setItem('wk_PlatformOrigin', new URL(payload.platform_origin).origin)
  if (payload.tenant_id !== undefined) sessionStorage.setItem('wk_TenantId', payload.tenant_id || '')
}

export const readPlatformToken = () => {
  const stored = sessionStorage.getItem('wk_Token')
  if (!stored) return ''
  try {
    return JSON.parse(stored)?.token || ''
  } catch {
    return ''
  }
}

const platformApiBase = () => (sessionStorage.getItem('wk_ApiBaseUrl') || '').replace(/\/$/, '')

export const refreshPlatformSession = (force = false) => {
  const token = readPlatformToken()
  const apiBase = platformApiBase()
  if (!token || !apiBase) return Promise.reject(new Error('平台登录信息不完整'))
  if (!force && Date.now() - lastRenewedAt < RENEW_THROTTLE_MS) return Promise.resolve(token)
  if (renewing) return renewing

  renewing = fetch(`${apiBase}/auth/activity`, {
    method: 'POST',
    headers: {
      Authorization: `Bearer ${token}`,
      'Content-Type': 'application/json'
    },
    body: '{}'
  }).then(async response => {
    if (!response.ok) throw Object.assign(new Error(`会话续期失败 (${response.status})`), { status: response.status })
    const body = await response.json().catch(() => ({}))
    const result = (body?.data || body) as PlatformActivityResponse
    if (!result.accessToken) throw new Error('会话续期未返回新令牌')
    persistPlatformAuth({ token: result.accessToken })
    if (result.idleTimeoutSeconds) sessionStorage.setItem('wk_IdleTimeoutSeconds', String(result.idleTimeoutSeconds))
    if (result.absoluteExpiresAt) sessionStorage.setItem('wk_AbsoluteExpiresAt', result.absoluteExpiresAt)
    sessionStorage.setItem('wk_LastActivityAt', String(Date.now()))
    lastRenewedAt = Date.now()
    return result.accessToken
  }).finally(() => {
    renewing = null
  })

  return renewing
}

export const initPlatformSessionRenewal = () => {
  if (renewalInitialized || !readPlatformToken() || !platformApiBase()) return
  renewalInitialized = true

  const renewFromActivity = (event?: Event) => {
    if (renewalPaused) return
    if (event && !event.isTrusted) return
    sessionStorage.setItem('wk_LastActivityAt', String(Date.now()))
    void refreshPlatformSession().catch(() => undefined)
  }

  activityEvents.forEach(eventName => window.addEventListener(eventName, renewFromActivity, { passive: true }))
  window.addEventListener('wk-session-paused', () => { renewalPaused = true })
  window.addEventListener('wk-session-restored', () => { renewalPaused = false })
  window.addEventListener('focus', () => renewFromActivity())
  document.addEventListener('visibilitychange', () => {
    if (document.visibilityState === 'visible') renewFromActivity()
  })

  // 页面刚打开时立即换取一枚新令牌，避免带入已使用较久的旧令牌。
  void refreshPlatformSession(true).catch(error => {
    if (error?.status === 401) {
      void import('./sessionRecovery').then(module => module.recoverPlatformSession(true))
    }
  })
}

export const initPlatformAuthFromUrl = () => {
  const params = readParams()

  persistPlatformAuth({
    token: params.get('token') || '',
    user_id: params.get('user_id') || params.get('userId') || '',
    config_id: params.get('config_id') || params.get('configId') || '',
    config_name: params.get('config_name') || params.get('configName') || '',
    api_base: params.get('api_base') || params.get('apiBase') || ''
  })

  // 兼容旧链接：完成一次读取后立即清理敏感参数，避免进入浏览器历史和复制链接。
  if (params.has('token')) removePlatformAuthParamsFromAddress()
}
