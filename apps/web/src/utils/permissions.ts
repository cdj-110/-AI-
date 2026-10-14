import type { UserInfo } from '../stores/user';

export function hasPermission(user: UserInfo | null | undefined, permission: string) {
  if (!user) return false;
  if (user.role !== 'TENANT_USER') return true;
  return !Array.isArray(user.permissions) || user.permissions.includes(permission);
}
