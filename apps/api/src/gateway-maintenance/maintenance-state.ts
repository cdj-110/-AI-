import { cloudPolicyTenantSelect, tenantBlocksCloud } from '../common/device-cloud-policy';
import { authorizationStatus, LicenseStatusInput } from '../factory-gateways/license-status';

export const maintenanceGatewayInclude = { device: { select: { cloudAccessEnabled: true, tenant: { select: cloudPolicyTenantSelect } } } } as const;
export const MAINTENANCE_HEARTBEAT_MS = 120_000;
export function maintenanceState(gateway: LicenseStatusInput & { lastSeenAt?: Date | null; device?: { cloudAccessEnabled: boolean; tenant?: { deviceCloudAccessDisabled: boolean } | null } | null }, channelReady: boolean, now = new Date()) {
  const age = gateway.lastSeenAt ? now.getTime() - gateway.lastSeenAt.getTime() : Infinity;
  const online = Number.isFinite(age) && age >= -30_000 && age <= MAINTENANCE_HEARTBEAT_MS;
  const activated = Boolean(gateway.licenseActivatedAt);
  const reason = gateway.status === 'DISABLED' || gateway.status === 'REMOVED' ? '网关已停用'
    : gateway.device?.cloudAccessEnabled === false ? '该网关的云连接已被管理平台禁用'
    : tenantBlocksCloud(gateway.device) ? '所属租户已停用设备云连接'
    : !online ? '未收到近期心跳，网关当前离线'
    : !activated ? '网关尚未完成授权激活'
    : !channelReady ? '远程维护通道暂未就绪' : '';
  return { online, activated, ...authorizationStatus(gateway, now), channelReady, canMaintain: !reason, reason,
    lastSeenAt: gateway.lastSeenAt ?? null, licenseActivatedAt: gateway.licenseActivatedAt ?? null,
    serverTime: now, heartbeatTimeoutSeconds: MAINTENANCE_HEARTBEAT_MS / 1000 };
}
