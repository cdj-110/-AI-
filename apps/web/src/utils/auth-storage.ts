import { portalMode } from '../portal';

export type AuthStorageKey = 'accessToken' | 'userInfo' | 'sessionIdleTimeoutSeconds' | 'sessionAbsoluteExpiresAt' | 'sessionLastActivityAt';

const authKeys: AuthStorageKey[] = ['accessToken', 'userInfo', 'sessionIdleTimeoutSeconds', 'sessionAbsoluteExpiresAt', 'sessionLastActivityAt'];
const rememberKey = `weikong:${portalMode}:remember-login`;
const usernameKey = `weikong:${portalMode}:remembered-username`;

export function rememberLoginEnabled() {
  return localStorage.getItem(rememberKey) !== '0';
}

export function rememberedUsername() {
  return rememberLoginEnabled() ? (localStorage.getItem(usernameKey) ?? '') : '';
}

export function setLoginPreference(remember: boolean, username: string) {
  localStorage.setItem(rememberKey, remember ? '1' : '0');
  if (remember) localStorage.setItem(usernameKey, username.trim());
  else localStorage.removeItem(usernameKey);
}

export function getAuthItem(key: AuthStorageKey) {
  return sessionStorage.getItem(key) ?? localStorage.getItem(key);
}

export function setAuthItem(key: AuthStorageKey, value: string, remember = rememberLoginEnabled()) {
  const target = remember ? localStorage : sessionStorage;
  const other = remember ? sessionStorage : localStorage;
  other.removeItem(key);
  target.setItem(key, value);
}

export function removeAuthItem(key: AuthStorageKey) {
  localStorage.removeItem(key);
  sessionStorage.removeItem(key);
}

export function clearAuthSession() {
  for (const key of authKeys) removeAuthItem(key);
}
