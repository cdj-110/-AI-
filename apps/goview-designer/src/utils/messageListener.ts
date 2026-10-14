import { persistPlatformAuth, readPlatformToken, removePlatformAuthParamsFromAddress } from '@/utils/platformAuth'

let isInitialized = false
let authReadyPromise: Promise<void> | null = null

const bridgeParams = () => {
  const hashQuery = window.location.hash.includes('?') ? window.location.hash.split('?').slice(1).join('?') : ''
  const params = new URLSearchParams(hashQuery)
  return {
    enabled: params.get('auth_bridge') === '1',
    nonce: params.get('auth_nonce') || '',
    platformOrigin: params.get('platform_origin') || ''
  }
}

const trustedPlatformOrigin = (candidate: string) => {
  try {
    const url = new URL(candidate)
    return ['http:', 'https:'].includes(url.protocol) && url.hostname === window.location.hostname
      ? url.origin
      : ''
  } catch {
    return ''
  }
}

export function initMessageListener() {
  if (authReadyPromise) return authReadyPromise
  const bridge = bridgeParams()
  const allowedOrigin = trustedPlatformOrigin(bridge.platformOrigin)

  // 桥接参数只用于本次初始化。先保存到内存，再立即从地址栏移除，
  // 避免等待平台消息期间暴露 nonce、来源地址等内部参数。
  if (bridge.enabled || bridge.nonce || bridge.platformOrigin) removePlatformAuthParamsFromAddress()

  authReadyPromise = new Promise(resolve => {
    // 刷新已有项目时复用本标签页会话，不重复等待首次打开的一次性握手。
    // 只对 reload 且项目一致生效，首次打开或切换项目仍必须完成桥接。
    const navigation = performance.getEntriesByType('navigation')[0] as PerformanceNavigationTiming | undefined
    const projectId = /^#\/chart\/(?:home|preview)\/([^/?]+)/.exec(window.location.hash)?.[1]
    const restoreSession = navigation?.type === 'reload' && projectId
      && projectId === sessionStorage.getItem('wk_Config_id')
      && !!readPlatformToken() && !!sessionStorage.getItem('wk_ApiBaseUrl')
    if (restoreSession) {
      resolve()
      return
    }
    if (!bridge.enabled || !bridge.nonce || !allowedOrigin) {
      resolve()
      return
    }

    const finish = () => {
      window.clearTimeout(timeout)
      window.removeEventListener('message', receiveAuth)
      resolve()
    }
    const receiveAuth = (e: MessageEvent) => {
      if (e.origin !== allowedOrigin || e.source !== window.opener) return
      if (e.data?.type !== 'wk-platform-auth' || e.data?.nonce !== bridge.nonce) return
      persistPlatformAuth({ ...e.data, platform_origin: allowedOrigin })
      removePlatformAuthParamsFromAddress()
      finish()
    }
    const timeout = window.setTimeout(finish, 15_000)
    window.addEventListener('message', receiveAuth)

    window.opener?.postMessage({ type: 'wk-auth-ready', nonce: bridge.nonce }, allowedOrigin)
  })

  isInitialized = true
  return authReadyPromise
}

export function removeMessageListener() {
  isInitialized = false
  authReadyPromise = null
}
