import { getAuthItem, setAuthItem } from './auth-storage';

export interface RecoveryIdentity { userId: string; tenantId: string }
export function assertRecoveryIdentity(user: { id: string; tenantId: string | null }, expected?: RecoveryIdentity) {
  if (expected && (user.id !== expected.userId || (user.tenantId || '') !== expected.tenantId)) {
    throw new Error('请使用原账号登录，当前画布属于原账号及租户');
  }
}

// 只接受配置的设计器来源，普通页面或其他同机端口不能获取登录凭据。
function designerOrigin() {
  const configured = String(import.meta.env.VITE_GOVIEW_DESIGNER_URL || '').trim();
  return new URL(configured || `${location.protocol}//${location.hostname}:${import.meta.env.DEV ? '3020' : '8080'}`, location.origin).origin;
}
export function initSessionLogin(onInit: (identity: RecoveryIdentity) => void) {
  const allowedOrigin = designerOrigin();
  let nonce = '';
  let initialized = false;
  const send = (token: string) => {
    if (nonce && window.parent !== window) window.parent.postMessage({ type: 'wk-recovery-auth', nonce, token }, allowedOrigin);
  };
  const reportHeight = (height: number) => {
    if (nonce && Number.isFinite(height)) window.parent.postMessage({ type: 'wk-recovery-size', nonce, height }, allowedOrigin);
  };
  const receive = async (event: MessageEvent) => {
    if (initialized || event.source !== window.parent || event.origin !== allowedOrigin || event.data?.type !== 'wk-recovery-init') return;
    if (typeof event.data.nonce !== 'string' || !event.data.nonce || typeof event.data.userId !== 'string' || !event.data.userId || typeof event.data.tenantId !== 'string') return;
    initialized = true;
    nonce = event.data.nonce;
    const identity = { userId: event.data.userId, tenantId: event.data.tenantId };
    onInit(identity);
    // 平台标签页仍有有效会话时，尝试一次无感恢复；过期后显示登录卡片。
    const token = getAuthItem('accessToken');
    if (!token) return;
    try {
      const profile = await fetch('/api/auth/profile', { headers: { Authorization: `Bearer ${token}` } });
      if (!profile.ok) return;
      const body = await profile.json();
      assertRecoveryIdentity(body.data, identity);
      const response = await fetch('/api/auth/activity', { method: 'POST', headers: { Authorization: `Bearer ${token}` } });
      if (!response.ok) return;
      const result = await response.json();
      if (result.data?.accessToken) {
        setAuthItem('accessToken', result.data.accessToken);
        setAuthItem('sessionIdleTimeoutSeconds', String(result.data.idleTimeoutSeconds));
        setAuthItem('sessionAbsoluteExpiresAt', result.data.absoluteExpiresAt);
        setAuthItem('sessionLastActivityAt', String(Date.now()));
        send(result.data.accessToken);
      }
    } catch { /* 登录卡片允许原账号重新认证 */ }
  };
  window.addEventListener('message', receive);
  if (window.parent !== window) window.parent.postMessage({ type: 'wk-recovery-ready' }, allowedOrigin);
  return { send, reportHeight, dispose: () => window.removeEventListener('message', receive) };
}
