import type { UserInfo } from '../types'

const TOKEN_KEY='iotCloudAccessToken'
const USER_KEY='iotCloudUser'
const SERVER_KEY='iotCloudServer'
const PORTAL_KEY='iotCloudPortal'
const fallbackServer=String(import.meta.env.VITE_API_BASE_URL||'http://127.0.0.1:3100').replace(/\/+$/,'')
function nativePortal(){
  if(typeof window==='undefined')return ''
  try{return String(window.WeikongNative?.portalMode?.()||'').toLowerCase()}catch{return ''}
}
const buildPortal=((nativePortal()||String(import.meta.env.VITE_APP_PORTAL||'tenant'))==='admin'?'admin':'tenant') as 'tenant'|'admin'

export function token(){return String(uni.getStorageSync(TOKEN_KEY)||'')}
export function currentUser(){return (uni.getStorageSync(USER_KEY)||null) as UserInfo|null}
export function serverURL(){
  const saved=String(uni.getStorageSync(SERVER_KEY)||'').trim().replace(/\/+$/,'')
  // A phone's loopback address points to the phone itself. Older test builds
  // persisted this default, so ignore it when running inside the native shell.
  if(nativePortal()&&/^https?:\/\/(?:127\.0\.0\.1|localhost)(?::|\/|$)/i.test(saved))return fallbackServer
  return saved||fallbackServer
}
export function defaultPortal(){return buildPortal}
export function isAdminBuild(){return buildPortal==='admin'}
export function portalMode(){return String(uni.getStorageSync(PORTAL_KEY)||buildPortal) as 'tenant'|'admin'}
export function saveServer(value:string){uni.setStorageSync(SERVER_KEY,value.trim().replace(/\/+$/,''))}
export function saveSession(accessToken:string,user:UserInfo,portal:'tenant'|'admin'='tenant'){
  uni.setStorageSync(TOKEN_KEY,accessToken)
  uni.setStorageSync(USER_KEY,user)
  uni.setStorageSync(PORTAL_KEY,portal)
}
export function clearSession(){
  uni.removeStorageSync(TOKEN_KEY)
  uni.removeStorageSync(USER_KEY)
  uni.removeStorageSync(PORTAL_KEY)
}
export function hasSession(){return Boolean(token())}
export function isAdminPortal(){return portalMode()==='admin'||currentUser()?.role==='SUPER_ADMIN'}
export function canManageDevices(){const role=currentUser()?.role;return role==='SUPER_ADMIN'||role==='TENANT_ADMIN'}
export function canBindGateway(){return currentUser()?.role==='TENANT_ADMIN'}
