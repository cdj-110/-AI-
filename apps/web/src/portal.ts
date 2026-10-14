import adminPlatformLogo from './assets/admin-platform-logo.png';
import tenantPlatformLogo from './assets/tenant-platform-logo.png';

export type PortalMode = 'admin' | 'tenant';

export const portalMode: PortalMode = import.meta.env.VITE_PORTAL_MODE === 'admin' ? 'admin' : 'tenant';
export const isAdminPortal = portalMode === 'admin';
export const portalName = import.meta.env.VITE_PORTAL_NAME
  || (isAdminPortal ? '微控管理平台' : '微控物联网平台');
export const portalEyebrow = isAdminPortal ? 'WEIKONG MANAGEMENT' : 'WEIKONG IOT CLOUD';
export const portalLogo = isAdminPortal ? adminPlatformLogo : tenantPlatformLogo;
export const allowedRoles = isAdminPortal ? ['SUPER_ADMIN'] : ['TENANT_ADMIN', 'TENANT_USER'];

function defaultAlternatePortalURL() {
  if (typeof window === 'undefined') return isAdminPortal ? 'http://127.0.0.1:5174' : 'http://127.0.0.1:5176';
  const url = new URL(window.location.href);
  url.port = isAdminPortal ? '5174' : '5176';
  url.pathname = '/login';
  url.search = '';
  url.hash = '';
  return url.toString();
}

export const alternatePortalURL = isAdminPortal
  ? (import.meta.env.VITE_TENANT_PORTAL_URL || defaultAlternatePortalURL())
  : (import.meta.env.VITE_ADMIN_PORTAL_URL || defaultAlternatePortalURL());

export function roleAllowed(role?: string) {
  return Boolean(role && allowedRoles.includes(role));
}
