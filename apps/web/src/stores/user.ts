import { defineStore } from 'pinia';
import { apiRequest } from '../api/request';
import { portalMode, roleAllowed } from '../portal';
import { clearAuthSession, getAuthItem, rememberLoginEnabled, setAuthItem, setLoginPreference } from '../utils/auth-storage';
import { assertRecoveryIdentity, type RecoveryIdentity } from '../utils/session-login';

export interface UserInfo {
  id: string;
  username: string;
  nickname?: string;
  avatarUrl?: string | null;
  email?: string | null;
  phone?: string | null;
  status?: string;
  role: string;
  tenantId: string | null;
  permissions?: string[];
  deviceScopeMode?: 'ALL' | 'CUSTOM';
}

interface LoginPayload {
  username: string;
  password: string;
  captchaToken?: string;
  remember?: boolean;
  recoveryIdentity?: RecoveryIdentity;
}

interface LoginCodePayload {
  username: string;
  code: string;
  remember?: boolean;
  recoveryIdentity?: RecoveryIdentity;
}

interface SendLoginCodePayload {
  username: string;
  captchaToken: string;
}

interface SendLoginCodeResponse {
  message: string;
  expiresInSeconds: number;
  retryAfterSeconds: number;
  email?: string;
}

export interface LoginResponse {
  accessToken: string;
  user: UserInfo;
  idleTimeoutSeconds: number;
  absoluteExpiresAt: string;
}

interface ActivityResponse {
  accessToken: string;
  idleTimeoutSeconds: number;
  absoluteExpiresAt: string;
}

export interface ProfileUpdateResponse {
  accessToken: string;
  user: UserInfo;
}

export const useUserStore = defineStore('user', {
  state: () => ({
    token: getAuthItem('accessToken') ?? '',
    userInfo: JSON.parse(getAuthItem('userInfo') ?? 'null') as UserInfo | null,
    idleTimeoutSeconds: Number(getAuthItem('sessionIdleTimeoutSeconds') || (portalMode === 'admin' ? 1800 : 3600)),
    absoluteExpiresAt: getAuthItem('sessionAbsoluteExpiresAt') ?? '',
  }),
  actions: {
    async login(payload: LoginPayload) {
      const remember = payload.remember !== false;
      const result = await apiRequest<LoginResponse>({ url: `/api/auth/login/${portalMode}`, method: 'POST', data: { username: payload.username, password: payload.password, captchaToken: payload.captchaToken || undefined } });
      if (!roleAllowed(result.user.role)) throw new Error('账号与当前平台不匹配');
      assertRecoveryIdentity(result.user, payload.recoveryIdentity);
      setLoginPreference(remember, payload.username);
      this.setSession(result, remember);
    },
    async sendLoginCode(payload: SendLoginCodePayload) {
      return apiRequest<SendLoginCodeResponse>({
        url: `/api/auth/login/${portalMode}/code/send`,
        method: 'POST',
        data: payload,
      });
    },
    async loginWithCode(payload: LoginCodePayload) {
      const remember = payload.remember !== false;
      const result = await apiRequest<LoginResponse>({
        url: `/api/auth/login/${portalMode}/code`,
        method: 'POST',
        data: { username: payload.username, code: payload.code },
      });
      if (!roleAllowed(result.user.role)) throw new Error('账号与当前平台不匹配');
      assertRecoveryIdentity(result.user, payload.recoveryIdentity);
      setLoginPreference(remember, payload.username);
      this.setSession(result, remember);
    },
    async fetchProfile() {
      const profile = await apiRequest<UserInfo>({ url: '/api/auth/profile', method: 'GET' });
      if (!roleAllowed(profile.role)) {
        await this.logout();
        throw new Error('账号与当前平台不匹配');
      }
      this.userInfo = profile;
      setAuthItem('userInfo', JSON.stringify(profile));
    },
    setSession(result: LoginResponse, remember = rememberLoginEnabled()) {
      this.token = result.accessToken;
      this.userInfo = result.user;
      setAuthItem('accessToken', result.accessToken, remember);
      setAuthItem('userInfo', JSON.stringify(result.user), remember);
      this.setLease(result, remember);
      setAuthItem('sessionLastActivityAt', String(Date.now()), remember);
    },
    applyProfileUpdate(result: ProfileUpdateResponse, remember = rememberLoginEnabled()) {
      this.token = result.accessToken;
      this.userInfo = result.user;
      setAuthItem('accessToken', result.accessToken, remember);
      setAuthItem('userInfo', JSON.stringify(result.user), remember);
    },
    async keepAlive() {
      const result = await apiRequest<ActivityResponse>({ url: '/api/auth/activity', method: 'POST', silentError: true });
      this.token = result.accessToken;
      setAuthItem('accessToken', result.accessToken);
      this.setLease(result);
      return result;
    },
    setLease(result: Pick<ActivityResponse, 'idleTimeoutSeconds' | 'absoluteExpiresAt'>, remember = rememberLoginEnabled()) {
      this.idleTimeoutSeconds = result.idleTimeoutSeconds;
      this.absoluteExpiresAt = result.absoluteExpiresAt;
      setAuthItem('sessionIdleTimeoutSeconds', String(result.idleTimeoutSeconds), remember);
      setAuthItem('sessionAbsoluteExpiresAt', result.absoluteExpiresAt, remember);
    },
    async logout(notifyServer = true) {
      if (notifyServer && this.token) {
        await apiRequest({ url: '/api/auth/logout', method: 'POST', silentError: true }).catch(() => undefined);
      }
      this.token = '';
      this.userInfo = null;
      clearAuthSession();
    },
  },
});
