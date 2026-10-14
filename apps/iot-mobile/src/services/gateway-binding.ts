import type { GatewayBindCredential } from '../types'

type NativeScanner = {
  scanQRCode: () => void
  portalMode?: () => string
  isNfcAvailable?: () => boolean
  prepareGatewayNfcWrite?: () => void
  writeGatewayNfc?: (payload: string) => void
  cancelNfcWrite?: () => void
  consumeGatewayNfc?: () => string
  hapticFeedback?: (type: 'success'|'error') => void
  showAlarmNotification?: (id: string, title: string, message: string) => void
  requestNotificationPermission?: () => void
}

declare global {
  interface Window {
    WeikongNative?: NativeScanner
    __weikongScanResult?: (value: string) => void
    __weikongScanError?: (message: string) => void
    __weikongNfcWriteSuccess?: () => void
    __weikongNfcWriteError?: (message: string) => void
    __weikongNfcRead?: (payload: string) => void
    __weikongOpenAlarms?: () => void
  }
}

const PENDING_NFC_KEY='weikongPendingGatewayNfc'

function normalizeCredential(snValue: unknown, codeValue: unknown): GatewayBindCredential | null {
  const sn = String(snValue ?? '').trim().toUpperCase()
  const bindCode = String(codeValue ?? '').trim()
  if (!/^[A-Z0-9_-]{6,32}$/.test(sn) || !/^\d{6}$/.test(bindCode)) return null
  return { sn, bindCode }
}

function normalizeNfcCredential(snValue:unknown,tokenValue:unknown):GatewayBindCredential|null{
  const sn=String(snValue??'').trim().toUpperCase()
  const nfcToken=String(tokenValue??'').trim()
  if(!/^[A-Z0-9_-]{6,32}$/.test(sn)||!/^[A-Za-z0-9_-]{24,128}$/.test(nfcToken))return null
  return{sn,nfcToken}
}

export function parseGatewayBindCode(rawValue: string): GatewayBindCredential {
  const raw = String(rawValue ?? '').trim()
  if (!raw) throw new Error('二维码内容为空')

  try {
    const data = JSON.parse(raw)
    if(data?.type==='weikong-gateway-bind'){
      const nfcParsed=normalizeNfcCredential(data?.sn??data?.gatewaySn,data?.nfcToken)
      if(nfcParsed)return nfcParsed
    }
    const parsed = normalizeCredential(data?.sn ?? data?.gatewaySn, data?.bindCode ?? data?.code)
    if (parsed) return parsed
  } catch {
    // 非 JSON 二维码继续按链接和交付文本解析。
  }

  try {
    const url = new URL(raw)
    const nfcParsed=normalizeNfcCredential(
      url.searchParams.get('sn')??url.searchParams.get('gatewaySn'),
      url.searchParams.get('nfcToken'),
    )
    if(nfcParsed)return nfcParsed
    const parsed = normalizeCredential(
      url.searchParams.get('sn') ?? url.searchParams.get('gatewaySn'),
      url.searchParams.get('bindCode') ?? url.searchParams.get('code'),
    )
    if (parsed) return parsed
  } catch {
    // 非 URL 二维码继续按交付文本解析。
  }

  const snMatch = raw.match(/(?:gateway\s*sn|网关\s*sn|\bsn)\s*[:：=]\s*([a-z0-9_-]{6,32})/i)
    ?? raw.match(/\b(WK[A-Z0-9_-]{6,30})\b/i)
  const codeMatch = raw.match(/(?:bind\s*code|bindCode|绑定码|交付码|\bcode)\s*[:：=]\s*(\d{6})/i)
    ?? raw.match(/(?:^|\D)(\d{6})(?:\D|$)/)
  const parsed = normalizeCredential(snMatch?.[1], codeMatch?.[1])
  if (parsed) return parsed

  throw new Error('未识别到有效的网关 SN 和 6 位绑定码')
}

export function nfcAvailable(){return Boolean(typeof window!=='undefined'&&window.WeikongNative?.isNfcAvailable?.())}

export function prepareGatewayNfcWrite(){window.WeikongNative?.prepareGatewayNfcWrite?.()}

export function nfcHapticFeedback(type:'success'|'error'){
  if(typeof window!=='undefined'&&window.WeikongNative?.hapticFeedback){window.WeikongNative.hapticFeedback(type);return}
  try{uni.vibrateShort({type:type==='error'?'heavy':'medium'})}catch{}
}

export function writeGatewayNfcTag(payload:string):Promise<void>{
  if(typeof window==='undefined'||!window.WeikongNative?.writeGatewayNfc)return Promise.reject(new Error('当前设备或版本不支持 NFC 写卡'))
  return new Promise((resolve,reject)=>{
    const cleanup=()=>{delete window.__weikongNfcWriteSuccess;delete window.__weikongNfcWriteError}
    window.__weikongNfcWriteSuccess=()=>{cleanup();resolve()}
    window.__weikongNfcWriteError=(message)=>{cleanup();reject(new Error(message||'NFC 写入失败'))}
    window.WeikongNative?.writeGatewayNfc?.(payload)
  })
}

export function cancelGatewayNfcWrite(){
  window.WeikongNative?.cancelNfcWrite?.()
  window.__weikongNfcWriteError?.('已取消 NFC 写入')
}

export function storePendingGatewayNfc(payload:string){uni.setStorageSync(PENDING_NFC_KEY,payload)}
export function takePendingGatewayNfc(){const value=String(uni.getStorageSync(PENDING_NFC_KEY)||'');if(value)uni.removeStorageSync(PENDING_NFC_KEY);return value}
export function hasPendingGatewayNfc(){return Boolean(uni.getStorageSync(PENDING_NFC_KEY))}
export function consumeNativeGatewayNfc(){return String(window.WeikongNative?.consumeGatewayNfc?.()||'')}

function scanWithNativeBridge(): Promise<string> | null {
  if (typeof window === 'undefined' || !window.WeikongNative?.scanQRCode) return null
  return new Promise((resolve, reject) => {
    window.__weikongScanResult = (value) => {
      cleanup()
      resolve(value)
    }
    window.__weikongScanError = (message) => {
      cleanup()
      reject(new Error(message || '扫码失败'))
    }
    const cleanup = () => {
      delete window.__weikongScanResult
      delete window.__weikongScanError
    }
    window.WeikongNative?.scanQRCode()
  })
}

export async function scanGatewayBindCode(): Promise<GatewayBindCredential> {
  const nativeResult = scanWithNativeBridge()
  if (nativeResult) return parseGatewayBindCode(await nativeResult)

  const result = await new Promise<UniApp.ScanCodeSuccessRes>((resolve, reject) => {
    uni.scanCode({
      onlyFromCamera: true,
      scanType: ['qrCode'],
      success: resolve,
      fail: (reason) => reject(new Error(reason?.errMsg?.includes('cancel') ? '已取消扫码' : '当前环境无法调用相机扫码，请手动输入绑定信息')),
    })
  })
  return parseGatewayBindCode(result.result)
}
