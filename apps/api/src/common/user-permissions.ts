import { Prisma } from '@prisma/client';
import { AuthUser } from './decorators/current-user.decorator';

export const TENANT_USER_PERMISSIONS = [
  'DASHBOARD_VIEW',
  'DEVICE_VIEW',
  'DEVICE_CREATE',
  'DEVICE_EDIT',
  'DEVICE_DELETE',
  'DEVICE_GROUP_MANAGE',
  'MAP_VIEW',
  'SCADA_VIEW',
  'MODEL_VIEW',
  'MODEL_CREATE',
  'MODEL_EDIT',
  'MODEL_DELETE',
  'ALARM_VIEW',
  'ALARM_RESOLVE',
  'EVENT_VIEW',
] as const;

export type TenantUserPermission = typeof TENANT_USER_PERMISSIONS[number];

export const DEFAULT_TENANT_USER_PERMISSIONS: TenantUserPermission[] = [
  'DASHBOARD_VIEW',
  'DEVICE_VIEW',
  'MAP_VIEW',
  'SCADA_VIEW',
  'MODEL_VIEW',
  'ALARM_VIEW',
  'EVENT_VIEW',
];

const REQUIRED_VIEW_PERMISSION: Partial<Record<TenantUserPermission, TenantUserPermission>> = {
  DEVICE_CREATE: 'DEVICE_VIEW',
  DEVICE_EDIT: 'DEVICE_VIEW',
  DEVICE_DELETE: 'DEVICE_VIEW',
  DEVICE_GROUP_MANAGE: 'DEVICE_VIEW',
  MODEL_CREATE: 'MODEL_VIEW',
  MODEL_EDIT: 'MODEL_VIEW',
  MODEL_DELETE: 'MODEL_VIEW',
  ALARM_RESOLVE: 'ALARM_VIEW',
};

export function normalizeTenantUserPermissions(permissions: readonly string[]): TenantUserPermission[] {
  const allowed = new Set<string>(TENANT_USER_PERMISSIONS);
  const selected = new Set<TenantUserPermission>(
    permissions.filter((permission): permission is TenantUserPermission => allowed.has(permission)),
  );
  for (const permission of [...selected]) {
    const requiredView = REQUIRED_VIEW_PERMISSION[permission];
    if (requiredView) selected.add(requiredView);
  }
  return TENANT_USER_PERMISSIONS.filter(permission => selected.has(permission));
}

export function deviceAccessWhere(actor: AuthUser): Prisma.DeviceWhereInput {
  if (actor.role === 'SUPER_ADMIN') return {};
  const tenantId = actor.tenantId ?? '__missing_tenant__';
  if (actor.role !== 'TENANT_USER' || actor.deviceScopeMode !== 'CUSTOM') return { tenantId };
  return {
    tenantId,
    OR: [
      { userAccesses: { some: { userId: actor.sub } } },
      { groupMemberships: { some: { group: { userAccesses: { some: { userId: actor.sub } } } } } },
    ],
  };
}
