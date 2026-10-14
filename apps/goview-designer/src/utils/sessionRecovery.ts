import { reactive } from 'vue'
import { persistPlatformAuth, readPlatformToken, refreshPlatformSession } from './platformAuth'

export const sessionRecovery = reactive({ paused: false, visible: false, error: '', draftSaved: false, revision: 0, draftReady: false })
export const recoveryDraftKey = () => {
  const identity = recoveryIdentity()
  return `wk_recovery_draft:${identity.tenantId}:${identity.userId}:${sessionStorage.getItem('wk_Config_id')}`
}
let recovering: Promise<string> | null = null
let saveDraft: (() => boolean) | undefined
let resolveLogin: ((token: string) => void) | undefined

export function setRecoveryDraftSaver(saver: () => boolean) {
  saveDraft = saver
  if (sessionRecovery.paused) sessionRecovery.draftSaved = saver()
  return () => { saveDraft = undefined }
}
export function showRecoveryLogin() {
  sessionRecovery.draftSaved = saveDraft?.() || false
  sessionRecovery.error = ''
  sessionRecovery.visible = true
}

export function recoveryIdentity() {
  // 旧标签页没有独立租户字段时，读取签发令牌中的身份，用于恢复比较；
  // 最终身份仍由 /auth/profile 在服务端验证。
  let claims: any = {}
  try {
    const part = readPlatformToken().split('.')[1].replace(/-/g, '+').replace(/_/g, '/')
    claims = JSON.parse(atob(part))
  } catch { /* 缺失身份时不接受恢复凭据 */ }
  return {
    userId: sessionStorage.getItem('wk_UserId') || claims.sub || '',
    tenantId: sessionStorage.getItem('wk_TenantId') ?? claims.tenantId ?? ''
  }
}

export function recoveryPlatformOrigin() {
  return new URL(sessionStorage.getItem('wk_PlatformOrigin') || sessionStorage.getItem('wk_ApiBaseUrl') || window.location.origin).origin
}

export async function acceptRecoveredSession(token: string) {
  const expected = recoveryIdentity()
  const response = await fetch(`${sessionStorage.getItem('wk_ApiBaseUrl')}/auth/profile`, {
    headers: { Authorization: `Bearer ${token}` }
  })
  if (!response.ok) throw new Error('登录验证失败，请重新登录')
  const body = await response.json()
  const user = body.data || body
  if (!expected.userId || user.id !== expected.userId || (user.tenantId || '') !== expected.tenantId) {
    throw new Error('请使用原账号登录，当前画布属于原账号及租户')
  }
  persistPlatformAuth({ token, user_id: user.id, tenant_id: user.tenantId })
  sessionRecovery.paused = false
  sessionRecovery.visible = false
  sessionRecovery.error = ''
  sessionRecovery.revision++
  resolveLogin?.(token)
  resolveLogin = undefined
  window.dispatchEvent(new Event('wk-session-restored'))
}

export function recoverPlatformSession(alreadyExpired = false): Promise<string> {
  if (recovering) return recovering
  recovering = (async () => {
    try {
      if (alreadyExpired) throw new Error('会话已确认过期')
      const token = await refreshPlatformSession(true)
      sessionRecovery.revision++
      window.dispatchEvent(new Event('wk-session-restored'))
      return token
    } catch { /* 续期失败进入登录 */ }
    sessionRecovery.paused = true
    window.dispatchEvent(new Event('wk-session-paused'))
    showRecoveryLogin()
    return new Promise<string>(resolve => { resolveLogin = resolve })
  })().finally(() => { recovering = null })
  return recovering
}

export function waitForRecoveredSession() {
  return sessionRecovery.paused ? recoverPlatformSession() : Promise.resolve(readPlatformToken())
}
